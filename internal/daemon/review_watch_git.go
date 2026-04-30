package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ReviewWatchGit is the narrow git boundary used by daemon review-watch runs.
type ReviewWatchGit interface {
	State(ctx context.Context, workspaceRoot string) (ReviewWatchGitState, error)
	Push(ctx context.Context, workspaceRoot string, remote string, branch string) error
}

// ReviewWatchGitState captures read-only repository state relevant to watch push safety.
type ReviewWatchGitState struct {
	Branch          string
	HeadSHA         string
	UpstreamRemote  string
	UpstreamBranch  string
	Dirty           bool
	UnpushedCommits int
}

type reviewWatchGitCommandRunner func(ctx context.Context, workspaceRoot string, args ...string) (string, error)

type execReviewWatchGit struct {
	run reviewWatchGitCommandRunner
}

var _ ReviewWatchGit = (*execReviewWatchGit)(nil)

func newExecReviewWatchGit() *execReviewWatchGit {
	return &execReviewWatchGit{run: runReviewWatchGitCommand}
}

func (g *execReviewWatchGit) State(
	ctx context.Context,
	workspaceRoot string,
) (ReviewWatchGitState, error) {
	if g == nil || g.run == nil {
		return ReviewWatchGitState{}, errors.New("review watch git runner is required")
	}
	workspace := strings.TrimSpace(workspaceRoot)
	if workspace == "" {
		return ReviewWatchGitState{}, errors.New("review watch git workspace is required")
	}

	branch, err := g.run(ctx, workspace, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ReviewWatchGitState{}, fmt.Errorf("inspect git branch: %w", err)
	}
	head, err := g.run(ctx, workspace, "rev-parse", "HEAD")
	if err != nil {
		return ReviewWatchGitState{}, fmt.Errorf("inspect git head: %w", err)
	}
	status, err := g.run(ctx, workspace, "status", "--porcelain")
	if err != nil {
		return ReviewWatchGitState{}, fmt.Errorf("inspect git status: %w", err)
	}

	state := ReviewWatchGitState{
		Branch:  strings.TrimSpace(branch),
		HeadSHA: strings.TrimSpace(head),
		Dirty:   strings.TrimSpace(status) != "",
	}
	if upstream, upstreamErr := g.run(
		ctx,
		workspace,
		"rev-parse",
		"--abbrev-ref",
		"--symbolic-full-name",
		"@{u}",
	); upstreamErr == nil {
		state.UpstreamRemote, state.UpstreamBranch = splitGitUpstream(strings.TrimSpace(upstream))
		if count, countErr := g.run(ctx, workspace, "rev-list", "--count", "@{u}..HEAD"); countErr == nil {
			state.UnpushedCommits = parseGitCount(count)
		}
	}
	return state, nil
}

func (g *execReviewWatchGit) Push(
	ctx context.Context,
	workspaceRoot string,
	remote string,
	branch string,
) error {
	if g == nil || g.run == nil {
		return errors.New("review watch git runner is required")
	}
	workspace := strings.TrimSpace(workspaceRoot)
	if workspace == "" {
		return errors.New("review watch git workspace is required")
	}
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" || branch == "" {
		return errors.New("review watch git push requires remote and branch")
	}
	if _, err := g.run(ctx, workspace, "push", remote, "HEAD:"+branch); err != nil {
		return fmt.Errorf("git push %s HEAD:%s: %w", remote, branch, err)
	}
	return nil
}

func runReviewWatchGitCommand(ctx context.Context, workspaceRoot string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", strings.TrimSpace(workspaceRoot)}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func splitGitUpstream(value string) (string, string) {
	remote, branch, ok := strings.Cut(strings.TrimSpace(value), "/")
	if !ok {
		return "", ""
	}
	return strings.TrimSpace(remote), strings.TrimSpace(branch)
}

func parseGitCount(value string) int {
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || count < 0 {
		return 0
	}
	return count
}
