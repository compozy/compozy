package recovery

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/worktree"
	"github.com/compozy/compozy/pkg/compozy/runs/layout"
)

const recoveryDiffAuditSchemaVersion = "compozy-recovery-diff-audit-v1"

const (
	recoveryBaselineFileName     = "baseline.json"
	recoveryFinalFileName        = "final.json"
	recoveryChangedFilesFileName = "changed_files.json"
	recoveryMetadataFileName     = "metadata.json"
)

const (
	changeKindAdded     = "added"
	changeKindDeleted   = "deleted"
	changeKindModified  = "modified"
	changeKindRenamed   = "renamed"
	changeKindUntracked = "untracked"
)

// DiffAudit captures baseline and final git state for one recovery attempt and
// writes the recovery artifact subtree.
type DiffAudit struct {
	workspaceRoot string
	recoveryDir   string
	baseline      auditSnapshotDocument
	baselineErr   error
}

// DiffAuditResult contains the documents written to the recovery artifact
// subtree.
type DiffAuditResult struct {
	Baseline     AuditSnapshot     `json:"baseline"`
	Final        AuditSnapshot     `json:"final"`
	ChangedFiles ChangedFilesAudit `json:"changed_files"`
	Metadata     AuditMetadata     `json:"metadata"`
}

// AuditSnapshot is the JSON shape persisted as baseline.json and final.json.
type AuditSnapshot struct {
	SchemaVersion          string `json:"schema_version"`
	Supported              bool   `json:"supported"`
	GitAvailable           bool   `json:"git_available"`
	IsGitRepo              bool   `json:"is_git_repo"`
	HasCommits             bool   `json:"has_commits"`
	Head                   string `json:"head,omitempty"`
	Branch                 string `json:"branch,omitempty"`
	Digest                 string `json:"digest,omitempty"`
	RawPorcelainZBase64    string `json:"raw_porcelain_z_base64"`
	UnsupportedReason      string `json:"unsupported_reason,omitempty"`
	Error                  string `json:"error,omitempty"`
	rawPorcelainForParsing []byte
}

type auditSnapshotDocument = AuditSnapshot

// ChangedFilesAudit is the JSON shape persisted as changed_files.json.
type ChangedFilesAudit struct {
	SchemaVersion string        `json:"schema_version"`
	Supported     bool          `json:"supported"`
	Added         []ChangedFile `json:"added"`
	Modified      []ChangedFile `json:"modified"`
	Deleted       []ChangedFile `json:"deleted"`
	Untracked     []ChangedFile `json:"untracked"`
	Renamed       []ChangedFile `json:"renamed,omitempty"`
}

// ChangedFile is one final dirty path with recovery attribution.
type ChangedFile struct {
	Path           string `json:"path"`
	OriginalPath   string `json:"original_path,omitempty"`
	Kind           string `json:"kind"`
	IndexStatus    string `json:"index_status"`
	WorktreeStatus string `json:"worktree_status"`
	PreExisting    bool   `json:"pre_existing"`
}

// AuditMetadata is the JSON shape persisted as metadata.json.
type AuditMetadata struct {
	SchemaVersion             string   `json:"schema_version"`
	Supported                 bool     `json:"supported"`
	GitAvailable              bool     `json:"git_available"`
	IsGitRepo                 bool     `json:"is_git_repo"`
	HasCommits                bool     `json:"has_commits"`
	BaselineSupported         bool     `json:"baseline_supported"`
	FinalSupported            bool     `json:"final_supported"`
	BaselineUnsupportedReason string   `json:"baseline_unsupported_reason,omitempty"`
	FinalUnsupportedReason    string   `json:"final_unsupported_reason,omitempty"`
	PreExistingDirty          bool     `json:"pre_existing_dirty"`
	PreExistingUntracked      bool     `json:"pre_existing_untracked"`
	CaptureErrors             []string `json:"capture_errors,omitempty"`
}

// BeginDiffAudit captures and writes the baseline snapshot before recovery
// remediation edits the workspace.
func BeginDiffAudit(ctx context.Context, workspaceRoot string, artifacts model.RunArtifacts) (*DiffAudit, error) {
	recoveryDir, err := recoveryDirForArtifacts(artifacts)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(recoveryDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir recovery audit dir: %w", err)
	}
	baseline, captureErr := captureAuditSnapshot(ctx, workspaceRoot)
	if err := writeAuditSnapshot(filepath.Join(recoveryDir, recoveryBaselineFileName), baseline); err != nil {
		return nil, err
	}
	return &DiffAudit{
		workspaceRoot: strings.TrimSpace(workspaceRoot),
		recoveryDir:   recoveryDir,
		baseline:      baseline,
		baselineErr:   captureErr,
	}, nil
}

