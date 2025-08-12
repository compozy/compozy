package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/joho/godotenv"
)

func setupEnvironment() (context.Context, string) {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Initialize logger
	appLogger := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(context.Background(), appLogger)

	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	return ctx, projectRoot
}

func createManager(ctx context.Context, projectRoot string) *runtime.BunManager {
	config := &runtime.Config{
		ToolExecutionTimeout: 5 * time.Minute,
		RuntimeType:          runtime.RuntimeTypeBun,
		EntrypointPath:       "engine/runtime/scripts/test_entrypoint.ts",
		BunPermissions: []string{
			"--allow-all",
		},
	}

	manager, err := runtime.NewBunManager(ctx, projectRoot, config)
	if err != nil {
		log.Fatalf("Failed to create runtime manager: %v", err)
	}
	return manager
}

func executeTest(ctx context.Context, manager *runtime.BunManager) (*core.Output, time.Duration, error) {
	scriptPath, err := filepath.Abs("engine/runtime/scripts/test_claude_code.ts")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get script path: %w", err)
	}

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, 0, fmt.Errorf("script not found: %s", scriptPath)
	}

	fmt.Printf("Using script: %s\n", scriptPath)

	inputData := core.Input{
		"prompt": "Just say 'Hello from Claude Code!'", // Simpler prompt for testing
	}
	input := &inputData

	toolExecID, err := core.NewID()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create execution ID: %w", err)
	}

	fmt.Printf("Input: %+v\n", input)
	fmt.Println("Executing script through runtime...")

	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	emptyConfig := core.Input{}
	toolConfig := &emptyConfig
	env := make(core.EnvMap)

	startTime := time.Now()
	output, err := manager.ExecuteToolWithTimeout(
		execCtx,
		"test_claude_code", // Use a simple tool ID, not the full path
		toolExecID,
		input,
		toolConfig,
		env,
		5*time.Minute,
	)
	duration := time.Since(startTime)

	if err != nil {
		return nil, duration, fmt.Errorf("script execution failed after %v: %w", duration, err)
	}

	return output, duration, nil
}

func displayResults(output *core.Output, duration time.Duration) {
	fmt.Printf("\nExecution completed in %v\n", duration)

	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Printf("Warning: Could not marshal output: %v", err)
		fmt.Printf("Raw output: %+v\n", output)
	} else {
		fmt.Printf("Output:\n%s\n", string(outputJSON))
	}

	if successVal, ok := (*output)["success"]; ok {
		if success, ok := successVal.(bool); ok && success {
			fmt.Println("\n✅ Test PASSED - Tool executed successfully")
		} else {
			fmt.Println("\n❌ Test FAILED - Tool execution failed")
			if errorVal, ok := (*output)["error"]; ok {
				fmt.Printf("Error: %v\n", errorVal)
			}
		}
	} else {
		fmt.Println("\n⚠️  Output does not contain 'success' field")
		fmt.Printf("Output keys: ")
		for key := range *output {
			fmt.Printf("%s ", key)
		}
		fmt.Println()
	}
}

func main() {
	ctx, projectRoot := setupEnvironment()
	manager := createManager(ctx, projectRoot)
	output, duration, err := executeTest(ctx, manager)
	if err != nil {
		log.Fatalf("Test failed: %v", err)
	}
	displayResults(output, duration)
}
