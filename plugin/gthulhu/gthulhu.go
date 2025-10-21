package gthulhu

import (
	"context"
	"time"

	"github.com/Gthulhu/plugin/models"
	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
)

func init() {
	// Register the gthulhu plugin with the factory
	err := reg.RegisterNewPlugin("gthulhu", func(ctx context.Context, config *reg.SchedConfig) (reg.CustomScheduler, error) {
		// Use Scheduler config if available, otherwise use SimpleScheduler config
		sliceNsDefault := config.Scheduler.SliceNsDefault
		sliceNsMin := config.Scheduler.SliceNsMin

		if sliceNsDefault == 0 && config.Scheduler.SliceNsDefault > 0 {
			sliceNsDefault = config.Scheduler.SliceNsDefault
		}
		if sliceNsMin == 0 && config.Scheduler.SliceNsMin > 0 {
			sliceNsMin = config.Scheduler.SliceNsMin
		}

		gthulhuPlugin := NewGthulhuPlugin(sliceNsDefault, sliceNsMin)

		// Initialize JWT client if API config is provided
		if config.APIConfig.PublicKeyPath != "" && config.APIConfig.BaseURL != "" {
			err := gthulhuPlugin.InitJWTClient(config.APIConfig.PublicKeyPath, config.APIConfig.BaseURL)
			if err != nil {
				return nil, err
			}

			// Initialize metrics client
			err = gthulhuPlugin.InitMetricsClient(config.APIConfig.BaseURL)
			if err != nil {
				return nil, err
			}
		}
		gthulhuPlugin.StartStrategyFetcher(ctx, config.APIConfig.BaseURL, time.Duration(config.APIConfig.Interval)*time.Second)
		return gthulhuPlugin, nil
	})
	if err != nil {
		panic(err)
	}
}

type GthulhuPlugin struct {
	// Scheduler configuration
	sliceNsDefault uint64
	sliceNsMin     uint64

	// Task pool state
	taskPool      []Task
	taskPoolCount int
	taskPoolHead  int
	taskPoolTail  int

	// Global vruntime
	minVruntime uint64

	// Strategy map for PID-based scheduling strategies
	strategyMap map[int32]SchedulingStrategy

	// JWT client for API authentication
	jwtClient *JWTClient

	// Metrics client for sending metrics to API server
	metricsClient *MetricsClient
}

func NewGthulhuPlugin(sliceNsDefault, sliceNsMin uint64) *GthulhuPlugin {
	plugin := &GthulhuPlugin{
		sliceNsDefault: 5000 * 1000, // 5ms (default)
		sliceNsMin:     500 * 1000,  // 0.5ms (default)
		taskPool:       make([]Task, taskPoolSize),
		taskPoolCount:  0,
		taskPoolHead:   0,
		taskPoolTail:   0,
		minVruntime:    0,
		strategyMap:    make(map[int32]SchedulingStrategy),
	}

	// Override defaults if provided
	if sliceNsDefault > 0 {
		plugin.sliceNsDefault = sliceNsDefault
	}
	if sliceNsMin > 0 {
		plugin.sliceNsMin = sliceNsMin
	}

	return plugin
}

var _ reg.CustomScheduler = (*GthulhuPlugin)(nil)

func (g *GthulhuPlugin) DrainQueuedTask(s reg.Sched) int {
	return g.drainQueuedTask(s)
}

func (g *GthulhuPlugin) SelectQueuedTask(s reg.Sched) *models.QueuedTask {
	return g.getTaskFromPool()
}

func (g *GthulhuPlugin) SelectCPU(s reg.Sched, t *models.QueuedTask) (error, int32) {
	return s.DefaultSelectCPU(t)
}

func (g *GthulhuPlugin) DetermineTimeSlice(s reg.Sched, t *models.QueuedTask) uint64 {
	return g.getTaskExecutionTime(t.Pid)
}

func (g *GthulhuPlugin) GetPoolCount() uint64 {
	return uint64(g.taskPoolCount)
}

// drainQueuedTask drains tasks from the scheduler queue into the task pool
func (g *GthulhuPlugin) drainQueuedTask(s reg.Sched) int {
	var count int
	for (g.taskPoolTail+1)%taskPoolSize != g.taskPoolHead {
		var newQueuedTask models.QueuedTask
		s.DequeueTask(&newQueuedTask)
		if newQueuedTask.Pid == -1 {
			return count
		}

		t := Task{
			QueuedTask: &newQueuedTask,
			Deadline:   g.updatedEnqueueTask(&newQueuedTask),
		}
		g.insertTaskToPool(t)
		count++
	}
	return 0
}

// updatedEnqueueTask updates the task's vtime based on scheduling strategy
func (g *GthulhuPlugin) updatedEnqueueTask(t *models.QueuedTask) uint64 {
	// Check if we have a specific strategy for this task
	strategyApplied := g.applySchedulingStrategy(t)

	if !strategyApplied {
		// Default behavior if no specific strategy is found
		if g.minVruntime < t.Vtime {
			g.minVruntime = t.Vtime
		}
		minVruntimeLocal := saturatingSub(g.minVruntime, g.sliceNsDefault)
		if t.Vtime == 0 {
			t.Vtime = minVruntimeLocal + (g.sliceNsDefault * 100 / t.Weight)
		} else if t.Vtime < minVruntimeLocal {
			t.Vtime = minVruntimeLocal
		}
		t.Vtime += (t.StopTs - t.StartTs) * t.Weight / 100
	}

	return 0
}

