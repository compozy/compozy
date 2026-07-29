package workspaceaccess

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// Mode mirrors the effective permission modes without importing config.
type Mode string

const (
	ModeDenyAll      Mode = "deny-all"
	ModeApproveReads Mode = "approve-reads"
	ModeApproveAll   Mode = "approve-all"
)

// ModeSource resolves the effective permission mode of a live session.
type ModeSource interface {
	SessionPermissionMode(ctx context.Context, sessionID string) (Mode, error)
}

// Consent is a session-lifetime answer captured from an ACP prompt.
type Consent string

const (
	ConsentAllow  Consent = "allow"
	ConsentReject Consent = "reject"
)

// SessionConsentCache stores one cross-workspace answer per live session.
type SessionConsentCache interface {
	ConsentFor(ctx context.Context, sessionID string) (Consent, bool)
	PutConsent(ctx context.Context, sessionID string, consent Consent)
}

// AuditEmitter appends one best-effort record per decision.
type AuditEmitter interface {
	EmitWorkspaceAccess(ctx context.Context, record AccessRecord) error
}

// Deps contains every dependency required by DefaultPolicy.
type Deps struct {
	Modes   ModeSource
	Consent SessionConsentCache
	Audit   AuditEmitter
	Log     *slog.Logger
}

// DefaultPolicy applies the fixed workspace-access decision chain.
type DefaultPolicy struct {
	modes   ModeSource
	consent SessionConsentCache
	audit   AuditEmitter
	log     *slog.Logger
}

var (
	workspaceIDPattern        = regexp.MustCompile(`^(?:ws_[0-9a-f]{16}|[0-9A-HJ-KMNP-TV-Z]{26})$`)
	_                  Policy = (*DefaultPolicy)(nil)
)

// New constructs the fail-closed default policy.
func New(deps Deps) (*DefaultPolicy, error) {
	if deps.Modes == nil {
		return nil, errors.New("workspace access: mode source is required")
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
		modes:   deps.Modes,
		consent: deps.Consent,
		audit:   deps.Audit,
		log:     deps.Log,
	}, nil
}

// Authorize evaluates and audits one access request.
func (p *DefaultPolicy) Authorize(ctx context.Context, req Request) (Decision, error) {
	normalizedReq := normalizeRequest(req)
	decision, mode, decisionErr := p.authorize(ctx, normalizedReq)
	record := AccessRecord{
		Actor:    normalizedReq.Actor,
		TargetID: normalizedReq.TargetWorkspaceID,
		Allowed:  decision.Allowed,
		Source:   decision.Source,
		Mode:     mode,
		Seam:     normalizedReq.Seam,
	}
	if decisionErr != nil {
		record.Err = decisionErr.Error()
	}
	if auditErr := p.audit.EmitWorkspaceAccess(ctx, record); auditErr != nil {
		p.warnAuditFailure(ctx, record, auditErr)
	}
	return decision, decisionErr
}

func (p *DefaultPolicy) authorize(ctx context.Context, req Request) (Decision, Mode, error) {
	if err := validateRequest(ctx, req); err != nil {
		return deniedDecision(false), "", err
	}
	if req.Actor.Operator {
		return Decision{Allowed: true, Source: SourceOperator}, "", nil
	}
	if req.Actor.Kind != ActorAgentSession {
		return deniedDecision(false), "", nil
	}
	if req.Actor.WorkspaceID != "" && req.Actor.WorkspaceID == req.TargetWorkspaceID {
		return Decision{Allowed: true, Source: SourceSameWorkspace}, "", nil
	}

	mode, err := p.modes.SessionPermissionMode(ctx, req.Actor.SessionID)
	if err != nil {
		return deniedDecision(false), mode, fmt.Errorf("workspace access: resolve session permission mode: %w", err)
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

func normalizeRequest(req Request) Request {
	req.Actor.SessionID = strings.TrimSpace(req.Actor.SessionID)
	req.Actor.WorkspaceID = strings.TrimSpace(req.Actor.WorkspaceID)
	req.Actor.AgentName = strings.TrimSpace(req.Actor.AgentName)
	req.TargetWorkspaceID = strings.TrimSpace(req.TargetWorkspaceID)
	return req
}

func validateRequest(ctx context.Context, req Request) error {
	if ctx == nil {
		return errors.New("workspace access: context is required")
	}
	if !validActorKind(req.Actor.Kind) {
		return fmt.Errorf("workspace access: unknown actor kind %q", req.Actor.Kind)
	}
	if !workspaceIDPattern.MatchString(req.TargetWorkspaceID) {
		return fmt.Errorf("workspace access: target workspace id %q is not canonical", req.TargetWorkspaceID)
	}
	if !validSeam(req.Seam) {
		return fmt.Errorf("workspace access: unknown seam %q", req.Seam)
	}
	if req.Actor.Kind == ActorAgentSession && req.Actor.SessionID == "" {
		return errors.New("workspace access: agent session id is required")
	}
	return nil
}

func validActorKind(kind ActorKind) bool {
	switch kind {
	case ActorAgentSession, ActorHuman, ActorExtension, ActorAutomation, ActorNetworkPeer, ActorDaemon:
		return true
	default:
		return false
	}
}

func validSeam(seam Seam) bool {
	switch seam {
	case SeamIdentity, SeamTask, SeamTool, SeamSpawn, SeamCoordination:
		return true
	default:
		return false
	}
}

func deniedDecision(promptEligible bool) Decision {
	return Decision{Source: SourceDenied, PromptEligible: promptEligible}
}
