package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/compozy/compozy/internal/core/gitenv"
)

const (
	reviewIsolationSeedMessage  = "compozy: seed isolated review workspace"
	reviewIndexReconcileTimeout = 5 * time.Second
)

// ErrOverlappingReviewEdits marks a review batch that could not be integrated
// because another concurrent batch already edited the same file in a way that
// truly conflicts (overlapping hunks). Non-overlapping edits to a shared file
// are merged automatically; only genuine conflicts surface this error, so the
// job can be parked deterministically for triage instead of silently dropped.
var ErrOverlappingReviewEdits = errors.New("overlapping review edits")

// sourcePathContent is a pre-apply snapshot of one source path, used to restore
// the source workspace atomically when a 3-way merge integration fails.
type sourcePathContent struct {
	data   []byte
	mode   fs.FileMode
	exists bool
}

// ReviewWorkspace identifies one private worktree used by a review batch.
type ReviewWorkspace struct {
	Root        string
	ReviewsDir  string
	BaselineRef string
}

// ReviewIsolation owns the private worktrees for one concurrent review run and
// serializes their write-back into the source workspace.
type ReviewIsolation struct {
	sourceRoot         string
	artifactRel        string
	workspaces         []ReviewWorkspace
	sourceIndex        gitIndexBackup
	sourceIndexPending bool
	sourceIndexErr     error
	captureSourceIndex func(context.Context, string) (gitIndexBackup, error)
	applyMu            sync.Mutex
}

type gitIndexBackup struct {
	path    string
	content []byte
	mode    fs.FileMode
}

type gitBaselineEntry struct {
	mode     string
	objectID string
}

// NewReviewIsolation creates one clean detached worktree per review batch. The
// current workflow artifacts are committed only inside each disposable worktree,
// making clean stall resets possible even when those artifacts are untracked in
// the source workspace.
func NewReviewIsolation(
	ctx context.Context,
	sourceRoot string,
	reviewsDir string,
	artifactDir string,
	worktreesRoot string,
	jobNames []string,
) (*ReviewIsolation, error) {
	if ctx == nil {
		return nil, errors.New("review isolation context is required")
	}
	source, artifact, artifactRel, reviewRel, root, err := validateReviewIsolationInputs(
		sourceRoot,
		reviewsDir,
		artifactDir,
		worktreesRoot,
		jobNames,
	)
	if err != nil {
		return nil, err
	}
	snapshot, err := CaptureExcluding(ctx, source, artifact)
	if err != nil {
		return nil, fmt.Errorf("capture review isolation source: %w", err)
	}
	if !snapshot.IsSupported() {
		return nil, fmt.Errorf(
			"review isolation requires a Git workspace with commits: %s",
			snapshot.UnsupportedReason(),
		)
	}
	if len(snapshot.Entries()) > 0 {
		return nil, fmt.Errorf(
			"review isolation requires source changes outside %s to be committed first: %s",
			artifactRel,
			strings.Join(snapshotEntryPaths(snapshot.Entries()), ", "),
		)
	}
	sourceIndex, err := captureGitIndex(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("capture review isolation source index: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create review isolation root %s: %w", root, err)
	}

	isolation := &ReviewIsolation{
		sourceRoot:         source,
		artifactRel:        artifactRel,
		workspaces:         make([]ReviewWorkspace, 0, len(jobNames)),
		sourceIndex:        sourceIndex,
		captureSourceIndex: captureGitIndex,
	}
	for index, name := range jobNames {
		workspace, createErr := createReviewWorkspace(
			ctx,
			source,
			artifact,
			artifactRel,
			reviewRel,
			root,
			snapshot.Head(),
			index,
			name,
		)
		if createErr != nil {
			return nil, errors.Join(createErr, isolation.removeCreatedWorkspaces(context.WithoutCancel(ctx)))
		}
		isolation.workspaces = append(isolation.workspaces, workspace)
	}
	return isolation, nil
}

func validateReviewIsolationInputs(
	sourceRoot string,
	reviewsDir string,
	artifactDir string,
	worktreesRoot string,
	jobNames []string,
) (string, string, string, string, string, error) {
	source, err := filepath.Abs(filepath.Clean(strings.TrimSpace(sourceRoot)))
	if err != nil || strings.TrimSpace(sourceRoot) == "" {
		return "", "", "", "", "", errors.New("review isolation source root is required")
	}
	reviews, err := filepath.Abs(filepath.Clean(strings.TrimSpace(reviewsDir)))
	if err != nil || strings.TrimSpace(reviewsDir) == "" {
		return "", "", "", "", "", errors.New("review isolation reviews directory is required")
	}
	reviewRel, ok := pathRelativeToRoot(source, reviews)
	if !ok {
		return "", "", "", "", "", fmt.Errorf("review directory %s is outside workspace %s", reviews, source)
	}
	artifact, err := filepath.Abs(filepath.Clean(strings.TrimSpace(artifactDir)))
	if err != nil || strings.TrimSpace(artifactDir) == "" {
		return "", "", "", "", "", errors.New("review isolation artifact directory is required")
	}
	artifactRel, ok := pathRelativeToRoot(source, artifact)
	if !ok {
		return "", "", "", "", "", fmt.Errorf(
			"review artifact directory %s is outside workspace %s",
			artifact,
			source,
		)
	}
	if _, ok := pathRelativeToRoot(artifact, reviews); !ok {
		return "", "", "", "", "", fmt.Errorf(
			"review directory %s is outside artifact directory %s",
			reviews,
			artifact,
		)
	}
	root, err := filepath.Abs(filepath.Clean(strings.TrimSpace(worktreesRoot)))
	if err != nil || strings.TrimSpace(worktreesRoot) == "" {
		return "", "", "", "", "", errors.New("review isolation worktrees root is required")
	}
	if _, inside := pathRelativeToRoot(source, root); inside || sameRoot(source, root) {
		return "", "", "", "", "", fmt.Errorf("review isolation root %s must be outside workspace %s", root, source)
	}
	if len(jobNames) == 0 {
		return "", "", "", "", "", errors.New("review isolation requires at least one job")
	}
	return source, artifact, artifactRel, reviewRel, root, nil
}

func snapshotEntryPaths(entries []StatusEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	sort.Strings(paths)
	return paths
}

