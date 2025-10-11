package orchestrator

import (
	"fmt"
	"sort"
	"strings"
)

const reactProtocolHeader = `<reasoning-protocol>
You must operate in a THINK → ACT → OBSERVE loop on every turn.
- THINK: reflect privately on the goal and available tools before acting.
- ACT: use tools by emitting {"tool":"name","arguments":{…}} only.
- OBSERVE: read results, update your plan, and continue or finalize.
Do not expose THINK deliberations or fabricate tool results.
When ready to finalize, respond with the required structured output only.
</reasoning-protocol>`

func composeSystemPrompt(base, dynamic string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = reactProtocolHeader
	} else if !strings.Contains(base, "<reasoning-protocol>") {
		base = base + "\n\n" + reactProtocolHeader
	}
	dynamic = strings.TrimSpace(dynamic)
	if dynamic == "" {
		return base
	}
	return base + "\n\n" + dynamic
}

func renderDynamicState(loopCtx *LoopContext, cfg *settings) string {
	if cfg == nil || !cfg.enableDynamicPromptState || loopCtx == nil || loopCtx.State == nil {
		return ""
	}
	state := loopCtx.State
	currentIter := loopCtx.Iteration + 1
	if currentIter > loopCtx.MaxIterations && loopCtx.MaxIterations > 0 {
		currentIter = loopCtx.MaxIterations
	}
	lines := []string{
		fmt.Sprintf("iteration=%d/%d", currentIter, loopCtx.MaxIterations),
	}
	if usage := formatToolUsage(state, cfg.toolCaps); usage != "" {
		lines = append(lines, "tool_usage="+usage)
	}
	if budget := formatErrorBudgets(state); budget != "" {
		lines = append(lines, "error_budgets="+budget)
	}
	if state.Progress.NoProgressCount > 0 {
		lines = append(lines, fmt.Sprintf("no_progress=%d", state.Progress.NoProgressCount))
	}
	if len(lines) == 0 {
		return ""
	}
	return "<dynamic-state>\n" + strings.Join(lines, "\n") + "\n</dynamic-state>"
}

func formatToolUsage(state *loopState, caps toolCallCaps) string {
	if state == nil || len(state.Budgets.ToolUsage) == 0 {
		return ""
	}
	names := make([]string, 0, len(state.Budgets.ToolUsage))
	for name := range state.Budgets.ToolUsage {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		count := state.Budgets.ToolUsage[name]
		limit := caps.limitFor(name)
		if limit > 0 {
			entries = append(entries, fmt.Sprintf("%s:%d/%d", name, count, limit))
			continue
		}
		entries = append(entries, fmt.Sprintf("%s:%d", name, count))
	}
	return strings.Join(entries, "; ")
}

func formatErrorBudgets(state *loopState) string {
	if state == nil || len(state.Budgets.ToolErrors) == 0 {
		return ""
	}
	names := make([]string, 0, len(state.Budgets.ToolErrors))
	for name := range state.Budgets.ToolErrors {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		remaining := state.budgetFor(name) - state.Budgets.ToolErrors[name]
		if remaining < 0 {
			remaining = 0
		}
		entries = append(entries, fmt.Sprintf("%s:%d", name, remaining))
	}
	return strings.Join(entries, "; ")
}
