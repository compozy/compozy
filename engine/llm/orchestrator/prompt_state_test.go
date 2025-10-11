package orchestrator

import (
	"strings"
	"testing"
)

func TestComposeSystemPromptAddsReactHeaderOnce(t *testing.T) {
	base := "system instructions"
	combined := composeSystemPrompt(base, "")
	if count := strings.Count(combined, "<reasoning-protocol>"); count != 1 {
		t.Fatalf("expected react header once, got %d occurrences", count)
	}

	combined = composeSystemPrompt(combined, "<dynamic>")
	if count := strings.Count(combined, "<reasoning-protocol>"); count != 1 {
		t.Fatalf("expected react header to remain single, got %d occurrences", count)
	}
	if !strings.Contains(combined, "<dynamic>") {
		t.Fatalf("expected dynamic fragment to be included")
	}
}

func TestRenderDynamicStateIncludesUsageAndBudgets(t *testing.T) {
	cfg := &settings{
		maxSequentialToolErrors:  4,
		finalizeOutputRetries:    1,
		enableDynamicPromptState: true,
		toolCaps:                 toolCallCaps{defaultLimit: 2},
	}
	state := newLoopState(cfg, nil, nil)
	state.Budgets.ToolUsage["search"] = 1
	state.Budgets.ToolErrors["search"] = 1

	loopCtx := &LoopContext{Iteration: 1, MaxIterations: 5, State: state}
	fragment := renderDynamicState(loopCtx, cfg)
	if fragment == "" {
		t.Fatalf("expected dynamic state fragment")
	}
	if !strings.Contains(fragment, "iteration=2/5") {
		t.Fatalf("expected iteration info, got %s", fragment)
	}
	if !strings.Contains(fragment, "tool_usage=search:1/2") {
		t.Fatalf("expected tool usage with cap, got %s", fragment)
	}
	if !strings.Contains(fragment, "error_budgets=search:3") {
		t.Fatalf("expected error budget info, got %s", fragment)
	}
}
