package plugin

import "github.com/Gthulhu/plugin/models"

type Sched interface {
	DequeueTask(task *models.QueuedTask)
}
