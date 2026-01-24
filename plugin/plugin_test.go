package plugin

import (
	"context"
	"log"
	"testing"

	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin/util"
)

// TestRegisterNewPlugin tests the plugin registration functionality
func TestRegisterNewPlugin(t *testing.T) {
	// Clear registry for testing
	originalRegistry := snapshotRegistryForTests()
	clearRegistryForTests()

	// Restore original registry after test
	defer func() {
		restoreRegistryForTests(originalRegistry)
	}()

	t.Run("SuccessfulRegistration", func(t *testing.T) {
		// Clear for this subtest
		clearRegistryForTests()

		factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
			return &mockScheduler{}, nil
		}

		err := RegisterNewPlugin("test-mode", factory)
		if err != nil {
			t.Errorf("RegisterNewPlugin failed: %v", err)
		}

		// Verify registration
		modes := GetRegisteredModes()
		if len(modes) != 1 || modes[0] != "test-mode" {
			t.Errorf("Expected 1 mode 'test-mode', got %v", modes)
		}
	})

	t.Run("EmptyModeError", func(t *testing.T) {
		factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
			return &mockScheduler{}, nil
		}

		err := RegisterNewPlugin("", factory)
		if err == nil {
			t.Error("Expected error for empty mode, got nil")
		}
		if err.Error() != "plugin mode cannot be empty" {
			t.Errorf("Expected 'plugin mode cannot be empty' error, got: %v", err)
		}
	})

	t.Run("NilFactoryError", func(t *testing.T) {
		err := RegisterNewPlugin("test-mode", nil)
		if err == nil {
			t.Error("Expected error for nil factory, got nil")
		}
		if err.Error() != "plugin factory cannot be nil" {
			t.Errorf("Expected 'plugin factory cannot be nil' error, got: %v", err)
		}
	})

	t.Run("DuplicateRegistrationError", func(t *testing.T) {
		// Clear for this subtest
		clearRegistryForTests()

		factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
			return &mockScheduler{}, nil
		}

		// Register once
		err := RegisterNewPlugin("duplicate-mode", factory)
		if err != nil {
			t.Errorf("First registration failed: %v", err)
		}

		// Try to register again
		err = RegisterNewPlugin("duplicate-mode", factory)
		if err == nil {
			t.Error("Expected error for duplicate registration, got nil")
		}
		if err.Error() != "plugin mode 'duplicate-mode' is already registered" {
			t.Errorf("Expected duplicate registration error, got: %v", err)
		}
	})
}

// TestNewSchedulerPlugin tests the factory function
func TestNewSchedulerPlugin(t *testing.T) {
	// Clear registry for testing
	originalRegistry := snapshotRegistryForTests()
	clearRegistryForTests()

	// Restore original registry after test
	defer func() {
		restoreRegistryForTests(originalRegistry)
	}()

	t.Run("NilConfigError", func(t *testing.T) {
		_, err := NewSchedulerPlugin(context.TODO(), nil)
		if err == nil {
			t.Error("Expected error for nil config, got nil")
		}
		if err.Error() != "config cannot be nil" {
			t.Errorf("Expected 'config cannot be nil' error, got: %v", err)
		}
	})

	t.Run("UnknownModeError", func(t *testing.T) {
		config := &SchedConfig{Mode: "unknown-mode"}
		_, err := NewSchedulerPlugin(context.TODO(), config)
		if err == nil {
			t.Error("Expected error for unknown mode, got nil")
		}
		if err.Error() != "unknown plugin mode: unknown-mode" {
			t.Errorf("Expected 'unknown plugin mode' error, got: %v", err)
		}
	})

	t.Run("SuccessfulPluginCreation", func(t *testing.T) {
		// Clear and register a test plugin
		clearRegistryForTests()

		factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
			return &mockScheduler{mode: config.Mode}, nil
		}

		err := RegisterNewPlugin("test-plugin", factory)
		if err != nil {
			t.Fatalf("Failed to register plugin: %v", err)
		}

		config := &SchedConfig{Mode: "test-plugin"}
		scheduler, err := NewSchedulerPlugin(context.TODO(), config)
		if err != nil {
			t.Errorf("NewSchedulerPlugin failed: %v", err)
		}
		if scheduler == nil {
			t.Error("Expected scheduler, got nil")
		}

		mock, ok := scheduler.(*mockScheduler)
		if !ok {
			t.Error("Expected mockScheduler type")
		}
		if mock.mode != "test-plugin" {
			t.Errorf("Expected mode 'test-plugin', got '%s'", mock.mode)
		}
	})
}

// TestGetRegisteredModes tests retrieving all registered modes
func TestGetRegisteredModes(t *testing.T) {
	// Clear registry for testing
	originalRegistry := snapshotRegistryForTests()
	clearRegistryForTests()

	// Restore original registry after test
	defer func() {
		restoreRegistryForTests(originalRegistry)
	}()

	t.Run("EmptyRegistry", func(t *testing.T) {
		// Clear registry
		clearRegistryForTests()

		modes := GetRegisteredModes()
		if len(modes) != 0 {
			t.Errorf("Expected 0 modes, got %d: %v", len(modes), modes)
		}
	})

	t.Run("MultipleModes", func(t *testing.T) {
		// Clear and register multiple plugins
		clearRegistryForTests()

		factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
			return &mockScheduler{}, nil
		}

		_ = RegisterNewPlugin("mode1", factory)
		_ = RegisterNewPlugin("mode2", factory)
		_ = RegisterNewPlugin("mode3", factory)

		modes := GetRegisteredModes()
		if len(modes) != 3 {
			t.Errorf("Expected 3 modes, got %d: %v", len(modes), modes)
		}

		// Check all modes are present
		modeMap := make(map[string]bool)
		for _, mode := range modes {
			modeMap[mode] = true
		}

		expectedModes := []string{"mode1", "mode2", "mode3"}
		for _, expected := range expectedModes {
			if !modeMap[expected] {
				t.Errorf("Expected mode '%s' not found in: %v", expected, modes)
			}
		}
	})
}

