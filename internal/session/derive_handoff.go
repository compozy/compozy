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
	// The error text carries the same recovery metadata as the failure summary; it is what
	// the prompt stream shows the operator, so both must prescribe the handoff.
	event.Error = withHandoffMetadata(event.Error, decorated)
	if event.Failure == nil {
		return
	}
	failure := *event.Failure
	failure.Summary = withHandoffMetadata(failure.Summary, decorated)
	event.Failure = &failure
}

// withHandoffMetadata rewrites the recovery metadata a failure text carries (if any).
func withHandoffMetadata(text string, decorated *acp.ProviderErrorDiagnostic) string {
	diagnostic, ok := acp.ProviderFailureDiagnosticFromSummary(text)
	if !ok {
		return text
	}
	diagnostic.Action = decorated.NextAction
	diagnostic.Guidance = decorated.Guidance
	return acp.ReplaceProviderFailureDiagnostic(text, diagnostic)
}
