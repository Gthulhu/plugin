package simple

import (
	"testing"

	"github.com/Gthulhu/plugin/models"
	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
)

// TestSimplePluginInstanceIsolation verifies that multiple SimplePlugin instances maintain independent state
func TestSimplePluginInstanceIsolation(t *testing.T) {
	// Create two instances with different configurations
	plugin1 := NewSimplePlugin(false) // Weighted vtime mode
	plugin2 := NewSimplePlugin(true)  // FIFO mode

	// Verify that each instance has its own configuration
	if plugin1.GetMode() != false {
		t.Errorf("Plugin1 mode = %v; want false", plugin1.GetMode())
	}
	if plugin2.GetMode() != true {
		t.Errorf("Plugin2 mode = %v; want true", plugin2.GetMode())
	}

	// Test configuration changes don't affect other instances
	plugin1.SetMode(true)
	if plugin2.GetMode() != true {
		t.Error("Plugin2 mode should remain unchanged")
	}
}

// TestSimplePluginDefaultConfiguration verifies default configuration is applied
func TestSimplePluginDefaultConfiguration(t *testing.T) {
	// Create instance with default configuration
	simplePlugin := NewSimplePlugin(false)

	// Verify default slice
	if simplePlugin.sliceDefault != sliceDefault {
		t.Errorf("Default sliceDefault = %d; want %d", simplePlugin.sliceDefault, sliceDefault)
	}

	// Verify initial mode
	if simplePlugin.GetMode() != false {
		t.Errorf("Default mode = %v; want false", simplePlugin.GetMode())
	}
}

// TestSimplePluginTaskPoolInitialization verifies task pool is initialized correctly
func TestSimplePluginTaskPoolInitialization(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)

	// Verify initial pool count is 0
	if simplePlugin.GetPoolCount() != 0 {
		t.Errorf("Initial pool count = %d; want 0", simplePlugin.GetPoolCount())
	}

	// Verify task pool is allocated
	if simplePlugin.taskPool == nil {
		t.Error("Task pool is nil; expected allocated slice")
	}

	// Verify task pool starts empty but has capacity
	if len(simplePlugin.taskPool) != 0 {
		t.Errorf("Task pool length = %d; want 0 (should start empty)", len(simplePlugin.taskPool))
	}

	// Verify initial statistics
	local, global := simplePlugin.GetStats()
	if local != 0 || global != 0 {
		t.Errorf("Initial stats = (%d, %d); want (0, 0)", local, global)
	}
}

// TestSimplePluginModeSwitch verifies mode switching works correctly
func TestSimplePluginModeSwitch(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)

	// Initially weighted vtime mode
	if simplePlugin.GetMode() != false {
		t.Error("Initial mode should be weighted vtime (false)")
	}

	// Switch to FIFO mode
	simplePlugin.SetMode(true)
	if simplePlugin.GetMode() != true {
		t.Error("Mode should be FIFO (true) after switching")
	}

	// Switch back to weighted vtime mode
	simplePlugin.SetMode(false)
	if simplePlugin.GetMode() != false {
		t.Error("Mode should be weighted vtime (false) after switching back")
	}
}

// TestSimplePluginStatistics verifies statistics tracking
func TestSimplePluginStatistics(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)

	// Initial stats should be zero
	local, global := simplePlugin.GetStats()
	if local != 0 || global != 0 {
		t.Errorf("Initial stats = (%d, %d); want (0, 0)", local, global)
	}

	// Reset should maintain zero stats
	simplePlugin.ResetStats()
	local, global = simplePlugin.GetStats()
	if local != 0 || global != 0 {
		t.Errorf("Stats after reset = (%d, %d); want (0, 0)", local, global)
	}
}

// MockScheduler implements the plugin.Sched interface for testing
type MockScheduler struct {
	taskQueue     []*models.QueuedTask
	queueIndex    int
	cpuAllocated  map[int32]int32 // PID -> CPU mapping
	defaultCPU    int32
	dequeueCount  int
	selectCPUCall int
}

