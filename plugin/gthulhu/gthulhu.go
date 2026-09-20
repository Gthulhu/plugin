package gthulhu

import (
	"context"
	"log"
	"maps"
	"sync"
	"time"

	"github.com/Gthulhu/plugin/models"
	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
	"github.com/Gthulhu/plugin/plugin/util"
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
		if config.APIConfig.Enabled &&
			config.APIConfig.PublicKeyPath != "" && config.APIConfig.BaseURL != "" {
			err := gthulhuPlugin.InitJWTClient(
				config.APIConfig.PublicKeyPath,
				config.APIConfig.BaseURL,
				config.APIConfig.AuthEnabled,
				config.APIConfig.MTLS,
			)
			if err != nil {
				return nil, err
			}
			// Initialize metrics client
			err = gthulhuPlugin.InitMetricsClient(config.APIConfig.BaseURL)
			if err != nil {
				return nil, err
			}
			gthulhuPlugin.StartStrategyFetcher(ctx, config.APIConfig.BaseURL, time.Duration(config.APIConfig.Interval)*time.Second)
		}
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
	poolMu        sync.Mutex

	// Global vruntime
	minVruntime uint64

	// strategyMap is the desired set; appliedStrategyMap is the last set handed
	// to the scheduler. GetChangedStrategies diffs them into changed/removed.
	strategyMap        map[int32]util.SchedulingStrategy
	appliedStrategyMap map[int32]util.SchedulingStrategy
	strategyMu         sync.RWMutex

	// JWT client for API authentication
	jwtClient *JWTClient

	// Metrics client for sending metrics to API server
	metricsClient *MetricsClient
}

