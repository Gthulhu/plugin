package main

import (
	"fmt"
	"log"

	"github.com/Gthulhu/plugin/plugin"
	// Import plugin packages to trigger init() registration
	_ "github.com/Gthulhu/plugin/plugin/gthulhu"
	_ "github.com/Gthulhu/plugin/plugin/simple"
)

// Example showing how to use this in Gthulhu main.go
func gthulhuMainExample() {
	// Create plugin configuration
	pluginConfig := &plugin.SchedConfig{
		Mode: "gthulhu",
		Scheduler: plugin.Scheduler{
			SliceNsDefault: 5000 * 1000,
			SliceNsMin:     500 * 1000,
		},
	}
	pluginConfig.Scheduler.SliceNsDefault = 5000 * 1000
	pluginConfig.Scheduler.SliceNsMin = 500 * 1000

	// Create the plugin using the factory
	scheduler, err := plugin.NewSchedulerPlugin(pluginConfig)
	if err != nil {
		log.Fatalf("Failed to create scheduler plugin: %v", err)
	}

	fmt.Printf("Created plugin successfully, pool count: %d\n", scheduler.GetPoolCount())
}

func main() {
	fmt.Println("=== Gthulhu Plugin Factory Example ===")

	// List available plugins
	modes := plugin.GetRegisteredModes()
	fmt.Printf("Available plugin modes: %v\n\n", modes)

	// Create plugin instance
	gthulhuMainExample()
}
