package recovery

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestBuildChangedFilesAudit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseline string
		final    string
		assert   func(*testing.T, ChangedFilesAudit)
	}{
		{
			name:  "Should classify a recovery-modified tracked file",
			final: " M X.txt\x00",
			assert: func(t *testing.T, audit ChangedFilesAudit) {
				t.Helper()
				file := requireSingleChangedFile(t, audit.Modified)
				if file.Path != "X.txt" || file.Kind != "modified" || file.PreExisting {
					t.Fatalf("unexpected modified file: %#v", file)
				}
			},
		},
		{
			name:     "Should flag baseline dirty files as pre-existing",
			baseline: " M Y.txt\x00",
			final:    " M Y.txt\x00 M X.txt\x00",
			assert: func(t *testing.T, audit ChangedFilesAudit) {
				t.Helper()
				files := mapChangedFilesByPath(audit.Modified)
				if files["Y.txt"].Path == "" || !files["Y.txt"].PreExisting {
					t.Fatalf("expected Y.txt to be pre-existing, got %#v", files["Y.txt"])
				}
				if files["X.txt"].Path == "" || files["X.txt"].PreExisting {
					t.Fatalf("expected X.txt to be attributed to recovery, got %#v", files["X.txt"])
				}
			},
		},
		{
			name:  "Should classify a recovery-created untracked file",
			final: "?? Z.txt\x00",
			assert: func(t *testing.T, audit ChangedFilesAudit) {
				t.Helper()
				file := requireSingleChangedFile(t, audit.Untracked)
				if file.Path != "Z.txt" || file.Kind != "untracked" || file.PreExisting {
					t.Fatalf("unexpected untracked file: %#v", file)
				}
			},
		},
		{
			name:  "Should classify a recovery-deleted tracked file",
			final: " D W.txt\x00",
			assert: func(t *testing.T, audit ChangedFilesAudit) {
				t.Helper()
				file := requireSingleChangedFile(t, audit.Deleted)
				if file.Path != "W.txt" || file.Kind != "deleted" || file.PreExisting {
					t.Fatalf("unexpected deleted file: %#v", file)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			audit := buildChangedFilesAudit(
				supportedAuditSnapshot(tc.baseline),
				supportedAuditSnapshot(tc.final),
			)
			if !audit.Supported {
				t.Fatalf("expected supported changed files audit")
			}
			tc.assert(t, audit)
		})
	}
}

func TestBuildAuditMetadataReportsPreExistingState(t *testing.T) {
	t.Parallel()

	metadata := buildAuditMetadata(
		supportedAuditSnapshot(" M dirty.txt\x00?? untracked.txt\x00"),
		supportedAuditSnapshot(" M dirty.txt\x00?? untracked.txt\x00"),
		nil,
		nil,
	)
	if !metadata.Supported {
		t.Fatalf("expected supported metadata")
	}
	if !metadata.PreExistingDirty {
		t.Fatalf("expected pre-existing dirty flag")
	}
	if !metadata.PreExistingUntracked {
		t.Fatalf("expected pre-existing untracked flag")
	}
}

func TestDiffAuditNonGitWorkspaceWritesUnsupportedArtifacts(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	artifacts := model.NewRunArtifacts(t.TempDir(), "non-git-audit")

	audit, err := BeginDiffAudit(context.Background(), workspaceRoot, artifacts)
	if err != nil {
		t.Fatalf("BeginDiffAudit(non-git) error = %v", err)
	}
	result, err := audit.Complete(context.Background())
	if err != nil {
		t.Fatalf("Complete(non-git) error = %v", err)
	}

	metadata := readAuditJSON[AuditMetadata](t, filepath.Join(artifacts.RecoveryDir, recoveryMetadataFileName))
	if metadata.Supported || metadata.IsGitRepo || metadata.BaselineSupported || metadata.FinalSupported {
		t.Fatalf("expected unsupported non-git metadata, got %#v", metadata)
	}
	if metadata.BaselineUnsupportedReason != "non_git" || metadata.FinalUnsupportedReason != "non_git" {
		t.Fatalf("expected non_git unsupported reasons, got %#v", metadata)
	}
	changed := readAuditJSON[ChangedFilesAudit](t, filepath.Join(artifacts.RecoveryDir, recoveryChangedFilesFileName))
	if changed.Supported || len(changed.Modified) != 0 || len(changed.Untracked) != 0 {
		t.Fatalf("expected no changed files for non-git workspace, got %#v", changed)
	}
	if result.Metadata.Supported {
		t.Fatalf("expected returned result metadata to be unsupported")
	}
}

