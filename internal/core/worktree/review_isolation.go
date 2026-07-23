package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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
// is serialized, and git apply is atomic: a conflicting batch remains in its
// private worktree for triage without partially changing the source.
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
	if err := ensureSourcePathsUnchanged(ctx, r.sourceRoot, workspace.BaselineRef, paths); err != nil {
		return err
	}
	var indexBackup gitIndexBackup
	if autoCommit {
		indexBackup, err = requireUnchangedGitIndex(ctx, r.sourceRoot, r.sourceIndex)
		if err != nil {
			return fmt.Errorf("validate source index before integrating %s: %w", workspace.Root, err)
		}
	}
	if err := runGitInput(ctx, r.sourceRoot, patch, "apply", "--binary", "--whitespace=nowarn"); err != nil {
		return fmt.Errorf("apply isolated review changes from %s: %w", workspace.Root, err)
	}
	if !autoCommit {
		return nil
	}
	return r.commitReviewPatch(ctx, workspace, paths, patch, commitMessage, indexBackup)
}

func (r *ReviewIsolation) commitReviewPatch(
	ctx context.Context,
	workspace ReviewWorkspace,
	paths []string,
	patch []byte,
	commitMessage string,
	indexBackup gitIndexBackup,
) error {
	message := strings.TrimSpace(commitMessage)
	if message == "" {
		message = "fix: resolve review batch"
	}
	stageArgs := []string{"add", "-f", "-A", "--"}
	stageArgs = append(stageArgs, literalPathspecs(paths)...)
	if _, err := runGit(ctx, r.sourceRoot, stageArgs...); err != nil {
		cause := fmt.Errorf("stage integrated review changes from %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewApply(ctx, r.sourceRoot, patch, indexBackup, indexBackup))
	}
	stagedIndex, err := captureGitIndex(ctx, r.sourceRoot)
	if err != nil {
		cause := fmt.Errorf("capture staged source index for %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewWorktree(ctx, r.sourceRoot, patch))
	}
	if err := validateStagedReviewIndex(
		ctx,
		r.sourceRoot,
		workspace.Root,
		paths,
		indexBackup,
	); err != nil {
		cause := fmt.Errorf("validate staged review changes from %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewApply(ctx, r.sourceRoot, patch, indexBackup, stagedIndex))
	}
	args := []string{"commit", "--only", "-m", message, "--"}
	args = append(args, literalPathspecs(paths)...)
	if _, err := runGit(ctx, r.sourceRoot, args...); err != nil {
		cause := fmt.Errorf("commit integrated review changes from %s: %w", workspace.Root, err)
		return errors.Join(cause, rollbackReviewApply(ctx, r.sourceRoot, patch, indexBackup, stagedIndex))
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

func ensureSourcePathsUnchanged(
	ctx context.Context,
	root string,
	baselineRef string,
	paths []string,
) (err error) {
	tempRoot, err := os.MkdirTemp("", "compozy-review-index-")
	if err != nil {
		return fmt.Errorf("create temporary review index directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeTemporaryReviewIndex(tempRoot))
	}()
	indexPath := filepath.Join(tempRoot, "index")
	if _, err := runGitWithIndex(ctx, root, indexPath, "read-tree", baselineRef); err != nil {
		return fmt.Errorf("read review isolation baseline %s: %w", baselineRef, err)
	}
	pathspecs := literalPathspecs(paths)
	args := []string{"ls-files", "--stage", "-z", "--"}
	baselineRaw, err := runGitWithIndex(ctx, root, indexPath, append(args, pathspecs...)...)
	if err != nil {
		return fmt.Errorf("list review isolation baseline entries: %w", err)
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
	changed := make([]string, 0)
	for _, path := range paths {
		entry, tracked := baselineEntries[path]
		matches, matchErr := sourcePathMatchesBaseline(ctx, root, path, entry, tracked)
		if matchErr != nil {
			return fmt.Errorf("compare source path %s with review isolation baseline: %w", path, matchErr)
		}
		if !matches {
			changed = append(changed, path)
		}
	}
	changed = uniqueSorted(changed)
	if len(changed) > 0 {
		return fmt.Errorf(
			"source paths changed since review isolation began: %s",
			strings.Join(changed, ", "),
		)
	}
	return nil
}

func sourcePathMatchesBaseline(
	ctx context.Context,
	root string,
	path string,
	baseline gitBaselineEntry,
	tracked bool,
) (bool, error) {
	absolutePath, err := safeWorkspacePath(root, path)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(absolutePath)
	if errors.Is(err, os.ErrNotExist) {
		return !tracked, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat source path: %w", err)
	}
	if !tracked {
		return false, nil
	}
	switch baseline.mode {
	case "100644", "100755":
		return regularSourcePathMatchesBaseline(ctx, root, path, absolutePath, info, baseline)
	case "120000":
		return symlinkSourcePathMatchesBaseline(ctx, root, absolutePath, info, baseline)
	case "160000":
		return gitlinkSourcePathMatchesBaseline(ctx, absolutePath, info, baseline)
	default:
		return false, fmt.Errorf("unsupported baseline mode %s", baseline.mode)
	}
}

func regularSourcePathMatchesBaseline(
	ctx context.Context,
	root string,
	path string,
	absolutePath string,
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
	raw, err := runGit(ctx, root, "hash-object", "--path="+path, "--", absolutePath)
	if err != nil {
		return false, fmt.Errorf("hash regular file: %w", err)
	}
	return strings.TrimSpace(string(raw)) == baseline.objectID, nil
}

func symlinkSourcePathMatchesBaseline(
	ctx context.Context,
	root string,
	absolutePath string,
	info fs.FileInfo,
	baseline gitBaselineEntry,
) (bool, error) {
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(absolutePath)
	if err != nil {
		return false, fmt.Errorf("read symlink: %w", err)
	}
	raw, err := runGitInputOutput(ctx, root, []byte(target), "hash-object", "--stdin")
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
	if current.path != expected.path || !bytes.Equal(current.content, expected.content) {
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
	args := []string{"ls-files", "--stage", "-z", "--"}
	args = append(args, literalPathspecs(paths)...)
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
) error {
	result := rollbackReviewWorktree(ctx, root, patch)
	result = errors.Join(result, restoreGitIndexCAS(indexBackup, expectedIndex))
	return result
}

func rollbackReviewWorktree(ctx context.Context, root string, patch []byte) error {
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

// OverlayTree copies a regular-file directory tree over an existing
// destination. Symlinks and special files are rejected so a runtime artifact
// mirror cannot escape its target worktree.
func OverlayTree(source string, destination string) error {
	source = filepath.Clean(strings.TrimSpace(source))
	destination = filepath.Clean(strings.TrimSpace(destination))
	if source == "." || destination == "." {
		return errors.New("overlay tree source and destination are required")
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve overlay path %s: %w", path, err)
		}
		if rel == "." {
			return os.MkdirAll(destination, 0o755)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("overlay path %s escapes source %s", path, source)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat overlay source %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("overlay source symlink %s is not supported", path)
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
