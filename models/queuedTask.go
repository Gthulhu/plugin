package models

// Task queued for scheduling from the BPF component (see bpf_intf::queued_task_ctx).
type QueuedTask struct {
	Pid            int32  // pid that uniquely identifies a task
	Cpu            int32  // CPU where the task is running
	NrCpusAllowed  uint64 // Number of CPUs that the task can use
	Flags          uint64 // task enqueue flags
	StartTs        uint64 // Timestamp since last time the task ran on a CPU
	StopTs         uint64 // Timestamp since last time the task released a CPU
	SumExecRuntime uint64 // Total cpu time
	Weight         uint64 // Task static priority
	Vtime          uint64 // Current vruntime
	Tgid           int32  // Task group id
}
