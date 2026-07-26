package heartbeat

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

func (s *ManagedHeartbeatAuthoringService) appendRevision(
	ctx context.Context,
	target resolvedAuthoringTarget,
	operation RevisionOperation,
	previousDigest string,
	newDigest string,
	newSnapshotID string,
	body string,
	actor AuthoringIdentity,
) (Revision, error) {
	actorKind, actorRef := normalizeAuthoringActor(actor)
	revision := Revision{
		ID:             s.newID("hrev"),
		WorkspaceID:    target.workspaceID,
		AgentName:      target.agentName,
		SourcePath:     target.sourcePath,
		Operation:      operation,
		PreviousDigest: previousDigest,
		NewDigest:      newDigest,
		NewSnapshotID:  newSnapshotID,
		Body:           body,
		ActorKind:      actorKind,
		ActorID:        actorRef,
		CreatedAt:      s.now(),
	}
	revision, err := s.store.AppendHeartbeatRevision(ctx, revision)
	if err != nil {
		return Revision{}, fmt.Errorf("heartbeat: append authoring revision: %w", err)
	}
	return revision, nil
}

func normalizeAuthoringActor(actor AuthoringIdentity) (ActorKind, string) {
	kind := ActorKind(strings.TrimSpace(actor.Kind))
	if !ValidActorKind(kind) {
		kind = ActorKindSystem
	}
	ref := strings.TrimSpace(actor.Ref)
	if ref == "" {
		ref = "unknown"
	}
	return kind, diagnostics.RedactAndBound(ref, 300)
}

func validateExpectedDigest(current *ResolvedPolicy, expected string, sourcePath string) error {
	if current == nil {
		return conflictError(sourcePath, "current HEARTBEAT.md state is required")
	}
	trimmedExpected := strings.TrimSpace(expected)
	currentDigest := strings.TrimSpace(current.Digest)
	if !current.Present {
		if trimmedExpected == "" {
			return nil
		}
		return conflictError(sourcePath, "expected_digest was provided but HEARTBEAT.md is absent")
	}
	if currentDigest == "" {
		if trimmedExpected == "" {
			return nil
		}
		return conflictError(sourcePath, "expected_digest does not match current HEARTBEAT.md digest")
	}
	if trimmedExpected == "" {
		return conflictError(sourcePath, "expected_digest is required for the current HEARTBEAT.md")
	}
	if trimmedExpected != currentDigest {
		return conflictError(sourcePath, "expected_digest does not match current HEARTBEAT.md digest")
	}
	return nil
}

func conflictError(sourcePath string, message string) error {
	return authoringDiagnosticError(diagnosticHeartbeatConflict, sourcePath, message, nil, ErrAuthoringConflict)
}

func authoringInvalidError(items []Diagnostic) error {
	return &AuthoringError{
		Code:        "heartbeat_invalid",
		Diagnostics: cloneDiagnostics(sanitizeDiagnostics(items)),
		cause:       ErrInvalid,
	}
}

func revisionMissingError(sourcePath string, err error) error {
	return authoringDiagnosticError(
		diagnosticHeartbeatRevisionMissing,
		sourcePath,
		"HEARTBEAT.md rollback target was not found",
		err,
		ErrRevisionNotFound,
	)
}

func authoringDiagnosticError(code string, sourcePath string, message string, err error, cause error) error {
	if err != nil {
		message = firstNonEmpty(message, err.Error()) + ": " + err.Error()
	}
	diagnostic := Diagnostic{
		Code:       code,
		Severity:   diagnosticError,
		Message:    diagnostics.RedactAndBound(message, 300),
		SourcePath: safePathWithoutRoot(sourcePath),
	}
	return &AuthoringError{
		Code:        code,
		Diagnostics: []Diagnostic{diagnostic},
		cause:       cause,
	}
}

func authoringDiagnosticFromDiagnostic(diagnostic *Diagnostic, cause error) error {
	if diagnostic == nil {
		return cause
	}
	return &AuthoringError{
		Code:        diagnostic.Code,
		Diagnostics: []Diagnostic{*diagnostic},
		cause:       cause,
	}
}
