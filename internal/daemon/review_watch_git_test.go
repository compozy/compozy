package daemon

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReviewWatchGitStateReadsOnlyRepositoryState(t *testing.T) {
	t.Run("Should read repository state without destructive git commands", func(t *testing.T) {
		t.Parallel()

		var calls [][]string
		git := &execReviewWatchGit{
			run: func(_ context.Context, workspaceRoot string, args ...string) (string, error) {
				if workspaceRoot != "/repo" {
					t.Fatalf("workspaceRoot = %q, want /repo", workspaceRoot)
				}
				calls = append(calls, append([]string(nil), args...))
				switch strings.Join(args, " ") {
				case "rev-parse --abbrev-ref HEAD":
					return "feature\n", nil
				case "rev-parse HEAD":
					return "head-123\n", nil
				case "status --porcelain":
					return " M internal/app.go\n", nil
				case "rev-parse --abbrev-ref --symbolic-full-name @{u}":
					return "origin/feature\n", nil
				case "rev-list --count @{u}..HEAD":
					return "2\n", nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}

		state, err := git.State(context.Background(), "/repo")
		if err != nil {
			t.Fatalf("State() error = %v", err)
		}
		if state.Branch != "feature" || state.HeadSHA != "head-123" || !state.Dirty ||
			state.UpstreamRemote != "origin" || state.UpstreamBranch != "feature" || state.UnpushedCommits != 2 {
			t.Fatalf("unexpected state: %#v", state)
		}
		for _, call := range calls {
			if len(call) > 0 && isReviewWatchDestructiveGitVerb(call[0]) {
				t.Fatalf("State() used destructive git command: %v", call)
			}
		}
	})
}

func TestReviewWatchGitPushUsesOnlyAllowedCommandShape(t *testing.T) {
	t.Run("Should use only the allowed git push command shape", func(t *testing.T) {
		t.Parallel()

		var calls [][]string
		git := &execReviewWatchGit{
			run: func(_ context.Context, workspaceRoot string, args ...string) (string, error) {
				if workspaceRoot != "/repo" {
					t.Fatalf("workspaceRoot = %q, want /repo", workspaceRoot)
				}
				calls = append(calls, append([]string(nil), args...))
				return "", nil
			},
		}

		if err := git.Push(context.Background(), "/repo", "origin", "feature"); err != nil {
			t.Fatalf("Push() error = %v", err)
		}
		want := [][]string{{"push", "origin", "HEAD:feature"}}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("git calls = %#v, want %#v", calls, want)
		}
	})
}

func TestReviewWatchGitPushRejectsMissingTarget(t *testing.T) {
	t.Run("Should reject a git push target with missing remote or branch", func(t *testing.T) {
		t.Parallel()

		git := &execReviewWatchGit{
			run: func(context.Context, string, ...string) (string, error) {
				t.Fatal("git command should not run when push target is incomplete")
				return "", nil
			},
		}
		if err := git.Push(context.Background(), "/repo", "", "feature"); err == nil ||
			!strings.Contains(err.Error(), "review watch git push requires remote and branch") {
			t.Fatalf("Push() error = %v, want missing target validation error", err)
		}
	})
}

func TestReviewWatchGitPushWrapsCommandFailure(t *testing.T) {
	t.Run("Should wrap git command failures during push", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("remote rejected")
		git := &execReviewWatchGit{
			run: func(context.Context, string, ...string) (string, error) {
				return "", wantErr
			},
		}
		err := git.Push(context.Background(), "/repo", "origin", "feature")
		if !errors.Is(err, wantErr) {
			t.Fatalf("Push() error = %v, want wrapped %v", err, wantErr)
		}
	})
}

func TestReviewWatchGitCommandRunnerAndParsers(t *testing.T) {
	t.Run("Should execute git commands and parse helper values", func(t *testing.T) {
		t.Parallel()

		git := newExecReviewWatchGit()
		output, err := git.run(context.Background(), t.TempDir(), "version")
		if err != nil {
			t.Fatalf("git version command error = %v", err)
		}
		if !strings.Contains(output, "git version") {
			t.Fatalf("git version output = %q, want git version", output)
		}
		if _, err := git.run(context.Background(), t.TempDir(), "not-a-real-git-command"); err == nil {
			t.Fatal("invalid git command error = nil, want error")
		}

		remote, branch := splitGitUpstream(" origin/feature/reviews-watch ")
		if remote != "origin" || branch != "feature/reviews-watch" {
			t.Fatalf("splitGitUpstream() = %q %q, want origin feature/reviews-watch", remote, branch)
		}
		remote, branch = splitGitUpstream("feature")
		if remote != "" || branch != "" {
			t.Fatalf("splitGitUpstream(no remote) = %q %q, want empty", remote, branch)
		}
		if got := parseGitCount(" 3\n"); got != 3 {
			t.Fatalf("parseGitCount(valid) = %d, want 3", got)
		}
		if got := parseGitCount("-1"); got != 0 {
			t.Fatalf("parseGitCount(negative) = %d, want 0", got)
		}
		if got := parseGitCount("bad"); got != 0 {
			t.Fatalf("parseGitCount(invalid) = %d, want 0", got)
		}
	})
}

