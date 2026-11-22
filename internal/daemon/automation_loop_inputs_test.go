package daemon

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestParseLoopInputCommandOutput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		output     string
		want       map[string]any
		wantErrSub string
	}{
		{
			name:   "Should parse one input per line",
			output: "slug=catalog-mutability\nreview_round=2\n",
			want:   map[string]any{"slug": "catalog-mutability", "review_round": "2"},
		},
		{
			name:   "Should skip blank lines and normalize CRLF output",
			output: "\n slug=one \r\n\t\nchannel=stable\r\n",
			want:   map[string]any{"slug": "one ", "channel": "stable"},
		},
		{
			name:   "Should keep the last value for a repeated key",
			output: "slug=first\nslug=second\n",
			want:   map[string]any{"slug": "second"},
		},
		{
			name:   "Should preserve raw value text including equals signs",
			output: "spec=a=b=c\nvalue=  spaced  \nempty=\n",
			want:   map[string]any{"spec": "a=b=c", "value": "  spaced  ", "empty": ""},
		},
		{
			name:   "Should report empty output as no inputs",
			output: "  \n\t\r\n",
			want:   nil,
		},
		{
			name:       "Should reject a line without a key value separator",
			output:     "slug=catalog\nnot-an-input\n",
			wantErrSub: "line 2 must be key=value",
		},
		{
			name:       "Should reject a line with an empty key",
			output:     "  =value\n",
			wantErrSub: "line 1 has an empty input key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLoopInputCommandOutput([]byte(tc.output))
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("parseLoopInputCommandOutput() error = nil, want %q", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("parseLoopInputCommandOutput() error = %q, want substring %q", err, tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLoopInputCommandOutput() error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseLoopInputCommandOutput() = %#v, want %#v", got, tc.want)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatalf("parseLoopInputCommandOutput()[%q] = %#v, want %#v", key, got[key], want)
				}
			}
		})
	}
}

func TestAutomationLoopStarterResolveLoopInputs(t *testing.T) {
	// Serial: one subtest mutates the process environment with t.Setenv.

	newStarter := func(t *testing.T, rootDir string) *automationLoopStarter {
		t.Helper()
		return &automationLoopStarter{
			workspaces: &daemonExtensionWorkspaceResolverStub{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws_alpha", RootDir: rootDir},
					WorkspaceID: "ws_alpha",
				},
			},
			commandTimeout: 30 * time.Second,
		}
	}

	request := func(command string) automationpkg.LoopInputResolutionRequest {
		return automationpkg.LoopInputResolutionRequest{
			ProfileID:       "profile-default",
			WorkspaceID:     "ws_alpha",
			LoopName:        "overnight",
			Kind:            automationpkg.LoopStartKindSchedule,
			Command:         command,
			AutomationID:    "job-0123456789abcdef",
			AutomationRunID: "run-0123456789abcdef",
		}
	}

	t.Run("Should run the command in the Loop workspace and parse stdout", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		resolvedRoot, err := filepath.EvalSymlinks(rootDir)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q) error = %v", rootDir, err)
		}
		starter := newStarter(t, rootDir)
		inputs, err := starter.ResolveLoopInputs(
			t.Context(),
			request("printf 'slug=catalog\\nloop=%s\\nworkspace=%s\\n' \"$COMPOZY_LOOP_NAME\" \"$(pwd -P)\""),
		)
		if err != nil {
			t.Fatalf("ResolveLoopInputs() error = %v", err)
		}
		if got, want := inputs["slug"], "catalog"; got != want {
			t.Fatalf("ResolveLoopInputs()[slug] = %#v, want %#v", got, want)
		}
		if got, want := inputs["loop"], "overnight"; got != want {
			t.Fatalf("ResolveLoopInputs()[loop] = %#v, want %#v", got, want)
		}
		if got, want := inputs["workspace"], resolvedRoot; got != want {
			t.Fatalf("ResolveLoopInputs()[workspace] = %#v, want the workspace root %#v", got, want)
		}
	})

	t.Run("Should report no inputs when the command prints only whitespace", func(t *testing.T) {
		t.Parallel()

		starter := newStarter(t, t.TempDir())
		inputs, err := starter.ResolveLoopInputs(t.Context(), request("printf '   \\n'"))
		if err != nil {
			t.Fatalf("ResolveLoopInputs() error = %v", err)
		}
		if len(inputs) != 0 {
			t.Fatalf("ResolveLoopInputs() = %#v, want no inputs", inputs)
		}
	})

	t.Run("Should fail with the bounded stderr when the command exits non-zero", func(t *testing.T) {
		t.Parallel()

		starter := newStarter(t, t.TempDir())
		_, err := starter.ResolveLoopInputs(
			t.Context(),
			request("printf 'queue is locked' >&2; exit 3"),
		)
		if err == nil {
			t.Fatal("ResolveLoopInputs() error = nil, want command failure")
		}
		if !strings.Contains(err.Error(), "queue is locked") {
			t.Fatalf("ResolveLoopInputs() error = %q, want the command stderr", err)
		}
		if !strings.Contains(err.Error(), "exit status 3") {
			t.Fatalf("ResolveLoopInputs() error = %q, want the exit status", err)
		}
	})

	t.Run("Should fail when the command exceeds the timeout", func(t *testing.T) {
		t.Parallel()

		starter := newStarter(t, t.TempDir())
		starter.commandTimeout = 100 * time.Millisecond
		_, err := starter.ResolveLoopInputs(t.Context(), request("sleep 5; printf 'slug=late\\n'"))
		if err == nil {
			t.Fatal("ResolveLoopInputs() error = nil, want timeout")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("ResolveLoopInputs() error = %q, want a timeout reason", err)
		}
	})

	t.Run("Should not inherit the daemon environment", func(t *testing.T) {
		// t.Setenv keeps the enclosing test serial.
		t.Setenv("COMPOZY_TEST_ONLY_SECRET", "leaked")
		starter := newStarter(t, t.TempDir())
		inputs, err := starter.ResolveLoopInputs(
			t.Context(),
			request("printf 'secret=%s\\n' \"${COMPOZY_TEST_ONLY_SECRET:-}\""),
		)
		if err != nil {
			t.Fatalf("ResolveLoopInputs() error = %v", err)
		}
		if got := inputs["secret"]; got != "" {
			t.Fatalf("ResolveLoopInputs()[secret] = %#v, want the daemon environment withheld", got)
		}
	})

	t.Run("Should fail when the workspace resolver is unavailable", func(t *testing.T) {
		t.Parallel()

		starter := &automationLoopStarter{commandTimeout: time.Second}
		_, err := starter.ResolveLoopInputs(t.Context(), request("printf 'slug=catalog\\n'"))
		if err == nil {
			t.Fatal("ResolveLoopInputs() error = nil, want missing resolver failure")
		}
		if !strings.Contains(err.Error(), "workspace resolver") {
			t.Fatalf("ResolveLoopInputs() error = %q, want the missing resolver reason", err)
		}
	})

	t.Run("Should fail when the workspace has no root directory", func(t *testing.T) {
		t.Parallel()

		starter := newStarter(t, "   ")
		_, err := starter.ResolveLoopInputs(t.Context(), request("printf 'slug=catalog\\n'"))
		if err == nil {
			t.Fatal("ResolveLoopInputs() error = nil, want missing root failure")
		}
		if !errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) {
			t.Fatalf("ResolveLoopInputs() error = %v, want ErrWorkspaceRootMissing", err)
		}
	})
}