func createReviewWorkspace(
	ctx context.Context,
	sourceRoot string,
	sourceArtifactDir string,
	artifactRel string,
	reviewRel string,
	worktreesRoot string,
	baseRef string,
	index int,
	jobName string,
) (ReviewWorkspace, error) {
	segment := fmt.Sprintf("%03d-%s", index, sanitizeReviewWorkspaceSegment(jobName))
	root := filepath.Join(worktreesRoot, segment)
	if _, err := runGit(ctx, sourceRoot, "worktree", "add", "--detach", root, baseRef); err != nil {
		return ReviewWorkspace{}, fmt.Errorf("create isolated review worktree %s: %w", root, err)
	}
	artifactDir := filepath.Join(root, filepath.FromSlash(artifactRel))
	if err := os.RemoveAll(artifactDir); err != nil {
		return failCreatedReviewWorkspace(ctx, sourceRoot, root, fmt.Errorf(
			"clear isolated review artifact baseline in %s: %w",
			root,
			err,
		))
	}
	if err := OverlayTree(sourceArtifactDir, artifactDir); err != nil {
		return failCreatedReviewWorkspace(ctx, sourceRoot, root, fmt.Errorf(
			"mirror review artifacts into %s: %w",
			root,
			err,
		))
	}
	if _, err := runGit(ctx, root, "add", "-f", "-A", "--", artifactRel); err != nil {
		return failCreatedReviewWorkspace(
			ctx,
			sourceRoot,
			root,
			fmt.Errorf("stage review baseline in %s: %w", root, err),
		)
	}
	staged, err := runGit(ctx, root, "diff", "--cached", "--name-only")
	if err != nil {
		return failCreatedReviewWorkspace(ctx, sourceRoot, root, fmt.Errorf(
			"inspect review baseline in %s: %w",
			root,
			err,
		))
	}
	if len(bytes.TrimSpace(staged)) > 0 {
		if _, err := runGit(
			ctx,
			root,
			"-c", "user.name=Compozy",
			"-c", "user.email=compozy@local",
			"-c", "commit.gpgSign=false",
			"commit", "--no-verify", "-m", reviewIsolationSeedMessage,
		); err != nil {
			return failCreatedReviewWorkspace(ctx, sourceRoot, root, fmt.Errorf(
				"commit review baseline in %s: %w",
				root,
				err,
			))
		}
	}
	baseline, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return failCreatedReviewWorkspace(ctx, sourceRoot, root, fmt.Errorf(
			"resolve isolated review baseline in %s: %w",
			root,
			err,
		))
	}
	return ReviewWorkspace{
		Root:        root,
		ReviewsDir:  filepath.Join(root, filepath.FromSlash(reviewRel)),
		BaselineRef: strings.TrimSpace(string(baseline)),
	}, nil
}

func failCreatedReviewWorkspace(
	ctx context.Context,
	sourceRoot string,
	root string,
	cause error,
) (ReviewWorkspace, error) {
	_, cleanupErr := runGit(context.WithoutCancel(ctx), sourceRoot, "worktree", "remove", "--force", root)
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("remove incomplete review worktree %s: %w", root, cleanupErr)
	}
	return ReviewWorkspace{}, errors.Join(cause, cleanupErr)
}

func sanitizeReviewWorkspaceSegment(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r):
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	segment := strings.Trim(b.String(), "-")
	if segment == "" {
		return "job"
	}
	return segment
}

// Workspace returns one prepared batch worktree.
func (r *ReviewIsolation) Workspace(index int) (ReviewWorkspace, error) {
	if r == nil || index < 0 || index >= len(r.workspaces) {
		return ReviewWorkspace{}, fmt.Errorf("review workspace index %d is out of range", index)
	}
	return r.workspaces[index], nil
}

// Apply writes one isolated batch delta into the source workspace. Write-back
// is serialized. Disjoint batches keep the original strict, atomic apply; when a
// prior concurrent batch already changed a shared secondary file, this batch is
// merged with a 3-way apply and only a true overlapping-hunk conflict parks the
// job (leaving its private worktree intact for triage).
func (r *ReviewIsolation) Apply(
	ctx context.Context,
	index int,
	autoCommit bool,
	commitMessage string,
) error {
	workspace, err := r.Workspace(index)
	if err != nil {
		return err
	}
	r.applyMu.Lock()
	defer r.applyMu.Unlock()
	if err := r.reconcileSourceIndex(ctx); err != nil {
		return fmt.Errorf(
			"reconcile source index after committed review batch: %w",
			errors.Join(r.sourceIndexErr, err),
		)
	}
	r.sourceIndexErr = nil

	if _, err := runGit(ctx, workspace.Root, "add", "-A"); err != nil {
		return fmt.Errorf("stage isolated review changes in %s: %w", workspace.Root, err)
	}
	if err := r.stageWorkspaceArtifacts(ctx, workspace); err != nil {
		return err
	}
	pathsRaw, err := runGit(
		ctx,
		workspace.Root,
		"diff", "--cached", "--name-only", "-z", "--no-renames", workspace.BaselineRef,
	)
	if err != nil {
		return fmt.Errorf("list isolated review changes in %s: %w", workspace.Root, err)
	}
	paths := splitNULTokens(pathsRaw)
	if len(paths) == 0 {
		return nil
	}
	patch, err := runGit(
		ctx,
		workspace.Root,
		"diff", "--cached", "--binary", "--full-index", "--no-renames", workspace.BaselineRef,
	)
	if err != nil {
		return fmt.Errorf("build isolated review patch in %s: %w", workspace.Root, err)
	}
	// Which patched paths a prior concurrent batch already changed in the source
	// since this batch's baseline. Empty for disjoint batches (the common case).
	drifted, err := driftedSourcePaths(ctx, r.sourceRoot, workspace.BaselineRef, paths)
	if err != nil {
		return err
	}
	var indexBackup gitIndexBackup
	if autoCommit {
		indexBackup, err = requireUnchangedGitIndex(ctx, r.sourceRoot, r.sourceIndex)
		if err != nil {
			return fmt.Errorf("validate source index before integrating %s: %w", workspace.Root, err)
		}
	}
	preApplySnapshot, err := r.prepareMergeSnapshot(workspace, paths, drifted)
	if err != nil {
		return err
	}
	if err := r.applyIsolatedPatch(ctx, workspace, patch, paths, drifted, preApplySnapshot); err != nil {
		return err
	}
	if !autoCommit {
		return nil
	}
	committable := r.committablePaths(paths)
	if len(committable) == 0 {
		// The batch only touched workflow artifacts (e.g. every issue was triaged
		// invalid). They are mirrored into the source working tree by the applied
		// patch above but are never committed, so there is nothing to commit here.
		return nil
	}
	return r.commitReviewPatch(
		ctx,
		workspace,
		committable,
		patch,
		commitMessage,
		indexBackup,
		drifted,
		preApplySnapshot,
	)
}

// stageWorkspaceArtifacts force-stages the workflow artifact tree inside the
// isolated worktree so that new artifact files Git would otherwise ignore (e.g.
// freshly written memory notes) are still captured in the batch delta and thus
// mirrored back to the source working tree. Artifact paths are excluded from the
// source commit by committablePaths, so this never leaks ignored files into
// history.
func (r *ReviewIsolation) stageWorkspaceArtifacts(ctx context.Context, workspace ReviewWorkspace) error {
	if strings.TrimSpace(r.artifactRel) == "" {
		return nil
	}
	if _, err := runGit(ctx, workspace.Root, "add", "-f", "-A", "--", r.artifactRel); err != nil {
		return fmt.Errorf("stage isolated review artifacts in %s: %w", workspace.Root, err)
	}
	return nil
}

