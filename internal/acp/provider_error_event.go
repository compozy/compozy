package acp

import (
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

const (
	ProviderErrorAuthRequired = "provider_auth_required"
	ProviderErrorRateLimited  = "provider_rate_limited"
)

// ProviderErrorDiagnostic describes a recoverable provider failure within one process lifetime.
type ProviderErrorDiagnostic struct {
	Code            string                `json:"code"`
	Provider        string                `json:"provider"`
	NextAction      ProviderFailureAction `json:"next_action"`
	Guidance        string                `json:"guidance"`
	OccurrenceCount uint64                `json:"occurrence_count"`
	FirstSeenAt     time.Time             `json:"first_seen_at"`
	LastSeenAt      time.Time             `json:"last_seen_at"`
}

func (p *AgentProcess) promptErrorEvent(req PromptRequest, err error, timestamp time.Time) AgentEvent {
	failure, _ := FailureFromError(err, store.FailurePrompt)
	providerError := p.providerErrorDiagnostic(err, failure, timestamp)
	if providerError != nil && providerError.Code == ProviderErrorAuthRequired {
		diagnostic := providerFailureDiagnostic(
			ProviderFailureUnauthenticated,
			providerError.NextAction,
			providerError.Guidance,
		)
		failure.Summary = diagnostics.RedactAndBound(diagnostic.Summary(err.Error()), maxFailureSummaryBytes)
	}
	return AgentEvent{
		Type:          EventTypeError,
		SessionID:     p.SessionID,
		TurnID:        req.TurnID,
		Timestamp:     timestamp,
		Error:         firstTrimmedNonEmpty(failureSummary(failure), err.Error()),
		Failure:       failure,
		ProviderError: providerError,
		Raw:           requestErrorRaw(err),
	}
}

func (p *AgentProcess) providerErrorDiagnostic(
	err error, failure *store.SessionFailure, timestamp time.Time,
) *ProviderErrorDiagnostic {
	if failure == nil || failure.Kind != store.FailurePrompt {
		return nil
	}
	diagnostic, ok := ProviderFailureDiagnosticFromError(err)
	if !ok {
		return nil
	}
	var code string
	switch diagnostic.Kind {
	case ProviderFailureUnauthenticated:
		code = ProviderErrorAuthRequired
		diagnostic = p.providerAuthRecovery(diagnostic)
	case ProviderFailureRateLimited:
		code = ProviderErrorRateLimited
	default:
		return nil
	}
	p.providerFailureMu.Lock()
	defer p.providerFailureMu.Unlock()
	if p.providerFailures == nil {
		p.providerFailures = make(map[string]ProviderErrorDiagnostic)
	}
	occurrence := p.providerFailures[code]
	if occurrence.OccurrenceCount == 0 {
		occurrence = ProviderErrorDiagnostic{
			Code: code, Provider: diagnostics.Redact(p.providerName),
			NextAction: diagnostic.Action, Guidance: diagnostic.Guidance,
			FirstSeenAt: timestamp,
		}
	}
	if occurrence.OccurrenceCount < ^uint64(0) {
		occurrence.OccurrenceCount++
	}
	occurrence.LastSeenAt = timestamp
	p.providerFailures[code] = occurrence
	return &occurrence
}

// CloneProviderErrorDiagnostic isolates diagnostics attached to emitted events.
func CloneProviderErrorDiagnostic(diagnostic *ProviderErrorDiagnostic) *ProviderErrorDiagnostic {
	if diagnostic == nil {
		return nil
	}
	return new(*diagnostic)
}

func (p *AgentProcess) providerAuthRecovery(diagnostic ProviderFailureDiagnostic) ProviderFailureDiagnostic {
	switch p.providerAuthMode {
	case compozyconfig.ProviderAuthModeBoundSecret:
		diagnostic.Action = ProviderFailureActionBindSecret
		diagnostic.Guidance = "update the provider credential binding and retry"
	case compozyconfig.ProviderAuthModeNone:
		diagnostic.Action = ProviderFailureActionInspect
		diagnostic.Guidance = "inspect the provider authentication configuration"
	}
	return diagnostic
}
