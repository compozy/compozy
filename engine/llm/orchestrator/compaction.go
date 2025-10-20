package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/pkg/logger"
)

var (
	ErrCompactionUnavailable = errors.New("memory compaction unavailable")
	ErrCompactionIncomplete  = errors.New("memory compaction incomplete")
	ErrCompactionSkipped     = errors.New("memory compaction skipped")
)

const (
	compactionSummaryPrefix   = "[memory-compaction]"
	maxCompactionSnippetRunes = 240
	maxToolResultSummaries    = 3
)

type compactionPlan struct {
	baseCount         int
	messages          []llmadapter.Message
	priorSummaryLines []string
}

func (m *memoryManager) Compact(ctx context.Context, loopCtx *LoopContext, usage telemetry.ContextUsage) error {
	plan, err := buildCompactionPlan(loopCtx)
	if err != nil {
		return err
	}
	summary := renderCompactionSummary(loopCtx.Iteration, usage.PercentOfLimit, plan)
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("%w: empty summary", ErrCompactionIncomplete)
	}
	applyCompactionSummary(loopCtx.LLMRequest, plan.baseCount, summary)
	if err := m.persistCompactionSummary(ctx, loopCtx.State.memories(), summary); err != nil {
		return fmt.Errorf("%w: %w", ErrCompactionIncomplete, err)
	}
	logger.FromContext(ctx).Info(
		"Memory compaction applied",
		"iteration", loopCtx.Iteration,
		"compacted_messages", len(plan.messages),
	)
	recordCompactionEvent(ctx, loopCtx.Iteration, usage.PercentOfLimit, len(plan.messages), summary)
	return nil
}

func buildCompactionPlan(loopCtx *LoopContext) (compactionPlan, error) {
	if loopCtx == nil || loopCtx.LLMRequest == nil || loopCtx.State == nil {
		return compactionPlan{}, ErrCompactionUnavailable
	}
	messages := loopCtx.LLMRequest.Messages
	base := min(max(loopCtx.baseMessageCount, 0), len(messages))
	target, err := llmadapter.CloneMessages(messages[base:])
	if err != nil {
		return compactionPlan{}, fmt.Errorf("clone messages for compaction: %w", err)
	}
	if len(target) == 0 {
		return compactionPlan{}, ErrCompactionSkipped
	}
	priorSummaries := make([]string, 0, len(target))
	filtered := make([]llmadapter.Message, 0, len(target))
	for i := range target {
		msg := target[i]
		if isCompactionSummaryMessage(&msg) {
			priorSummaries = append(priorSummaries, extractSummaryLines(msg.Content)...)
			continue
		}
		filtered = append(filtered, msg)
	}
	if len(filtered) == 0 {
		return compactionPlan{}, ErrCompactionSkipped
	}
	return compactionPlan{baseCount: base, messages: filtered, priorSummaryLines: priorSummaries}, nil
}

func renderCompactionSummary(iteration int, usage float64, plan compactionPlan) string {
	lines := make([]string, 0, len(plan.priorSummaryLines)+len(plan.messages))
	lines = append(lines, plan.priorSummaryLines...)
	for i := range plan.messages {
		line := summariseCompactionLine(&plan.messages[i])
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	header := fmt.Sprintf("%s iteration %d (ctx %.1f%%)", compactionSummaryPrefix, iteration, usage*100)
	return header + "\n\n" + strings.Join(lines, "\n")
}

func summariseCompactionLine(msg *llmadapter.Message) string {
	if msg == nil {
		return ""
	}
	fragments := collectCompactionFragments(msg)
	if len(fragments) == 0 {
		return ""
	}
	return fmt.Sprintf("- %s: %s", compactionRoleLabel(msg.Role), strings.Join(fragments, " | "))
}

func collectCompactionFragments(msg *llmadapter.Message) []string {
	if msg == nil {
		return nil
	}
	fragments := make([]string, 0, 1+len(msg.ToolResults))
	if text := strings.TrimSpace(msg.Content); text != "" {
		fragments = append(fragments, truncateCompactionText(text))
	}
	if text := extractPartText(msg); text != "" {
		fragments = append(fragments, truncateCompactionText(text))
	}
	for idx, result := range msg.ToolResults {
		if idx >= maxToolResultSummaries {
			fragments = append(fragments, "...")
			break
		}
		fragments = append(fragments, summariseToolResult(result))
	}
	return fragments
}

func extractPartText(msg *llmadapter.Message) string {
	if msg == nil || len(msg.Parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range msg.Parts {
		textPart, ok := part.(llmadapter.TextPart)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(textPart.Text)
		if trimmed == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(trimmed)
	}
	return builder.String()
}

func summariseToolResult(result llmadapter.ToolResult) string {
	name := strings.TrimSpace(result.Name)
	if name == "" {
		name = "tool"
	}
	if len(result.JSONContent) > 0 {
		return fmt.Sprintf("%s(json): %s", name, truncateCompactionText(compactJSON(result.JSONContent)))
	}
	if strings.TrimSpace(result.Content) == "" {
		return name
	}
	return fmt.Sprintf("%s: %s", name, truncateCompactionText(result.Content))
}

func compactJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func truncateCompactionText(text string) string {
	cleaned := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(cleaned) <= maxCompactionSnippetRunes {
		return cleaned
	}
	var builder strings.Builder
	count := 0
	for _, r := range cleaned {
		if count >= maxCompactionSnippetRunes {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return builder.String() + "..."
}

func compactionRoleLabel(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		return "Message"
	}
	lower := strings.ToLower(role)
	switch lower {
	case llmadapter.RoleUser:
		return "User"
	case llmadapter.RoleAssistant:
		return "Assistant"
	case llmadapter.RoleSystem:
		return "System"
	case llmadapter.RoleTool:
		return "Tool"
	default:
		return strings.ToUpper(role[:1]) + strings.ToLower(role[1:])
	}
}

func applyCompactionSummary(req *llmadapter.LLMRequest, base int, summary string) {
	if req == nil {
		return
	}
	compacted := make([]llmadapter.Message, base, base+1)
	copy(compacted, req.Messages[:base])
	compacted = append(compacted, llmadapter.Message{Role: llmadapter.RoleSystem, Content: summary})
	req.Messages = compacted
}

func (m *memoryManager) persistCompactionSummary(ctx context.Context, ctxData *MemoryContext, summary string) error {
	if ctxData == nil || len(ctxData.memories) == 0 || strings.TrimSpace(summary) == "" {
		return nil
	}
	storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryStoreTimeout)
	defer cancel()
	memoryIDs := m.extractMemoryIDs(ctxData)
	messages := []contracts.Message{{
		Role:    contracts.MessageRoleAssistant,
		Content: summary,
	}}
	return m.executeEpisodeStoreWithLocks(storeCtx, ctxData, messages, memoryIDs)
}

func recordCompactionEvent(ctx context.Context, iteration int, usage float64, compacted int, summary string) {
	payload := map[string]any{
		"compacted_messages": compacted,
		"summary_length":     len(summary),
	}
	if telemetry.CaptureContentEnabled(ctx) {
		payload["summary"] = summary
	}
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:     "memory_compaction",
		Severity:  telemetry.SeverityInfo,
		Iteration: iteration,
		Metadata: map[string]any{
			"usage_pct": usage,
		},
		Payload: payload,
	})
}

func isCompactionSummaryMessage(msg *llmadapter.Message) bool {
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(msg.Content), compactionSummaryPrefix)
}

func extractSummaryLines(content string) []string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, compactionSummaryPrefix) {
			continue
		}
		out = append(out, line)
	}
	return out
}