// committablePaths drops workflow-artifact paths from the batch delta before it
// is staged and committed into the source. Everything under artifactRel (issue
// statuses, memory, specs) is workspace-local review state that is ignored in the
// source; it is still written to the source working tree by the applied patch,
// but must never be tracked or committed there. The code fix always lives outside
// artifactRel, so the resulting commit carries only the fix.
func (r *ReviewIsolation) committablePaths(paths []string) []string {
	artifactRel := strings.TrimSpace(r.artifactRel)
	if artifactRel == "" {
		return paths
	}
	prefix := artifactRel + "/"
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == artifactRel || strings.HasPrefix(path, prefix) {
			continue
		}
		out = append(out, path)
	}
	return out
}

// applyIsolatedPatch integrates one batch patch into the source workspace. It
// tries the strict, atomic apply first; only when that fails and a prior batch
// already changed one of the patched paths does it fall back to a per-path 3-way
// merge. A true overlapping-hunk conflict restores the snapshot to keep the
// source clean and returns ErrOverlappingReviewEdits so the caller can park the
// job deterministically for triage.
func (r *ReviewIsolation) applyIsolatedPatch(
	ctx context.Context,
	workspace ReviewWorkspace,
	patch []byte,
	paths []string,
	drifted []string,
	snapshot map[string]sourcePathContent,
) error {
	strictErr := runGitInput(ctx, r.sourceRoot, patch, "apply", "--binary", "--whitespace=nowarn")
	if strictErr == nil {
		return nil
	}
	if len(drifted) == 0 {
		// No concurrent batch touched these paths, so a strict failure is a
		// genuine failure rather than a mergeable overlap.
		return fmt.Errorf("apply isolated review changes: %w", strictErr)
	}
	// A prior batch changed a shared secondary file. Merge every patched path with
	// git merge-file, an index-independent 3-way text merge that works the same in
	// auto-commit and working-tree-only runs and cleanly reports true conflicts.
	conflicts, mergeErr := r.mergeIsolatedBatch(ctx, workspace, paths)
	if mergeErr != nil {
		restoreErr := restoreSourcePathContents(r.sourceRoot, snapshot)
		return errors.Join(fmt.Errorf("merge isolated review changes: %w", mergeErr), restoreErr)
	}
	if len(conflicts) > 0 {
		restoreErr := restoreSourcePathContents(r.sourceRoot, snapshot)
		return errors.Join(
			fmt.Errorf("%w on %s", ErrOverlappingReviewEdits, strings.Join(uniqueSorted(conflicts), ", ")),
			restoreErr,
		)
	}
	return nil
}

// mergeIsolatedBatch 3-way merges every patched path into the source workspace
// and returns the paths that truly conflicted (overlapping hunks). Paths a prior
// batch left untouched merge cleanly to the batch's version.
func (r *ReviewIsolation) mergeIsolatedBatch(
	ctx context.Context,
	workspace ReviewWorkspace,
	paths []string,
) ([]string, error) {
	conflicts := make([]string, 0)
	for _, path := range paths {
		conflicted, err := mergeSourcePathThreeWay(ctx, r.sourceRoot, workspace.Root, workspace.BaselineRef, path)
		if err != nil {
			return nil, err
		}
		if conflicted {
			conflicts = append(conflicts, path)
		}
	}
	return conflicts, nil
}

// mergeSourcePathThreeWay merges one path's batch edits into the source using
// the isolated baseline as the merge ancestor. It handles additions and
// deletions and reports whether the merge conflicted.
func mergeSourcePathThreeWay(
	ctx context.Context,
	sourceRoot string,
	workspaceRoot string,
	baselineRef string,
	path string,
) (conflicted bool, err error) {
	source, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return false, fmt.Errorf("open source workspace for review merge: %w", err)
	}
	defer func() {
		err = errors.Join(err, source.Close())
	}()
	workspace, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("open isolated workspace for review merge: %w", err)
	}
	defer func() {
		err = errors.Join(err, workspace.Close())
	}()

	theirsContent, theirsMode, theirsExists, err := readFileIfExists(workspace, workspaceRoot, path)
	if err != nil {
		return false, err
	}
	baseContent, baseExists, err := readBaselineBlob(ctx, sourceRoot, baselineRef, path)
	if err != nil {
		return false, err
	}
	oursContent, _, oursExists, err := readFileIfExists(source, sourceRoot, path)
	if err != nil {
		return false, err
	}
	if !theirsExists {
		return mergeDeletedSourcePath(source, sourceRoot, path, oursContent, baseContent, baseExists, oursExists)
	}

	oursAbs, cleanupOurs, err := writeTempMergeFile(oursContent)
	if err != nil {
		return false, err
	}
	defer cleanupOurs()
	baseAbs, cleanup, err := writeTempMergeFile(baseContent)
	if err != nil {
		return false, err
	}
	defer cleanup()
	theirsAbs, cleanupTheirs, err := writeTempMergeFile(theirsContent)
	if err != nil {
		return false, err
	}
	defer cleanupTheirs()

	conflicted, err = runGitMergeFile(ctx, sourceRoot, oursAbs, baseAbs, theirsAbs)
	if err != nil {
		return false, err
	}
	if conflicted {
		return true, nil
	}
	mergedContent, err := os.ReadFile(oursAbs)
	if err != nil {
		return false, fmt.Errorf("read merged review content for %s: %w", path, err)
	}
	if err := writeWorkspaceFileAtomically(source, sourceRoot, path, mergedContent, theirsMode.Perm()); err != nil {
		return false, fmt.Errorf("install merged review content for %s: %w", path, err)
	}
	return false, nil
}

// mergeDeletedSourcePath integrates a batch that deleted path. It removes an
// unchanged source copy and reports a delete/modify conflict when the source
// diverged from the isolated baseline.
func mergeDeletedSourcePath(
	root *os.Root,
	rootPath string,
	path string,
	oursContent []byte,
	baseContent []byte,
	baseExists bool,
	oursExists bool,
) (bool, error) {
	if !oursExists {
		return false, nil
	}
	if baseExists && bytes.Equal(oursContent, baseContent) {
		clean, info, exists, err := inspectWorkspacePath(root, rootPath, path)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, fmt.Errorf("refuse to delete non-regular workspace path %s during review merge", path)
		}
		if err := root.Remove(clean); err != nil {
			return false, fmt.Errorf("apply review deletion of %s: %w", path, err)
		}
		return false, nil
	}
	// The source changed a path the batch deleted: a delete/modify conflict.
	return true, nil
}

// runGitMergeFile performs an in-place 3-way merge of ours with theirs against
// base, writing the result to ours. It returns true when the merge conflicted.
// git merge-file returns the conflict count (capped at 127) on a successful but
// conflicted merge, and 255 (or any code above 127) on a hard error such as an
// attempt to merge binary files, so only the conflict-count range is a mergeable
// conflict; every other non-zero code is a genuine failure. -q is intentionally
// omitted so the diagnostic for a hard error (e.g. "Cannot merge binary files")
// reaches stderr; ordinary text conflicts stay silent on stderr regardless.
func runGitMergeFile(ctx context.Context, root string, oursAbs, baseAbs, theirsAbs string) (bool, error) {
	cmd := gitenv.Command(
		ctx,
		root,
		"merge-file",
		"-L", "current source",
		"-L", "review baseline",
		"-L", "review batch",
		oursAbs, baseAbs, theirsAbs,
	)
	cmd.Env = append(cmd.Env, "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 1 && code <= 127 {
			return true, nil
		}
	}
	return false, fmt.Errorf("git merge-file %s: %w (%s)", oursAbs, err, strings.TrimSpace(stderr.String()))
}