func TestDiffAuditGitMissingWritesUnsupportedMetadata(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspaceRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	artifacts := model.NewRunArtifacts(t.TempDir(), "git-missing-audit")

	audit, err := BeginDiffAudit(context.Background(), workspaceRoot, artifacts)
	if err != nil {
		t.Fatalf("BeginDiffAudit(git-missing) error = %v", err)
	}
	if _, err := audit.Complete(context.Background()); err != nil {
		t.Fatalf("Complete(git-missing) error = %v", err)
	}

	metadata := readAuditJSON[AuditMetadata](t, filepath.Join(artifacts.RecoveryDir, recoveryMetadataFileName))
	if metadata.Supported || metadata.GitAvailable || !metadata.IsGitRepo {
		t.Fatalf("expected unsupported git-missing metadata, got %#v", metadata)
	}
	if metadata.BaselineUnsupportedReason != "git_missing" || metadata.FinalUnsupportedReason != "git_missing" {
		t.Fatalf("expected git_missing unsupported reasons, got %#v", metadata)
	}
}

func TestDiffAuditEmptyGitRepoWritesUnsupportedMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	workspaceRoot := t.TempDir()
	mustRecoveryAuditGit(t, workspaceRoot, "init", "-q", "-b", "main")
	mustRecoveryAuditGit(t, workspaceRoot, "config", "user.email", "recovery-audit@example.com")
	mustRecoveryAuditGit(t, workspaceRoot, "config", "user.name", "Recovery Audit Tester")
	artifacts := model.NewRunArtifacts(t.TempDir(), "empty-git-audit")

	audit, err := BeginDiffAudit(context.Background(), workspaceRoot, artifacts)
	if err != nil {
		t.Fatalf("BeginDiffAudit(empty git) error = %v", err)
	}
	if _, err := audit.Complete(context.Background()); err != nil {
		t.Fatalf("Complete(empty git) error = %v", err)
	}

	metadata := readAuditJSON[AuditMetadata](t, filepath.Join(artifacts.RecoveryDir, recoveryMetadataFileName))
	if metadata.Supported || !metadata.GitAvailable || !metadata.IsGitRepo || metadata.HasCommits {
		t.Fatalf("expected unsupported empty-repo metadata, got %#v", metadata)
	}
	if metadata.BaselineUnsupportedReason != "no_commits" || metadata.FinalUnsupportedReason != "no_commits" {
		t.Fatalf("expected no_commits unsupported reasons, got %#v", metadata)
	}
}

func TestDiffAuditUsesOnlyReadOnlyGitSubcommands(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspaceRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git.log")
	writeFakeGit(t, filepath.Join(fakeBin, "git"), logPath)
	t.Setenv("PATH", fakeBin)
	t.Setenv("GIT_FAKE_LOG", logPath)
	artifacts := model.NewRunArtifacts(t.TempDir(), "readonly-git-audit")

	audit, err := BeginDiffAudit(context.Background(), workspaceRoot, artifacts)
	if err != nil {
		t.Fatalf("BeginDiffAudit(fake-git) error = %v", err)
	}
	if _, err := audit.Complete(context.Background()); err != nil {
		t.Fatalf("Complete(fake-git) error = %v", err)
	}

	logPayload, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git log: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logPayload)), "\n") {
		if line == "" {
			continue
		}
		command := strings.Fields(line)[0]
		switch command {
		case "rev-parse", "status", "diff":
		default:
			t.Fatalf("unexpected git subcommand %q in line %q", command, line)
		}
		forbidden := []string{"commit", "reset", "checkout", "clean", "rm"}
		for _, item := range forbidden {
			if command == item {
				t.Fatalf("forbidden git subcommand %q in line %q", item, line)
			}
		}
	}
}

