// Package worktree captures deterministic fingerprints of a workspace's
// uncommitted state and derives the set of paths produced by a task run.
package worktree

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/gitenv"
)

const (
	captureSchemaVersion = "compozy-worktree-v2"
	scopeSchemaVersion   = "compozy-worktree-scope-v1"
)

// UnsupportedReason describes why a working-tree snapshot could not be captured.
type UnsupportedReason string

const (
	// UnsupportedReasonBlankRoot means the caller did not provide a workspace root.
	UnsupportedReasonBlankRoot UnsupportedReason = "blank_root"
	// UnsupportedReasonNonGit means the workspace root does not contain git metadata.
	UnsupportedReasonNonGit UnsupportedReason = "non_git"
	// UnsupportedReasonGitMissing means the git executable could not be found.
	UnsupportedReasonGitMissing UnsupportedReason = "git_missing"
	// UnsupportedReasonNoCommits means HEAD could not be resolved.
	UnsupportedReasonNoCommits UnsupportedReason = "no_commits"
)

// StatusEntry is one git porcelain v1 -z status entry captured in a snapshot.
type StatusEntry struct {
	Path           string `json:"path"`
	OriginalPath   string `json:"original_path,omitempty"`
	Kind           string `json:"kind"`
	IndexStatus    string `json:"index_status"`
	WorktreeStatus string `json:"worktree_status"`
	Fingerprint    string `json:"fingerprint,omitempty"`
}

// Snapshot is a deterministic fingerprint of a working tree.
type Snapshot struct {
	digest            string
	head              string
	branch            string
	root              string
	porcelain         []byte
	entries           []StatusEntry
	trackedSymlinks   map[string]string
	unsupportedReason UnsupportedReason
	gitAvailable      bool
	gitRepo           bool
	hasCommits        bool
	supported         bool
}

// SnapshotDocument is the JSON-safe representation used in scope artifacts.
type SnapshotDocument struct {
	SchemaVersion     string            `json:"schema_version"`
	Supported         bool              `json:"supported"`
	GitAvailable      bool              `json:"git_available"`
	IsGitRepo         bool              `json:"is_git_repo"`
	HasCommits        bool              `json:"has_commits"`
	Head              string            `json:"head,omitempty"`
	Branch            string            `json:"branch,omitempty"`
	Digest            string            `json:"digest,omitempty"`
	UnsupportedReason string            `json:"unsupported_reason,omitempty"`
	RawPorcelainZHex  string            `json:"raw_porcelain_z_hex,omitempty"`
	Entries           []StatusEntry     `json:"entries,omitempty"`
	TrackedSymlinks   map[string]string `json:"tracked_symlinks,omitempty"`
}

// Scope describes which dirty paths were produced by a task relative to a
// pre-agent baseline snapshot.
type Scope struct {
	SchemaVersion           string           `json:"schema_version"`
	Supported               bool             `json:"supported"`
	ProducedPaths           []string         `json:"produced_paths"`
	PreExistingPaths        []string         `json:"pre_existing_paths,omitempty"`
	PreExistingChangedPaths []string         `json:"pre_existing_changed_paths,omitempty"`
	UnsupportedReason       string           `json:"unsupported_reason,omitempty"`
	Error                   string           `json:"error,omitempty"`
	Baseline                SnapshotDocument `json:"baseline"`
	Final                   SnapshotDocument `json:"final"`
}

func (s Snapshot) IsSupported() bool { return s.supported }
func (s Snapshot) Digest() string    { return s.digest }
func (s Snapshot) Head() string      { return s.head }
func (s Snapshot) Branch() string    { return s.branch }

// Porcelain returns a copy of the raw git porcelain v1 -z status payload.
func (s Snapshot) Porcelain() []byte { return append([]byte(nil), s.porcelain...) }

// Entries returns parsed porcelain entries with per-path fingerprints.
func (s Snapshot) Entries() []StatusEntry { return append([]StatusEntry(nil), s.entries...) }

