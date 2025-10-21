package plugin

import (
	"context"

	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
)

// Type aliases to preserve public API while delegating to internal registry
type (
	Sched           = reg.Sched
	CustomScheduler = reg.CustomScheduler
	Scheduler       = reg.Scheduler
	APIConfig       = reg.APIConfig
	SchedConfig     = reg.SchedConfig
	PluginFactory   = reg.PluginFactory
)

// Forwarder functions to internal registry
func RegisterNewPlugin(mode string, factory PluginFactory) error {
	return reg.RegisterNewPlugin(mode, factory)
}

func NewSchedulerPlugin(ctx context.Context, config *SchedConfig) (CustomScheduler, error) {
	return reg.NewSchedulerPlugin(ctx, config)
}

func GetRegisteredModes() []string {
	return reg.GetRegisteredModes()
}

// Test helpers delegation (kept unexported)
func clearRegistryForTests()                             { reg.ClearRegistryForTests() }
func snapshotRegistryForTests() map[string]PluginFactory { return reg.SnapshotRegistryForTests() }
func restoreRegistryForTests(m map[string]PluginFactory) { reg.RestoreRegistryForTests(m) }
