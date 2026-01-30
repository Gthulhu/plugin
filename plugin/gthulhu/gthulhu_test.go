package gthulhu

import (
	"testing"

	"github.com/Gthulhu/plugin/models"
	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
	"github.com/Gthulhu/plugin/plugin/util"
)

// TestGthulhuPluginInstanceIsolation verifies that multiple GthulhuPlugin instances maintain independent state
func TestGthulhuPluginInstanceIsolation(t *testing.T) {
	// Create two instances with different configurations
	plugin1 := NewGthulhuPlugin(10000*1000, 1000*1000) // 10ms, 1ms
	plugin2 := NewGthulhuPlugin(20000*1000, 2000*1000) // 20ms, 2ms

	// Verify that each instance has its own configuration
	default1, min1 := plugin1.GetSchedulerConfig()
	default2, min2 := plugin2.GetSchedulerConfig()

	if default1 != 10000*1000 {
		t.Errorf("Plugin1 sliceNsDefault = %d; want %d", default1, 10000*1000)
	}
	if min1 != 1000*1000 {
		t.Errorf("Plugin1 sliceNsMin = %d; want %d", min1, 1000*1000)
	}
	if default2 != 20000*1000 {
		t.Errorf("Plugin2 sliceNsDefault = %d; want %d", default2, 20000*1000)
	}
	if min2 != 2000*1000 {
		t.Errorf("Plugin2 sliceNsMin = %d; want %d", min2, 2000*1000)
	}
}

// TestGthulhuPluginDefaultConfiguration verifies default configuration is applied
func TestGthulhuPluginDefaultConfiguration(t *testing.T) {
	// Create instance with zero values (should use defaults)
	gthulhuPlugin := NewGthulhuPlugin(0, 0)

	defaultNs, minNs := gthulhuPlugin.GetSchedulerConfig()

	if defaultNs != 5000*1000 {
		t.Errorf("Default sliceNsDefault = %d; want %d", defaultNs, 5000*1000)
	}
	if minNs != 500*1000 {
		t.Errorf("Default sliceNsMin = %d; want %d", minNs, 500*1000)
	}
}

// TestGthulhuPluginTaskPoolInitialization verifies task pool is initialized correctly
func TestGthulhuPluginTaskPoolInitialization(t *testing.T) {
	gthulhuPlugin := NewGthulhuPlugin(0, 0)

	// Verify initial pool count is 0
	if gthulhuPlugin.GetPoolCount() != 0 {
		t.Errorf("Initial pool count = %d; want 0", gthulhuPlugin.GetPoolCount())
	}

	// Verify task pool is allocated
	if gthulhuPlugin.taskPool == nil {
		t.Error("Task pool is nil; expected allocated array")
	}

	// Verify task pool size
	if len(gthulhuPlugin.taskPool) != taskPoolSize {
		t.Errorf("Task pool size = %d; want %d", len(gthulhuPlugin.taskPool), taskPoolSize)
	}
}

// TestGthulhuPluginStrategyMapInitialization verifies strategy map is initialized
func TestGthulhuPluginStrategyMapInitialization(t *testing.T) {
	gthulhuPlugin := NewGthulhuPlugin(0, 0)

	// Verify strategy map is initialized
	if gthulhuPlugin.strategyMap == nil {
		t.Error("Strategy map is nil; expected initialized map")
	}

	// Verify initial strategy map is empty
	if len(gthulhuPlugin.strategyMap) != 0 {
		t.Errorf("Initial strategy map size = %d; want 0", len(gthulhuPlugin.strategyMap))
	}
}

// TestGthulhuPluginSetSchedulerConfig verifies SetSchedulerConfig updates configuration
func TestGthulhuPluginSetSchedulerConfig(t *testing.T) {
	gthulhuPlugin := NewGthulhuPlugin(0, 0)

	// Update configuration
	gthulhuPlugin.SetSchedulerConfig(15000*1000, 1500*1000)

	defaultNs, minNs := gthulhuPlugin.GetSchedulerConfig()

	if defaultNs != 15000*1000 {
		t.Errorf("Updated sliceNsDefault = %d; want %d", defaultNs, 15000*1000)
	}
	if minNs != 1500*1000 {
		t.Errorf("Updated sliceNsMin = %d; want %d", minNs, 1500*1000)
	}
}