// Compile-time check that MockScheduler implements plugin.Sched
var _ reg.Sched = (*MockScheduler)(nil)

// NewMockScheduler creates a new mock scheduler for testing
func NewMockScheduler() *MockScheduler {
	return &MockScheduler{
		taskQueue:    make([]*models.QueuedTask, 0),
		queueIndex:   0,
		cpuAllocated: make(map[int32]int32),
		defaultCPU:   0,
	}
}

// EnqueueTask adds a task to the mock scheduler's queue
func (m *MockScheduler) EnqueueTask(task *models.QueuedTask) {
	m.taskQueue = append(m.taskQueue, task)
}

// DequeueTask implements plugin.Sched.DequeueTask
func (m *MockScheduler) DequeueTask(task *models.QueuedTask) {
	m.dequeueCount++
	if m.queueIndex >= len(m.taskQueue) {
		// No more tasks, return sentinel value
		task.Pid = -1
		return
	}

	// Copy the task from queue
	qt := m.taskQueue[m.queueIndex]
	*task = *qt
	m.queueIndex++
}

// DefaultSelectCPU implements plugin.Sched.DefaultSelectCPU
func (m *MockScheduler) DefaultSelectCPU(t *models.QueuedTask) (error, int32) {
	m.selectCPUCall++
	// Simple round-robin CPU selection
	cpu := m.defaultCPU
	m.defaultCPU = (m.defaultCPU + 1) % 4 // Assume 4 CPUs
	m.cpuAllocated[t.Pid] = cpu
	return nil, cpu
}

// Reset resets the mock scheduler state
func (m *MockScheduler) Reset() {
	m.taskQueue = make([]*models.QueuedTask, 0)
	m.queueIndex = 0
	m.cpuAllocated = make(map[int32]int32)
	m.defaultCPU = 0
	m.dequeueCount = 0
	m.selectCPUCall = 0
}

// GetCPUMapping returns the current CPU allocation for testing
func (m *MockScheduler) GetCPUMapping() map[int32]int32 {
	return m.cpuAllocated
}

// GetDequeueCount returns the number of dequeue operations
func (m *MockScheduler) GetDequeueCount() int {
	return m.dequeueCount
}

// GetSelectCPUCount returns the number of SelectCPU calls
func (m *MockScheduler) GetSelectCPUCount() int {
	return m.selectCPUCall
}

