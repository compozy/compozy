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
	"unicode"

	"github.com/compozy/compozy/internal/core/gitenv"
)

const reviewIsolationSeedMessage = "compozy: seed isolated review workspace"

// ReviewWorkspace identifies one private worktree used by a review batch.
type ReviewWorkspace struct {
	Root        string
	ReviewsDir  string
	BaselineRef string
}

// ReviewIsolation owns the private worktrees for one concurrent review run and
// serializes their write-back into the source workspace.
type ReviewIsolation struct {
	sourceRoot string
	workspaces []ReviewWorkspace
	applyMu    sync.Mutex
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
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create review isolation root %s: %w", root, err)
	}

	isolation := &ReviewIsolation{sourceRoot: source, workspaces: make([]ReviewWorkspace, 0, len(jobNames))}
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

	if _, err := runGit(ctx, workspace.Root, "add", "-A"); err != nil {
		return fmt.Errorf("stage isolated review changes in %s: %w", workspace.Root, err)
	}
	pathsRaw, err := runGit(
		ctx,
		workspace.Root,
		"diff", "--cached", "--name-only", "-z", workspace.BaselineRef,
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
		"diff", "--cached", "--binary", "--full-index", workspace.BaselineRef,
	)
	if err != nil {
		return fmt.Errorf("build isolated review patch in %s: %w", workspace.Root, err)
	}
	if err := runGitInput(ctx, r.sourceRoot, patch, "apply", "--binary", "--whitespace=nowarn"); err != nil {
		return fmt.Errorf("apply isolated review changes from %s: %w", workspace.Root, err)
	}
	if !autoCommit {
		return nil
	}
	message := strings.TrimSpace(commitMessage)
	if message == "" {
		message = "fix: resolve review batch"
	}
	stageArgs := []string{"add", "-f", "-A", "--"}
	stageArgs = append(stageArgs, paths...)
	if _, err := runGit(ctx, r.sourceRoot, stageArgs...); err != nil {
		return fmt.Errorf("stage integrated review changes from %s: %w", workspace.Root, err)
	}
	args := []string{"commit", "--only", "-m", message, "--"}
	args = append(args, paths...)
	if _, err := runGit(ctx, r.sourceRoot, args...); err != nil {
		return fmt.Errorf("commit integrated review changes from %s: %w", workspace.Root, err)
	}
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
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, message)
	}
	return nil
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
