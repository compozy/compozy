package soul

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

func (s *ManagedSoulAuthoringService) persistPostWrite(
	ctx context.Context,
	target resolvedAuthoringTarget,
	previousDigest string,
	action RevisionAction,
	body string,
	actor AuthoringIdentity,
	origin AuthoringIdentity,
) (MutationResult, error) {
	resolved, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Soul: resolved}, authoringSoulResolutionError(err, resolved.Diagnostics)
	}
	provenance, err := NewConfigProvenance(target.config, target.configSource)
	if err != nil {
		return MutationResult{}, err
	}
	now := s.now()
	snapshot, err := SnapshotFromResolved(
		s.newID("soul"),
		target.workspaceID,
		target.agentName,
		&resolved,
		provenance,
		now,
	)
	if err != nil {
		return MutationResult{}, err
	}
	snapshot, err = s.store.UpsertSoulSnapshot(ctx, snapshot)
	if err != nil {
		return MutationResult{}, fmt.Errorf("soul: upsert post-mutation snapshot: %w", err)
	}
	revision, err := s.appendRevision(
		ctx,
		target,
		action,
		previousDigest,
		resolved.Digest,
		body,
		resolved.Diagnostics,
		actor,
		origin,
	)
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Soul: resolved, Snapshot: snapshot, Revision: revision}, nil
}

func (s *ManagedSoulAuthoringService) appendRevision(
	ctx context.Context,
	target resolvedAuthoringTarget,
	action RevisionAction,
	previousDigest string,
	newDigest string,
	body string,
	items []Diagnostic,
	actor AuthoringIdentity,
	origin AuthoringIdentity,
) (Revision, error) {
	encodedDiagnostics, err := DiagnosticsJSON(items)
	if err != nil {
		return Revision{}, err
	}
	revision := Revision{
		ID:              s.newID("srev"),
		WorkspaceID:     target.workspaceID,
		AgentName:       target.agentName,
		SourcePath:      target.sourcePath,
		Action:          action,
		PreviousDigest:  previousDigest,
		NewDigest:       newDigest,
		Body:            body,
		DiagnosticsJSON: encodedDiagnostics,
		ActorKind:       actor.Kind,
		ActorID:         actor.Ref,
		OriginKind:      origin.Kind,
		OriginRef:       origin.Ref,
		CreatedAt:       s.now(),
	}
	revision, err = s.store.AppendSoulRevision(ctx, revision)
	if err != nil {
		return Revision{}, fmt.Errorf("soul: append authoring revision: %w", err)
	}
	return revision, nil
}

func validateExpectedDigest(current *ResolvedSoul, expected string, sourcePath string) error {
	if current == nil {
		return conflictError(sourcePath, "current SOUL.md state is required")
	}
	trimmedExpected := strings.TrimSpace(expected)
	currentDigest := strings.TrimSpace(current.Digest)
	if !current.Present {
		if trimmedExpected == "" {
			return nil
		}
		return conflictError(sourcePath, "expected_digest was provided but SOUL.md is absent")
	}
	if currentDigest == "" {
		if trimmedExpected == "" {
			return nil
		}
		return conflictError(sourcePath, "expected_digest does not match current SOUL.md digest")
	}
	if trimmedExpected == "" {
		return conflictError(sourcePath, "expected_digest is required for the current SOUL.md")
	}
	if trimmedExpected != currentDigest {
		return conflictError(sourcePath, "expected_digest does not match current SOUL.md digest")
	}
	return nil
}

func conflictError(sourcePath string, message string) error {
	return authoringDiagnosticError(diagnosticSoulConflict, sourcePath, message, nil, ErrAuthoringConflict)
}

func authoringInvalidError(items []Diagnostic) error {
	return &AuthoringError{
		Code:        "soul_invalid",
		Diagnostics: cloneDiagnostics(sanitizeDiagnostics(items)),
		cause:       ErrInvalid,
	}
}

func authoringSoulResolutionError(err error, items []Diagnostic) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvalid) {
		return authoringInvalidError(items)
	}
	return err
}

func authoringDiagnosticError(code string, sourcePath string, message string, err error, cause error) error {
	if err != nil {
		message = firstNonEmpty(message, err.Error()) + ": " + err.Error()
	}
	diagnostic := Diagnostic{
		Code:       code,
		Message:    diagnostics.RedactAndBound(message, 300),
		SourcePath: sanitizeDiagnosticPath(sourcePath),
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

func hasBlockingCurrentDiagnostic(items []Diagnostic) bool {
	for _, item := range items {
		switch item.Code {
		case authoringPathEscapeKey, "invalid_source_path", "parser_io":
			return true
		}
	}
	return false
}
