package acp

import (
	"context"
	"log/slog"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	// agyInteractionToolCallPrefix marks an Antigravity session/request_permission call that
	// carries a fixed-choice question: every option is an answer reported as allow_once.
	agyInteractionToolCallPrefix = "interaction_"
	agyInteractionFallbackPrompt = "Antigravity needs an answer to continue."
)

// isAgyInteractionQuestion reports an Antigravity question delivered as a permission
// request. Its options are answers, so no permission policy may choose one.
func (p *AgentProcess) isAgyInteractionQuestion(request acpsdk.RequestPermissionRequest) bool {
	if compozyconfig.CanonicalProviderName(p.permissionModeProvider()) != providerAgy {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(request.ToolCall.ToolCallId)), agyInteractionToolCallPrefix)
}

// answerAgyInteractionQuestion asks the operator to choose one of the provider's answers.
// An unavailable operator, an expired wait, or an unrepresentable question cancels the
// request instead of selecting an answer on the operator's behalf.
func (p *AgentProcess) answerAgyInteractionQuestion(
	ctx context.Context,
	request acpsdk.RequestPermissionRequest,
) acpsdk.RequestPermissionResponse {
	cancelled := acpsdk.RequestPermissionResponse{Outcome: acpsdk.NewRequestPermissionOutcomeCancelled()}
	question := agyInteractionClarifyQuestion(request)
	if p.elicitationAsker == nil || len(question.Choices) == 0 {
		p.logAgyInteraction(ctx, "unavailable", nil)
		return cancelled
	}
	askCtx := ctx
	if wait := p.permissionTimeoutOrDefault(); wait > 0 {
		var cancel context.CancelFunc
		askCtx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	answer, err := p.elicitationAsker(askCtx, question)
	if err != nil {
		p.logAgyInteraction(ctx, "unanswered", err)
		return cancelled
	}
	if answer.Fallback || answer.Choice == nil || *answer.Choice < 0 || *answer.Choice >= len(request.Options) {
		p.logAgyInteraction(ctx, "unanswered", nil)
		return cancelled
	}
	return acpsdk.RequestPermissionResponse{
		Outcome: acpsdk.NewRequestPermissionOutcomeSelected(request.Options[*answer.Choice].OptionId),
	}
}

// agyInteractionClarifyQuestion renders the request as a choice-only question whose
// choice indexes match the provider's option order.
func agyInteractionClarifyQuestion(request acpsdk.RequestPermissionRequest) toolspkg.ClarifyQuestion {
	prompt := agyInteractionFallbackPrompt
	if request.ToolCall.Title != nil && strings.TrimSpace(*request.ToolCall.Title) != "" {
		prompt = strings.TrimSpace(*request.ToolCall.Title)
	}
	choices := make([]string, 0, len(request.Options))
	for _, option := range request.Options {
		name := strings.TrimSpace(option.Name)
		if name == "" {
			name = strings.TrimSpace(string(option.OptionId))
		}
		choices = append(choices, name)
	}
	return toolspkg.ClarifyQuestion{Question: prompt, Choices: choices, ChoicesOnly: true}
}

func (p *AgentProcess) logAgyInteraction(ctx context.Context, outcome string, err error) {
	attrs := []any{"provider", p.permissionModeProvider(), "outcome", outcome}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	slog.InfoContext(ctx, "permission.agy_interaction_cancelled", attrs...)
}
