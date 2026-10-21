package workspaceaccess

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// DefaultPolicy applies the fixed workspace-access decision chain.
type DefaultPolicy struct {
	facts   SessionFactsSource
	consent SessionConsentCache
	audit   AuditEmitter
	log     *slog.Logger
}

var _ Policy = (*DefaultPolicy)(nil)

// New constructs the fail-closed default policy.
func New(deps Deps) (*DefaultPolicy, error) {
	if deps.Facts == nil {
		return nil, errors.New("workspace access: session facts source is required")
	}
	if deps.Consent == nil {
		return nil, errors.New("workspace access: session consent cache is required")
	}
	if deps.Audit == nil {
		return nil, errors.New("workspace access: audit emitter is required")
	}
	if deps.Log == nil {
		return nil, errors.New("workspace access: logger is required")
	}
	return &DefaultPolicy{
		facts:   deps.Facts,
		consent: deps.Consent,
		audit:   deps.Audit,
		log:     deps.Log,
	}, nil
}

// Authorize evaluates and audits one access request.
func (p *DefaultPolicy) Authorize(ctx context.Context, req Request) (Decision, error) {
	normalizedReq := normalizeRequest(req)
	record := AccessRecord{Actor: normalizedReq.Actor, TargetID: normalizedReq.Target.WorkspaceID, Target: normalizedReq.Target, Seam: normalizedReq.Seam}
	decision, mode, decisionErr := p.authorize(ctx, normalizedReq, &record)
	record.Allowed, record.Source, record.Mode = decision.Allowed, decision.Source, mode
	if decisionErr != nil {
		record.Err = decisionErr.Error()
	}
	if auditErr := p.audit.EmitWorkspaceAccess(ctx, record); auditErr != nil {
		p.warnAuditFailure(ctx, record, auditErr)
	}
	return decision, decisionErr
}

func (p *DefaultPolicy) authorize(ctx context.Context, req Request, record *AccessRecord) (Decision, Mode, error) {
	if err := validateRequest(ctx, req); err != nil {
		return deniedDecision(false), "", err
	}
	if req.Actor.Operator {
		return Decision{Allowed: true, Source: SourceOperator}, "", nil
	}
	if req.Actor.Kind != ActorAgentSession {
		return deniedDecision(false), "", nil
	}
	facts, err := p.facts.SessionFacts(ctx, req.Actor.SessionID)
	mode := facts.Mode
	record.ActorProfileID = facts.ProfileID
	if err != nil {
		return deniedDecision(false), mode, fmt.Errorf("workspace access: resolve session facts: %w", err)
	}
	if facts.ProfileID == "" {
		return deniedDecision(false), mode, errors.New("workspace access: session profile is required")
	}
	sameProfile := !req.Target.AllProfiles && req.Target.ProfileID == facts.ProfileID
	if sameProfile && req.Target.WorkspaceID == facts.WorkspaceID && ((req.Target.Kind == TargetUser && facts.WorkspaceID == "") || req.Target.Kind == TargetWorkspace) {
		return Decision{Allowed: true, Source: SourceSameWorkspace}, mode, nil
	}
	if facts.WorkspaceAccess == Unrestricted {
		return Decision{Allowed: true, Source: SourceAgentGrant}, mode, nil
	}
	if !sameProfile || req.Target.Kind == TargetAggregate {
		return deniedDecision(false), mode, nil
	}
	switch mode {
	case ModeApproveAll:
		return Decision{Allowed: true, Source: SourcePermissionMode}, mode, nil
	case ModeDenyAll:
		return deniedDecision(false), mode, nil
	case ModeApproveReads:
		return p.authorizeFromConsent(ctx, req.Actor.SessionID, mode)
	default:
		return deniedDecision(false), mode, fmt.Errorf("workspace access: unrecognized permission mode %q", mode)
	}
}

func (p *DefaultPolicy) authorizeFromConsent(
	ctx context.Context,
	sessionID string,
	mode Mode,
) (Decision, Mode, error) {
	consent, ok := p.consent.ConsentFor(ctx, sessionID)
	if !ok {
		return deniedDecision(true), mode, nil
	}
	switch consent {
	case ConsentAllow:
		return Decision{Allowed: true, Source: SourceSessionConsent}, mode, nil
	case ConsentReject:
		return deniedDecision(false), mode, nil
	default:
		return deniedDecision(false), mode, fmt.Errorf("workspace access: unrecognized session consent %q", consent)
	}
}

func (p *DefaultPolicy) warnAuditFailure(ctx context.Context, record AccessRecord, err error) {
	args := []any{
		"error", err,
		"session_id", record.Actor.SessionID,
		"workspace_id", record.Actor.WorkspaceID,
		"target_workspace_id", record.TargetID,
		"seam", record.Seam,
	}
	if ctx == nil {
		p.log.Warn("workspace access audit emission failed", args...)
		return
	}
	p.log.WarnContext(ctx, "workspace access audit emission failed", args...)
}

func deniedDecision(promptEligible bool) Decision {
	return Decision{Source: SourceDenied, PromptEligible: promptEligible}
}