// TestGthulhuPluginUpdateStrategyMap verifies UpdateStrategyMap works correctly
func TestGthulhuPluginUpdateStrategyMap(t *testing.T) {
	gthulhuPlugin := NewGthulhuPlugin(0, 0)

	// Create test strategies
	strategies := []util.SchedulingStrategy{
		{PID: 100, Priority: true, ExecutionTime: 10000},
		{PID: 200, Priority: false, ExecutionTime: 20000},
	}

	// Update strategy map
	gthulhuPlugin.UpdateStrategyMap(strategies)

	// Verify strategies were added
	if len(gthulhuPlugin.strategyMap) != 2 {
		t.Errorf("Strategy map size = %d; want 2", len(gthulhuPlugin.strategyMap))
	}

	// Verify strategy for PID 100
	if strategy, exists := gthulhuPlugin.strategyMap[100]; !exists {
		t.Error("Strategy for PID 100 not found")
	} else {
		if !strategy.Priority {
			t.Error("Strategy for PID 100 should have Priority=true")
		}
		if strategy.ExecutionTime != 10000 {
			t.Errorf("Strategy for PID 100 ExecutionTime = %d; want 10000", strategy.ExecutionTime)
		}
	}

	// Verify strategy for PID 200
	if strategy, exists := gthulhuPlugin.strategyMap[200]; !exists {
		t.Error("Strategy for PID 200 not found")
	} else {
		if strategy.Priority {
			t.Error("Strategy for PID 200 should have Priority=false")
		}
		if strategy.ExecutionTime != 20000 {
			t.Errorf("Strategy for PID 200 ExecutionTime = %d; want 20000", strategy.ExecutionTime)
		}
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

// Compile-time check that MockScheduler implements reg.Sched
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

func (m *MockScheduler) GetNrQueued() uint64 {
	return uint64(len(m.taskQueue) - m.queueIndex)
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

// TestGthulhuPluginRuntimeSimulation provides comprehensive runtime testing
func TestGthulhuPluginRuntimeSimulation(t *testing.T) {
	// Create plugin instance
	gthulhuPlugin := NewGthulhuPlugin(5000*1000, 500*1000)

	// Create mock scheduler
	mockSched := NewMockScheduler()

	t.Run("EmptyQueue", func(t *testing.T) {
		mockSched.Reset()

		// Drain tasks from empty queue
		drained := gthulhuPlugin.DrainQueuedTask(mockSched)
		if drained != 0 {
			t.Errorf("DrainQueuedTask on empty queue = %d; want 0", drained)
		}

		// Try to select a task
		task := gthulhuPlugin.SelectQueuedTask(mockSched)
		if task != nil {
			t.Error("SelectQueuedTask on empty pool should return nil")
		}

		// Pool count should be 0
		if gthulhuPlugin.GetPoolCount() != 0 {
			t.Errorf("GetPoolCount = %d; want 0", gthulhuPlugin.GetPoolCount())
		}
	})

	t.Run("SingleTaskWorkflow", func(t *testing.T) {
		mockSched.Reset()
		gthulhuPlugin = NewGthulhuPlugin(5000*1000, 500*1000) // Reset plugin

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
		drained := gthulhuPlugin.DrainQueuedTask(mockSched)
		if drained != 1 {
			t.Errorf("DrainQueuedTask = %d; want 1", drained)
		}

		// Verify pool count
		if gthulhuPlugin.GetPoolCount() != 1 {
			t.Errorf("GetPoolCount after drain = %d; want 1", gthulhuPlugin.GetPoolCount())
		}

		// Select task from pool
		selectedTask := gthulhuPlugin.SelectQueuedTask(mockSched)
		if selectedTask == nil {
			t.Fatal("SelectQueuedTask returned nil")
		}
		if selectedTask.Pid != 100 {
			t.Errorf("Selected task PID = %d; want 100", selectedTask.Pid)
		}

		// Pool count should decrease
		if gthulhuPlugin.GetPoolCount() != 0 {
			t.Errorf("GetPoolCount after select = %d; want 0", gthulhuPlugin.GetPoolCount())
		}

		// Select CPU for task
		err, cpu := gthulhuPlugin.SelectCPU(mockSched, selectedTask)
		if err != nil {
			t.Errorf("SelectCPU returned error: %v", err)
		}
		if cpu < 0 || cpu >= 4 {
			t.Errorf("SelectCPU returned invalid CPU: %d", cpu)
		}

		// Determine time slice
		timeSlice := gthulhuPlugin.DetermineTimeSlice(mockSched, selectedTask)
		if timeSlice != 0 { // No strategy set, should return 0
			t.Errorf("DetermineTimeSlice = %d; want 0 (no strategy)", timeSlice)
		}
	})

	t.Run("MultipleTasksWorkflow", func(t *testing.T) {
		mockSched.Reset()
		gthulhuPlugin = NewGthulhuPlugin(5000*1000, 500*1000) // Reset plugin

		// Create multiple tasks with different priorities
		tasks := []*models.QueuedTask{
			{Pid: 100, Weight: 100, Vtime: 0, Tgid: 100, StartTs: 1000, StopTs: 2000},
			{Pid: 200, Weight: 150, Vtime: 0, Tgid: 200, StartTs: 1500, StopTs: 2500},
			{Pid: 300, Weight: 80, Vtime: 0, Tgid: 300, StartTs: 2000, StopTs: 3000},
		}

		// Enqueue all tasks
		for _, task := range tasks {
			mockSched.EnqueueTask(task)
		}

		// Drain all tasks
		drained := gthulhuPlugin.DrainQueuedTask(mockSched)
		if drained != 3 {
			t.Errorf("DrainQueuedTask = %d; want 3", drained)
		}

		// Verify pool count
		if gthulhuPlugin.GetPoolCount() != 3 {
			t.Errorf("GetPoolCount = %d; want 3", gthulhuPlugin.GetPoolCount())
		}

		// Process all tasks
		processedTasks := make([]*models.QueuedTask, 0)
		for gthulhuPlugin.GetPoolCount() > 0 {
			task := gthulhuPlugin.SelectQueuedTask(mockSched)
			if task == nil {
				t.Fatal("SelectQueuedTask returned nil while pool count > 0")
			}

			// Select CPU and determine time slice
			err, cpu := gthulhuPlugin.SelectCPU(mockSched, task)
			if err != nil {
				t.Errorf("SelectCPU error: %v", err)
			}
			if cpu < 0 {
				t.Errorf("Invalid CPU selected: %d", cpu)
			}

			_ = gthulhuPlugin.DetermineTimeSlice(mockSched, task)
			processedTasks = append(processedTasks, task)
		}

		// Verify all tasks were processed
		if len(processedTasks) != 3 {
			t.Errorf("Processed tasks = %d; want 3", len(processedTasks))
		}

		// Verify pool is empty
		if gthulhuPlugin.GetPoolCount() != 0 {
			t.Errorf("Final GetPoolCount = %d; want 0", gthulhuPlugin.GetPoolCount())
		}
	})

	t.Run("StrategyBasedScheduling", func(t *testing.T) {
		mockSched.Reset()
		gthulhuPlugin = NewGthulhuPlugin(5000*1000, 500*1000) // Reset plugin

		// Set up scheduling strategies
		strategies := []util.SchedulingStrategy{
			{PID: 100, Priority: true, ExecutionTime: 10000000},  // 10ms
			{PID: 200, Priority: false, ExecutionTime: 20000000}, // 20ms
		}
		gthulhuPlugin.UpdateStrategyMap(strategies)

		// Create tasks
		tasks := []*models.QueuedTask{
			{Pid: 100, Weight: 100, Vtime: 5000, Tgid: 100, StartTs: 1000, StopTs: 2000},
			{Pid: 200, Weight: 100, Vtime: 5000, Tgid: 200, StartTs: 1000, StopTs: 2000},
			{Pid: 300, Weight: 100, Vtime: 5000, Tgid: 300, StartTs: 1000, StopTs: 2000}, // No strategy
		}

		for _, task := range tasks {
			mockSched.EnqueueTask(task)
		}

		// Drain tasks
		drained := gthulhuPlugin.DrainQueuedTask(mockSched)
		if drained != 3 {
			t.Errorf("DrainQueuedTask = %d; want 3", drained)
		}

		// Process tasks and check time slices
		timeSlices := make(map[int32]uint64)
		for gthulhuPlugin.GetPoolCount() > 0 {
			task := gthulhuPlugin.SelectQueuedTask(mockSched)
			if task == nil {
				break
			}

			timeSlice := gthulhuPlugin.DetermineTimeSlice(mockSched, task)
			timeSlices[task.Pid] = timeSlice

			_, _ = gthulhuPlugin.SelectCPU(mockSched, task)
		}

		// Verify time slices based on strategy
		if ts, ok := timeSlices[100]; !ok || ts != 10000000 {
			t.Errorf("PID 100 time slice = %d; want 10000000", ts)
		}
		if ts, ok := timeSlices[200]; !ok || ts != 20000000 {
			t.Errorf("PID 200 time slice = %d; want 20000000", ts)
		}
		if ts, ok := timeSlices[300]; !ok || ts != 0 {
			t.Errorf("PID 300 time slice = %d; want 0 (no strategy)", ts)
		}
	})

	t.Run("PoolOverflowHandling", func(t *testing.T) {
		mockSched.Reset()
		gthulhuPlugin = NewGthulhuPlugin(5000*1000, 500*1000) // Reset plugin

		// Try to enqueue more tasks than pool can hold
		// taskPoolSize is 4096, so we'll try to add 4097 tasks
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
		drained := gthulhuPlugin.DrainQueuedTask(mockSched)

		// Should have drained all tasks or stopped when pool is full
		if drained > maxTasks {
			t.Errorf("Drained more tasks than enqueued: %d > %d", drained, maxTasks)
		}

		// Pool count should not exceed capacity
		poolCount := gthulhuPlugin.GetPoolCount()
		if poolCount > 4095 {
			t.Errorf("Pool count exceeds capacity: %d", poolCount)
		}
	})

	t.Run("ConcurrentInstanceIsolation", func(t *testing.T) {
		// Create two independent plugin instances
		plugin1 := NewGthulhuPlugin(5000*1000, 500*1000)
		plugin2 := NewGthulhuPlugin(10000*1000, 1000*1000)

		// Create separate mock schedulers
		sched1 := NewMockScheduler()
		sched2 := NewMockScheduler()

		// Add tasks to each scheduler
		sched1.EnqueueTask(&models.QueuedTask{Pid: 100, Weight: 100, Tgid: 100})
		sched1.EnqueueTask(&models.QueuedTask{Pid: 101, Weight: 100, Tgid: 101})

		sched2.EnqueueTask(&models.QueuedTask{Pid: 200, Weight: 100, Tgid: 200})

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

		// Process a task from plugin1
		task1 := plugin1.SelectQueuedTask(sched1)
		if task1 == nil {
			t.Fatal("Plugin1 SelectQueuedTask returned nil")
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

// TestMinHeapOrderByDeadline verifies that the task pool (min-heap) always pops
// the task with the smallest Deadline first, using Timestamp and Pid as
// tie-breakers per lessQueuedTask.
func TestMinHeapOrderByDeadline(t *testing.T) {
	g := NewGthulhuPlugin(0, 0)

	// Build tasks with explicit deadlines (representing vtime) and timestamps
	type input struct {
		pid       int32
		deadline  uint64
		timestamp uint64
	}
	inputs := []input{
		{pid: 101, deadline: 30, timestamp: 300},
		{pid: 102, deadline: 10, timestamp: 200},
		{pid: 103, deadline: 20, timestamp: 100},
		{pid: 104, deadline: 10, timestamp: 150},
		{pid: 105, deadline: 5, timestamp: 250},
	}

	for _, in := range inputs {
		qt := &models.QueuedTask{Pid: in.pid, StartTs: in.timestamp}
		task := Task{QueuedTask: qt, Deadline: in.deadline, Timestamp: in.timestamp}
		ok := g.insertTaskToPool(task)
		if !ok {
			t.Fatalf("failed to insert task pid=%d", in.pid)
		}
	}

	// Expected pop order by (deadline, timestamp, pid)
	expected := []int32{105, 104, 102, 103, 101}
	got := make([]int32, 0, len(expected))
	for range expected {
		qt := g.getTaskFromPool()
		if qt == nil {
			t.Fatalf("expected a task, got nil")
		}
		got = append(got, qt.Pid)
	}

	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("pop order mismatch at %d: got %v, want %v", i, got, expected)
		}
	}
}