func TestDiffAuditTempGitRepoCapturesRecoveryEdits(t *testing.T) {
	workspaceRoot := initRecoveryAuditGitRepo(t)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "pre_existing.txt"), []byte("user edit\n"), 0o600); err != nil {
		t.Fatalf("write pre-existing edit: %v", err)
	}
	artifacts := model.NewRunArtifacts(t.TempDir(), "real-git-audit")

	audit, err := BeginDiffAudit(context.Background(), workspaceRoot, artifacts)
	if err != nil {
		t.Fatalf("BeginDiffAudit(real git) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "README.md"), []byte("# recovered\n"), 0o600); err != nil {
		t.Fatalf("modify README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "Z.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	if err := os.Remove(filepath.Join(workspaceRoot, "W.txt")); err != nil {
		t.Fatalf("delete tracked file: %v", err)
	}
	result, err := audit.Complete(context.Background())
	if err != nil {
		t.Fatalf("Complete(real git) error = %v", err)
	}

	if !result.Metadata.Supported {
		t.Fatalf("expected supported metadata, got %#v", result.Metadata)
	}
	if !result.Metadata.PreExistingDirty {
		t.Fatalf("expected pre-existing dirty metadata, got %#v", result.Metadata)
	}
	changed := readAuditJSON[ChangedFilesAudit](t, filepath.Join(artifacts.RecoveryDir, recoveryChangedFilesFileName))
	modified := mapChangedFilesByPath(changed.Modified)
	if modified["README.md"].Path == "" || modified["README.md"].PreExisting {
		t.Fatalf("expected README.md recovery modification, got %#v", modified["README.md"])
	}
	if modified["pre_existing.txt"].Path == "" || !modified["pre_existing.txt"].PreExisting {
		t.Fatalf("expected pre_existing.txt attribution, got %#v", modified["pre_existing.txt"])
	}
	if file := requireSingleChangedFile(t, changed.Untracked); file.Path != "Z.txt" || file.PreExisting {
		t.Fatalf("expected Z.txt untracked recovery file, got %#v", file)
	}
	if file := requireSingleChangedFile(t, changed.Deleted); file.Path != "W.txt" || file.PreExisting {
		t.Fatalf("expected W.txt deleted recovery file, got %#v", file)
	}

	baseline := readAuditJSON[AuditSnapshot](t, filepath.Join(artifacts.RecoveryDir, recoveryBaselineFileName))
	final := readAuditJSON[AuditSnapshot](t, filepath.Join(artifacts.RecoveryDir, recoveryFinalFileName))
	if !baseline.Supported || !final.Supported || baseline.Digest == final.Digest {
		t.Fatalf("expected supported distinct snapshots, baseline=%#v final=%#v", baseline, final)
	}
}

func supportedAuditSnapshot(porcelain string) auditSnapshotDocument {
	return auditSnapshotDocument{
		SchemaVersion:          recoveryDiffAuditSchemaVersion,
		Supported:              true,
		GitAvailable:           true,
		IsGitRepo:              true,
		HasCommits:             true,
		rawPorcelainForParsing: []byte(porcelain),
	}
}

func requireSingleChangedFile(t *testing.T, files []ChangedFile) ChangedFile {
	t.Helper()
	if len(files) != 1 {
		t.Fatalf("expected one changed file, got %#v", files)
	}
	return files[0]
}

func mapChangedFilesByPath(files []ChangedFile) map[string]ChangedFile {
	result := make(map[string]ChangedFile, len(files))
	for _, file := range files {
		result[file.Path] = file
	}
	return result
}

func readAuditJSON[T AuditSnapshot | AuditMetadata | ChangedFilesAudit](t *testing.T, path string) T {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc T
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return doc
}

func writeFakeGit(t *testing.T, path string, logPath string) {
	t.Helper()
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$GIT_FAKE_LOG"
case "$*" in
"rev-parse HEAD")
  printf 'abc123\n'
  ;;
"rev-parse --abbrev-ref HEAD")
  printf 'main\n'
  ;;
"status --porcelain=v1 -z --untracked-files=all")
  printf ' M fake.txt\0'
  ;;
*)
  printf 'unexpected git args: %s\n' "$*" >&2
  exit 1
  ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("seed fake git log: %v", err)
	}
}

func initRecoveryAuditGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	mustRecoveryAuditGit(t, dir, "init", "-q", "-b", "main")
	mustRecoveryAuditGit(t, dir, "config", "user.email", "recovery-audit@example.com")
	mustRecoveryAuditGit(t, dir, "config", "user.name", "Recovery Audit Tester")
	mustRecoveryAuditGit(t, dir, "config", "commit.gpgsign", "false")
	writeRepoFile(t, dir, "README.md", "# initial\n")
	writeRepoFile(t, dir, "W.txt", "tracked\n")
	writeRepoFile(t, dir, "pre_existing.txt", "clean\n")
	mustRecoveryAuditGit(t, dir, "add", "README.md", "W.txt", "pre_existing.txt")
	mustRecoveryAuditGit(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func writeRepoFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write repo file %s: %v", name, err)
	}
}

func mustRecoveryAuditGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, string(out))
	}
}

func TestParsePorcelainHandlesRenameRecords(t *testing.T) {
	t.Parallel()

	audit := buildChangedFilesAudit(
		supportedAuditSnapshot("R  old.txt\x00original.txt\x00"),
		supportedAuditSnapshot("R  new.txt\x00old.txt\x00"),
	)
	file := requireSingleChangedFile(t, audit.Renamed)
	want := ChangedFile{
		Path:           "new.txt",
		OriginalPath:   "old.txt",
		Kind:           "renamed",
		IndexStatus:    "R",
		WorktreeStatus: " ",
		PreExisting:    true,
	}
	if !reflect.DeepEqual(file, want) {
		t.Fatalf("rename changed file = %#v, want %#v", file, want)
	}
}