func (s Snapshot) UnsupportedReason() UnsupportedReason { return s.unsupportedReason }
func (s Snapshot) GitAvailable() bool                   { return s.gitAvailable }
func (s Snapshot) IsGitRepo() bool                      { return s.gitRepo }
func (s Snapshot) HasCommits() bool                     { return s.hasCommits }

// Equal reports whether two snapshots represent the same working-tree state.
func (s Snapshot) Equal(other Snapshot) bool {
	if !s.supported || !other.supported {
		return false
	}
	return s.digest == other.digest
}

// Document returns a JSON-safe representation of the snapshot.
func (s Snapshot) Document() SnapshotDocument {
	return SnapshotDocument{
		SchemaVersion:     captureSchemaVersion,
		Supported:         s.supported,
		GitAvailable:      s.gitAvailable,
		IsGitRepo:         s.gitRepo,
		HasCommits:        s.hasCommits,
		Head:              s.head,
		Branch:            s.branch,
		Digest:            s.digest,
		UnsupportedReason: string(s.unsupportedReason),
		RawPorcelainZHex:  hex.EncodeToString(s.porcelain),
		Entries:           append([]StatusEntry(nil), s.entries...),
		TrackedSymlinks:   copyStringMap(s.trackedSymlinks),
	}
}

// Unchanged reports whether no task-produced or contaminated changes are present.
func (s Scope) Unchanged() bool {
	return s.Supported && len(s.ProducedPaths) == 0 && len(s.PreExistingChangedPaths) == 0
}

// Capture fingerprints the workspace at root using git.
func Capture(ctx context.Context, root string) (Snapshot, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Snapshot{gitAvailable: gitAvailable(), unsupportedReason: UnsupportedReasonBlankRoot}, nil
	}
	snapshot := Snapshot{gitAvailable: gitAvailable()}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			snapshot.unsupportedReason = UnsupportedReasonNonGit
			return snapshot, nil
		}
		return Snapshot{}, fmt.Errorf("worktree: stat .git in %s: %w", root, err)
	}
	snapshot.gitRepo = true
	if !snapshot.gitAvailable {
		snapshot.unsupportedReason = UnsupportedReasonGitMissing
		return snapshot, nil
	}
	head, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		if isExecLookupError(err) {
			snapshot.gitAvailable = false
			snapshot.unsupportedReason = UnsupportedReasonGitMissing
			return snapshot, nil
		}
		snapshot.unsupportedReason = UnsupportedReasonNoCommits
		return snapshot, nil
	}
	snapshot.head = string(bytes.TrimSpace(head))
	snapshot.root = root
	snapshot.hasCommits = true
	branch, err := runGit(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		if isExecLookupError(err) {
			snapshot.gitAvailable = false
			snapshot.unsupportedReason = UnsupportedReasonGitMissing
			return snapshot, nil
		}
		return snapshot, fmt.Errorf("worktree: git branch in %s: %w", root, err)
	}
	snapshot.branch = string(bytes.TrimSpace(branch))
	porcelain, entries, err := captureStatusEntries(ctx, root)
	if err != nil {
		if isExecLookupError(err) {
			snapshot.gitAvailable = false
			snapshot.unsupportedReason = UnsupportedReasonGitMissing
			return snapshot, nil
		}
		return snapshot, err
	}
	trackedSymlinks, err := captureTrackedSymlinks(ctx, root)
	if err != nil {
		return snapshot, fmt.Errorf("worktree: capture tracked symlinks in %s: %w", root, err)
	}
	return buildSupportedSnapshot(root, snapshot.head, snapshot.branch, porcelain, entries, trackedSymlinks), nil
}

func captureStatusEntries(ctx context.Context, root string) ([]byte, []StatusEntry, error) {
	porcelain, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, nil, fmt.Errorf("worktree: git status in %s: %w", root, err)
	}
	entries, err := ParsePorcelain(porcelain)
	if err != nil {
		return nil, nil, fmt.Errorf("worktree: parse git status in %s: %w", root, err)
	}
	for idx := range entries {
		fingerprint, err := fingerprintEntry(ctx, root, entries[idx])
		if err != nil {
			return nil, nil, fmt.Errorf("worktree: fingerprint %s in %s: %w", entries[idx].Path, root, err)
		}
		entries[idx].Fingerprint = fingerprint
	}
	return porcelain, entries, nil
}