// readBaselineBlob returns the baseline content of path, reporting exists=false
// when the path was added since the isolated baseline.
func readBaselineBlob(ctx context.Context, root, baselineRef, path string) ([]byte, bool, error) {
	if _, err := runGit(ctx, root, "cat-file", "-e", baselineRef+":"+path); err != nil {
		return nil, false, nil
	}
	content, err := runGit(ctx, root, "cat-file", "blob", baselineRef+":"+path)
	if err != nil {
		return nil, false, fmt.Errorf("read review baseline blob %s: %w", path, err)
	}
	return content, true, nil
}

// readFileIfExists returns a rooted path's content and mode, reporting
// exists=false for a missing path. Every existing component must be a real
// directory or regular file; symlinks are never merge inputs.
func readFileIfExists(
	root *os.Root,
	rootPath string,
	path string,
) ([]byte, fs.FileMode, bool, error) {
	clean, info, exists, err := inspectWorkspacePath(root, rootPath, path)
	if err != nil {
		return nil, 0, false, err
	}
	if !exists {
		return nil, 0, false, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, false, fmt.Errorf("workspace path %s is a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, fmt.Errorf("unsupported workspace path %s for review merge", path)
	}
	data, err := readRootedRegularFile(root, rootPath, path, clean, info)
	if err != nil {
		return nil, 0, false, err
	}
	return data, info.Mode(), true, nil
}

func readRootedRegularFile(
	root *os.Root,
	rootPath string,
	path string,
	clean string,
	expected fs.FileInfo,
) (data []byte, err error) {
	file, err := root.Open(clean)
	if err != nil {
		return nil, fmt.Errorf("open workspace path %s under %s: %w", path, rootPath, err)
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened workspace path %s under %s: %w", path, rootPath, err)
	}
	current, err := root.Lstat(clean)
	if err != nil {
		return nil, fmt.Errorf("restat workspace path %s under %s: %w", path, rootPath, err)
	}
	if current.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("workspace path %s became a symlink before review merge read", path)
	}
	if !current.Mode().IsRegular() ||
		!opened.Mode().IsRegular() ||
		!os.SameFile(expected, opened) ||
		!os.SameFile(opened, current) {
		return nil, fmt.Errorf("workspace path %s changed before review merge read", path)
	}
	data, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read workspace path %s under %s: %w", path, rootPath, err)
	}
	return data, nil
}

// inspectWorkspacePath resolves a lexical workspace path through an os.Root and
// rejects symlink or non-directory parent components. The final component is
// returned to the caller for operation-specific type validation.
func inspectWorkspacePath(
	root *os.Root,
	rootPath string,
	path string,
) (string, fs.FileInfo, bool, error) {
	if _, err := safeWorkspacePath(rootPath, path); err != nil {
		return "", nil, false, err
	}
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	parts := strings.Split(clean, string(filepath.Separator))
	current := ""
	for index, part := range parts {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return clean, nil, false, nil
		}
		if err != nil {
			return "", nil, false, fmt.Errorf(
				"stat workspace path component %s under %s: %w",
				current,
				rootPath,
				err,
			)
		}
		if index == len(parts)-1 {
			return clean, info, true, nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, false, fmt.Errorf(
				"workspace path %s contains symlink component %s",
				path,
				current,
			)
		}
		if !info.IsDir() {
			return "", nil, false, fmt.Errorf(
				"workspace path %s contains non-directory component %s",
				path,
				current,
			)
		}
	}
	return clean, nil, false, nil
}

func ensureWorkspaceParent(root *os.Root, rootPath string, path string) (string, error) {
	if _, err := safeWorkspacePath(rootPath, path); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	parent := filepath.Dir(clean)
	if parent == "." {
		return clean, nil
	}
	_, info, exists, err := inspectWorkspacePath(root, rootPath, parent)
	if err != nil {
		return "", err
	}
	if exists {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("workspace path %s contains symlink parent %s", path, parent)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("workspace path %s contains non-directory parent %s", path, parent)
		}
		return clean, nil
	}
	if err := root.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create workspace parent %s under %s: %w", parent, rootPath, err)
	}
	_, info, exists, err = inspectWorkspacePath(root, rootPath, parent)
	if err != nil {
		return "", err
	}
	if !exists || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("workspace parent %s is not a real directory", parent)
	}
	return clean, nil
}

func writeWorkspaceFileAtomically(
	root *os.Root,
	rootPath string,
	path string,
	content []byte,
	mode fs.FileMode,
) (result error) {
	clean, err := ensureWorkspaceParent(root, rootPath, path)
	if err != nil {
		return err
	}
	if err := validateWorkspaceMergeTarget(root, rootPath, path, false); err != nil {
		return err
	}

	tempPath, file, err := createWorkspaceMergeTemp(root, clean)
	if err != nil {
		return err
	}
	fileOpen := true
	installed := false
	defer func() {
		result = errors.Join(
			result,
			cleanupWorkspaceMergeTemp(root, tempPath, file, fileOpen, installed),
		)
	}()
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("write review merge temp for %s: %w", path, err)
	}
	if err := file.Chmod(mode.Perm()); err != nil {
		return fmt.Errorf("set review merge mode for %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync review merge temp for %s: %w", path, err)
	}
	closeErr := file.Close()
	fileOpen = false
	if closeErr != nil {
		return fmt.Errorf("close review merge temp for %s: %w", path, closeErr)
	}

	if _, err := ensureWorkspaceParent(root, rootPath, path); err != nil {
		return err
	}
	if err := validateWorkspaceMergeTarget(root, rootPath, path, true); err != nil {
		return err
	}
	if err := root.Rename(tempPath, clean); err != nil {
		return fmt.Errorf("replace workspace path %s under %s: %w", path, rootPath, err)
	}
	installed = true
	return nil
}

func validateWorkspaceMergeTarget(root *os.Root, rootPath string, path string, changed bool) error {
	_, info, exists, err := inspectWorkspacePath(root, rootPath, path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if changed {
			return fmt.Errorf("workspace path %s became a symlink during review merge", path)
		}
		return fmt.Errorf("workspace path %s is a symlink", path)
	}
	if !info.Mode().IsRegular() {
		if changed {
			return fmt.Errorf("workspace path %s became non-regular during review merge", path)
		}
		return fmt.Errorf("unsupported workspace path %s for review merge", path)
	}
	return nil
}