// TestSimplePluginRuntimeSimulation provides comprehensive runtime testing
func TestSimplePluginRuntimeSimulation(t *testing.T) {
	t.Run("EmptyQueue", func(t *testing.T) {
		simplePlugin := NewSimplePlugin(false)
		mockSched := NewMockScheduler()

		// Drain tasks from empty queue
		drained := simplePlugin.DrainQueuedTask(mockSched)
		if drained != 0 {
			t.Errorf("DrainQueuedTask on empty queue = %d; want 0", drained)
		}

		// Try to select a task
		task := simplePlugin.SelectQueuedTask(mockSched)
		if task != nil {
			t.Error("SelectQueuedTask on empty pool should return nil")
		}

		// Pool count should be 0
		if simplePlugin.GetPoolCount() != 0 {
			t.Errorf("GetPoolCount = %d; want 0", simplePlugin.GetPoolCount())
		}
	})

	t.Run("SingleTaskWorkflow", func(t *testing.T) {
		simplePlugin := NewSimplePlugin(false)
		mockSched := NewMockScheduler()

		// Create a task
		task1 := &models.QueuedTask{
			Pid:            100,
			Cpu:            -1,
			NrCpusAllowed:  4,
			Flags:          0,
			StartTs:        1000,
			StopTs:         2000,
			SumExecRuntime: 1000,
			Weight:         100,
			Vtime:          0,
			Tgid:           100,
		}

		// Enqueue task
		mockSched.EnqueueTask(task1)

		// Drain tasks
		drained := simplePlugin.DrainQueuedTask(mockSched)
		if drained != 1 {
			t.Errorf("DrainQueuedTask = %d; want 1", drained)
		}

		// Verify pool count
		if simplePlugin.GetPoolCount() != 1 {
			t.Errorf("GetPoolCount after drain = %d; want 1", simplePlugin.GetPoolCount())
		}

		// Verify statistics
		_, global := simplePlugin.GetStats()
		if global != 1 {
			t.Errorf("Global queue count = %d; want 1", global)
		}

		// Select task from pool
		selectedTask := simplePlugin.SelectQueuedTask(mockSched)
		if selectedTask == nil {
			t.Fatal("SelectQueuedTask returned nil")
		}
		if selectedTask.Pid != 100 {
			t.Errorf("Selected task PID = %d; want 100", selectedTask.Pid)
		}

		// Pool count should decrease
		if simplePlugin.GetPoolCount() != 0 {
			t.Errorf("GetPoolCount after select = %d; want 0", simplePlugin.GetPoolCount())
		}

		// Select CPU for task
		err, cpu := simplePlugin.SelectCPU(mockSched, selectedTask)
		if err != nil {
			t.Errorf("SelectCPU returned error: %v", err)
		}
		if (cpu < 0 || cpu >= 4) && cpu != 1<<20 {
			t.Errorf("SelectCPU returned invalid CPU: %d", cpu)
		}

		// Determine time slice
		timeSlice := simplePlugin.DetermineTimeSlice(mockSched, selectedTask)
		if timeSlice != sliceDefault {
			t.Errorf("DetermineTimeSlice = %d; want %d", timeSlice, sliceDefault)
		}
	})

	t.Run("MultipleTasksWeightedVtime", func(t *testing.T) {
		simplePlugin := NewSimplePlugin(false) // Weighted vtime mode
		mockSched := NewMockScheduler()

		// Create multiple tasks with different weights and vtimes
		tasks := []*models.QueuedTask{
			{Pid: 100, Weight: 100, Vtime: 5000, Tgid: 100, StartTs: 1000, StopTs: 2000},
			{Pid: 200, Weight: 150, Vtime: 3000, Tgid: 200, StartTs: 1500, StopTs: 2500},
			{Pid: 300, Weight: 80, Vtime: 7000, Tgid: 300, StartTs: 2000, StopTs: 3000},
		}

		// Enqueue all tasks
		for _, task := range tasks {
			mockSched.EnqueueTask(task)
		}

		// Drain all tasks
		drained := simplePlugin.DrainQueuedTask(mockSched)
		if drained != 3 {
			t.Errorf("DrainQueuedTask = %d; want 3", drained)
		}

		// Verify pool count
		if simplePlugin.GetPoolCount() != 3 {
			t.Errorf("GetPoolCount = %d; want 3", simplePlugin.GetPoolCount())
		}

		// Process all tasks - should be ordered by vtime
		processedTasks := make([]*models.QueuedTask, 0)
		for simplePlugin.GetPoolCount() > 0 {
			task := simplePlugin.SelectQueuedTask(mockSched)
			if task == nil {
				t.Fatal("SelectQueuedTask returned nil while pool count > 0")
			}

			// Select CPU and determine time slice
			err, cpu := simplePlugin.SelectCPU(mockSched, task)
			if err != nil {
				t.Errorf("SelectCPU error: %v", err)
			}
			if cpu < 0 {
				t.Errorf("Invalid CPU selected: %d", cpu)
			}

			timeSlice := simplePlugin.DetermineTimeSlice(mockSched, task)
			if timeSlice != sliceDefault {
				t.Errorf("DetermineTimeSlice = %d; want %d", timeSlice, sliceDefault)
			}

			processedTasks = append(processedTasks, task)
		}

		// Verify all tasks were processed
		if len(processedTasks) != 3 {
			t.Errorf("Processed tasks = %d; want 3", len(processedTasks))
		}

		// In weighted vtime mode, tasks should be processed in vtime order
		// Task with PID 200 should be first (lowest vtime: 3000)
		if processedTasks[0].Pid != 200 {
			t.Errorf("First processed task PID = %d; want 200", processedTasks[0].Pid)
		}
	})

	t.Run("MultipleTasksFIFO", func(t *testing.T) {
		simplePlugin := NewSimplePlugin(true) // FIFO mode
		mockSched := NewMockScheduler()

		// Create multiple tasks
		tasks := []*models.QueuedTask{
			{Pid: 100, Weight: 100, Vtime: 7000, Tgid: 100, StartTs: 1000, StopTs: 2000},
			{Pid: 200, Weight: 150, Vtime: 3000, Tgid: 200, StartTs: 1500, StopTs: 2500},
			{Pid: 300, Weight: 80, Vtime: 5000, Tgid: 300, StartTs: 2000, StopTs: 3000},
		}

		// Enqueue all tasks
		for _, task := range tasks {
			mockSched.EnqueueTask(task)
		}

		// Drain all tasks
		drained := simplePlugin.DrainQueuedTask(mockSched)
		if drained != 3 {
			t.Errorf("DrainQueuedTask = %d; want 3", drained)
		}

		// Process all tasks - should be in FIFO order
		processedTasks := make([]*models.QueuedTask, 0)
		for simplePlugin.GetPoolCount() > 0 {
			task := simplePlugin.SelectQueuedTask(mockSched)
			if task == nil {
				t.Fatal("SelectQueuedTask returned nil while pool count > 0")
			}
			processedTasks = append(processedTasks, task)
		}

		// In FIFO mode, tasks should be processed in enqueue order
		expectedOrder := []int32{100, 200, 300}
		for i, expectedPID := range expectedOrder {
			if processedTasks[i].Pid != expectedPID {
				t.Errorf("Task %d PID = %d; want %d", i, processedTasks[i].Pid, expectedPID)
			}
		}
	})

	t.Run("PoolOverflowHandling", func(t *testing.T) {
		simplePlugin := NewSimplePlugin(false)
		mockSched := NewMockScheduler()

		// Try to enqueue more tasks than pool can hold
		maxTasks := 4095 // Leave room for pool management

		for i := 0; i < maxTasks; i++ {
			task := &models.QueuedTask{
				Pid:    int32(1000 + i),
				Weight: 100,
				Vtime:  0,
				Tgid:   int32(1000 + i),
			}
			mockSched.EnqueueTask(task)
		}

		// Drain tasks - should stop when pool is full
		drained := simplePlugin.DrainQueuedTask(mockSched)

		// Should have drained all tasks or stopped when pool is full
		if drained > maxTasks {
			t.Errorf("Drained more tasks than enqueued: %d > %d", drained, maxTasks)
		}

		// Pool count should not exceed capacity
		poolCount := simplePlugin.GetPoolCount()
		if poolCount > 4095 {
			t.Errorf("Pool count exceeds capacity: %d", poolCount)
		}
	})

	t.Run("ConcurrentInstanceIsolation", func(t *testing.T) {
		// Create two independent plugin instances
		plugin1 := NewSimplePlugin(false) // Weighted vtime
		plugin2 := NewSimplePlugin(true)  // FIFO

		// Create separate mock schedulers
		sched1 := NewMockScheduler()
		sched2 := NewMockScheduler()

		// Add tasks to each scheduler
		sched1.EnqueueTask(&models.QueuedTask{Pid: 100, Weight: 100, Vtime: 5000, Tgid: 100})
		sched1.EnqueueTask(&models.QueuedTask{Pid: 101, Weight: 100, Vtime: 3000, Tgid: 101})

		sched2.EnqueueTask(&models.QueuedTask{Pid: 200, Weight: 100, Vtime: 7000, Tgid: 200})

		// Drain tasks in both plugins
		drained1 := plugin1.DrainQueuedTask(sched1)
		drained2 := plugin2.DrainQueuedTask(sched2)

		if drained1 != 2 {
			t.Errorf("Plugin1 drained = %d; want 2", drained1)
		}
		if drained2 != 1 {
			t.Errorf("Plugin2 drained = %d; want 1", drained2)
		}

		// Verify pool counts are independent
		if plugin1.GetPoolCount() != 2 {
			t.Errorf("Plugin1 pool count = %d; want 2", plugin1.GetPoolCount())
		}
		if plugin2.GetPoolCount() != 1 {
			t.Errorf("Plugin2 pool count = %d; want 1", plugin2.GetPoolCount())
		}

		// Process a task from plugin1 - should get task with lower vtime first
		task1 := plugin1.SelectQueuedTask(sched1)
		if task1 == nil {
			t.Fatal("Plugin1 SelectQueuedTask returned nil")
		}
		if task1.Pid != 101 { // PID 101 has lower vtime (3000)
			t.Errorf("Plugin1 first task PID = %d; want 101", task1.Pid)
		}

		// Plugin2's pool should be unaffected
		if plugin2.GetPoolCount() != 1 {
			t.Errorf("Plugin2 pool count after plugin1 select = %d; want 1", plugin2.GetPoolCount())
		}

		// Plugin1's pool should decrease
		if plugin1.GetPoolCount() != 1 {
			t.Errorf("Plugin1 pool count after select = %d; want 1", plugin1.GetPoolCount())
		}
	})
}