// BuildScope captures the final workspace state and compares it to baseline.
func BuildScope(ctx context.Context, root string, baseline Snapshot) (Scope, error) {
	final, err := Capture(ctx, root)
	scope := CompareSnapshots(baseline, final)
	if err != nil {
		scope.Supported = false
		scope.Error = err.Error()
		return scope, err
	}
	return scope, nil
}

// CompareSnapshots derives the task-produced scope from two snapshots.
func CompareSnapshots(baseline Snapshot, final Snapshot) Scope {
	scope := Scope{
		SchemaVersion: scopeSchemaVersion,
		ProducedPaths: []string{},
		Baseline:      baseline.Document(),
		Final:         final.Document(),
	}
	if !baseline.IsSupported() {
		scope.UnsupportedReason = string(baseline.UnsupportedReason())
		return scope
	}
	if !final.IsSupported() {
		scope.UnsupportedReason = string(final.UnsupportedReason())
		return scope
	}
	scope.Supported = true
	baselineEntries := entriesByPath(baseline.Entries())
	finalEntries := entriesByPath(final.Entries())
	for path, entry := range baselineEntries {
		scope.PreExistingPaths = append(scope.PreExistingPaths, path)
		if finalEntry, ok := finalEntries[path]; !ok || !sameEntry(entry, finalEntry) {
			scope.PreExistingChangedPaths = append(scope.PreExistingChangedPaths, path)
		}
	}
	for path, entry := range finalEntries {
		if baselineEntry, ok := baselineEntries[path]; !ok {
			if isWorktreeSymlinkRetarget(baseline, final, path) {
				scope.PreExistingPaths = append(scope.PreExistingPaths, path)
				continue
			}
			scope.ProducedPaths = append(scope.ProducedPaths, path)
		} else if !sameEntry(baselineEntry, entry) {
			scope.PreExistingChangedPaths = append(scope.PreExistingChangedPaths, path)
		}
	}
	scope.ProducedPaths = uniqueSorted(scope.ProducedPaths)
	scope.PreExistingPaths = uniqueSorted(scope.PreExistingPaths)
	scope.PreExistingChangedPaths = uniqueSorted(scope.PreExistingChangedPaths)
	return scope
}

// WriteScope writes a scope artifact as indented JSON.
func WriteScope(path string, scope Scope) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("worktree scope path is required")
	}
	if scope.SchemaVersion == "" {
		scope.SchemaVersion = scopeSchemaVersion
	}
	raw, err := json.MarshalIndent(scope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal worktree scope: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir worktree scope dir: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write worktree scope %s: %w", path, err)
	}
	return nil
}

// ReadScope reads a persisted worktree scope artifact.
func ReadScope(path string) (Scope, error) {
	raw, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return Scope{}, err
	}
	var scope Scope
	if err := json.Unmarshal(raw, &scope); err != nil {
		return Scope{}, fmt.Errorf("decode worktree scope %s: %w", path, err)
	}
	return scope, nil
}