func TestReviewWatchGitStateWithoutUpstreamStillReportsHead(t *testing.T) {
	t.Run("Should still report head state when no upstream is configured", func(t *testing.T) {
		t.Parallel()

		git := &execReviewWatchGit{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --abbrev-ref HEAD":
					return "feature\n", nil
				case "rev-parse HEAD":
					return "head-123\n", nil
				case "status --porcelain":
					return "\n", nil
				case "rev-parse --abbrev-ref --symbolic-full-name @{u}":
					return "", errors.New("no upstream")
				default:
					t.Fatalf("unexpected git args: %v", args)
					return "", nil
				}
			},
		}

		state, err := git.State(context.Background(), "/repo")
		if err != nil {
			t.Fatalf("State() error = %v", err)
		}
		if state.Branch != "feature" || state.HeadSHA != "head-123" || state.Dirty ||
			state.UpstreamRemote != "" || state.UpstreamBranch != "" || state.UnpushedCommits != 0 {
			t.Fatalf("unexpected state without upstream: %#v", state)
		}
	})
}

func TestReviewWatchGitStateValidatesRunnerWorkspaceAndRequiredReads(t *testing.T) {
	t.Run("Should validate the git runner and workspace before state reads", func(t *testing.T) {
		t.Parallel()

		if _, err := (*execReviewWatchGit)(nil).State(context.Background(), "/repo"); err == nil ||
			!strings.Contains(err.Error(), "review watch git runner is required") {
			t.Fatalf("nil State() error = %v, want missing runner validation", err)
		}
		if _, err := (&execReviewWatchGit{}).State(context.Background(), "/repo"); err == nil ||
			!strings.Contains(err.Error(), "review watch git runner is required") {
			t.Fatalf("missing runner State() error = %v, want missing runner validation", err)
		}
		if _, err := (&execReviewWatchGit{run: func(context.Context, string, ...string) (string, error) {
			t.Fatal("git command should not run for empty workspace")
			return "", nil
		}}).State(context.Background(), " "); err == nil ||
			!strings.Contains(err.Error(), "review watch git workspace is required") {
			t.Fatalf("empty workspace State() error = %v, want workspace validation", err)
		}
	})

	requiredReads := []struct {
		name string
		call string
	}{
		{
			name: "Should wrap branch read failures",
			call: "rev-parse --abbrev-ref HEAD",
		},
		{
			name: "Should wrap head read failures",
			call: "rev-parse HEAD",
		},
		{
			name: "Should wrap status read failures",
			call: "status --porcelain",
		},
	}
	for _, tc := range requiredReads {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantErr := errors.New("read failed")
			git := &execReviewWatchGit{
				run: func(_ context.Context, _ string, args ...string) (string, error) {
					call := strings.Join(args, " ")
					if call == tc.call {
						return "", wantErr
					}
					switch call {
					case "rev-parse --abbrev-ref HEAD":
						return "feature", nil
					case "rev-parse HEAD":
						return "head-123", nil
					case "status --porcelain":
						return "", nil
					default:
						return "", errors.New("stop before optional reads")
					}
				},
			}
			if _, err := git.State(context.Background(), "/repo"); !errors.Is(err, wantErr) {
				t.Fatalf("State() error = %v, want wrapped %v", err, wantErr)
			}
		})
	}
}

func TestReviewWatchGitPushValidatesRunnerAndWorkspace(t *testing.T) {
	t.Run("Should validate the git runner and workspace before push", func(t *testing.T) {
		t.Parallel()

		if err := (*execReviewWatchGit)(nil).Push(context.Background(), "/repo", "origin", "feature"); err == nil ||
			!strings.Contains(err.Error(), "review watch git runner is required") {
			t.Fatalf("nil Push() error = %v, want missing runner validation", err)
		}
		if err := (&execReviewWatchGit{}).Push(context.Background(), "/repo", "origin", "feature"); err == nil ||
			!strings.Contains(err.Error(), "review watch git runner is required") {
			t.Fatalf("missing runner Push() error = %v, want missing runner validation", err)
		}
		err := (&execReviewWatchGit{run: func(context.Context, string, ...string) (string, error) {
			t.Fatal("git command should not run for empty workspace")
			return "", nil
		}}).Push(context.Background(), " ", "origin", "feature")
		if err == nil || !strings.Contains(err.Error(), "review watch git workspace is required") {
			t.Fatalf("empty workspace Push() error = %v, want workspace validation", err)
		}
	})
}

func isReviewWatchDestructiveGitVerb(verb string) bool {
	switch strings.TrimSpace(verb) {
	case "restore", "checkout", "reset", "clean", "rm", "add":
		return true
	default:
		return false
	}
}
