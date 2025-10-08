# Plugin

This repo collects all of scheduler implementations for Gthulhu project.

## Gthulhu Plugin Execution Flow

The following diagram shows the main execution flow after Gthulhu loads the plugin:

```mermaid
flowchart TD
    A[Start Scheduler Loop] --> B{Check context Done}
    B -->|Yes| C[Log Exit Message]
    C --> D[End]
    B -->|No| E[DrainQueuedTask - Drain queued tasks from eBPF]
    E --> F[SelectQueuedTask - Select queued task]
    F --> G{Is task nil?}
    G -->|Yes| H[BlockTilReadyForDequeue - Block until ready for dequeue]
    H --> B
    G -->|No| I{Is task PID not equal to -1?}
    I -->|No| B
    I -->|Yes| J[Create DispatchedTask]
    J --> K[Calculate waiting task count<br/>nrWaiting = GetNrQueued + GetNrScheduled + 1]
    K --> L[Set task Vtime]
    L --> M[DetermineTimeSlice - Determine time slice]
    M --> N{Has custom execution time?<br/>customTime greater than 0}
    N -->|Yes| O[Use custom time slice<br/>SliceNs = min customTime and duration]
    N -->|No| P[Use default algorithm<br/>SliceNs = max default slice divided by nrWaiting]
    O --> Q[SelectCPU - Select CPU]
    P --> Q
    Q --> R{SelectCPU successful?}
    R -->|No| S[Log SelectCPU error]
    S --> B
    R -->|Yes| T[Set task CPU]
    T --> U[DispatchTask - Dispatch task]
    U --> V{DispatchTask successful?}
    V -->|No| W[Log DispatchTask error]
    W --> B
    V -->|Yes| X[NotifyComplete - Notify completion<br/>Pass GetPoolCount result]
    X --> Y{NotifyComplete successful?}
    Y -->|No| Z[Log NotifyComplete error]
    Z --> B
    Y -->|Yes| B

    style A fill:#e1f5fe
    style D fill:#ffebee
    style J fill:#f3e5f5
    style U fill:#e8f5e8
```

## Plugin Interface Functions

The following diagram shows the relationship between the plugin interface functions defined in `plugin.go`:

```mermaid
flowchart TD
    subgraph "Sched Interface"
        S1[DequeueTask]
        S2[DefaultSelectCPU]
    end
    
    subgraph "CustomScheduler Interface"
        CS1[DrainQueuedTask]
        CS2[SelectQueuedTask]
        CS3[SelectCPU]
        CS4[DetermineTimeSlice]
        CS5[GetPoolCount]
    end
    
    subgraph "Main Scheduler Loop"
        ML1[Context Check]
        ML2[Task Processing]
        ML3[CPU Selection]
        ML4[Task Dispatch]
        ML5[Completion Notification]
    end
    
    ML1 --> CS1
    CS1 --> S1
    S1 --> CS2
    CS2 --> ML2
    ML2 --> CS4
    CS4 --> ML3
    ML3 --> CS3
    CS3 --> S2
    S2 --> ML4
    ML4 --> ML5
    ML5 --> CS5
    CS5 --> ML1
    
    style CS1 fill:#e3f2fd
    style CS2 fill:#e3f2fd
    style CS3 fill:#e3f2fd
    style CS4 fill:#e3f2fd
    style CS5 fill:#e3f2fd
    style S1 fill:#fff3e0
    style S2 fill:#fff3e0
```

### Flow Description

1. **Main Loop**: The program runs in an infinite loop until receiving a context cancellation signal
2. **Task Draining**: Uses `DrainQueuedTask()` to drain queued tasks from eBPF
3. **Task Selection**: Selects tasks to process through `SelectQueuedTask()`
4. **Blocking Wait**: If no tasks are available, blocks until tasks are ready for dequeue
5. **Time Slice Calculation**: 
   - First attempts to get custom time slice using `DetermineTimeSlice()`
   - If no custom time is available, uses default algorithm to calculate time slice
6. **CPU Selection**: Uses `SelectCPU()` to select appropriate CPU for the task
7. **Task Dispatch**: Dispatches tasks to selected CPU through `DispatchTask()`
8. **Completion Notification**: Uses `NotifyComplete()` to notify system of task completion status

### Interface Functions Description

#### Sched Interface
- **`DequeueTask(task *models.QueuedTask)`**: Called by `DrainQueuedTask` to retrieve pending tasks sent up from eBPF
- **`DefaultSelectCPU(t *models.QueuedTask) (error, int32)`**: Provides default CPU selection logic when custom selection is not available

#### CustomScheduler Interface
- **`DrainQueuedTask(s Sched) int`**: Drains queued tasks from eBPF by calling `s.DequeueTask()` to retrieve pending tasks and returns the number of tasks drained
- **`SelectQueuedTask(s Sched) *models.QueuedTask`**: Selects and returns a task from the queued tasks for processing
- **`SelectCPU(s Sched, t *models.QueuedTask) (error, int32)`**: Selects the most appropriate CPU for the given task
- **`DetermineTimeSlice(s Sched, t *models.QueuedTask) uint64`**: Calculates the time slice duration for task execution
- **`GetPoolCount() uint64`**: Returns the number of tasks waiting to be dispatched in the pool

The plugin architecture allows custom scheduler implementations to override default behavior while maintaining compatibility with the core Gthulhu scheduler framework.

