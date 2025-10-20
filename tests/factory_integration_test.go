package tests

import (
	"testing"

	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"

	// Import plugin packages to trigger init() functions
	_ "github.com/Gthulhu/plugin/plugin/gthulhu"
	_ "github.com/Gthulhu/plugin/plugin/simple"
)

// TestGthulhuPluginThroughFactory tests creating gthulhu plugin via factory
func TestGthulhuPluginThroughFactory(t *testing.T) {
	t.Run("BasicCreation", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode:           "gthulhu",
			SliceNsDefault: 5000 * 1000,
			SliceNsMin:     500 * 1000,
		}

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create gthulhu plugin: %v", err)
		}
		if scheduler == nil {
			t.Fatal("Expected scheduler, got nil")
		}

		// Verify it implements CustomScheduler interface
		if scheduler.GetPoolCount() != 0 {
			t.Errorf("Expected initial pool count 0, got %d", scheduler.GetPoolCount())
		}
	})

	t.Run("WithSchedulerConfig", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode: "gthulhu",
		}
		config.Scheduler.SliceNsDefault = 10000 * 1000
		config.Scheduler.SliceNsMin = 1000 * 1000

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create gthulhu plugin: %v", err)
		}
		if scheduler == nil {
			t.Fatal("Expected scheduler, got nil")
		}

		// Test basic operations
		if scheduler.GetPoolCount() != 0 {
			t.Errorf("Expected pool count 0, got %d", scheduler.GetPoolCount())
		}
	})

	t.Run("FunctionalTest", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode:           "gthulhu",
			SliceNsDefault: 5000 * 1000,
		}

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create gthulhu plugin: %v", err)
		}

		// Create a mock Sched
		mockSched := &testSched{
			tasks: []*models.QueuedTask{
				{Pid: 100, Weight: 100, Vtime: 1000, Tgid: 100},
				{Pid: 200, Weight: 100, Vtime: 2000, Tgid: 200},
			},
		}

		// Drain tasks
		drained := scheduler.DrainQueuedTask(mockSched)
		if drained != 2 {
			t.Errorf("Expected to drain 2 tasks, got %d", drained)
		}

		// Check pool count
		if scheduler.GetPoolCount() != 2 {
			t.Errorf("Expected pool count 2, got %d", scheduler.GetPoolCount())
		}

		// Select a task
		task := scheduler.SelectQueuedTask(mockSched)
		if task == nil {
			t.Fatal("Expected task, got nil")
		}

		// Check pool count decreased
		if scheduler.GetPoolCount() != 1 {
			t.Errorf("Expected pool count 1 after select, got %d", scheduler.GetPoolCount())
		}
	})
}

// TestSimplePluginThroughFactory tests creating simple plugin via factory
func TestSimplePluginThroughFactory(t *testing.T) {
	t.Run("SimpleWeightedVtime", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode:           "simple",
			SliceNsDefault: 5000 * 100,
		}

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create simple plugin: %v", err)
		}
		if scheduler == nil {
			t.Fatal("Expected scheduler, got nil")
		}

		// Verify initial state
		if scheduler.GetPoolCount() != 0 {
			t.Errorf("Expected initial pool count 0, got %d", scheduler.GetPoolCount())
		}
	})

	t.Run("SimpleFIFO", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode:           "simple-fifo",
			SliceNsDefault: 5000 * 100,
		}

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create simple-fifo plugin: %v", err)
		}
		if scheduler == nil {
			t.Fatal("Expected scheduler, got nil")
		}

		// Verify initial state
		if scheduler.GetPoolCount() != 0 {
			t.Errorf("Expected initial pool count 0, got %d", scheduler.GetPoolCount())
		}
	})

	t.Run("FunctionalTest", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode:           "simple",
			SliceNsDefault: 5000 * 100,
		}

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create simple plugin: %v", err)
		}

		// Create a mock Sched
		mockSched := &testSched{
			tasks: []*models.QueuedTask{
				{Pid: 100, Weight: 100, Vtime: 5000, Tgid: 100},
				{Pid: 200, Weight: 100, Vtime: 3000, Tgid: 200},
				{Pid: 300, Weight: 100, Vtime: 7000, Tgid: 300},
			},
		}

		// Drain tasks
		drained := scheduler.DrainQueuedTask(mockSched)
		if drained != 3 {
			t.Errorf("Expected to drain 3 tasks, got %d", drained)
		}

		// Check pool count
		if scheduler.GetPoolCount() != 3 {
			t.Errorf("Expected pool count 3, got %d", scheduler.GetPoolCount())
		}

		// Select tasks - should be in vtime order
		task1 := scheduler.SelectQueuedTask(mockSched)
		if task1 == nil {
			t.Fatal("Expected first task, got nil")
		}
		if task1.Pid != 200 { // Task with lowest vtime (3000)
			t.Errorf("Expected first task PID 200, got %d", task1.Pid)
		}

		task2 := scheduler.SelectQueuedTask(mockSched)
		if task2 == nil {
			t.Fatal("Expected second task, got nil")
		}

		// Check pool count
		if scheduler.GetPoolCount() != 1 {
			t.Errorf("Expected pool count 1, got %d", scheduler.GetPoolCount())
		}
	})

	t.Run("FIFOFunctionalTest", func(t *testing.T) {
		config := &plugin.SchedConfig{
			Mode: "simple-fifo",
		}

		scheduler, err := plugin.NewSchedulerPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create simple-fifo plugin: %v", err)
		}

		// Create a mock Sched with tasks in specific order
		mockSched := &testSched{
			tasks: []*models.QueuedTask{
				{Pid: 100, Weight: 100, Vtime: 7000, Tgid: 100},
				{Pid: 200, Weight: 100, Vtime: 3000, Tgid: 200},
				{Pid: 300, Weight: 100, Vtime: 5000, Tgid: 300},
			},
		}

		// Drain tasks
		drained := scheduler.DrainQueuedTask(mockSched)
		if drained != 3 {
			t.Errorf("Expected to drain 3 tasks, got %d", drained)
		}

		// Select tasks - should be in FIFO order (100, 200, 300)
		task1 := scheduler.SelectQueuedTask(mockSched)
		if task1 == nil {
			t.Fatal("Expected first task, got nil")
		}
		if task1.Pid != 100 { // First enqueued
			t.Errorf("Expected first task PID 100, got %d", task1.Pid)
		}

		task2 := scheduler.SelectQueuedTask(mockSched)
		if task2 == nil {
			t.Fatal("Expected second task, got nil")
		}
		if task2.Pid != 200 { // Second enqueued
			t.Errorf("Expected second task PID 200, got %d", task2.Pid)
		}

		task3 := scheduler.SelectQueuedTask(mockSched)
		if task3 == nil {
			t.Fatal("Expected third task, got nil")
		}
		if task3.Pid != 300 { // Third enqueued
			t.Errorf("Expected third task PID 300, got %d", task3.Pid)
		}
	})
}

