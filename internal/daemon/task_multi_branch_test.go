package daemon

import (
	"context"
	"strings"
	"testing"

	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
)

// Suite: parallel task-group branch rendering
// Invariant: every returned group branch is valid, deterministic, and unique within its launch.
// Boundary IN: token expansion, sanitization, uniqueness adjustment, and real Git ref validation.
// Boundary OUT: branch allocation and collision checks against existing repository refs.

func TestRenderResultBranches(t *testing.T) {
	t.Parallel()
	requireGitForTaskMulti(t)

	tests := []struct {
		name         string
		input        BranchRenderInput
		want         map[string]string
		wantAdjusted bool
		wantErr      string
	}{
		{
			name: "UT-010 Should render unique names from the default template",
			input: BranchRenderInput{
				Template:   workspacecfg.DefaultParallelTaskGroupsBranchTemplate,
				Initiative: "Init",
				RunSegment: "a3f9c2b1",
				Groups: []RenderedGroupContext{
					{ID: "TG-001", Directory: "001-oauth-store", Index: 1},
					{ID: "TG-002", Directory: "002-token-rotate", Index: 2},
				},
			},
			want: map[string]string{
				"TG-001": "compozy/init-001-oauth-store-a3f9c2b1",
				"TG-002": "compozy/init-002-token-rotate-a3f9c2b1",
			},
		},
		{
			name: "UT-011 Should sanitize the stable task group token",
			input: BranchRenderInput{
				Template: "{group}",
				Groups:   []RenderedGroupContext{{ID: "TG-001"}},
			},
			want: map[string]string{"TG-001": "tg-001"},
		},
		{
			name: "UT-012 Should fall back to the group id when the directory has no brief",
			input: BranchRenderInput{
				Template: "{group_brief}",
				Groups: []RenderedGroupContext{{
					ID:        "TG-003",
					Directory: "_task_groups/TG-003",
				}},
			},
			want: map[string]string{"TG-003": "tg-003"},
		},
		{
			name: "UT-013 Should append every group token when the template collides",
			input: BranchRenderInput{
				Template:   "compozy/{initiative}",
				Initiative: "Init",
				Groups: []RenderedGroupContext{
					{ID: "TG-001"},
					{ID: "TG-002"},
				},
			},
			want: map[string]string{
				"TG-001": "compozy/init-tg-001",
				"TG-002": "compozy/init-tg-002",
			},
			wantAdjusted: true,
		},
		{
			name: "UT-014 Should reject an invalid Git ref without returning allocations",
			input: BranchRenderInput{
				Template: "compozy/..bad",
				Groups:   []RenderedGroupContext{{ID: "TG-004"}},
			},
			wantErr: "TG-004",
		},
		{
			name: "UT-015 Should preserve literal template text",
			input: BranchRenderInput{
				Template: "feat/{group_brief}",
				Groups: []RenderedGroupContext{{
					ID:        "TG-001",
					Directory: "001-OAuth Store",
				}},
			},
			want: map[string]string{"TG-001": "feat/001-oauth-store"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			branches, adjusted, err := RenderResultBranches(context.Background(), t.TempDir(), test.input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("RenderResultBranches() error = %v, want group %q", err, test.wantErr)
				}
				if branches != nil {
					t.Fatalf("RenderResultBranches() branches = %#v, want nil on validation failure", branches)
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderResultBranches() error = %v", err)
			}
			if adjusted != test.wantAdjusted {
				t.Fatalf("uniquenessAdjusted = %v, want %v", adjusted, test.wantAdjusted)
			}
			if len(branches) != len(test.want) {
				t.Fatalf("branches = %#v, want %#v", branches, test.want)
			}
			for groupID, want := range test.want {
				if got := branches[groupID]; got != want {
					t.Fatalf("branch[%s] = %q, want %q", groupID, got, want)
				}
			}
		})
	}
}

func TestRenderResultBranchesRunToken(t *testing.T) {
	t.Parallel()
	requireGitForTaskMulti(t)

	group := []RenderedGroupContext{{ID: "TG-001"}}
	t.Run("UT-016 Should vary across runs only when the template includes run", func(t *testing.T) {
		t.Parallel()
		withRun := BranchRenderInput{
			Template:   "compozy/{group}-{run}",
			Initiative: "init",
			Groups:     group,
		}
		withRun.RunSegment = "tasks-init-20260723-11111111"
		first, _, err := RenderResultBranches(context.Background(), t.TempDir(), withRun)
		if err != nil {
			t.Fatalf("first RenderResultBranches() error = %v", err)
		}
		withRun.RunSegment = "tasks-init-20260723-22222222"
		second, _, err := RenderResultBranches(context.Background(), t.TempDir(), withRun)
		if err != nil {
			t.Fatalf("second RenderResultBranches() error = %v", err)
		}
		if first["TG-001"] == second["TG-001"] {
			t.Fatalf("branches across runs = %q, want distinct names", first["TG-001"])
		}

		withoutRun := withRun
		withoutRun.Template = "compozy/{initiative}-{group}"
		stableFirst, _, err := RenderResultBranches(context.Background(), t.TempDir(), withoutRun)
		if err != nil {
			t.Fatalf("stable first RenderResultBranches() error = %v", err)
		}
		withoutRun.RunSegment = ""
		stableSecond, _, err := RenderResultBranches(context.Background(), t.TempDir(), withoutRun)
		if err != nil {
			t.Fatalf("stable second RenderResultBranches() error = %v", err)
		}
		if stableFirst["TG-001"] != stableSecond["TG-001"] {
			t.Fatalf(
				"branches without run token = %q and %q, want stable name",
				stableFirst["TG-001"],
				stableSecond["TG-001"],
			)
		}
	})

	t.Run("UT-018 Should render exactly eight sanitized run characters", func(t *testing.T) {
		t.Parallel()
		branches, _, err := RenderResultBranches(context.Background(), t.TempDir(), BranchRenderInput{
			Template:   "compozy/{run}",
			RunSegment: "tasks-init-20260723-abcdef1234",
			Groups:     group,
		})
		if err != nil {
			t.Fatalf("RenderResultBranches() error = %v", err)
		}
		runSegment := strings.TrimPrefix(branches["TG-001"], "compozy/")
		if len(runSegment) != taskMultiResultBranchRunSegmentLength {
			t.Fatalf("run segment length = %d, want %d", len(runSegment), taskMultiResultBranchRunSegmentLength)
		}
		if runSegment != "abcdef12" {
			t.Fatalf("run segment = %q, want %q", runSegment, "abcdef12")
		}
	})
}
