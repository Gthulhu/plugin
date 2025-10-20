# Factory Pattern Implementation Guide

This document describes the factory pattern implementation for Gthulhu plugins, which allows for dynamic plugin registration and instantiation.

## Overview

The factory pattern implementation provides a flexible way to create scheduler plugins without hardcoding plugin instantiation logic. This eliminates the need for switch-case statements in the Gthulhu main repository and allows new plugins to be added without modifying existing code.

## Key Components

### 1. SchedConfig Structure

The `SchedConfig` struct holds all configuration parameters needed to create a plugin:

```go
type SchedConfig struct {
    // Mode specifies which scheduler plugin to use (e.g., "gthulhu", "simple", "simple-fifo")
    Mode string

    // SimpleScheduler configuration
    SliceNsDefault uint64
    SliceNsMin     uint64
    FifoMode       bool

    // Scheduler configuration (for Gthulhu plugin)
    Scheduler struct {
        SliceNsDefault uint64
        SliceNsMin     uint64
    }

    // API configuration
    APIConfig struct {
        PublicKeyPath string
        BaseURL       string
    }
}
```

### 2. Plugin Registration API

The `RegisterNewPlugin` function allows plugins to register themselves during initialization:

```go
func RegisterNewPlugin(mode string, factory PluginFactory) error
```

- **mode**: Unique identifier for the plugin (e.g., "gthulhu", "simple")
- **factory**: Function that creates an instance of the plugin
- **Returns**: Error if mode is empty, factory is nil, or mode is already registered

### 3. Factory Function

The `NewSchedulerPlugin` function creates plugin instances based on configuration:

```go
func NewSchedulerPlugin(config *SchedConfig) (CustomScheduler, error)
```

- **config**: Configuration specifying which plugin to create and its parameters
- **Returns**: CustomScheduler instance or error if mode is unknown

## Usage Examples

### Creating a Plugin Instance

```go
import "github.com/Gthulhu/plugin/plugin"

// Create a Gthulhu plugin
config := &plugin.SchedConfig{
    Mode:           "gthulhu",
    SliceNsDefault: 5000 * 1000,  // 5ms
    SliceNsMin:     500 * 1000,   // 0.5ms
}

scheduler, err := plugin.NewSchedulerPlugin(config)
if err != nil {
    log.Fatalf("Failed to create plugin: %v", err)
}

// Use the scheduler
scheduler.DrainQueuedTask(sched)
```

### Creating Different Plugin Types

```go
// Create a simple plugin with weighted vtime
simpleConfig := &plugin.SchedConfig{
    Mode:           "simple",
    SliceNsDefault: 500000,
}
simpleScheduler, _ := plugin.NewSchedulerPlugin(simpleConfig)

// Create a simple plugin with FIFO mode
fifoConfig := &plugin.SchedConfig{
    Mode: "simple-fifo",
}
fifoScheduler, _ := plugin.NewSchedulerPlugin(fifoConfig)
```

### Using API Configuration

```go
// Create a Gthulhu plugin with API authentication
config := &plugin.SchedConfig{
    Mode: "gthulhu",
}
config.Scheduler.SliceNsDefault = 10000 * 1000
config.Scheduler.SliceNsMin = 1000 * 1000
config.APIConfig.PublicKeyPath = "/path/to/public.key"
config.APIConfig.BaseURL = "https://api.example.com"

scheduler, err := plugin.NewSchedulerPlugin(config)
if err != nil {
    log.Fatalf("Failed to create plugin: %v", err)
}
```

## Implementing a New Plugin

To create a new plugin that integrates with the factory pattern:

### Step 1: Implement the CustomScheduler Interface

```go
package myplugin

import (
    "github.com/Gthulhu/plugin/models"
    "github.com/Gthulhu/plugin/plugin"
)

type MyPlugin struct {
    // Your plugin fields
}

func NewMyPlugin(config *plugin.SchedConfig) *MyPlugin {
    return &MyPlugin{
        // Initialize based on config
    }
}

// Implement CustomScheduler interface methods
func (m *MyPlugin) DrainQueuedTask(s plugin.Sched) int { /* ... */ }
func (m *MyPlugin) SelectQueuedTask(s plugin.Sched) *models.QueuedTask { /* ... */ }
func (m *MyPlugin) SelectCPU(s plugin.Sched, t *models.QueuedTask) (error, int32) { /* ... */ }
func (m *MyPlugin) DetermineTimeSlice(s plugin.Sched, t *models.QueuedTask) uint64 { /* ... */ }
func (m *MyPlugin) GetPoolCount() uint64 { /* ... */ }
```

### Step 2: Register the Plugin in init()

