# GthulhuPlugin Singleton Pattern Refactoring

## Summary of Changes

The GthulhuPlugin has been refactored to follow the singleton pattern, where all global variables and package-level functions have been embedded into the `GthulhuPlugin` struct. This allows developers to create independent instances of the plugin with isolated state.

## What Changed

### Before (Global State)
Previously, the plugin used global variables that were shared across all code:
- `SLICE_NS_DEFAULT` and `SLICE_NS_MIN` - scheduler configuration
- `taskPool`, `taskPoolCount`, `taskPoolHead`, `taskPoolTail` - task pool state
- `minVruntime` - global vruntime tracking
- `strategyMap` - PID-based scheduling strategies
- `jwtClient` - JWT client for API authentication

Package-level functions accessed these global variables directly.

### After (Instance-Based State)
Now, all state is encapsulated within the `GthulhuPlugin` struct:
```go
type GthulhuPlugin struct {
    // Scheduler configuration
    sliceNsDefault uint64
    sliceNsMin     uint64

    // Task pool state
    taskPool      []Task
    taskPoolCount int
    taskPoolHead  int
    taskPoolTail  int

    // Global vruntime
    minVruntime uint64

    // Strategy map for PID-based scheduling strategies
    strategyMap map[int32]SchedulingStrategy

    // JWT client for API authentication
    jwtClient *JWTClient

    // Metrics client for sending metrics to API server
    metricsClient *MetricsClient
}
```

## Usage Examples

### Creating a Plugin Instance

```go
// Create a new plugin instance with custom configuration
plugin := gthulhu.NewGthulhuPlugin(10000*1000, 1000*1000) // 10ms default, 1ms min

// Or use default configuration (5ms default, 0.5ms min)
plugin := gthulhu.NewGthulhuPlugin(0, 0)
```

### Multiple Independent Instances

```go
// Create two independent plugin instances
plugin1 := gthulhu.NewGthulhuPlugin(5000*1000, 500*1000)
plugin2 := gthulhu.NewGthulhuPlugin(10000*1000, 1000*1000)

// Each instance maintains its own state
// Changes to plugin1 do not affect plugin2
```

### Initializing JWT Client

```go
plugin := gthulhu.NewGthulhuPlugin(0, 0)

// Initialize JWT client for API authentication
err := plugin.InitJWTClient("/path/to/public.key", "https://api.example.com")
if err != nil {
    log.Fatal(err)
}

// Initialize metrics client (requires JWT client to be initialized first)
err = plugin.InitMetricsClient("https://api.example.com")
if err != nil {
    log.Fatal(err)
}
```

### Managing Scheduling Strategies

```go
plugin := gthulhu.NewGthulhuPlugin(0, 0)

// Manually update strategy map
strategies := []gthulhu.SchedulingStrategy{
    {PID: 100, Priority: true, ExecutionTime: 10000},
    {PID: 200, Priority: false, ExecutionTime: 20000},
}
plugin.UpdateStrategyMap(strategies)

// Or start automatic strategy fetching (requires JWT client)
ctx := context.Background()
plugin.StartStrategyFetcher(ctx, "https://api.example.com/strategies", 30*time.Second)
```

### Using the Plugin with Scheduler

```go
// Create plugin instance
plugin := gthulhu.NewGthulhuPlugin(0, 0)

// The plugin implements the plugin.CustomScheduler interface
// and can be loaded into the Gthulhu scheduler
scheduler.LoadPlugin(plugin)
```

## Benefits

1. **Instance Isolation**: Each plugin instance maintains its own state, allowing multiple independent schedulers
2. **No Global State**: Eliminates global variable pollution and race conditions
3. **Testability**: Easier to test since each test can create isolated instances
4. **Thread Safety**: Each instance's state is independent, reducing contention
5. **Flexibility**: Developers can create multiple plugin instances with different configurations

## API Changes

### Methods Added to GthulhuPlugin

- `GetSchedulerConfig() (uint64, uint64)` - Get current scheduler configuration
- `SetSchedulerConfig(sliceNsDefault, sliceNsMin uint64)` - Update scheduler configuration
- `InitJWTClient(publicKeyPath, apiBaseURL string) error` - Initialize JWT client
- `GetJWTClient() *JWTClient` - Get JWT client instance
- `InitMetricsClient(apiBaseURL string) error` - Initialize metrics client
- `GetMetricsClient() *MetricsClient` - Get metrics client instance
- `FetchSchedulingStrategies(apiUrl string) ([]SchedulingStrategy, error)` - Fetch strategies
- `UpdateStrategyMap(strategies []SchedulingStrategy)` - Update strategy map
- `StartStrategyFetcher(ctx context.Context, apiUrl string, interval time.Duration)` - Start periodic fetching

### Removed Global Functions

The following package-level functions have been removed:
- `SetSchedulerConfig()` - Use `plugin.SetSchedulerConfig()` instead
- `GetSchedulerConfig()` - Use `plugin.GetSchedulerConfig()` instead
- `InitJWTClient()` - Use `plugin.InitJWTClient()` instead
- `GetJWTClient()` - Use `plugin.GetJWTClient()` instead
- `FetchSchedulingStrategies()` - Use `plugin.FetchSchedulingStrategies()` instead
- `UpdateStrategyMap()` - Use `plugin.UpdateStrategyMap()` instead
- `StartStrategyFetcher()` - Use `plugin.StartStrategyFetcher()` instead
- `GetTaskExecutionTime()` - Internal method, not exposed
- `ApplySchedulingStrategy()` - Internal method, not exposed
- `GetPoolCount()` - Use `plugin.GetPoolCount()` instead
- `GetTaskFromPool()` - Internal method, not exposed

## Migration Guide

If you were previously using the global functions, migrate as follows:

```go
// Before
InitJWTClient("/path/to/key", "https://api.example.com")
StartStrategyFetcher(ctx, apiUrl, interval)

// After
plugin := NewGthulhuPlugin(0, 0)
plugin.InitJWTClient("/path/to/key", "https://api.example.com")
plugin.StartStrategyFetcher(ctx, apiUrl, interval)
```

## Testing

The refactoring includes comprehensive tests in `gthulhu_test.go`:
- Instance isolation verification
- Default configuration testing
- Task pool initialization
- Strategy map management
- Configuration updates

Run tests with:
```bash
go test ./plugin/gthulhu/...
```