// TestSimplePluginVtimeUpdates tests vtime tracking functionality
func TestSimplePluginVtimeUpdates(t *testing.T) {
	simplePlugin := NewSimplePlugin(false) // Weighted vtime mode
	mockSched := NewMockScheduler()

	// Test with task that has existing vtime
	task := &models.QueuedTask{
		Pid:    100,
		Weight: 100,
		Vtime:  10000,
		Tgid:   100,
	}

	mockSched.EnqueueTask(task)
	simplePlugin.DrainQueuedTask(mockSched)

	// Select the task to trigger vtime update
	selectedTask := simplePlugin.SelectQueuedTask(mockSched)
	if selectedTask == nil {
		t.Fatal("SelectQueuedTask returned nil")
	}

	// The global vtime should be updated
	if simplePlugin.vtimeNow != 10000 {
		t.Errorf("Global vtime = %d; want 10000", simplePlugin.vtimeNow)
	}

	// Test stopping task vtime update
	simplePlugin.updateStoppingTask(selectedTask, 1000000) // 1ms execution time
	expectedVtime := uint64(10000 + 1000000*100/100)       // 10000 + 1000000
	if selectedTask.Vtime != expectedVtime {
		t.Errorf("Task vtime after stopping = %d; want %d", selectedTask.Vtime, expectedVtime)
	}
}

