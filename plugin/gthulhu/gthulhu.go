package gthulhu

import (
	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"
)

type GthulhuPlugin struct{}

func NewGthulhuPlugin(sliceNsDefault, sliceNsMin uint64) *GthulhuPlugin {
	SetSchedulerConfig(sliceNsDefault, sliceNsMin)
	return &GthulhuPlugin{}
}

var _ plugin.CustomScheduler = (*GthulhuPlugin)(nil)

func (g *GthulhuPlugin) DrainQueuedTask(s plugin.Sched) int {
	return drainQueuedTask(s)
}

func (g *GthulhuPlugin) SelectQueuedTask(s plugin.Sched) *models.QueuedTask {
	return GetTaskFromPool()
}

func (g *GthulhuPlugin) SelectCPU(s plugin.Sched, t *models.QueuedTask) (error, int32) {
	return s.SelectCPU(t)
}

func (g *GthulhuPlugin) DetermineTimeSlice(s plugin.Sched, t *models.QueuedTask) uint64 {
	return GetTaskExecutionTime(t.Pid)
}

func (g *GthulhuPlugin) GetPoolCount() uint64 {
	return uint64(GetPoolCount())
}
