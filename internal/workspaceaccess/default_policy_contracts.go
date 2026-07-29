package workspaceaccess

import (
	"context"
	"log/slog"
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