// TestSimplePluginStatisticsUpdate tests statistics tracking during operations
func TestSimplePluginStatisticsUpdate(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)
	mockSched := NewMockScheduler()

	// Add some tasks
	for i := 0; i < 5; i++ {
		task := &models.QueuedTask{
			Pid:    int32(100 + i),
			Weight: 100,
			Vtime:  uint64(1000 * i),
			Tgid:   int32(100 + i),
		}
		mockSched.EnqueueTask(task)
	}

	// Drain tasks
	drained := simplePlugin.DrainQueuedTask(mockSched)
	if drained != 5 {
		t.Errorf("Drained tasks = %d; want 5", drained)
	}

	// Check statistics
	_, global := simplePlugin.GetStats()
	if global != 5 {
		t.Errorf("Global queue count = %d; want 5", global)
	}

	// Reset and verify
	simplePlugin.ResetStats()
	_, global = simplePlugin.GetStats()
	if global != 0 {
		t.Errorf("Global queue count after reset = %d; want 0", global)
	}
}

// TestSimplePluginVtimeNeverZero tests that vtime is never 0
func TestSimplePluginVtimeNeverZero(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)
	mockSched := NewMockScheduler()

	// Test 1: Initial vtimeNow should not be 0
	if simplePlugin.vtimeNow == 0 {
		t.Error("Initial vtimeNow should not be 0")
	}

	// Test 2: Task with vtime 0 should be adjusted
	taskWithZeroVtime := &models.QueuedTask{
		Pid:    100,
		Weight: 100,
		Vtime:  0, // This should be adjusted to 1
		Tgid:   100,
	}
	mockSched.EnqueueTask(taskWithZeroVtime)

	drained := simplePlugin.DrainQueuedTask(mockSched)
	if drained != 1 {
		t.Errorf("DrainQueuedTask = %d; want 1", drained)
	}

	// Select the task and verify its vtime is not 0
	selectedTask := simplePlugin.SelectQueuedTask(mockSched)
	if selectedTask == nil {
		t.Fatal("SelectQueuedTask returned nil")
	}
	if selectedTask.Vtime == 0 {
		t.Error("Task vtime should not be 0 after processing")
	}

	// Test 3: Global vtime should never be 0 after updateRunningTask
	if simplePlugin.vtimeNow == 0 {
		t.Error("Global vtimeNow should not be 0 after task processing")
	}

	// Test 4: updateStoppingTask should not result in 0 vtime
	originalVtime := selectedTask.Vtime
	simplePlugin.updateStoppingTask(selectedTask, 0) // Even with 0 exec time
	if selectedTask.Vtime == 0 {
		t.Error("Task vtime should not be 0 after updateStoppingTask")
	}
	if selectedTask.Vtime < originalVtime {
		t.Error("Task vtime should not decrease after updateStoppingTask")
	}
}

