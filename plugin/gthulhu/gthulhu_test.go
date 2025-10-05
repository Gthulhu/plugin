package gthulhu

import (
	"testing"
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
	plugin := NewGthulhuPlugin(0, 0)

	defaultNs, minNs := plugin.GetSchedulerConfig()

	if defaultNs != 5000*1000 {
		t.Errorf("Default sliceNsDefault = %d; want %d", defaultNs, 5000*1000)
	}
	if minNs != 500*1000 {
		t.Errorf("Default sliceNsMin = %d; want %d", minNs, 500*1000)
	}
}

// TestGthulhuPluginTaskPoolInitialization verifies task pool is initialized correctly
func TestGthulhuPluginTaskPoolInitialization(t *testing.T) {
	plugin := NewGthulhuPlugin(0, 0)

	// Verify initial pool count is 0
	if plugin.GetPoolCount() != 0 {
		t.Errorf("Initial pool count = %d; want 0", plugin.GetPoolCount())
	}

	// Verify task pool is allocated
	if plugin.taskPool == nil {
		t.Error("Task pool is nil; expected allocated array")
	}

	// Verify task pool size
	if len(plugin.taskPool) != taskPoolSize {
		t.Errorf("Task pool size = %d; want %d", len(plugin.taskPool), taskPoolSize)
	}
}

// TestGthulhuPluginStrategyMapInitialization verifies strategy map is initialized
func TestGthulhuPluginStrategyMapInitialization(t *testing.T) {
	plugin := NewGthulhuPlugin(0, 0)

	// Verify strategy map is initialized
	if plugin.strategyMap == nil {
		t.Error("Strategy map is nil; expected initialized map")
	}

	// Verify initial strategy map is empty
	if len(plugin.strategyMap) != 0 {
		t.Errorf("Initial strategy map size = %d; want 0", len(plugin.strategyMap))
	}
}

// TestGthulhuPluginSetSchedulerConfig verifies SetSchedulerConfig updates configuration
func TestGthulhuPluginSetSchedulerConfig(t *testing.T) {
	plugin := NewGthulhuPlugin(0, 0)

	// Update configuration
	plugin.SetSchedulerConfig(15000*1000, 1500*1000)

	defaultNs, minNs := plugin.GetSchedulerConfig()

	if defaultNs != 15000*1000 {
		t.Errorf("Updated sliceNsDefault = %d; want %d", defaultNs, 15000*1000)
	}
	if minNs != 1500*1000 {
		t.Errorf("Updated sliceNsMin = %d; want %d", minNs, 1500*1000)
	}
}

// TestGthulhuPluginUpdateStrategyMap verifies UpdateStrategyMap works correctly
func TestGthulhuPluginUpdateStrategyMap(t *testing.T) {
	plugin := NewGthulhuPlugin(0, 0)

	// Create test strategies
	strategies := []SchedulingStrategy{
		{PID: 100, Priority: true, ExecutionTime: 10000},
		{PID: 200, Priority: false, ExecutionTime: 20000},
	}

	// Update strategy map
	plugin.UpdateStrategyMap(strategies)

	// Verify strategies were added
	if len(plugin.strategyMap) != 2 {
		t.Errorf("Strategy map size = %d; want 2", len(plugin.strategyMap))
	}

	// Verify strategy for PID 100
	if strategy, exists := plugin.strategyMap[100]; !exists {
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
	if strategy, exists := plugin.strategyMap[200]; !exists {
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
