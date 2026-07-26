package memory

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/fileutil"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (s *Store) writeDreamArtifact(
	ctx context.Context,
	workspace dreamRunWorkspace,
	run dreamSignalGateResult,
	at time.Time,
) (string, error) {
	if ctx == nil {
		return "", errors.New("memory: dream artifact context is required")
	}
	path, err := s.dreamSystemPath(workspace.scope, "dreaming", dreamArtifactFilename(at))
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return "", fmt.Errorf("memory: ensure dream artifact directory %q: %w", filepath.Dir(path), err)
	}
	content := renderDreamArtifact(run, workspace, at)
	if err := fileutil.AtomicWrite(path, []byte(content)); err != nil {
		return "", fmt.Errorf("memory: write dream artifact %q: %w", path, err)
	}
	return path, nil
}

func (s *Store) writeDreamFailure(
	ctx context.Context,
	workspace dreamRunWorkspace,
	run dreamSignalGateResult,
	cause error,
	at time.Time,
) (string, error) {
	if ctx == nil {
		return "", errors.New("memory: dream failure context is required")
	}
	path, err := s.dreamSystemPath(workspace.scope, "dream", "failures", safeDreamRunFilename(run.runID)+".json")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return "", fmt.Errorf("memory: ensure dream failure directory %q: %w", filepath.Dir(path), err)
	}
	payload, err := json.MarshalIndent(dreamFailurePayload(run, workspace, cause, at), "", "  ")
	if err != nil {
		return "", fmt.Errorf("memory: encode dream failure %q: %w", run.runID, err)
	}
	if err := fileutil.AtomicWrite(path, append(payload, '\n')); err != nil {
		return "", fmt.Errorf("memory: write dream failure %q: %w", path, err)
	}
	return path, nil
}

func (s *Store) dreamSystemPath(scope memcontract.Scope, parts ...string) (string, error) {
	dir, err := s.dirForScope(scope.Normalize())
	if err != nil {
		return "", err
	}
	cleanParts := []string{dir, "_system"}
	for _, part := range parts {
		trimmed, err := cleanSystemPathSegment(part)
		if err != nil {
			return "", err
		}
		cleanParts = append(cleanParts, trimmed)
	}
	return filepath.Join(cleanParts...), nil
}

func cleanSystemPathSegment(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("memory: dream system path segment is required")
	}
	if trimmed == "." || trimmed == ".." || filepath.IsAbs(trimmed) || strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("memory: invalid dream system path segment %q", value)
	}
	return trimmed, nil
}

func dreamArtifactFilename(at time.Time) string {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return at.UTC().Format("20060102") + "-" + aghconfig.BuiltinDreamingCuratorAgentName + ".md"
}

func safeDreamRunFilename(runID string) string {
	cleaned := strings.TrimSpace(runID)
	if cleaned == "" {
		return "dream-run"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ".", "-")
	return replacer.Replace(cleaned)
}

func renderDreamArtifact(run dreamSignalGateResult, workspace dreamRunWorkspace, at time.Time) string {
	var builder strings.Builder
	builder.WriteString("# Dreaming Run\n\n")
	builder.WriteString("- run_id: " + strings.TrimSpace(run.runID) + "\n")
	builder.WriteString("- workspace_id: " + strings.TrimSpace(workspace.id) + "\n")
	builder.WriteString("- prompt_version: " + dreamPromptVersion + "\n")
	builder.WriteString("- promoted_at: " + at.UTC().Format(time.RFC3339) + "\n\n")
	builder.WriteString("## Candidates\n\n")
	for _, candidate := range run.candidates {
		builder.WriteString("- ")
		builder.WriteString(strings.TrimSpace(candidate.Title))
		builder.WriteString(" (score=")
		fmt.Fprintf(&builder, "%.3f", candidate.Score)
		builder.WriteString(")\n")
	}
	return builder.String()
}

func dreamFailurePayload(
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	cause error,
	at time.Time,
) map[string]any {
	errText := ""
	if cause != nil {
		errText = diagnostics.RedactAndBound(cause.Error(), maxOperationSummaryBytes)
	}
	return map[string]any{
		"run_id":                strings.TrimSpace(run.runID),
		dreamV2WorkspaceIDKey:   strings.TrimSpace(workspace.id),
		dreamV2PromptVersionKey: dreamPromptVersion,
		"failed_at":             at.UTC().Format(time.RFC3339),
		dreamErrorFieldKey:      errText,
		"candidate_count":       len(run.candidates),
		"candidate_ids":         dreamCandidateChunkIDs(run.candidates),
	}
}
