package gthulhu

import (
	"github.com/Gthulhu/plugin/models"
)

const (
	MAX_LATENCY_WEIGHT = 1000
	SCX_ENQ_WAKEUP     = 1
	NSEC_PER_SEC       = 1000000000 // 1 second in nanoseconds
	PF_WQ_WORKER       = 0x00000020
)

const taskPoolSize = 4096

type Task struct {
	*models.QueuedTask
	Deadline  uint64
	Timestamp uint64
}