func cleanupWorkspaceMergeTemp(
	root *os.Root,
	tempPath string,
	file *os.File,
	fileOpen bool,
	installed bool,
) error {
	var result error
	if fileOpen {
		result = file.Close()
	}
	if !installed {
		if err := root.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, fmt.Errorf("remove review merge temp %s: %w", tempPath, err))
		}
	}
	return result
}

func createWorkspaceMergeTemp(root *os.Root, path string) (string, *os.File, error) {
	parent := filepath.Dir(path)
	for attempt := 0; attempt < 16; attempt++ {
		name := fmt.Sprintf(".compozy-review-merge-%d-%d", time.Now().UTC().UnixNano(), attempt)
		if parent != "." {
			name = filepath.Join(parent, name)
		}
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return "", nil, fmt.Errorf("create rooted review merge temp for %s: %w", path, err)
	}
	return "", nil, fmt.Errorf("exhausted rooted review merge temp names for %s", path)
}

// writeTempMergeFile materializes merge input content to a temp file and returns
// a cleanup func for it.
func writeTempMergeFile(content []byte) (string, func(), error) {
	file, err := os.CreateTemp("", "compozy-review-merge-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create review merge input: %w", err)
	}
	name := file.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write review merge input: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close review merge input: %w", err)
	}
	return name, cleanup, nil
}

func (r *ReviewIsolation) commitReviewPatch(
	ctx context.Context,
	workspace ReviewWorkspace,
	paths []string,
	patch []byte,
	commitMessage string,
	indexBackup gitIndexBackup,
	mergedPaths []string,
	snapshot map[string]sourcePathContent,
) error {
	message := strings.TrimSpace(commitMessage)
	if message == "" {
		message = "fix: resolve review batch"
	}
	stageArgs := []string{"add", "-f", "-A", "--"}
	stageArgs = append(stageArgs, literalPathspecs(paths)...)
	if _, err := runGit(ctx, r.sourceRoot, stageArgs...); err != nil {
		cause := fmt.Errorf("stage integrated review changes from %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewApply(ctx, r.sourceRoot, patch, indexBackup, indexBackup, snapshot))
	}
	stagedIndex, err := captureGitIndex(ctx, r.sourceRoot)
	if err != nil {
		cause := fmt.Errorf("capture staged source index for %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewWorktree(ctx, r.sourceRoot, patch, snapshot))
	}
	if err := validateStagedReviewIndex(
		ctx,
		r.sourceRoot,
		workspace.Root,
		paths,
		indexBackup,
		mergedPaths,
	); err != nil {
		cause := fmt.Errorf("validate staged review changes from %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewApply(ctx, r.sourceRoot, patch, indexBackup, stagedIndex, snapshot))
	}
	args := []string{"commit", "--only", "-m", message, "--"}
	args = append(args, literalPathspecs(paths)...)
	if _, err := runGit(ctx, r.sourceRoot, args...); err != nil {
		cause := fmt.Errorf("commit integrated review changes from %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewApply(ctx, r.sourceRoot, patch, indexBackup, stagedIndex, snapshot))
	}
	r.sourceIndex = stagedIndex
	r.sourceIndexPending = true
	r.sourceIndexErr = r.reconcileSourceIndex(ctx)
	return nil
}

func (r *ReviewIsolation) reconcileSourceIndex(ctx context.Context) error {
	if !r.sourceIndexPending {
		return nil
	}
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reviewIndexReconcileTimeout)
	defer cancel()
	current, err := r.captureSourceIndex(reconcileCtx, r.sourceRoot)
	if err != nil {
		return fmt.Errorf("capture source index: %w", err)
	}
	matches, err := gitIndexesMatch(reconcileCtx, r.sourceRoot, r.sourceIndex, current)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("source git index changed after committed review integration")
	}
	r.sourceIndex = current
	r.sourceIndexPending = false
	return nil
}

// driftedSourcePaths returns the patched paths whose current source content
// differs from this batch's isolated baseline — i.e. those a prior concurrent
// batch already changed. The result drives the strict-vs-3way apply decision and
// which paths are exempt from the source-equals-workspace integration check.
func driftedSourcePaths(
	ctx context.Context,
	root string,
	baselineRef string,
	paths []string,
) (changed []string, err error) {
	tempRoot, err := os.MkdirTemp("", "compozy-review-index-")
	if err != nil {
		return nil, fmt.Errorf("create temporary review index directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeTemporaryReviewIndex(tempRoot))
	}()
	indexPath := filepath.Join(tempRoot, "index")
	if _, err := runGitWithIndex(ctx, root, indexPath, "read-tree", baselineRef); err != nil {
		return nil, fmt.Errorf("read review isolation baseline %s: %w", baselineRef, err)
	}
	pathspecs := literalPathspecs(paths)
	args := []string{"ls-files", "--stage", "-z", "--"}
	baselineRaw, err := runGitWithIndex(ctx, root, indexPath, append(args, pathspecs...)...)
	if err != nil {
		return nil, fmt.Errorf("list review isolation baseline entries: %w", err)
	}
	baselineEntries := make(map[string]gitBaselineEntry, len(paths))
	for len(baselineRaw) > 0 {
		var record []byte
		record, baselineRaw = nextPorcelainToken(baselineRaw)
		mode, objectID, stage, path, ok := parseLsFilesRecord(record)
		if !ok || stage != "0" {
			continue
		}
		baselineEntries[path] = gitBaselineEntry{mode: mode, objectID: objectID}
	}
	changed = make([]string, 0)
	for _, path := range paths {
		entry, tracked := baselineEntries[path]
		matches, matchErr := sourcePathMatchesBaseline(ctx, root, path, entry, tracked)
		if matchErr != nil {
			return nil, fmt.Errorf("compare source path %s with review isolation baseline: %w", path, matchErr)
		}
		if !matches {
			changed = append(changed, path)
		}
	}
	return uniqueSorted(changed), nil
}

// prepareMergeSnapshot handles a batch whose patch touches paths a prior
// concurrent batch already changed. It rejects an already-integrated retry with
// the same signal a drifted baseline gives, and otherwise snapshots the source
// so a failed 3-way merge can be rolled back atomically. Disjoint batches (no
// drift) return a nil snapshot and take the strict, atomic apply path.
func (r *ReviewIsolation) prepareMergeSnapshot(
	workspace ReviewWorkspace,
	paths []string,
	drifted []string,
) (map[string]sourcePathContent, error) {
	if len(drifted) == 0 {
		return nil, nil
	}
	integrated, err := batchAlreadyIntegrated(r.sourceRoot, workspace.Root, paths)
	if err != nil {
		return nil, err
	}
	if integrated {
		return nil, fmt.Errorf(
			"source paths changed since review isolation began: %s",
			strings.Join(drifted, ", "),
		)
	}
	snapshot, err := captureSourcePathContents(r.sourceRoot, paths)
	if err != nil {
		return nil, fmt.Errorf("snapshot source paths before merging %s: %w", workspace.Root, err)
	}
	return snapshot, nil
}

// batchAlreadyIntegrated reports whether every patched path in the source
// already matches the isolated workspace result, meaning the batch has already
// been applied (e.g. a retry after a committed batch whose post-commit index
// refresh failed). It distinguishes that idempotent-retry case from a genuine
// shared-secondary-file drift, where the source carries a different concurrent
// batch's edit and must be merged.
func batchAlreadyIntegrated(
	sourceRoot string,
	workspaceRoot string,
	paths []string,
) (integrated bool, err error) {
	source, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return false, fmt.Errorf("open source workspace for integration check: %w", err)
	}
	defer func() {
		err = errors.Join(err, source.Close())
	}()
	workspace, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("open isolated workspace for integration check: %w", err)
	}
	defer func() {
		err = errors.Join(err, workspace.Close())
	}()

	for _, path := range paths {
		sourceData, _, sourceExists, err := readFileIfExists(source, sourceRoot, path)
		if err != nil {
			return false, err
		}
		workspaceData, _, workspaceExists, err := readFileIfExists(workspace, workspaceRoot, path)
		if err != nil {
			return false, err
		}
		if sourceExists != workspaceExists || !bytes.Equal(sourceData, workspaceData) {
			return false, nil
		}
	}
	return true, nil
}

// captureSourcePathContents snapshots the current source content of paths so a
// failed 3-way integration can restore them exactly, keeping the source clean
// even after git apply --3way writes conflict markers.
func captureSourcePathContents(rootPath string, paths []string) (snapshot map[string]sourcePathContent, err error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("open source workspace for review snapshot: %w", err)
	}
	defer func() {
		err = errors.Join(err, root.Close())
	}()

	snapshot = make(map[string]sourcePathContent, len(paths))
	for _, path := range paths {
		data, mode, exists, readErr := readFileIfExists(root, rootPath, path)
		if readErr != nil {
			return nil, fmt.Errorf("read source path %s for review snapshot: %w", path, readErr)
		}
		if !exists {
			snapshot[path] = sourcePathContent{exists: false}
			continue
		}
		snapshot[path] = sourcePathContent{data: data, mode: mode, exists: true}
	}
	return snapshot, nil
}

