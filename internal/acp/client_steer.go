package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/config"
)

type SteerResult struct {
	Attempt    SteerAttempt
	Completion <-chan error
}

type SteerAttempt string

const (
	SteerAttemptInjected         SteerAttempt = "injected"
	SteerAttemptPendingInjection SteerAttempt = "pending_injection"
	SteerAttemptUnsupported      SteerAttempt = "unsupported"
)

var ErrSteerTurnMismatch = errors.New("acp: steer target is no longer the active turn")

type wireSteerResponse struct {
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}

func (d *Driver) Steer(ctx context.Context, proc *AgentProcess, turnID, text string) (SteerResult, error) {
	if ctx == nil {
		return SteerResult{Attempt: SteerAttemptUnsupported}, errors.New("acp: steer context is required")
	}
	if proc == nil || proc.conn == nil {
		return SteerResult{Attempt: SteerAttemptUnsupported}, errProcessConnectionUninitialized
	}
	if strings.TrimSpace(text) == "" {
		return SteerResult{Attempt: SteerAttemptUnsupported}, errors.New("acp: steer text is required")
	}
	active := proc.currentPrompt()
	if active == nil {
		return SteerResult{Attempt: SteerAttemptUnsupported}, ErrSteerTurnMismatch
	}
	active.steerMu.Lock()
	defer active.steerMu.Unlock()
	if active.finishing || proc.currentPrompt() != active || active.turnID != turnID {
		return SteerResult{Attempt: SteerAttemptUnsupported}, ErrSteerTurnMismatch
	}
	capability := proc.CapsSnapshot().SteerCapability
	if capability != config.SteerCapabilityExtension && capability != config.SteerCapabilityConcurrentPrompt {
		return SteerResult{Attempt: SteerAttemptUnsupported}, nil
	}
	request := acpsdk.PromptRequest{
		SessionId: acpsdk.SessionId(proc.SessionID),
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(text)},
	}
	if capability == config.SteerCapabilityConcurrentPrompt {
		return d.steerConcurrentPrompt(proc, active, request)
	}

	// Keep idle fallback host-owned so a provider cannot create an untracked detached turn.
	request.Meta = map[string]any{"steering": map[string]any{"idleBehavior": "promptRequired"}}
	response, err := acpsdk.SendRequest[wireSteerResponse](proc.conn, ctx, steerExtensionMethod, request)
	if err != nil {
		return SteerResult{Attempt: SteerAttemptUnsupported}, fmt.Errorf("acp: steer extension: %w", err)
	}
	switch response.Outcome {
	case "injected":
		return SteerResult{Attempt: SteerAttemptInjected}, nil
	case "pending_injection":
		return SteerResult{Attempt: SteerAttemptPendingInjection}, nil
	case "promptRequired":
		return SteerResult{Attempt: SteerAttemptUnsupported}, ErrSteerTurnMismatch
	case "failed":
		return SteerResult{Attempt: SteerAttemptUnsupported}, nil
	default:
		return SteerResult{Attempt: SteerAttemptUnsupported}, fmt.Errorf(
			"acp: unsupported steering outcome %q", response.Outcome,
		)
	}
}