// TestSchedConfigStructure tests the SchedConfig struct
func TestSchedConfigStructure(t *testing.T) {
	t.Run("CompleteConfig", func(t *testing.T) {
		config := &SchedConfig{
			Mode: "gthulhu",
			Scheduler: Scheduler{
				SliceNsDefault: 5000000,
				SliceNsMin:     500000,
			},
		}

		config.APIConfig.PublicKeyPath = "/path/to/key"
		config.APIConfig.BaseURL = "https://api.example.com"

		if config.Mode != "gthulhu" {
			t.Errorf("Expected mode 'gthulhu', got '%s'", config.Mode)
		}
		if config.Scheduler.SliceNsDefault != 5000000 {
			t.Errorf("Expected SliceNsDefault 5000000, got %d", config.Scheduler.SliceNsDefault)
		}
		if config.Scheduler.SliceNsDefault != 5000000 {
			t.Errorf("Expected Scheduler.SliceNsDefault 5000000, got %d", config.Scheduler.SliceNsDefault)
		}
		if config.APIConfig.BaseURL != "https://api.example.com" {
			t.Errorf("Expected BaseURL 'https://api.example.com', got '%s'", config.APIConfig.BaseURL)
		}
	})

	t.Run("MinimalConfig", func(t *testing.T) {
		config := &SchedConfig{
			Mode: "simple",
		}

		if config.Mode != "simple" {
			t.Errorf("Expected mode 'simple', got '%s'", config.Mode)
		}
		if config.Scheduler.SliceNsDefault != 0 {
			t.Errorf("Expected default SliceNsDefault 0, got %d", config.Scheduler.SliceNsDefault)
		}
	})
}

// TestConcurrentRegistration tests thread-safety of plugin registration
func TestConcurrentRegistration(t *testing.T) {
	// Clear registry for testing
	originalRegistry := snapshotRegistryForTests()
	clearRegistryForTests()

	// Restore original registry after test
	defer func() {
		restoreRegistryForTests(originalRegistry)
	}()

	factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
		return &mockScheduler{}, nil
	}

	// Register plugins concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			mode := "concurrent-mode-" + string(rune('0'+id))
			_ = RegisterNewPlugin(mode, factory)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that some plugins were registered
	modes := GetRegisteredModes()
	if len(modes) == 0 {
		t.Error("Expected at least some modes to be registered")
	}
}

// TestFactoryWithConfigParameters tests that config parameters are passed correctly
func TestFactoryWithConfigParameters(t *testing.T) {
	// Clear registry for testing
	originalRegistry := snapshotRegistryForTests()
	clearRegistryForTests()

	// Restore original registry after test
	defer func() {
		restoreRegistryForTests(originalRegistry)
	}()

	// Register a plugin that captures config
	var capturedConfig *SchedConfig
	factory := func(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
		capturedConfig = config
		return &mockScheduler{}, nil
	}

	_ = RegisterNewPlugin("config-test", factory)

	config := &SchedConfig{
		Mode: "config-test",
		Scheduler: Scheduler{
			SliceNsDefault: 12345,
			SliceNsMin:     6789,
		},
	}

	_, err := NewSchedulerPlugin(context.TODO(), config)
	if err != nil {
		t.Fatalf("NewSchedulerPlugin failed: %v", err)
	}

	if capturedConfig == nil {
		t.Fatal("Config was not passed to factory")
	}
	if capturedConfig.Mode != "config-test" {
		t.Errorf("Expected mode 'config-test', got '%s'", capturedConfig.Mode)
	}
	if capturedConfig.Scheduler.SliceNsDefault != 12345 {
		t.Errorf("Expected SliceNsDefault 12345, got %d", capturedConfig.Scheduler.SliceNsDefault)
	}
	if capturedConfig.Scheduler.SliceNsMin != 6789 {
		t.Errorf("Expected SliceNsMin 6789, got %d", capturedConfig.Scheduler.SliceNsMin)
	}
}

// mockScheduler is a mock implementation of CustomScheduler for testing
type mockScheduler struct {
	mode string
}

func (m *mockScheduler) SendMetrics(data interface{}) {
	// Mock implementation: just log the data
	log.Printf("Sending metrics: %+v", data)
}

func (m *mockScheduler) DrainQueuedTask(s Sched) int {
	return 0
}

func (m *mockScheduler) SelectQueuedTask(s Sched) *models.QueuedTask {
	return nil
}

func (m *mockScheduler) SelectCPU(s Sched, t *models.QueuedTask) (error, int32) {
	return nil, 0
}

func (m *mockScheduler) DetermineTimeSlice(s Sched, t *models.QueuedTask) uint64 {
	return 0
}

func (m *mockScheduler) GetPoolCount() uint64 {
	return 0
}

func (m *mockScheduler) GetChangedStrategies() ([]util.SchedulingStrategy, []util.SchedulingStrategy) {
	return nil, nil
}
