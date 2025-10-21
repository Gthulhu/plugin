package gthulhu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
)

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

const SCX_ENQ_PREEMPT = 1 << 32

// fetchSchedulingStrategies fetches scheduling strategies from the API server with JWT authentication
func fetchSchedulingStrategies(jwtClient *JWTClient, apiUrl string) ([]SchedulingStrategy, error) {
	if jwtClient == nil {
		return nil, fmt.Errorf("JWT client not initialized")
	}
	resp, err := jwtClient.MakeAuthenticatedRequest("GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			fmt.Printf("Body.Close() failed: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response SchedulingStrategiesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// Only update if successful
	if response.Success {
		return response.Scheduling, nil
	}

	return nil, nil
}

// StartStrategyFetcher starts a background goroutine to periodically fetch scheduling strategies
func (g *GthulhuPlugin) StartStrategyFetcher(ctx context.Context, apiUrl string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		// Fetch immediately on start
		if strategies, err := g.FetchSchedulingStrategies(apiUrl + "/api/v1/scheduling/strategies"); err == nil && strategies != nil {
			log.Printf("Initial scheduling strategies fetched: %d strategies", len(strategies))
			g.UpdateStrategyMap(strategies)
		} else if err != nil {
			log.Printf("Failed to fetch initial scheduling strategies: %v", err)
		}

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if strategies, err := g.FetchSchedulingStrategies(apiUrl + "/api/v1/scheduling/strategies"); err == nil && strategies != nil {
					log.Printf("Scheduling strategies updated: %d strategies", len(strategies))
					g.UpdateStrategyMap(strategies)
				} else if err != nil {
					log.Printf("Failed to fetch scheduling strategies: %v", err)
				}
			}
		}
	}()
}