func NewGthulhuPlugin(sliceNsDefault, sliceNsMin uint64) *GthulhuPlugin {
	plugin := &GthulhuPlugin{
		sliceNsDefault:     5000 * 1000, // 5ms (default)
		sliceNsMin:         500 * 1000,  // 0.5ms (default)
		taskPool:           make([]Task, taskPoolSize),
		taskPoolCount:      0,
		minVruntime:        0,
		strategyMap:        make(map[int32]util.SchedulingStrategy),
		appliedStrategyMap: make(map[int32]util.SchedulingStrategy),
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

func (g *GthulhuPlugin) SendMetrics(data interface{}) {
	if g.metricsClient != nil {
		if bssData, ok := data.(BssData); ok {
			err := g.metricsClient.SendMetrics(bssData)
			if err != nil {
				// Log the error but do not disrupt scheduling
				log.Printf("Failed to send metrics: %v", err)
			}
		} else {
			log.Printf("Invalid metrics data type: %T", data)
		}
	}
}

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
	return g.getTaskExecutionTime(t)
}

func (g *GthulhuPlugin) GetPoolCount() uint64 {
	return uint64(g.taskPoolCount)
}

// drainQueuedTask drains tasks from the scheduler queue into the task pool
func (g *GthulhuPlugin) drainQueuedTask(s reg.Sched) int {
	var count int
	// Hold the lock across capacity check and insertion to avoid TOCTOU race
	for {
		g.poolMu.Lock()
		if g.taskPoolCount >= taskPoolSize-1 {
			g.poolMu.Unlock()
			break
		}
		var newQueuedTask models.QueuedTask
		s.DequeueTask(&newQueuedTask)
		if newQueuedTask.Pid == -1 || count == int(s.GetNrQueued()) {
			g.poolMu.Unlock()
			return count
		}
		t := Task{
			QueuedTask: &newQueuedTask,
			Deadline:   g.updatedEnqueueTask(&newQueuedTask),
			Timestamp:  newQueuedTask.StartTs,
		}
		// Direct heap insert (no second lock acquisition)
		g.taskPool[g.taskPoolCount] = t
		g.heapSiftUp(g.taskPoolCount)
		g.taskPoolCount++
		g.poolMu.Unlock()
		count++
	}
	return count
}

// updatedEnqueueTask updates the task's vtime based on scheduling strategy
func (g *GthulhuPlugin) updatedEnqueueTask(t *models.QueuedTask) uint64 {
	// Check if we have a specific strategy for this task
	strategyApplied := g.applySchedulingStrategy(t)

	if !strategyApplied {
		// Default behavior if no specific strategy is found
		minVruntime := saturatingSub(g.minVruntime, g.sliceNsDefault)
		if t.Vtime == 0 {
			t.Vtime = minVruntime + (g.sliceNsDefault * 100 / t.Weight)
		} else if t.Vtime < minVruntime {
			t.Vtime = minVruntime
		}
		vslice := (t.StopTs - t.StartTs) * 100 / t.Weight
		t.Vtime += vslice
		g.minVruntime += vslice
		return t.Vtime + min(t.SumExecRuntime, g.sliceNsDefault*100)
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
	// Pop-min from binary heap stored in g.taskPool[0:g.taskPoolCount]
	g.poolMu.Lock()
	defer g.poolMu.Unlock()
	if g.taskPoolCount == 0 {
		return nil
	}
	// Take the root element
	top := g.taskPool[0]
	g.taskPoolCount--
	if g.taskPoolCount > 0 {
		// Move last element to root and sift down
		g.taskPool[0] = g.taskPool[g.taskPoolCount]
		g.heapSiftDown(0)
	}
	return top.QueuedTask
}

// insertTaskToPool inserts a task into the pool in sorted order
func (g *GthulhuPlugin) insertTaskToPool(newTask Task) bool {
	// In-place binary min-heap using preallocated array
	g.poolMu.Lock()
	defer g.poolMu.Unlock()
	if g.taskPoolCount >= taskPoolSize-1 {
		return false
	}
	// Place at the end and sift up
	g.taskPool[g.taskPoolCount] = newTask
	g.heapSiftUp(g.taskPoolCount)
	g.taskPoolCount++
	return true
}

// heapLess compares elements at indices i and j in the heap according to lessQueuedTask
func (g *GthulhuPlugin) heapLess(i, j int) bool {
	return lessQueuedTask(&g.taskPool[i], &g.taskPool[j])
}

// heapSiftUp moves the element at idx up to restore heap property
func (g *GthulhuPlugin) heapSiftUp(idx int) {
	for idx > 0 {
		parent := (idx - 1) / 2
		if !g.heapLess(idx, parent) {
			break
		}
		g.taskPool[idx], g.taskPool[parent] = g.taskPool[parent], g.taskPool[idx]
		idx = parent
	}
}

// heapSiftDown moves the element at idx down to restore heap property
func (g *GthulhuPlugin) heapSiftDown(idx int) {
	n := g.taskPoolCount
	for {
		left := 2*idx + 1
		if left >= n {
			break
		}
		smallest := left
		right := left + 1
		if right < n && g.heapLess(right, left) {
			smallest = right
		}
		if !g.heapLess(smallest, idx) {
			break
		}
		g.taskPool[idx], g.taskPool[smallest] = g.taskPool[smallest], g.taskPool[idx]
		idx = smallest
	}
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

// applySchedulingStrategy gives a task minimum vtime when a matching strategy
// boosts it (Priority > 0), and reports whether it was boosted. A strategy that
// only sets a custom time slice (Priority == 0) must not jump the run queue, so
// it reports false here; getTaskExecutionTime still supplies its slice.
func (g *GthulhuPlugin) applySchedulingStrategy(task *models.QueuedTask) bool {
	strategy, exists := g.lookupTaskStrategy(task)
	if !exists || strategy.Priority <= 0 {
		return false
	}
	task.Vtime = 0
	return true
}

// getTaskExecutionTime returns the custom time slice for a task, or 0 when no
// matching strategy defines one.
func (g *GthulhuPlugin) getTaskExecutionTime(task *models.QueuedTask) uint64 {
	strategy, exists := g.lookupTaskStrategy(task)
	if exists && strategy.ExecutionTime > 0 {
		return strategy.ExecutionTime
	}
	return 0
}

// lookupTaskStrategy returns the strategy for a task, preferring an exact
// thread (TID) match over a thread-group (TGID) match. Node policies key by
// TID, so a thread-specific rule wins; Pod policies key by the group leader's
// PID, so every thread of the group still resolves through the TGID fallback.
// Priority and time-slice must share this lookup, or one strategy would act
// differently between the two paths.
func (g *GthulhuPlugin) lookupTaskStrategy(task *models.QueuedTask) (util.SchedulingStrategy, bool) {
	g.strategyMu.RLock()
	defer g.strategyMu.RUnlock()
	if strategy, ok := g.strategyMap[task.Pid]; ok {
		return strategy, true
	}
	if task.Tgid != task.Pid {
		if strategy, ok := g.strategyMap[task.Tgid]; ok {
			return strategy, true
		}
	}
	return util.SchedulingStrategy{}, false
}

// InitJWTClient initializes the JWT client for API authentication
func (g *GthulhuPlugin) InitJWTClient(
	publicKeyPath,
	apiBaseURL string,
	authEnabled bool,
	mtlsCfg reg.MTLSConfig,
) error {
	client, err := NewJWTClient(publicKeyPath, apiBaseURL, authEnabled, mtlsCfg)
	if err != nil {
		return err
	}
	g.jwtClient = client
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
func (g *GthulhuPlugin) FetchSchedulingStrategies(apiUrl string) ([]util.SchedulingStrategy, error) {
	if g.jwtClient == nil {
		return nil, nil // Silently skip if JWT client not initialized
	}
	return fetchSchedulingStrategies(g.jwtClient, apiUrl)
}

// UpdateStrategyMap replaces the desired strategy set. The changed/removed diff
// is computed later in GetChangedStrategies against the last applied set, so
// intermediate churn (e.g. a strategy removed then re-added before the next
// drain) coalesces to the correct final state instead of a stale event stream.
func (g *GthulhuPlugin) UpdateStrategyMap(strategies []util.SchedulingStrategy) {
	newMap := make(map[int32]util.SchedulingStrategy)
	for _, strategy := range strategies {
		newMap[int32(strategy.PID)] = strategy
	}
	g.strategyMu.Lock()
	g.strategyMap = newMap
	g.strategyMu.Unlock()
}

// GetChangedStrategies returns the strategies to apply (changed or new) and to
// remove so the scheduler's applied set matches the current desired set, then
// records the current set as applied. Diffing against the last applied set (not
// a running event queue) guarantees a strategy is never in both lists, so a
// remove-then-re-add between drains is not mistaken for a deletion.
func (g *GthulhuPlugin) GetChangedStrategies() ([]util.SchedulingStrategy, []util.SchedulingStrategy) {
	changed := []util.SchedulingStrategy{}
	removed := []util.SchedulingStrategy{}

	g.strategyMu.Lock()
	defer g.strategyMu.Unlock()

	for pid, strategy := range g.strategyMap {
		if applied, ok := g.appliedStrategyMap[pid]; !ok || applied != strategy {
			changed = append(changed, strategy)
		}
	}
	for pid, applied := range g.appliedStrategyMap {
		if _, ok := g.strategyMap[pid]; !ok {
			removed = append(removed, applied)
		}
	}

	g.appliedStrategyMap = maps.Clone(g.strategyMap)
	return changed, removed
}
