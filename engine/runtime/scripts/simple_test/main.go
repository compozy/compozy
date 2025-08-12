package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
)

func main() {
	// Initialize logger
	appLogger := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(context.Background(), appLogger)

	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Create runtime manager with configuration
	config := &runtime.Config{
		ToolExecutionTimeout: 30 * time.Second,
		RuntimeType:          runtime.RuntimeTypeBun,
		EntrypointPath:       "engine/runtime/scripts/simple_entrypoint.ts",
		BunPermissions: []string{
			"--allow-all",
		},
	}

	fmt.Println("Creating BunManager...")
	manager, err := runtime.NewBunManager(ctx, projectRoot, config)
	if err != nil {
		log.Fatalf("Failed to create runtime manager: %v", err)
	}

	// Create test input
	inputData := core.Input{
		"message": "Hello from Go test!",
	}
	input := &inputData

	// Create execution ID
	toolExecID, err := core.NewID()
	if err != nil {
		log.Fatalf("Failed to create execution ID: %v", err)
	}

	fmt.Printf("Input: %+v\n", input)
	fmt.Printf("Tool ID: simple_test\n")
	fmt.Printf("Exec ID: %s\n", toolExecID)
	fmt.Println("Executing tool through runtime...")

	// Execute the tool
	emptyConfig := core.Input{}
	toolConfig := &emptyConfig
	env := make(core.EnvMap)

	startTime := time.Now()
	output, err := manager.ExecuteTool(
		ctx,
		"simple_test", // Tool ID matches the exported function name
		toolExecID,
		input,
		toolConfig,
		env,
	)
	duration := time.Since(startTime)

	if err != nil {
		log.Fatalf("Tool execution failed after %v: %v", duration, err)
	}

	fmt.Printf("\n✅ Execution completed in %v\n", duration)

	// Display output
	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Printf("Warning: Could not marshal output: %v", err)
		fmt.Printf("Raw output: %+v\n", output)
	} else {
		fmt.Printf("Output:\n%s\n", string(outputJSON))
	}
}