// Complete captures final git state and writes final.json, changed_files.json,
// and metadata.json. Git capture failures are represented in the metadata and
// do not fail the recovery run; artifact write failures are returned.
func (a *DiffAudit) Complete(ctx context.Context) (*DiffAuditResult, error) {
	if a == nil {
		return nil, errors.New("complete recovery diff audit: missing audit")
	}
	if strings.TrimSpace(a.recoveryDir) == "" {
		return nil, errors.New("complete recovery diff audit: missing recovery dir")
	}

	final, finalErr := captureAuditSnapshot(ctx, a.workspaceRoot)
	changedFiles := buildChangedFilesAudit(a.baseline, final)
	metadata := buildAuditMetadata(a.baseline, final, a.baselineErr, finalErr)

	if err := writeAuditSnapshot(filepath.Join(a.recoveryDir, recoveryFinalFileName), final); err != nil {
		return nil, err
	}
	if err := writeChangedFilesAudit(
		filepath.Join(a.recoveryDir, recoveryChangedFilesFileName),
		changedFiles,
	); err != nil {
		return nil, err
	}
	if err := writeAuditMetadata(filepath.Join(a.recoveryDir, recoveryMetadataFileName), metadata); err != nil {
		return nil, err
	}

	return &DiffAuditResult{
		Baseline:     a.baseline,
		Final:        final,
		ChangedFiles: changedFiles,
		Metadata:     metadata,
	}, nil
}

func recoveryDirForArtifacts(artifacts model.RunArtifacts) (string, error) {
	if dir := strings.TrimSpace(artifacts.RecoveryDir); dir != "" {
		return dir, nil
	}
	if runDir := strings.TrimSpace(artifacts.RunDir); runDir != "" {
		return layout.RecoveryDir(runDir), nil
	}
	return "", errors.New("recovery audit requires a run artifact directory")
}

func captureAuditSnapshot(ctx context.Context, workspaceRoot string) (auditSnapshotDocument, error) {
	snapshot, err := worktree.Capture(ctx, workspaceRoot)
	porcelain := snapshot.Porcelain()
	doc := auditSnapshotDocument{
		SchemaVersion:          recoveryDiffAuditSchemaVersion,
		Supported:              snapshot.IsSupported(),
		GitAvailable:           snapshot.GitAvailable(),
		IsGitRepo:              snapshot.IsGitRepo(),
		HasCommits:             snapshot.HasCommits(),
		Head:                   snapshot.Head(),
		Branch:                 snapshot.Branch(),
		Digest:                 snapshot.Digest(),
		UnsupportedReason:      string(snapshot.UnsupportedReason()),
		rawPorcelainForParsing: porcelain,
	}
	if len(porcelain) > 0 {
		doc.RawPorcelainZBase64 = base64.StdEncoding.EncodeToString(porcelain)
	}
	if err != nil {
		doc.Supported = false
		doc.Error = err.Error()
		return doc, err
	}
	return doc, nil
}

func buildChangedFilesAudit(
	baseline auditSnapshotDocument,
	final auditSnapshotDocument,
) ChangedFilesAudit {
	audit := ChangedFilesAudit{
		SchemaVersion: recoveryDiffAuditSchemaVersion,
		Supported:     baseline.Supported && final.Supported,
		Added:         []ChangedFile{},
		Modified:      []ChangedFile{},
		Deleted:       []ChangedFile{},
		Untracked:     []ChangedFile{},
	}
	if !audit.Supported {
		return audit
	}
	baselineEntries := parsePorcelain(baseline.rawPorcelainForParsing)
	finalEntries := parsePorcelain(final.rawPorcelainForParsing)
	for _, entry := range finalEntries {
		changed := ChangedFile{
			Path:           entry.path,
			OriginalPath:   entry.originalPath,
			Kind:           entry.kind,
			IndexStatus:    string([]byte{entry.indexStatus}),
			WorktreeStatus: string([]byte{entry.worktreeStatus}),
			PreExisting:    porcelainContains(baselineEntries, entry.path, entry.originalPath),
		}
		switch entry.kind {
		case changeKindAdded:
			audit.Added = append(audit.Added, changed)
		case changeKindDeleted:
			audit.Deleted = append(audit.Deleted, changed)
		case changeKindUntracked:
			audit.Untracked = append(audit.Untracked, changed)
		case changeKindRenamed:
			audit.Renamed = append(audit.Renamed, changed)
		default:
			audit.Modified = append(audit.Modified, changed)
		}
	}
	sortChangedFiles(audit.Added)
	sortChangedFiles(audit.Modified)
	sortChangedFiles(audit.Deleted)
	sortChangedFiles(audit.Untracked)
	sortChangedFiles(audit.Renamed)
	return audit
}