// saturatingSub performs saturating subtraction (returns 0 if b > a)
func saturatingSub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}

// getTaskFromPool retrieves a task from the pool
func (g *GthulhuPlugin) getTaskFromPool() *models.QueuedTask {
	if g.taskPoolHead == g.taskPoolTail {
		return nil
	}
	t := &g.taskPool[g.taskPoolHead]
	g.taskPoolHead = (g.taskPoolHead + 1) % taskPoolSize
	g.taskPoolCount--
	return t.QueuedTask
}

// insertTaskToPool inserts a task into the pool in sorted order
func (g *GthulhuPlugin) insertTaskToPool(newTask Task) bool {
	if g.taskPoolCount >= taskPoolSize-1 {
		return false
	}
	insertIdx := g.taskPoolTail
	for i := 0; i < g.taskPoolCount; i++ {
		idx := (g.taskPoolHead + i) % taskPoolSize
		if lessQueuedTask(&newTask, &g.taskPool[idx]) {
			insertIdx = idx
			break
		}
	}

	cur := g.taskPoolTail
	for cur != insertIdx {
		next := (cur - 1 + taskPoolSize) % taskPoolSize
		g.taskPool[cur] = g.taskPool[next]
		cur = next
	}
	g.taskPool[insertIdx] = newTask
	g.taskPoolTail = (g.taskPoolTail + 1) % taskPoolSize
	g.taskPoolCount++
	return true
}

// lessQueuedTask compares two tasks for priority ordering
func lessQueuedTask(a, b *Task) bool {
	if a.Deadline != b.Deadline {
		return a.Deadline < b.Deadline
	}
	if a.Timestamp != b.Timestamp {
		return a.Timestamp < b.Timestamp
	}
	return a.QueuedTask.Pid < b.QueuedTask.Pid
}

// applySchedulingStrategy applies scheduling strategies to a task
func (g *GthulhuPlugin) applySchedulingStrategy(task *models.QueuedTask) bool {
	if strategy, exists := g.strategyMap[task.Tgid]; exists {
		// Apply strategy
		if strategy.Priority {
			// Priority tasks get minimum vtime
			task.Vtime = 0
		}
		return true
	}
	return false
}

// getTaskExecutionTime returns the custom execution time for a task if defined
func (g *GthulhuPlugin) getTaskExecutionTime(pid int32) uint64 {
	if strategy, exists := g.strategyMap[pid]; exists && strategy.ExecutionTime > 0 {
		return strategy.ExecutionTime
	}
	return 0
}

// InitJWTClient initializes the JWT client for API authentication
func (g *GthulhuPlugin) InitJWTClient(publicKeyPath, apiBaseURL string) error {
	g.jwtClient = NewJWTClient(publicKeyPath, apiBaseURL)
	return nil
}

// GetJWTClient returns the current JWT client instance
func (g *GthulhuPlugin) GetJWTClient() *JWTClient {
	return g.jwtClient
}

// InitMetricsClient initializes the metrics client
func (g *GthulhuPlugin) InitMetricsClient(apiBaseURL string) error {
	if g.jwtClient == nil {
		return nil // Silently skip if JWT client is not initialized
	}
	g.metricsClient = NewMetricsClient(g.jwtClient, apiBaseURL)
	return nil
}

// GetMetricsClient returns the metrics client instance
func (g *GthulhuPlugin) GetMetricsClient() *MetricsClient {
	return g.metricsClient
}

// SetSchedulerConfig updates the scheduler parameters
func (g *GthulhuPlugin) SetSchedulerConfig(sliceNsDefault, sliceNsMin uint64) {
	if sliceNsDefault > 0 {
		g.sliceNsDefault = sliceNsDefault
	}
	if sliceNsMin > 0 {
		g.sliceNsMin = sliceNsMin
	}
}

// GetSchedulerConfig returns current scheduler configuration
func (g *GthulhuPlugin) GetSchedulerConfig() (uint64, uint64) {
	return g.sliceNsDefault, g.sliceNsMin
}

// FetchSchedulingStrategies fetches scheduling strategies from the API server
func (g *GthulhuPlugin) FetchSchedulingStrategies(apiUrl string) ([]SchedulingStrategy, error) {
	if g.jwtClient == nil {
		return nil, nil // Silently skip if JWT client not initialized
	}
	return fetchSchedulingStrategies(g.jwtClient, apiUrl)
}

// UpdateStrategyMap updates the strategy map from a slice of strategies
func (g *GthulhuPlugin) UpdateStrategyMap(strategies []SchedulingStrategy) {
	// Create a new map to avoid concurrent access issues
	newMap := make(map[int32]SchedulingStrategy)

	for _, strategy := range strategies {
		newMap[int32(strategy.PID)] = strategy
	}

	// Replace the old map with the new one
	g.strategyMap = newMap
}