// TestMultiplePluginInstances tests creating multiple plugin instances
func TestMultiplePluginInstances(t *testing.T) {
	t.Run("MultipleGthulhuInstances", func(t *testing.T) {
		config1 := &plugin.SchedConfig{
			Mode:           "gthulhu",
			SliceNsDefault: 5000 * 1000,
		}
		config2 := &plugin.SchedConfig{
			Mode:           "gthulhu",
			SliceNsDefault: 10000 * 1000,
		}

		scheduler1, err := plugin.NewSchedulerPlugin(config1)
		if err != nil {
			t.Fatalf("Failed to create first plugin: %v", err)
		}

		scheduler2, err := plugin.NewSchedulerPlugin(config2)
		if err != nil {
			t.Fatalf("Failed to create second plugin: %v", err)
		}

		// Verify they are independent
		mockSched := &testSched{
			tasks: []*models.QueuedTask{
				{Pid: 100, Weight: 100, Vtime: 1000, Tgid: 100},
			},
		}

		scheduler1.DrainQueuedTask(mockSched)
		if scheduler1.GetPoolCount() != 1 {
			t.Errorf("Scheduler1 pool count = %d; want 1", scheduler1.GetPoolCount())
		}
		if scheduler2.GetPoolCount() != 0 {
			t.Errorf("Scheduler2 pool count = %d; want 0", scheduler2.GetPoolCount())
		}
	})

	t.Run("MixedPluginTypes", func(t *testing.T) {
		gthulhuConfig := &plugin.SchedConfig{Mode: "gthulhu"}
		simpleConfig := &plugin.SchedConfig{Mode: "simple"}
		fifoConfig := &plugin.SchedConfig{Mode: "simple-fifo"}

		gthulhu, err := plugin.NewSchedulerPlugin(gthulhuConfig)
		if err != nil {
			t.Fatalf("Failed to create gthulhu plugin: %v", err)
		}

		simple, err := plugin.NewSchedulerPlugin(simpleConfig)
		if err != nil {
			t.Fatalf("Failed to create simple plugin: %v", err)
		}

		fifo, err := plugin.NewSchedulerPlugin(fifoConfig)
		if err != nil {
			t.Fatalf("Failed to create simple-fifo plugin: %v", err)
		}

		// Verify all are independent
		if gthulhu == nil || simple == nil || fifo == nil {
			t.Fatal("One or more plugins are nil")
		}

		// All should start with empty pools
		if gthulhu.GetPoolCount() != 0 {
			t.Errorf("Gthulhu pool count = %d; want 0", gthulhu.GetPoolCount())
		}
		if simple.GetPoolCount() != 0 {
			t.Errorf("Simple pool count = %d; want 0", simple.GetPoolCount())
		}
		if fifo.GetPoolCount() != 0 {
			t.Errorf("FIFO pool count = %d; want 0", fifo.GetPoolCount())
		}
	})
}

// TestRegisteredModesIntegration tests that all expected modes are registered
func TestRegisteredModesIntegration(t *testing.T) {
	modes := plugin.GetRegisteredModes()

	expectedModes := []string{"gthulhu", "simple", "simple-fifo"}
	modeMap := make(map[string]bool)
	for _, mode := range modes {
		modeMap[mode] = true
	}

	for _, expected := range expectedModes {
		if !modeMap[expected] {
			t.Errorf("Expected mode '%s' to be registered, but it's not", expected)
		}
	}

	if len(modes) < 3 {
		t.Errorf("Expected at least 3 registered modes, got %d: %v", len(modes), modes)
	}
}

// testSched is a mock implementation of plugin.Sched interface for testing
type testSched struct {
	tasks []*models.QueuedTask
	index int
}

var _ plugin.Sched = (*testSched)(nil)

func (s *testSched) DequeueTask(task *models.QueuedTask) {
	if s.index >= len(s.tasks) {
		task.Pid = -1
		return
	}
	*task = *s.tasks[s.index]
	s.index++
}

func (s *testSched) DefaultSelectCPU(t *models.QueuedTask) (error, int32) {
	return nil, 0
}
