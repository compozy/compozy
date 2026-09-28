package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

const handoffGuidanceFormat = "continue this session with another agent or route: " +
	"compozy session continue %s --agent <name>"

// decorateHandoffAction prescribes handoff on a user session's post-acceptance prompt
// failure classified rate-limited or unauthenticated. The decision uses the typed session
// type only; other session types keep the classifier's retry/login action. The daemon
// never continues a session on its own.
func decorateHandoffAction(sessionType Type, sessionID string, event *acp.AgentEvent) {
	if event == nil || event.ProviderError == nil || normalizeSessionType(sessionType) != SessionTypeUser {
		return
	}
	switch event.ProviderError.Code {
	case acp.ProviderErrorRateLimited, acp.ProviderErrorAuthRequired:
	default:
		return
	}
	decorated := acp.CloneProviderErrorDiagnostic(event.ProviderError)
	decorated.NextAction = acp.ProviderFailureActionHandoff
	decorated.Guidance = fmt.Sprintf(handoffGuidanceFormat, strings.TrimSpace(sessionID))
	event.ProviderError = decorated
	if event.Failure == nil {
		return
	}
	diagnostic, ok := acp.ProviderFailureDiagnosticFromSummary(event.Failure.Summary)
	if !ok {
		return
	}
	diagnostic.Action = decorated.NextAction
	diagnostic.Guidance = decorated.Guidance
	failure := *event.Failure
	failure.Summary = acp.ReplaceProviderFailureDiagnostic(failure.Summary, diagnostic)
	event.Failure = &failure
}
