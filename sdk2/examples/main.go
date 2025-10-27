package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func main() {
	exampleName := flag.String("example", "", "Example to run")
	flag.Usage = showHelp
	flag.Parse()
	if *exampleName == "" {
		showHelp()
		os.Exit(1)
	}
	os.Exit(run(*exampleName))
}

func run(exampleName string) int {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		return 1
	}
	defer cleanup()
	if err := runExample(ctx, exampleName); err != nil {
		logger.FromContext(ctx).Error("example failed", "example", exampleName, "error", err)
		return 1
	}
	return 0
}

func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.WithoutCancel(context.Background()))
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", err)
	}
	ctx = config.ContextWithManager(ctx, manager)
	cleanup := func() {
		if err := manager.Close(ctx); err != nil {
			logger.FromContext(ctx).Warn("failed to close configuration manager", "error", err)
		}
		cancel()
	}
	return ctx, cleanup, nil
}

func runExample(ctx context.Context, name string) error {
	examples := map[string]func(context.Context) error{
		"simple-workflow":      RunSimpleWorkflow,
		"parallel-tasks":       RunParallelTasks,
		"knowledge-rag":        RunKnowledgeRag,
		"memory-conversation":  RunMemoryConversation,
		"runtime-native-tools": RunRuntimeNativeTools,
		"scheduled-workflow":   RunScheduledWorkflow,
		"signal-communication": RunSignalCommunication,
		"complete-project":     RunCompleteProject,
		"debugging":            RunDebugging,
	}
	fn, exists := examples[name]
	if !exists {
		return fmt.Errorf("unknown example: %s (use --help to see available examples)", name)
	}
	return fn(ctx)
}

func showHelp() {
	fmt.Println("Compozy SDK v2 Examples")
	fmt.Println()
	fmt.Println("Usage: go run sdk2/examples --example <name>")
	fmt.Println()
	fmt.Println("Available examples:")
	fmt.Println("  simple-workflow        - Basic agent and workflow creation")
	fmt.Println("  parallel-tasks         - Parallel task execution")
	fmt.Println("  knowledge-rag          - Agent with knowledge base (RAG)")
	fmt.Println("  memory-conversation    - Agent with memory for conversations")
	fmt.Println("  runtime-native-tools   - Runtime with native tool definitions")
	fmt.Println("  scheduled-workflow     - Workflow with schedule configuration")
	fmt.Println("  signal-communication   - Signal tasks for workflow communication")
	fmt.Println("  complete-project       - Full project with multiple components")
	fmt.Println("  debugging              - Debugging configuration and patterns")
	fmt.Println()
	fmt.Println("Example: go run sdk2/examples --example simple-workflow")
	fmt.Println()
	fmt.Println("Note: Most examples require OPENAI_API_KEY environment variable")
}

// RunSimpleWorkflow demonstrates basic agent and workflow creation
func RunSimpleWorkflow(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running simple workflow example")
	fmt.Println("✅ Simple workflow example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Basic agent creation with agent.New()")
	fmt.Println("  - Workflow configuration with workflow.New()")
	fmt.Println("  - Project setup with project.New()")
	return nil
}

// RunParallelTasks demonstrates parallel task execution
func RunParallelTasks(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running parallel tasks example")
	fmt.Println("✅ Parallel tasks example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Multiple agents working in parallel")
	fmt.Println("  - Parallel task configuration with task.NewParallel()")
	fmt.Println("  - Result aggregation")
	return nil
}

// RunKnowledgeRag demonstrates agent with knowledge base
func RunKnowledgeRag(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running knowledge RAG example")
	fmt.Println("✅ Knowledge RAG example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Knowledge base configuration with knowledge.NewBase()")
	fmt.Println("  - Embedder setup with knowledge.NewEmbedder()")
	fmt.Println("  - Vector DB integration with knowledge.NewVectorDB()")
	fmt.Println("  - Agent with knowledge binding")
	return nil
}

// RunMemoryConversation demonstrates agent with memory
func RunMemoryConversation(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running memory conversation example")
	fmt.Println("✅ Memory conversation example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Memory configuration with memory.New()")
	fmt.Println("  - Conversation persistence")
	fmt.Println("  - Agent with memory")
	return nil
}

// RunRuntimeNativeTools demonstrates runtime with native tool definitions
func RunRuntimeNativeTools(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running runtime native tools example")
	fmt.Println("✅ Runtime native tools example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Runtime configuration with runtime.New()")
	fmt.Println("  - Native tool definitions with tool.New()")
	fmt.Println("  - Tool execution")
	return nil
}

// RunScheduledWorkflow demonstrates workflow with schedule configuration
func RunScheduledWorkflow(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running scheduled workflow example")
	fmt.Println("✅ Scheduled workflow example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Schedule configuration with schedule.New()")
	fmt.Println("  - Cron-based execution")
	fmt.Println("  - Workflow scheduling")
	return nil
}

// RunSignalCommunication demonstrates signal tasks for workflow communication
func RunSignalCommunication(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running signal communication example")
	fmt.Println("✅ Signal communication example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Signal tasks with task.NewSignal()")
	fmt.Println("  - Wait tasks with task.NewWait()")
	fmt.Println("  - Workflow communication patterns")
	return nil
}

// RunCompleteProject demonstrates full project with multiple components
func RunCompleteProject(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running complete project example")
	fmt.Println("✅ Complete project example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Full project with all components")
	fmt.Println("  - Multiple workflows")
	fmt.Println("  - Complex integrations")
	return nil
}

// RunDebugging demonstrates debugging configuration and patterns
func RunDebugging(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running debugging example")
	fmt.Println("✅ Debugging example (implementation in progress)")
	fmt.Println("This example will demonstrate:")
	fmt.Println("  - Debugging configuration")
	fmt.Println("  - Retry logic")
	fmt.Println("  - Timeout handling")
	fmt.Println("  - Error recovery patterns")
	return nil
}
