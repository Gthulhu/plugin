package util

import "time"

// SchedulingStrategy represents a strategy for process scheduling
type SchedulingStrategy struct {
	Priority      bool   `json:"priority"`       // If true, set vtime to minimum vtime
	ExecutionTime uint64 `json:"execution_time"` // Time slice for this process in nanoseconds
	PID           int    `json:"pid"`            // Process ID to apply this strategy to
}

// SchedulingStrategiesResponse represents the response structure from the API
type SchedulingStrategiesResponse struct {
	Success    bool                 `json:"success"`
	Message    string               `json:"message"`
	Timestamp  string               `json:"timestamp"`
	Scheduling []SchedulingStrategy `json:"scheduling"`
}

func Now() uint64 {
	return uint64(time.Now().UnixNano())
}

func CalcAvg(oldVal uint64, newVal uint64) uint64 {
	return (oldVal - (oldVal >> 2)) + (newVal >> 2)
}

func SaturatingSub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}
