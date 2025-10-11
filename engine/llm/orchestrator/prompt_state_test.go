package orchestrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComposeSystemPromptAddsReactHeaderOnce(t *testing.T) {
	base := "system instructions"
	combined := composeSystemPrompt(base, "")
	count := strings.Count(combined, "<reasoning-protocol>")
	assert.Equal(t, 1, count, "expected react header once")
	combined = composeSystemPrompt(combined, "<dynamic>")
	count = strings.Count(combined, "<reasoning-protocol>")
	assert.Equal(t, 1, count, "expected react header to remain single")
	assert.Contains(t, combined, "<dynamic>", "expected dynamic fragment")
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
	assert.NotEmpty(t, fragment, "expected dynamic state fragment")
	assert.Contains(t, fragment, "iteration=2/5", "expected iteration info")
	assert.Contains(t, fragment, "tool_usage=search:1/2", "expected tool usage with cap")
	assert.Contains(t, fragment, "error_budgets=search:3", "expected error budget info")
}
