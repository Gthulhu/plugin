package gthulhu

import (
	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"
	"github.com/Gthulhu/plugin/plugin/util"
)

const (
	MAX_LATENCY_WEIGHT = 1000
	SCX_ENQ_WAKEUP     = 1
	NSEC_PER_SEC       = 1000000000 // 1 second in nanoseconds
	PF_WQ_WORKER       = 0x00000020
)

// Configurable scheduler parameters
var (
	SLICE_NS_DEFAULT uint64 = 5000 * 1000 // 5ms (default)
	SLICE_NS_MIN     uint64 = 500 * 1000  // 0.5ms (default)
)

const taskPoolSize = 4096

var taskPool = make([]Task, taskPoolSize)
var taskPoolCount = 0
var taskPoolHead, taskPoolTail int

func drainQueuedTask(s plugin.Sched) int {
	var count int
	for (taskPoolTail+1)%taskPoolSize != taskPoolHead {
		var newQueuedTask models.QueuedTask
		s.DequeueTask(&newQueuedTask)
		if newQueuedTask.Pid == -1 {
			return count
		}

		t := Task{
			QueuedTask: &newQueuedTask,
			Deadline:   updatedEnqueueTask(&newQueuedTask),
		}
		InsertTaskToPool(t)
		count++
	}
	return 0
}

func updatedEnqueueTask(t *models.QueuedTask) uint64 {
	// Check if we have a specific strategy for this task
	strategyApplied := ApplySchedulingStrategy(t)

	if !strategyApplied {
		// Default behavior if no specific strategy is found
		if minVruntime < t.Vtime {
			minVruntime = t.Vtime
		}
		minVruntimeLocal := util.SaturatingSub(minVruntime, SLICE_NS_DEFAULT)
		if t.Vtime == 0 {
			t.Vtime = minVruntimeLocal + (SLICE_NS_DEFAULT * 100 / t.Weight)
		} else if t.Vtime < minVruntimeLocal {
			t.Vtime = minVruntimeLocal
		}
		t.Vtime += (t.StopTs - t.StartTs) * t.Weight / 100
	}

	return 0
}

func GetPoolCount() int {
	return taskPoolCount
}

func GetTaskFromPool() *models.QueuedTask {
	if taskPoolHead == taskPoolTail {
		return nil
	}
	t := &taskPool[taskPoolHead]
	taskPoolHead = (taskPoolHead + 1) % taskPoolSize
	taskPoolCount--
	return t.QueuedTask
}

// SetSchedulerConfig updates the scheduler parameters from configuration
func SetSchedulerConfig(sliceNsDefault, sliceNsMin uint64) {
	if sliceNsDefault > 0 {
		SLICE_NS_DEFAULT = sliceNsDefault
	}
	if sliceNsMin > 0 {
		SLICE_NS_MIN = sliceNsMin
	}
}

// GetSchedulerConfig returns current scheduler configuration
func GetSchedulerConfig() (uint64, uint64) {
	return SLICE_NS_DEFAULT, SLICE_NS_MIN
}

var minVruntime uint64 = 0 // global vruntime

type Task struct {
	*models.QueuedTask
	Deadline  uint64
	Timestamp uint64
}

func LessQueuedTask(a, b *Task) bool {
	if a.Deadline != b.Deadline {
		return a.Deadline < b.Deadline
	}
	if a.Timestamp != b.Timestamp {
		return a.Timestamp < b.Timestamp
	}
	return a.QueuedTask.Pid < b.QueuedTask.Pid
}

func InsertTaskToPool(newTask Task) bool {
	if taskPoolCount >= taskPoolSize-1 {
		return false
	}
	insertIdx := taskPoolTail
	for i := 0; i < taskPoolCount; i++ {
		idx := (taskPoolHead + i) % taskPoolSize
		if LessQueuedTask(&newTask, &taskPool[idx]) {
			insertIdx = idx
			break
		}
	}

	cur := taskPoolTail
	for cur != insertIdx {
		next := (cur - 1 + taskPoolSize) % taskPoolSize
		taskPool[cur] = taskPool[next]
		cur = next
	}
	taskPool[insertIdx] = newTask
	taskPoolTail = (taskPoolTail + 1) % taskPoolSize
	taskPoolCount++
	return true
}