// TestSimplePluginInvalidTaskHandling tests handling of invalid tasks
func TestSimplePluginInvalidTaskHandling(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)
	mockSched := NewMockScheduler()

	// Add a valid task
	validTask := &models.QueuedTask{
		Pid:    100,
		Weight: 100,
		Vtime:  1000,
		Tgid:   100,
	}
	mockSched.EnqueueTask(validTask)

	// Add an invalid task (will be created internally during test)
	invalidTask := &models.QueuedTask{
		Pid:    0, // This will be caught by validation (changed from -1 to 0)
		Weight: 100,
		Vtime:  1000,
		Tgid:   0,
	}
	mockSched.EnqueueTask(invalidTask)

	// Drain tasks - should only get the valid one
	drained := simplePlugin.DrainQueuedTask(mockSched)
	if drained != 1 {
		t.Errorf("DrainQueuedTask = %d; want 1 (invalid task should be skipped)", drained)
	}

	// Pool should only contain 1 valid task
	if simplePlugin.GetPoolCount() != 1 {
		t.Errorf("Pool count = %d; want 1", simplePlugin.GetPoolCount())
	}
}

// TestSimplePluginPoolStatusAndCleanup tests pool status and cleanup functionality
func TestSimplePluginPoolStatusAndCleanup(t *testing.T) {
	simplePlugin := NewSimplePlugin(false)

	// Check initial pool status
	head, tail, count, _ := simplePlugin.GetPoolStatus()
	if head != 0 || tail != 0 || count != 0 {
		t.Errorf("Initial pool status = (%d, %d, %d); want (0, 0, 0)",
			head, tail, count)
	}

	// Add some tasks manually to test pool status
	for i := 0; i < 3; i++ {
		task := &models.QueuedTask{
			Pid:    int32(100 + i),
			Weight: 100,
			Vtime:  uint64(1000 * i),
			Tgid:   int32(100 + i),
		}
		taskWrapper := simplePlugin.enqueueTask(task)
		simplePlugin.insertTaskToPool(taskWrapper)
	}

	// Check pool status after adding tasks
	_, _, count, _ = simplePlugin.GetPoolStatus()
	if count != 3 {
		t.Errorf("Pool count after adding tasks = %d; want 3", count)
	}

	// Pool count should remain the same
	if simplePlugin.GetPoolCount() != 3 {
		t.Errorf("Pool count after cleanup = %d; want 3", simplePlugin.GetPoolCount())
	}
}
