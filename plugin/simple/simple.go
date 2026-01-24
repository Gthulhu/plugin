package simple

import (
	"context"

	"github.com/Gthulhu/plugin/models"
	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
	"github.com/Gthulhu/plugin/plugin/util"
)

func init() {
	// Register the simple plugin with weighted vtime mode
	err := reg.RegisterNewPlugin("simple", func(ctx context.Context, config *reg.SchedConfig) (reg.CustomScheduler, error) {
		simplePlugin := NewSimplePlugin(false) // weighted vtime mode

		if config.Scheduler.SliceNsDefault > 0 {
			simplePlugin.SetSliceDefault(config.Scheduler.SliceNsDefault)
		}

		return simplePlugin, nil
	})
	if err != nil {
		panic(err)
	}

	// Register the simple plugin with FIFO mode
	err = reg.RegisterNewPlugin("simple-fifo", func(ctx context.Context, config *reg.SchedConfig) (reg.CustomScheduler, error) {
		simplePlugin := NewSimplePlugin(true) // FIFO mode

		if config.Scheduler.SliceNsDefault > 0 {
			simplePlugin.SetSliceDefault(config.Scheduler.SliceNsDefault)
		}

		return simplePlugin, nil
	})
	if err != nil {
		panic(err)
	}
}

// SimplePlugin implements a basic scheduler that can operate in two modes:
// 1. Weighted vtime scheduling (default)
// 2. FIFO scheduling
type SimplePlugin struct {
	// Configuration
	fifoMode     bool
	sliceDefault uint64

	// Task pool for managing queued tasks
	taskPool []Task

	// Global vtime tracking (for weighted vtime scheduling)
	vtimeNow uint64

	// Statistics
	localQueueCount  uint64
	globalQueueCount uint64
}

// Task represents a task in the scheduler pool
type Task struct {
	QueuedTask *models.QueuedTask
	VTime      uint64
	Timestamp  uint64
}

const (
	sliceDefault = 5000 * 100 // 0.5ms in nanoseconds
)

// NewSimplePlugin creates a new SimplePlugin instance
func NewSimplePlugin(fifoMode bool) *SimplePlugin {
	return &SimplePlugin{
		fifoMode:         fifoMode,
		sliceDefault:     sliceDefault,
		taskPool:         make([]Task, 0), // Start with empty slice, no pre-allocation
		vtimeNow:         1,               // Start with 1 to ensure vtime is never 0
		localQueueCount:  0,
		globalQueueCount: 0,
	}
}

func (s *SimplePlugin) SetSliceDefault(slice uint64) {
	s.sliceDefault = slice
}

func (s *SimplePlugin) SendMetrics(data interface{}) {}

// Verify that SimplePlugin implements the plugin.CustomScheduler interface
var _ reg.CustomScheduler = (*SimplePlugin)(nil)

// DrainQueuedTask drains tasks from the scheduler queue into the task pool
func (s *SimplePlugin) DrainQueuedTask(sched reg.Sched) int {
	count := 0

	// Keep draining until the pool is full or no more tasks available
	for {
		var queuedTask models.QueuedTask
		sched.DequeueTask(&queuedTask)

		// Validate task before processing to prevent corruption
		if queuedTask.Pid <= 0 {
			// Skip invalid tasks
			return count
		}

		// Create task and enqueue it
		task := s.enqueueTask(&queuedTask)
		s.insertTaskToPool(task)

		count++
		s.globalQueueCount++
	}
}

// SelectQueuedTask selects and returns the next task to be scheduled
func (s *SimplePlugin) SelectQueuedTask(sched reg.Sched) *models.QueuedTask {
	return s.getTaskFromPool()
}

// SelectCPU selects a CPU for the given task
func (s *SimplePlugin) SelectCPU(sched reg.Sched, task *models.QueuedTask) (error, int32) {
	return nil, 1 << 20
}

// DetermineTimeSlice determines the time slice for the given task
func (s *SimplePlugin) DetermineTimeSlice(sched reg.Sched, task *models.QueuedTask) uint64 {
	// Always return default slice
	return s.sliceDefault
}

// GetPoolCount returns the number of tasks in the pool
func (s *SimplePlugin) GetPoolCount() uint64 {
	return uint64(len(s.taskPool))
}