func buildAuditMetadata(
	baseline auditSnapshotDocument,
	final auditSnapshotDocument,
	baselineErr error,
	finalErr error,
) AuditMetadata {
	baselineEntries := parsePorcelain(baseline.rawPorcelainForParsing)
	metadata := AuditMetadata{
		SchemaVersion:             recoveryDiffAuditSchemaVersion,
		Supported:                 baseline.Supported && final.Supported,
		GitAvailable:              baseline.GitAvailable && final.GitAvailable,
		IsGitRepo:                 baseline.IsGitRepo && final.IsGitRepo,
		HasCommits:                baseline.HasCommits && final.HasCommits,
		BaselineSupported:         baseline.Supported,
		FinalSupported:            final.Supported,
		BaselineUnsupportedReason: baseline.UnsupportedReason,
		FinalUnsupportedReason:    final.UnsupportedReason,
		PreExistingDirty:          hasTrackedPorcelainEntry(baselineEntries),
		PreExistingUntracked:      hasUntrackedPorcelainEntry(baselineEntries),
	}
	if baselineErr != nil {
		metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("baseline: %v", baselineErr))
	}
	if finalErr != nil {
		metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("final: %v", finalErr))
	}
	return metadata
}

type porcelainEntry struct {
	path           string
	originalPath   string
	kind           string
	indexStatus    byte
	worktreeStatus byte
}

func parsePorcelain(raw []byte) []porcelainEntry {
	if len(raw) == 0 {
		return nil
	}
	records := bytes.Split(raw, []byte{0})
	entries := make([]porcelainEntry, 0, len(records))
	for idx := 0; idx < len(records); idx++ {
		record := records[idx]
		if len(record) == 0 {
			continue
		}
		if len(record) < 4 || record[2] != ' ' {
			continue
		}
		entry := porcelainEntry{
			indexStatus:    record[0],
			worktreeStatus: record[1],
			path:           string(record[3:]),
		}
		if entry.indexStatus == 'R' || entry.indexStatus == 'C' ||
			entry.worktreeStatus == 'R' || entry.worktreeStatus == 'C' {
			if idx+1 < len(records) {
				idx++
				entry.originalPath = string(records[idx])
			}
		}
		entry.kind = classifyPorcelainEntry(entry)
		entries = append(entries, entry)
	}
	return entries
}

func classifyPorcelainEntry(entry porcelainEntry) string {
	switch {
	case entry.indexStatus == '?' && entry.worktreeStatus == '?':
		return changeKindUntracked
	case entry.indexStatus == 'R' || entry.worktreeStatus == 'R':
		return changeKindRenamed
	case entry.indexStatus == 'D' || entry.worktreeStatus == 'D':
		return changeKindDeleted
	case entry.indexStatus == 'A' || entry.worktreeStatus == 'A':
		return changeKindAdded
	default:
		return changeKindModified
	}
}

func porcelainContains(entries []porcelainEntry, path string, originalPath string) bool {
	for _, entry := range entries {
		if entry.path == path || entry.originalPath == path {
			return true
		}
		if originalPath != "" && (entry.path == originalPath || entry.originalPath == originalPath) {
			return true
		}
	}
	return false
}

func hasTrackedPorcelainEntry(entries []porcelainEntry) bool {
	for _, entry := range entries {
		if entry.kind != changeKindUntracked {
			return true
		}
	}
	return false
}

func hasUntrackedPorcelainEntry(entries []porcelainEntry) bool {
	for _, entry := range entries {
		if entry.kind == changeKindUntracked {
			return true
		}
	}
	return false
}

func sortChangedFiles(files []ChangedFile) {
	slices.SortFunc(files, func(a ChangedFile, b ChangedFile) int {
		return strings.Compare(a.Path, b.Path)
	})
}

func writeAuditSnapshot(path string, doc auditSnapshotDocument) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recovery snapshot: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write recovery snapshot %q: %w", path, err)
	}
	return nil
}

func writeChangedFilesAudit(path string, doc ChangedFilesAudit) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recovery changed files: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write recovery changed files %q: %w", path, err)
	}
	return nil
}

func writeAuditMetadata(path string, doc AuditMetadata) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recovery metadata: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write recovery metadata %q: %w", path, err)
	}
	return nil
}