// restoreSourcePathContents rewrites the snapshotted source paths to their
// pre-apply state, removing any file the apply newly created.
func restoreSourcePathContents(rootPath string, snapshot map[string]sourcePathContent) (result error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("open source workspace for review rollback: %w", err)
	}
	defer func() {
		result = errors.Join(result, root.Close())
	}()

	for path, content := range snapshot {
		clean, info, exists, err := inspectWorkspacePath(root, rootPath, path)
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		if !content.exists {
			if !exists {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				result = errors.Join(result, fmt.Errorf(
					"refuse to remove non-regular source path %s during review rollback",
					path,
				))
				continue
			}
			if err := root.Remove(clean); err != nil {
				result = errors.Join(result, fmt.Errorf(
					"clear source path %s for review rollback: %w",
					path,
					err,
				))
			}
			continue
		}
		if err := writeWorkspaceFileAtomically(
			root,
			rootPath,
			path,
			content.data,
			content.mode.Perm(),
		); err != nil {
			result = errors.Join(result, fmt.Errorf("restore source path %s during review rollback: %w", path, err))
		}
	}
	return result
}

// subtractPaths returns paths with every entry in remove excluded.
func subtractPaths(paths []string, remove []string) []string {
	if len(remove) == 0 {
		return paths
	}
	removeSet := make(map[string]struct{}, len(remove))
	for _, path := range remove {
		removeSet[path] = struct{}{}
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := removeSet[path]; ok {
			continue
		}
		out = append(out, path)
	}
	return out
}

func sourcePathMatchesBaseline(
	ctx context.Context,
	rootPath string,
	path string,
	baseline gitBaselineEntry,
	tracked bool,
) (matches bool, err error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return false, fmt.Errorf("open source workspace for baseline comparison: %w", err)
	}
	defer func() {
		err = errors.Join(err, root.Close())
	}()

	clean, info, exists, err := inspectWorkspacePath(root, rootPath, path)
	if err != nil {
		return false, err
	}
	if !exists {
		return !tracked, nil
	}
	if !tracked {
		return false, nil
	}
	switch baseline.mode {
	case "100644", "100755":
		return regularSourcePathMatchesBaseline(ctx, root, rootPath, path, clean, info, baseline)
	case "120000":
		return symlinkSourcePathMatchesBaseline(ctx, root, rootPath, path, clean, info, baseline)
	case "160000":
		absolutePath, pathErr := safeWorkspacePath(rootPath, path)
		if pathErr != nil {
			return false, pathErr
		}
		return gitlinkSourcePathMatchesBaseline(ctx, absolutePath, info, baseline)
	default:
		return false, fmt.Errorf("unsupported baseline mode %s", baseline.mode)
	}
}

func regularSourcePathMatchesBaseline(
	ctx context.Context,
	root *os.Root,
	rootPath string,
	path string,
	clean string,
	info fs.FileInfo,
	baseline gitBaselineEntry,
) (bool, error) {
	if !info.Mode().IsRegular() {
		return false, nil
	}
	currentMode := "100644"
	if info.Mode().Perm()&0o111 != 0 {
		currentMode = "100755"
	}
	if currentMode != baseline.mode {
		return false, nil
	}
	content, err := readRootedRegularFile(root, rootPath, path, clean, info)
	if err != nil {
		return false, err
	}
	raw, err := runGitInputOutput(ctx, rootPath, content, "hash-object", "--path="+path, "--stdin")
	if err != nil {
		return false, fmt.Errorf("hash regular file: %w", err)
	}
	return strings.TrimSpace(string(raw)) == baseline.objectID, nil
}

func symlinkSourcePathMatchesBaseline(
	ctx context.Context,
	root *os.Root,
	rootPath string,
	path string,
	clean string,
	info fs.FileInfo,
	baseline gitBaselineEntry,
) (bool, error) {
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := root.Readlink(clean)
	if err != nil {
		return false, fmt.Errorf("read source symlink %s under %s: %w", path, rootPath, err)
	}
	raw, err := runGitInputOutput(ctx, rootPath, []byte(target), "hash-object", "--stdin")
	if err != nil {
		return false, fmt.Errorf("hash symlink: %w", err)
	}
	return strings.TrimSpace(string(raw)) == baseline.objectID, nil
}

func gitlinkSourcePathMatchesBaseline(
	ctx context.Context,
	absolutePath string,
	info fs.FileInfo,
	baseline gitBaselineEntry,
) (bool, error) {
	if !info.IsDir() {
		return false, nil
	}
	raw, err := runGit(ctx, absolutePath, "rev-parse", "HEAD")
	if err != nil {
		return false, fmt.Errorf("resolve gitlink HEAD: %w", err)
	}
	return strings.TrimSpace(string(raw)) == baseline.objectID, nil
}

func removeTemporaryReviewIndex(root string) error {
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove temporary review index directory %s: %w", root, err)
	}
	return nil
}