// enqueueTask processes a task for enqueueing
func (s *SimplePlugin) enqueueTask(queuedTask *models.QueuedTask) Task {
	task := Task{
		QueuedTask: queuedTask,
		VTime:      queuedTask.Vtime,
		Timestamp:  queuedTask.StartTs,
	}
	vtime := queuedTask.Vtime

	// Weighted vtime scheduling logic
	if !s.fifoMode {
		// Limit the amount of budget that an idling task can accumulate to one slice
		if vtime < saturatingSub(s.vtimeNow, s.sliceDefault) {
			vtime = saturatingSub(s.vtimeNow, s.sliceDefault)
		}
	}

	// Ensure vtime is never 0 - use minimum value of 1 if needed
	if vtime == 0 {
		vtime = 1
	}

	task.VTime = vtime

	return task
}

// getTaskFromPool retrieves the next task from the pool
func (s *SimplePlugin) getTaskFromPool() *models.QueuedTask {
	if len(s.taskPool) == 0 {
		return nil
	}

	// Get the first task
	task := &s.taskPool[0]

	// Remove the first task from slice
	selectedTask := task.QueuedTask
	s.taskPool = s.taskPool[1:]

	// Update running task vtime (for weighted vtime scheduling)
	if !s.fifoMode {
		// Ensure task vtime is never 0 before updating global vtime
		if selectedTask.Vtime == 0 {
			selectedTask.Vtime = 1
		}
		s.updateRunningTask(selectedTask)
	}

	return selectedTask
}

// insertTaskToPool inserts a task into the pool
func (s *SimplePlugin) insertTaskToPool(newTask Task) {
	if s.fifoMode {
		// FIFO: just append to end
		s.taskPool = append(s.taskPool, newTask)
		return
	}

	// Weighted vtime: insert in sorted order by vtime
	insertIdx := len(s.taskPool) // Default to end
	for i := 0; i < len(s.taskPool); i++ {
		if lessTask(&newTask, &s.taskPool[i]) {
			insertIdx = i
			break
		}
	}

	// Insert at the correct position
	if insertIdx == len(s.taskPool) {
		// Insert at end
		s.taskPool = append(s.taskPool, newTask)
	} else {
		// Insert in middle - grow slice and shift elements
		s.taskPool = append(s.taskPool, Task{}) // Add empty element at end
		copy(s.taskPool[insertIdx+1:], s.taskPool[insertIdx:])
		s.taskPool[insertIdx] = newTask
	}
}

// lessTask compares two tasks for priority ordering
func lessTask(a, b *Task) bool {
	if a.VTime != b.VTime {
		return a.VTime < b.VTime
	}
	if a.Timestamp != b.Timestamp {
		return a.Timestamp < b.Timestamp
	}
	return a.QueuedTask.Pid < b.QueuedTask.Pid
}

// updateRunningTask updates the vtime when a task starts running
func (s *SimplePlugin) updateRunningTask(task *models.QueuedTask) {
	// Global vtime always progresses forward as tasks start executing
	if s.vtimeNow < task.Vtime {
		s.vtimeNow = task.Vtime
	}

	// Ensure global vtime is never 0
	if s.vtimeNow == 0 {
		s.vtimeNow = 1
	}
}

// updateStoppingTask updates the vtime when a task stops running
func (s *SimplePlugin) updateStoppingTask(task *models.QueuedTask, execTime uint64) {
	if s.fifoMode {
		return
	}

	// Scale the execution time by the inverse of the weight and charge
	task.Vtime += execTime * 100 / task.Weight

	// Ensure task vtime is never 0
	if task.Vtime == 0 {
		task.Vtime = 1
	}
}

// saturatingSub performs saturating subtraction (returns 0 if b > a)
func saturatingSub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}

// GetMode returns whether the scheduler is in FIFO mode
func (s *SimplePlugin) GetMode() bool {
	return s.fifoMode
}

// SetMode sets the scheduling mode
func (s *SimplePlugin) SetMode(fifoMode bool) {
	s.fifoMode = fifoMode
}

// GetStats returns scheduling statistics
func (s *SimplePlugin) GetStats() (uint64, uint64) {
	return s.localQueueCount, s.globalQueueCount
}

// ResetStats resets scheduling statistics
func (s *SimplePlugin) ResetStats() {
	s.localQueueCount = 0
	s.globalQueueCount = 0
}

// GetPoolStatus returns detailed pool status for debugging
func (s *SimplePlugin) GetPoolStatus() (head, tail, count, capacity int) {
	// For slice implementation: head=0, tail=len, count=len, capacity=cap
	return 0, len(s.taskPool), len(s.taskPool), cap(s.taskPool)
}

func (s *SimplePlugin) GetChangedStrategies() ([]util.SchedulingStrategy, []util.SchedulingStrategy) {
	return nil, nil
}
