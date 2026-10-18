package workspace_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/compozy/compozy/internal/workspace"
)

func TestWorkspaceErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		left error
		want error
	}{
		{
			name: "not found does not match root missing",
			left: workspace.ErrWorkspaceNotFound,
			want: workspace.ErrWorkspaceRootMissing,
		},
		{
			name: "not found does not match agent unavailable",
			left: workspace.ErrWorkspaceNotFound,
			want: workspace.ErrAgentNotAvailable,
		},
		{
			name: "name taken does not match path taken",
			left: workspace.ErrWorkspaceNameTaken,
			want: workspace.ErrWorkspacePathTaken,
		},
		{
			name: "path taken does not match has sessions",
			left: workspace.ErrWorkspacePathTaken,
			want: workspace.ErrWorkspaceHasSessions,
		},
		{
			name: "has sessions does not match has active sessions",
			left: workspace.ErrWorkspaceHasSessions,
			want: workspace.ErrWorkspaceHasActiveSessions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := fmt.Errorf("wrapped: %w", tt.left)
			if errors.Is(err, tt.want) {
				t.Fatalf("errors.Is(%v, %v) = true, want false", err, tt.want)
			}
		})
	}
}

func TestUniqueWorkspaceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rootDir string
		taken   map[string]struct{}
		want    string
	}{
		{
			name:    "uses base directory name",
			rootDir: "/tmp/project",
			taken:   map[string]struct{}{},
			want:    "project",
		},
		{
			name:    "deduplicates taken name",
			rootDir: "/tmp/project",
			taken:   map[string]struct{}{"project": {}},
			want:    "project-2",
		},
		{
			name:    "falls back for blankish path",
			rootDir: " / ",
			taken:   map[string]struct{}{"workspace": {}},
			want:    "workspace-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := workspace.UniqueWorkspaceName(tt.rootDir, tt.taken); got != tt.want {
				t.Fatalf("UniqueWorkspaceName(%q) = %q, want %q", tt.rootDir, got, tt.want)
			}
		})
	}
}