```go
func init() {
    plugin.RegisterNewPlugin("myplugin", func(config *plugin.SchedConfig) (plugin.CustomScheduler, error) {
        // Create and configure your plugin
        myPlugin := NewMyPlugin(config)
        
        // Apply any additional configuration
        if config.SliceNsDefault > 0 {
            myPlugin.SetSliceDefault(config.SliceNsDefault)
        }
        
        return myPlugin, nil
    })
}
```

### Step 3: Use Your Plugin

```go
import _ "github.com/yourorg/myplugin"  // Import to trigger init()

config := &plugin.SchedConfig{
    Mode:           "myplugin",
    SliceNsDefault: 5000000,
}

scheduler, err := plugin.NewSchedulerPlugin(config)
```

## Registered Plugins

The following plugins are registered by default:

| Mode | Description | Configuration |
|------|-------------|---------------|
| `gthulhu` | Advanced scheduler with API integration | Scheduler.SliceNsDefault, Scheduler.SliceNsMin, APIConfig |
| `simple` | Simple weighted vtime scheduler | SliceNsDefault |
| `simple-fifo` | Simple FIFO scheduler | SliceNsDefault |

### Checking Registered Modes

You can retrieve all registered plugin modes:

```go
modes := plugin.GetRegisteredModes()
fmt.Printf("Available plugins: %v\n", modes)
```

## Benefits

1. **Extensibility**: New plugins can be added without modifying existing code
2. **Decoupling**: Plugin implementation is separate from instantiation logic
3. **Type Safety**: Factory functions are type-safe and validated at registration
4. **Thread Safety**: Registration uses mutex protection for concurrent safety
5. **Flexibility**: Each plugin can define its own configuration requirements

## Migration from Previous Implementation

### Before (Hardcoded in Gthulhu)

```go
// In Gthulhu main.go
switch schedConfig.Mode {
case "gthulhu":
    plugin = gthulhu.NewGthulhuPlugin(cfg.SimpleScheduler.SliceNsDefault, cfg.SimpleScheduler.SliceNsMin)
case "simple":
    plugin = simple.NewSimplePlugin(false)
case "simple-fifo":
    plugin = simple.NewSimplePlugin(true)
default:
    log.Fatalf("Unknown mode: %s", schedConfig.Mode)
}
```

### After (Using Factory Pattern)

```go
// In Gthulhu main.go
config := &plugin.SchedConfig{
    Mode:           schedConfig.Mode,
    SliceNsDefault: cfg.SimpleScheduler.SliceNsDefault,
    SliceNsMin:     cfg.SimpleScheduler.SliceNsMin,
}
config.Scheduler.SliceNsDefault = cfg.Scheduler.SliceNsDefault
config.Scheduler.SliceNsMin = cfg.Scheduler.SliceNsMin
config.APIConfig.PublicKeyPath = cfg.API.PublicKeyPath
config.APIConfig.BaseURL = cfg.API.BaseURL

plugin, err := plugin.NewSchedulerPlugin(config)
if err != nil {
    log.Fatalf("Failed to create plugin: %v", err)
}
```

## Testing

The factory pattern implementation includes comprehensive tests:

- **Unit Tests**: Test registration, factory creation, and error handling
- **Integration Tests**: Test actual plugin creation through the factory
- **Coverage**: 100% coverage for factory pattern code

Run tests:

```bash
# Test plugin package (factory pattern)
go test ./plugin -v -cover

# Test integration
go test ./tests -v

# Overall coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## Error Handling

The factory pattern provides clear error messages:

```go
// Empty mode
err := plugin.RegisterNewPlugin("", factory)
// Error: "plugin mode cannot be empty"

// Nil factory
err := plugin.RegisterNewPlugin("mode", nil)
// Error: "plugin factory cannot be nil"

// Duplicate registration
err := plugin.RegisterNewPlugin("existing", factory)
// Error: "plugin mode 'existing' is already registered"

// Unknown mode
scheduler, err := plugin.NewSchedulerPlugin(&plugin.SchedConfig{Mode: "unknown"})
// Error: "unknown plugin mode: unknown"

// Nil config
scheduler, err := plugin.NewSchedulerPlugin(nil)
// Error: "config cannot be nil"
```

## Best Practices

1. **Register in init()**: Always register plugins in the init() function to ensure they're available before use
2. **Validate Configuration**: Factory functions should validate configuration parameters
3. **Handle Errors**: Always check for errors when creating plugins
4. **Document Modes**: Document available modes and their configuration requirements
5. **Test Registration**: Include tests that verify plugin registration and creation

## Thread Safety

The plugin registry is protected by a `sync.RWMutex`:

- **Registration** (`RegisterNewPlugin`): Uses write lock
- **Factory Creation** (`NewSchedulerPlugin`): Uses read lock
- **Getting Modes** (`GetRegisteredModes`): Uses read lock

This ensures safe concurrent access to the plugin registry.
