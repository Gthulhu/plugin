package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin/util"
)

type Sched interface {
	DequeueTask(task *models.QueuedTask)
	DefaultSelectCPU(t *models.QueuedTask) (error, int32)
	GetNrQueued() uint64
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
	// SendMetrics sends custom metrics to the monitoring system
	SendMetrics(interface{})
	// GetChangedStrategies returns the list of scheduling strategies that have changed since the last call
	GetChangedStrategies() ([]util.SchedulingStrategy, []util.SchedulingStrategy)
}

type Scheduler struct {
	SliceNsDefault uint64 `yaml:"slice_ns_default"`
	SliceNsMin     uint64 `yaml:"slice_ns_min"`
}

// MTLSConfig holds the mutual TLS configuration used for plugin → API server communication.
// CertPem and KeyPem are the plugin's own certificate/key pair signed by the private CA.
// CAPem is the private CA certificate used to verify the API server's certificate.
type MTLSConfig struct {
	Enable  bool   `yaml:"enable"`
	CertPem string `yaml:"cert_pem"`
	KeyPem  string `yaml:"key_pem"`
	CAPem   string `yaml:"ca_pem"`
}

type APIConfig struct {
	PublicKeyPath string     `yaml:"public_key_path"`
	BaseURL       string     `yaml:"base_url"`
	Interval      int        `yaml:"interval"`
	Enabled       bool       `yaml:"enabled"`
	AuthEnabled   bool       `yaml:"auth_enabled"`
	MTLS          MTLSConfig `yaml:"mtls"`
}

// SchedConfig holds the configuration parameters for creating a scheduler plugin
type SchedConfig struct {
	// Mode specifies which scheduler plugin to use (e.g., "gthulhu", "simple", "simple-fifo")
	Mode string `yaml:"mode"`

	// Scheduler configuration (for Gthulhu plugin)
	// These match the parameters that would be passed from the Gthulhu main repo
	Scheduler Scheduler `yaml:"scheduler"`

	// API configuration
	APIConfig APIConfig `yaml:"api_config"`
}

// PluginFactory is a function type that creates a CustomScheduler instance
type PluginFactory func(ctx context.Context, config *SchedConfig) (CustomScheduler, error)

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
func NewSchedulerPlugin(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	registryMutex.RLock()
	factory, exists := pluginRegistry[config.Mode]
	registryMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown plugin mode: %s", config.Mode)
	}

	return factory(ctx, config)
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

// The following helpers are intended for tests only.
func ClearRegistryForTests() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	pluginRegistry = make(map[string]PluginFactory)
}

func SnapshotRegistryForTests() map[string]PluginFactory {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	copyMap := make(map[string]PluginFactory, len(pluginRegistry))
	for k, v := range pluginRegistry {
		copyMap[k] = v
	}
	return copyMap
}

func RestoreRegistryForTests(m map[string]PluginFactory) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	pluginRegistry = m
}
