package plugin

import "github.com/Gthulhu/plugin/models"

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
