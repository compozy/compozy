package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type exampleRunner func(context.Context) error

var examples = map[string]exampleRunner{
	"simple-workflow":       RunSimpleWorkflow,
	"parallel-tasks":        RunParallelTasks,
	"knowledge-rag":         RunKnowledgeRag,
	"memory-conversation":   RunMemoryConversation,
	"runtime-native-tools":  RunRuntimeNativeTools,
	"scheduled-workflow":    RunScheduledWorkflow,
	"signal-communication":  RunSignalCommunication,
	"model-routing":         RunModelRouting,
	"debugging-and-tracing": RunDebuggingAndTracing,
	"complete-project":      RunCompleteProject,
}

var descriptions = map[string]string{
	"simple-workflow":       "Basic agent workflow executed synchronously",
	"parallel-tasks":        "Parallel branches with aggregation",
	"knowledge-rag":         "Retrieval-augmented workflow with OpenAI embeddings",
	"memory-conversation":   "Conversation that persists state across turns",
	"runtime-native-tools":  "Hybrid native and inline tool execution",
	"scheduled-workflow":    "Deterministic scheduled workflow trigger",
	"signal-communication":  "Signal and wait coordination between tasks",
	"model-routing":         "Per-task model override and fallback",
	"debugging-and-tracing": "Runtime observability and tracing options",
	"complete-project":      "End-to-end project integrating multiple resources",
}

func main() {
	selected := flag.String("example", "", "Example to run")
	flag.Usage = showHelp
	flag.Parse()
	if strings.TrimSpace(*selected) == "" {
		showHelp()
		os.Exit(1)
	}
	exit := run(*selected)
	os.Exit(exit)
}

func run(name string) int {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		return 1
	}
	defer cleanup()
	if err := runExample(ctx, name); err != nil {
		logger.FromContext(ctx).Error("example failed", "example", name, "error", err)
		return 1
	}
	return 0
}

func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.Background())
	ctx := baseCtx
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", err)
	}
	ctx = config.ContextWithManager(ctx, manager)
	cfg := config.FromContext(ctx)
	level := logger.InfoLevel
	addSource := false
	if cfg != nil {
		if cfg.CLI.Quiet {
			level = logger.DisabledLevel
		} else if cfg.CLI.Debug {
			level = logger.DebugLevel
		}
		addSource = cfg.CLI.Debug
	}
	log := logger.SetupLogger(level, false, addSource)
	ctx = logger.ContextWithLogger(ctx, log)
	cleanup := func() {
		if err := manager.Close(ctx); err != nil {
			logger.FromContext(ctx).Warn("failed to close configuration manager", "error", err)
		}
		cancel()
	}
	return ctx, cleanup, nil
}

func runExample(ctx context.Context, name string) error {
	runner, ok := examples[name]
	if !ok {
		return fmt.Errorf("unknown example %q (use --help)", name)
	}
	log := logger.FromContext(ctx)
	log.Info("starting example", "example", name)
	if err := runner(ctx); err != nil {
		return err
	}
	log.Info("example finished", "example", name)
	return nil
}

func showHelp() {
	fmt.Println("Compozy SDK v2 Examples")
	fmt.Println("Usage: go run sdk/examples --example <name>")
	fmt.Println()
	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		desc := descriptions[name]
		fmt.Printf("  %-22s %s\n", name, desc)
	}
	fmt.Println()
	fmt.Println("Most examples require OPENAI_API_KEY and Bun when inline tools run.")
}