// ParsePorcelain parses git status --porcelain=v1 -z output.
func ParsePorcelain(raw []byte) ([]StatusEntry, error) {
	entries := make([]StatusEntry, 0)
	for len(raw) > 0 {
		token, rest := nextPorcelainToken(raw)
		raw = rest
		if len(token) == 0 {
			continue
		}
		if len(token) < 4 || token[2] != ' ' {
			return nil, fmt.Errorf("malformed porcelain entry %q", string(token))
		}
		entry := StatusEntry{
			Path:           string(token[3:]),
			Kind:           classifyPorcelainStatus(token[0], token[1]),
			IndexStatus:    string([]byte{token[0]}),
			WorktreeStatus: string([]byte{token[1]}),
		}
		if token[0] == 'R' || token[0] == 'C' {
			var original []byte
			original, raw = nextPorcelainToken(raw)
			if len(original) == 0 {
				return nil, fmt.Errorf("missing original path for porcelain entry %q", string(token))
			}
			entry.OriginalPath = string(original)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func buildSupportedSnapshot(
	root string,
	head string,
	branch string,
	porcelain []byte,
	entries []StatusEntry,
	trackedSymlinks map[string]string,
) Snapshot {
	h := sha256.New()
	h.Write([]byte(captureSchemaVersion))
	h.Write([]byte{0})
	h.Write([]byte(head))
	h.Write([]byte{0})
	h.Write(porcelain)
	for _, entry := range entries {
		h.Write([]byte{0})
		h.Write([]byte(entry.Path))
		h.Write([]byte{0})
		h.Write([]byte(entry.Fingerprint))
	}
	return Snapshot{
		digest:          hex.EncodeToString(h.Sum(nil)),
		head:            head,
		branch:          branch,
		root:            root,
		porcelain:       append([]byte(nil), porcelain...),
		entries:         append([]StatusEntry(nil), entries...),
		trackedSymlinks: copyStringMap(trackedSymlinks),
		gitAvailable:    true,
		gitRepo:         true,
		hasCommits:      true,
		supported:       true,
	}
}

func captureTrackedSymlinks(ctx context.Context, root string) (map[string]string, error) {
	out, err := runGit(ctx, root, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	links := make(map[string]string)
	for len(out) > 0 {
		var record []byte
		record, out = nextPorcelainToken(out)
		if len(record) == 0 {
			continue
		}
		mode, objectID, stage, path, ok := parseLsFilesRecord(record)
		if !ok || mode != "120000" || stage != "0" {
			continue
		}
		target, err := runGit(ctx, root, "cat-file", "-p", objectID)
		if err != nil {
			return nil, fmt.Errorf("read tracked symlink %s: %w", path, err)
		}
		links[path] = string(bytes.TrimRight(target, "\n"))
	}
	return links, nil
}

func parseLsFilesRecord(record []byte) (mode string, objectID string, stage string, path string, ok bool) {
	meta, rawPath, found := bytes.Cut(record, []byte{'\t'})
	if !found {
		return "", "", "", "", false
	}
	parts := bytes.Fields(meta)
	if len(parts) != 3 {
		return "", "", "", "", false
	}
	return string(parts[0]), string(parts[1]), string(parts[2]), string(rawPath), true
}

func isWorktreeSymlinkRetarget(baseline Snapshot, final Snapshot, path string) bool {
	before := baseline.trackedSymlinks[path]
	if !filepath.IsAbs(before) || strings.TrimSpace(final.root) == "" {
		return false
	}
	after, err := readWorkspaceSymlink(final.root, path)
	if err != nil || !filepath.IsAbs(after) {
		return false
	}
	rel, ok := pathRelativeToRoot(final.root, after)
	if !ok || rel == "" {
		return false
	}
	beforeSlash := filepath.ToSlash(filepath.Clean(before))
	relSlash := filepath.ToSlash(rel)
	return beforeSlash == relSlash || strings.HasSuffix(beforeSlash, "/"+relSlash)
}

func readWorkspaceSymlink(root string, rel string) (string, error) {
	path, err := safeWorkspacePath(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", errors.New("workspace path is not a symlink")
	}
	return os.Readlink(path)
}

func pathRelativeToRoot(root string, target string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func nextPorcelainToken(raw []byte) ([]byte, []byte) {
	idx := bytes.IndexByte(raw, 0)
	if idx < 0 {
		return raw, nil
	}
	return raw[:idx], raw[idx+1:]
}

func classifyPorcelainStatus(index, worktree byte) string {
	switch {
	case index == '?' && worktree == '?':
		return "untracked"
	case index == 'U' || worktree == 'U' || (index == 'A' && worktree == 'A') || (index == 'D' && worktree == 'D'):
		return "unmerged"
	case index == 'R':
		return "renamed"
	case index == 'C':
		return "copied"
	case index == 'A' || worktree == 'A':
		return "added"
	case index == 'D' || worktree == 'D':
		return "deleted"
	default:
		return "modified"
	}
}

func fingerprintEntry(ctx context.Context, root string, entry StatusEntry) (string, error) {
	h := sha256.New()
	for _, value := range []string{
		entry.Path,
		entry.OriginalPath,
		entry.Kind,
		entry.IndexStatus,
		entry.WorktreeStatus,
	} {
		h.Write([]byte(value))
		h.Write([]byte{0})
	}
	for _, args := range [][]string{
		{"diff", "--binary", "--", entry.Path},
		{"diff", "--cached", "--binary", "--", entry.Path},
	} {
		out, err := runGit(ctx, root, args...)
		if err != nil {
			return "", err
		}
		h.Write(out)
		h.Write([]byte{0})
	}
	if gitlinkDigest, ok, err := gitlinkFingerprint(ctx, root, entry.Path); err != nil {
		return "", err
	} else if ok {
		h.Write([]byte(gitlinkDigest))
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	fileDigest, err := filesystemFingerprint(root, entry.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	h.Write([]byte(fileDigest))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func gitlinkFingerprint(ctx context.Context, root string, rel string) (string, bool, error) {
	out, err := runGit(ctx, root, "ls-files", "-s", "-z", "--", rel)
	if err != nil {
		return "", false, err
	}
	normalizedRel := filepath.ToSlash(filepath.Clean(rel))
	for len(out) > 0 {
		var record []byte
		record, out = nextPorcelainToken(out)
		if len(record) == 0 {
			continue
		}
		mode, objectID, stage, path, ok := parseLsFilesRecord(record)
		if !ok || mode != "160000" || stage != "0" {
			continue
		}
		if filepath.ToSlash(filepath.Clean(path)) != normalizedRel {
			continue
		}
		h := sha256.New()
		for _, value := range []string{mode, objectID, stage, path} {
			h.Write([]byte(value))
			h.Write([]byte{0})
		}
		return hex.EncodeToString(h.Sum(nil)), true, nil
	}
	return "", false, nil
}

func filesystemFingerprint(root string, rel string) (string, error) {
	path, err := safeWorkspacePath(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if !info.IsDir() {
		if err := writeFileFingerprint(h, path, info, ""); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	err = filepath.WalkDir(path, func(child string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		relative, relErr := filepath.Rel(path, child)
		if relErr != nil {
			return relErr
		}
		return writeFileFingerprint(h, child, info, filepath.ToSlash(relative))
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeFileFingerprint(h io.Writer, path string, info fs.FileInfo, rel string) error {
	mode := info.Mode()
	for _, value := range []string{rel, mode.String()} {
		if _, err := h.Write([]byte(value)); err != nil {
			return err
		}
		if _, err := h.Write([]byte{0}); err != nil {
			return err
		}
	}
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		_, err = h.Write([]byte("symlink\x00" + target))
		return err
	}
	if !mode.IsRegular() {
		_, err := h.Write([]byte("nonregular"))
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = h.Write(content)
	return err
}

func safeWorkspacePath(root string, rel string) (string, error) {
	root = strings.TrimSpace(root)
	rel = strings.TrimSpace(rel)
	if root == "" {
		return "", errors.New("workspace root is required")
	}
	if rel == "" {
		return "", errors.New("workspace relative path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute workspace path %q is not allowed", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace path %q escapes root", rel)
	}
	return filepath.Join(root, clean), nil
}

func entriesByPath(entries []StatusEntry) map[string]StatusEntry {
	result := make(map[string]StatusEntry, len(entries))
	for _, entry := range entries {
		result[entry.Path] = entry
	}
	return result
}

func sameEntry(a, b StatusEntry) bool {
	return a.Path == b.Path &&
		a.OriginalPath == b.OriginalPath &&
		a.Kind == b.Kind &&
		a.IndexStatus == b.IndexStatus &&
		a.WorktreeStatus == b.WorktreeStatus &&
		a.Fingerprint == b.Fingerprint
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	result := values[:0]
	var previous string
	for idx, value := range values {
		if idx > 0 && value == previous {
			continue
		}
		result = append(result, value)
		previous = value
	}
	return result
}

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := gitenv.Command(ctx, root, args...)
	cmd.Env = append(cmd.Env, "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func isExecLookupError(err error) bool {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return errors.Is(execErr.Err, exec.ErrNotFound)
	}
	return errors.Is(err, exec.ErrNotFound)
}
