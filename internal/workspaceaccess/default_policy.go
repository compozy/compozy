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
	record := AccessRecord{
		Actor:    normalizedReq.Actor,
		TargetID: normalizedReq.Target.WorkspaceID,
		Target:   normalizedReq.Target,
		Seam:     normalizedReq.Seam,
	}
	decision, mode, decisionErr := p.authorize(ctx, normalizedReq, &record)
	record.Allowed, record.Source, record.Mode = decision.Allowed, decision.Source, mode
	if decisionErr != nil {
		record.Err = decisionErr.Error()
	}
	// Same-scope access crosses no boundary; auditing it would record every ordinary read.
	if decision.Source == SourceSameWorkspace && decisionErr == nil {
		return decision, nil
	}
	if auditErr := p.audit.EmitWorkspaceAccess(ctx, record); auditErr != nil {
		p.warnAuditFailure(ctx, record, auditErr)
	}
	return decision, decisionErr
}

func (p *DefaultPolicy) authorize(ctx context.Context, req Request, record *AccessRecord) (Decision, Mode, error) {
	if err := validateRequest(ctx, req); err != nil {
		return deniedDecisionFor(DenialInvalid), "", err
	}
	if req.Actor.Operator {
		return Decision{Allowed: true, Source: SourceOperator}, "", nil
	}
	if req.Actor.Kind != ActorAgentSession {
		return deniedDecisionFor(DenialNotAgentSession), "", nil
	}
	facts, err := p.facts.SessionFacts(ctx, req.Actor.SessionID)
	mode := facts.Mode
	record.ActorProfileID = facts.ProfileID
	if err != nil {
		return deniedDecisionFor(DenialInvalid), mode, fmt.Errorf("workspace access: resolve session facts: %w", err)
	}
	if facts.ProfileID == "" {
		return deniedDecisionFor(DenialInvalid), mode, errors.New("workspace access: session profile is required")
	}
	if !req.Target.AllProfiles && req.Target.ProfileID == "" && req.Target.Kind != TargetUnresolved {
		// Callers that only know the target workspace (task claims from a project session) act in
		// the session's own profile; the boundary is still decided below.
		req.Target.ProfileID = facts.ProfileID
		record.Target.ProfileID = facts.ProfileID
	}
	sameProfile := !req.Target.AllProfiles && req.Target.ProfileID == facts.ProfileID
	if sameProfile && req.Target.WorkspaceID == facts.WorkspaceID &&
		((req.Target.Kind == TargetUser && facts.WorkspaceID == "") || req.Target.Kind == TargetWorkspace) {
		return Decision{Allowed: true, Source: SourceSameWorkspace}, mode, nil
	}
	if facts.WorkspaceAccess == Unrestricted {
		return Decision{Allowed: true, Source: SourceAgentGrant}, mode, nil
	}
	decision, mode, err := p.authorizeFromMode(ctx, req.Actor.SessionID, mode)
	if err != nil {
		return decision, mode, err
	}
	return requireGrantForWrite(decision, req, sameProfile), mode, nil
}

// requireGrantForWrite keeps the permission mode's reach to reads for another profile and the
// All projects aggregate; writes there need the grant. Same-profile crossings keep the full
// mode table. The mode is applied first so an unresolved target, which never needs the grant,
// gets the same answer as a real one whenever the caller is denied reach at all.
func requireGrantForWrite(decision Decision, req Request, sameProfile bool) Decision {
	crossProfile := !sameProfile && req.Target.Kind != TargetUnresolved
	if req.ReadOnly || (!crossProfile && req.Target.Kind != TargetAggregate) {
		return decision
	}
	if decision.Allowed {
		return deniedDecisionFor(DenialNeedsGrant)
	}
	decision.NeedsGrantAfterConsent = decision.PromptEligible
	return decision
}

// authorizeFromMode applies the permission-mode table to a crossing the grant did not decide.
func (p *DefaultPolicy) authorizeFromMode(ctx context.Context, sessionID string, mode Mode) (Decision, Mode, error) {
	switch mode {
	case ModeApproveAll:
		return Decision{Allowed: true, Source: SourcePermissionMode}, mode, nil
	case ModeDenyAll:
		return deniedDecisionFor(DenialModeDenyAll), mode, nil
	case ModeApproveReads, ModeApproveEdits, ModeAuto:
		// approve-edits and auto widen provider edits only; reaching another scope asks as in approve-reads.
		return p.authorizeFromConsent(ctx, sessionID, mode)
	default:
		return deniedDecisionFor(DenialInvalid), mode,
			fmt.Errorf("workspace access: unrecognized permission mode %q", mode)
	}
}

func (p *DefaultPolicy) authorizeFromConsent(
	ctx context.Context,
	sessionID string,
	mode Mode,
) (Decision, Mode, error) {
	consent, ok := p.consent.ConsentFor(ctx, sessionID)
	if !ok {
		return Decision{Source: SourceDenied, PromptEligible: true, Denial: DenialConsentRequired}, mode, nil
	}
	switch consent {
	case ConsentAllow:
		return Decision{Allowed: true, Source: SourceSessionConsent}, mode, nil
	case ConsentReject:
		return deniedDecisionFor(DenialConsentRejected), mode, nil
	default:
		return deniedDecisionFor(
				DenialInvalid,
			), mode, fmt.Errorf(
				"workspace access: unrecognized session consent %q",
				consent,
			)
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

func deniedDecisionFor(denial Denial) Decision {
	return Decision{Source: SourceDenied, Denial: denial}
}

// RecordToolConsent audits a successful native read after its separate operator consent gate.
func (p *DefaultPolicy) RecordToolConsent(ctx context.Context, req Request) {
	if ctx == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ConsentAuditTimeout)
	defer cancel()
	req = normalizeRequest(req)
	record := AccessRecord{Actor: req.Actor, Target: req.Target, TargetID: req.Target.WorkspaceID, Seam: req.Seam}
	decision, mode, err := p.authorize(ctx, req, &record)
	if err != nil || !req.ReadOnly || req.Seam != SeamTool ||
		(!decision.PromptEligible && (!decision.Allowed || decision.Source != SourceSessionConsent)) {
		return
	}
	record.Allowed, record.Source, record.Mode = true, SourceToolConsent, mode
	if err := p.audit.EmitWorkspaceAccess(ctx, record); err != nil {
		p.warnAuditFailure(ctx, record, err)
	}
}
