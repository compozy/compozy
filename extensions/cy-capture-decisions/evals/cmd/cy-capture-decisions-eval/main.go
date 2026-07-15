package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/extensions/cy-capture-decisions/evals"
)

func main() {
	os.Exit(run())
}

func run() int {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repository root: %v\n", err)
		return 2
	}
	config := evals.DefaultConfig(repoRoot)
	var cases string
	flag.StringVar(&config.IDE, "ide", config.IDE, "ACP runtime")
	flag.StringVar(&config.Model, "model", os.Getenv("COMPOZY_EVAL_MODEL"), "required model name")
	flag.StringVar(&config.ReasoningEffort, "reasoning-effort", config.ReasoningEffort, "model reasoning effort")
	flag.IntVar(&config.Repetitions, "repetitions", config.Repetitions, "trials per case")
	flag.DurationVar(&config.Timeout, "timeout", config.Timeout, "timeout per model invocation")
	flag.StringVar(&config.ResultsDir, "results", config.ResultsDir, "artifact output directory")
	flag.StringVar(&cases, "cases", "", "comma-separated case IDs; empty runs the full matrix")
	flag.Parse()
	config.OnResult = printResult
	if strings.TrimSpace(cases) != "" {
		for _, id := range strings.Split(cases, ",") {
			config.CaseIDs = append(config.CaseIDs, strings.TrimSpace(id))
		}
	}

	ctx, stop := signalContext(context.Background())
	defer stop()
	_, runErr := evals.Run(ctx, config)
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "%v\n", runErr)
		return 1
	}
	return 0
}

func printResult(result evals.Result) {
	status := "PASS"
	if result.Skipped {
		status = "SKIP"
	} else if !result.Passed {
		status = "FAIL"
	}
	fmt.Printf("%s run-%d %s %s\n", result.CaseID, result.Trial, status, result.Duration)
}