func requireUnchangedGitIndex(
	ctx context.Context,
	root string,
	expected gitIndexBackup,
) (gitIndexBackup, error) {
	current, err := captureGitIndex(ctx, root)
	if err != nil {
		return gitIndexBackup{}, err
	}
	// Compare semantically: Git rewrites the raw index whenever it refreshes its
	// stat cache (e.g. a benign `git status` from an editor), which changes the
	// bytes without changing any entry. A byte compare would abort valid batches;
	// gitIndexesMatch compares the staged entries instead. The freshly captured
	// byte snapshot is still returned as the rollback payload for
	// restoreGitIndexCAS.
	matches, err := gitIndexesMatch(ctx, root, expected, current)
	if err != nil {
		return gitIndexBackup{}, err
	}
	if !matches {
		return gitIndexBackup{}, errors.New("source git index changed since review isolation began")
	}
	return current, nil
}

func validateStagedReviewIndex(
	ctx context.Context,
	sourceRoot string,
	workspaceRoot string,
	paths []string,
	indexBackup gitIndexBackup,
	mergedPaths []string,
) (err error) {
	tempRoot, err := os.MkdirTemp("", "compozy-review-staged-index-")
	if err != nil {
		return fmt.Errorf("create temporary staged index directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeTemporaryReviewIndex(tempRoot))
	}()
	expectedIndexPath := filepath.Join(tempRoot, "index")
	if err := os.WriteFile(expectedIndexPath, indexBackup.content, indexBackup.mode.Perm()); err != nil {
		return fmt.Errorf("copy source index baseline for validation: %w", err)
	}
	stageArgs := []string{"add", "-f", "-A", "--"}
	stageArgs = append(stageArgs, literalPathspecs(paths)...)
	if _, err := runGitWithIndex(ctx, sourceRoot, expectedIndexPath, stageArgs...); err != nil {
		return fmt.Errorf("stage expected review index entries: %w", err)
	}
	expectedEntries, err := runGitWithIndex(ctx, sourceRoot, expectedIndexPath, "ls-files", "--stage", "-z")
	if err != nil {
		return fmt.Errorf("inspect expected source index: %w", err)
	}
	actualEntries, err := runGit(ctx, sourceRoot, "ls-files", "--stage", "-z")
	if err != nil {
		return fmt.Errorf("inspect staged source index: %w", err)
	}
	if !bytes.Equal(actualEntries, expectedEntries) {
		return errors.New("source git index changed during review integration")
	}
	// A merged path legitimately differs from the isolated workspace (it now also
	// carries the earlier batch's edits), so the exact source-equals-workspace
	// check only applies to the batch's disjoint paths.
	comparePaths := subtractPaths(paths, mergedPaths)
	if len(comparePaths) == 0 {
		return nil
	}
	args := []string{"ls-files", "--stage", "-z", "--"}
	args = append(args, literalPathspecs(comparePaths)...)
	sourceEntries, err := runGit(ctx, sourceRoot, args...)
	if err != nil {
		return fmt.Errorf("inspect staged source entries: %w", err)
	}
	workspaceEntries, err := runGit(ctx, workspaceRoot, args...)
	if err != nil {
		return fmt.Errorf("inspect isolated review entries: %w", err)
	}
	if !bytes.Equal(sourceEntries, workspaceEntries) {
		return errors.New("staged source entries differ from isolated review results")
	}
	return nil
}

func literalPathspecs(paths []string) []string {
	pathspecs := make([]string, 0, len(paths))
	for _, path := range paths {
		pathspecs = append(pathspecs, ":(top,literal)"+path)
	}
	return pathspecs
}

func captureGitIndex(ctx context.Context, root string) (gitIndexBackup, error) {
	pathRaw, err := runGit(ctx, root, "rev-parse", "--git-path", "index")
	if err != nil {
		return gitIndexBackup{}, err
	}
	path := strings.TrimSpace(string(pathRaw))
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return gitIndexBackup{}, fmt.Errorf("stat git index %s: %w", path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return gitIndexBackup{}, fmt.Errorf("read git index %s: %w", path, err)
	}
	return gitIndexBackup{path: path, content: content, mode: info.Mode()}, nil
}

func gitIndexesMatch(
	ctx context.Context,
	root string,
	expected gitIndexBackup,
	current gitIndexBackup,
) (matches bool, err error) {
	if expected.path == "" || expected.path != current.path {
		return false, nil
	}
	tempRoot, err := os.MkdirTemp("", "compozy-review-reconcile-index-")
	if err != nil {
		return false, fmt.Errorf("create temporary index reconciliation directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeTemporaryReviewIndex(tempRoot))
	}()
	entries := make([][]byte, 0, 2)
	for index, backup := range []gitIndexBackup{expected, current} {
		indexPath := filepath.Join(tempRoot, fmt.Sprintf("index-%d", index))
		if err := os.WriteFile(indexPath, backup.content, backup.mode.Perm()); err != nil {
			return false, fmt.Errorf("write temporary source index: %w", err)
		}
		raw, err := runGitWithIndex(ctx, root, indexPath, "ls-files", "--stage", "-z")
		if err != nil {
			return false, fmt.Errorf("inspect source index entries: %w", err)
		}
		entries = append(entries, raw)
	}
	return bytes.Equal(entries[0], entries[1]), nil
}

func rollbackReviewApply(
	ctx context.Context,
	root string,
	patch []byte,
	indexBackup gitIndexBackup,
	expectedIndex gitIndexBackup,
	snapshot map[string]sourcePathContent,
) error {
	result := rollbackReviewWorktree(ctx, root, patch, snapshot)
	result = errors.Join(result, restoreGitIndexCAS(indexBackup, expectedIndex))
	return result
}

func rollbackReviewWorktree(
	ctx context.Context,
	root string,
	patch []byte,
	snapshot map[string]sourcePathContent,
) error {
	// A merged batch was applied with a 3-way merge, which a strict --reverse
	// cannot faithfully undo; restore the exact pre-apply bytes instead.
	if len(snapshot) > 0 {
		if err := restoreSourcePathContents(root, snapshot); err != nil {
			return fmt.Errorf("roll back merged review paths: %w", err)
		}
		return nil
	}
	if err := runGitInput(
		context.WithoutCancel(ctx),
		root,
		patch,
		"apply", "--reverse", "--binary", "--whitespace=nowarn",
	); err != nil {
		return fmt.Errorf("roll back integrated review patch: %w", err)
	}
	return nil
}

func restoreGitIndexCAS(backup gitIndexBackup, expected gitIndexBackup) (result error) {
	if backup.path == "" || backup.path != expected.path {
		return errors.New("restore source git index: index path changed during review integration")
	}
	lockPath := backup.path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, backup.mode.Perm())
	if err != nil {
		return fmt.Errorf("lock source git index %s for rollback: %w", backup.path, err)
	}
	lockOpen := true
	installed := false
	defer func() {
		if lockOpen {
			result = errors.Join(result, lockFile.Close())
		}
		if !installed {
			if removeErr := os.Remove(lockPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				result = errors.Join(result, fmt.Errorf("remove source git index lock %s: %w", lockPath, removeErr))
			}
		}
	}()
	current, err := os.ReadFile(backup.path)
	if err != nil {
		return fmt.Errorf("read source git index %s for rollback: %w", backup.path, err)
	}
	if !bytes.Equal(current, expected.content) {
		return errors.New("source git index changed during review rollback; preserved concurrent index state")
	}
	if err := lockFile.Chmod(backup.mode.Perm()); err != nil {
		return fmt.Errorf("set source git index rollback mode: %w", err)
	}
	if _, err := lockFile.Write(backup.content); err != nil {
		return fmt.Errorf("write source git index rollback: %w", err)
	}
	if err := lockFile.Sync(); err != nil {
		return fmt.Errorf("sync source git index rollback: %w", err)
	}
	if err := lockFile.Close(); err != nil {
		return fmt.Errorf("close source git index rollback: %w", err)
	}
	lockOpen = false
	if err := os.Rename(lockPath, backup.path); err != nil {
		return fmt.Errorf("install source git index rollback: %w", err)
	}
	installed = true
	return nil
}

// Cleanup removes an integrated disposable worktree. Failed or parked jobs do
// not call Cleanup, preserving their private filesystem for triage.
func (r *ReviewIsolation) Cleanup(ctx context.Context, index int) error {
	workspace, err := r.Workspace(index)
	if err != nil {
		return err
	}
	if _, err := runGit(ctx, r.sourceRoot, "worktree", "remove", "--force", workspace.Root); err != nil {
		return fmt.Errorf("remove integrated review worktree %s: %w", workspace.Root, err)
	}
	return nil
}

func (r *ReviewIsolation) removeCreatedWorkspaces(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var result error
	for index := range r.workspaces {
		if err := r.Cleanup(ctx, index); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func splitNULTokens(raw []byte) []string {
	parts := bytes.Split(raw, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(string(part)); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func runGitInput(ctx context.Context, root string, input []byte, args ...string) error {
	_, err := runGitInputOutput(ctx, root, input, args...)
	return err
}

func runGitInputOutput(ctx context.Context, root string, input []byte, args ...string) ([]byte, error) {
	cmd := gitenv.Command(ctx, root, args...)
	cmd.Env = append(cmd.Env, "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, message)
	}
	return stdout.Bytes(), nil
}

func runGitWithIndex(
	ctx context.Context,
	root string,
	indexPath string,
	args ...string,
) ([]byte, error) {
	cmd := gitenv.Command(ctx, root, args...)
	cmd.Env = append(
		cmd.Env,
		"LC_ALL=C",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_INDEX_FILE="+indexPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"git %s: %w (%s)",
			strings.Join(args, " "),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return stdout.Bytes(), nil
}

// OverlayTree replaces destination with an exact mirror of a regular-file
// directory tree. Symlinks and special files are rejected so a runtime artifact
// mirror cannot escape its target worktree.
func OverlayTree(source string, destination string) (result error) {
	source = filepath.Clean(strings.TrimSpace(source))
	destination = filepath.Clean(strings.TrimSpace(destination))
	if source == "." || destination == "." {
		return errors.New("overlay tree source and destination are required")
	}
	if sameRoot(source, destination) {
		return errors.New("overlay tree source and destination must differ")
	}
	if _, inside := pathRelativeToRoot(source, destination); inside {
		return fmt.Errorf("overlay tree destination %s is inside source %s", destination, source)
	}
	if _, inside := pathRelativeToRoot(destination, source); inside {
		return fmt.Errorf("overlay tree source %s is inside destination %s", source, destination)
	}
	parent := filepath.Dir(destination)
	if sameRoot(parent, destination) {
		return fmt.Errorf("overlay tree destination %s must not be a filesystem root", destination)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create overlay destination parent %s: %w", parent, err)
	}
	prepared, err := os.MkdirTemp(parent, "."+filepath.Base(destination)+".mirror-")
	if err != nil {
		return fmt.Errorf("create temporary overlay beside %s: %w", destination, err)
	}
	defer func() {
		if err := os.RemoveAll(prepared); err != nil {
			result = errors.Join(result, fmt.Errorf("remove temporary overlay %s: %w", prepared, err))
		}
	}()
	if err := copyOverlayTree(source, prepared); err != nil {
		return err
	}
	if err := replaceOverlayTree(prepared, destination); err != nil {
		return err
	}
	return nil
}

func copyOverlayTree(source string, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve overlay path %s: %w", path, err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("overlay path %s escapes source %s", path, source)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("overlay source symlink %s is not supported", path)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat overlay source %s: %w", path, err)
		}
		if rel == "." {
			if !entry.IsDir() {
				return fmt.Errorf("overlay source %s is not a directory", source)
			}
			return os.Chmod(destination, safeDirectoryMode(info.Mode().Perm()))
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, safeDirectoryMode(info.Mode().Perm()))
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("overlay source %s is not a regular file", path)
		}
		return OverlayFile(path, target, info.Mode().Perm())
	})
}

func replaceOverlayTree(prepared string, destination string) (result error) {
	info, err := os.Lstat(destination)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.Rename(prepared, destination); err != nil {
			return fmt.Errorf("install overlay destination %s: %w", destination, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("stat overlay destination %s: %w", destination, err)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("overlay destination %s is a symlink", destination)
	case !info.IsDir():
		return fmt.Errorf("overlay destination %s is not a directory", destination)
	}
	if err := validateOverlayDestination(destination); err != nil {
		return err
	}

	backup := prepared + ".previous"
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("move existing overlay destination %s: %w", destination, err)
	}
	installed := false
	defer func() {
		if installed {
			if err := os.RemoveAll(backup); err != nil {
				result = errors.Join(result, fmt.Errorf("remove previous overlay %s: %w", backup, err))
			}
		}
	}()
	if err := os.Rename(prepared, destination); err != nil {
		rollbackErr := os.Rename(backup, destination)
		if rollbackErr != nil {
			rollbackErr = fmt.Errorf("restore previous overlay destination %s: %w", destination, rollbackErr)
		}
		return errors.Join(fmt.Errorf("install overlay destination %s: %w", destination, err), rollbackErr)
	}
	installed = true
	return nil
}

func validateOverlayDestination(destination string) error {
	return filepath.WalkDir(destination, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk overlay destination %s: %w", path, walkErr)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("overlay destination %s is a symlink", path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat overlay destination %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("overlay destination %s is not a regular file", path)
		}
		return nil
	})
}

// OverlayFile copies one regular file over an existing destination while
// rejecting a destination symlink.
func OverlayFile(source string, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create overlay parent for %s: %w", destination, err)
	}
	if info, err := os.Lstat(destination); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("overlay destination %s is a symlink", destination)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat overlay destination %s: %w", destination, err)
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open overlay source %s: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, safeFileMode(mode))
	if err != nil {
		return fmt.Errorf("create overlay destination %s: %w", destination, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy overlay %s to %s: %w", source, destination, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close overlay destination %s: %w", destination, closeErr)
	}
	return nil
}

func safeDirectoryMode(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o755
	}
	return mode
}

func safeFileMode(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o600
	}
	return mode
}
