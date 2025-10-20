package plugin

import (
	"fmt"
	"sync"

	"github.com/Gthulhu/plugin/models"
)

type Sched interface {
	DequeueTask(task *models.QueuedTask)
	DefaultSelectCPU(t *models.QueuedTask) (error, int32)
}

type CustomScheduler interface {
	// Drain the queued task from eBPF and return the number of tasks drained
	DrainQueuedTask(s Sched) int
	// Select a task from the queued tasks and return it
	SelectQueuedTask(s Sched) *models.QueuedTask
	// Select a CPU for the given queued task, After selecting the CPU, the task will be dispatched to that CPU by Scheduler
	SelectCPU(s Sched, t *models.QueuedTask) (error, int32)
	// Determine the time slice for the given task
	DetermineTimeSlice(s Sched, t *models.QueuedTask) uint64
	// Get the number of objects in the pool (waiting to be dispatched)
	// GetPoolCount will be called by the scheduler to notify the number of tasks waiting to be dispatched (NotifyComplete)
	GetPoolCount() uint64
}

// SchedConfig holds the configuration parameters for creating a scheduler plugin
type SchedConfig struct {
	// Mode specifies which scheduler plugin to use (e.g., "gthulhu", "simple", "simple-fifo")
	Mode string `yaml:"mode"`

	// SimpleScheduler configuration
	SliceNsDefault uint64 `yaml:"slice_ns_default"`
	SliceNsMin     uint64 `yaml:"slice_ns_min"`
	FifoMode       bool   `yaml:"fifo_mode"`

	// Scheduler configuration (for Gthulhu plugin)
	// These match the parameters that would be passed from the Gthulhu main repo
	Scheduler struct {
		SliceNsDefault uint64 `yaml:"slice_ns_default"`
		SliceNsMin     uint64 `yaml:"slice_ns_min"`
	} `yaml:"scheduler"`

	// API configuration
	APIConfig struct {
		PublicKeyPath string `yaml:"public_key_path"`
		BaseURL       string `yaml:"base_url"`
	} `yaml:"api_config"`
}

// PluginFactory is a function type that creates a CustomScheduler instance
type PluginFactory func(config *SchedConfig) (CustomScheduler, error)

var (
	// pluginRegistry stores registered plugin factories
	pluginRegistry = make(map[string]PluginFactory)
	registryMutex  sync.RWMutex
)

// RegisterNewPlugin registers a plugin factory for a specific mode
// This should be called in the init() function of each plugin implementation
func RegisterNewPlugin(mode string, factory PluginFactory) error {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if mode == "" {
		return fmt.Errorf("plugin mode cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("plugin factory cannot be nil")
	}

	if _, exists := pluginRegistry[mode]; exists {
		return fmt.Errorf("plugin mode '%s' is already registered", mode)
	}

	pluginRegistry[mode] = factory
	return nil
}

// NewSchedulerPlugin creates a new scheduler plugin based on the configuration
// This is the factory function that follows the simple factory pattern
func NewSchedulerPlugin(config *SchedConfig) (CustomScheduler, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	registryMutex.RLock()
	factory, exists := pluginRegistry[config.Mode]
	registryMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown plugin mode: %s", config.Mode)
	}

	return factory(config)
}

// GetRegisteredModes returns a list of all registered plugin modes
func GetRegisteredModes() []string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	modes := make([]string, 0, len(pluginRegistry))
	for mode := range pluginRegistry {
		modes = append(modes, mode)
	}
	return modes
}
