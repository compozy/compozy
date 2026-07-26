package workspace_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/workspace"
)

func TestWorkspaceErrorsMatchViaErrorsIs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sentinel error
	}{
		{name: "not found", sentinel: workspace.ErrWorkspaceNotFound},
		{name: "root missing", sentinel: workspace.ErrWorkspaceRootMissing},
		{name: "agent unavailable", sentinel: workspace.ErrAgentNotAvailable},
		{name: "name taken", sentinel: workspace.ErrWorkspaceNameTaken},
		{name: "path taken", sentinel: workspace.ErrWorkspacePathTaken},
		{name: "has sessions", sentinel: workspace.ErrWorkspaceHasSessions},
		{name: "has active sessions", sentinel: workspace.ErrWorkspaceHasActiveSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := fmt.Errorf("workspace api: %w", tt.sentinel)
			if !errors.Is(err, tt.sentinel) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", err, tt.sentinel)
			}
		})
	}
}

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

func TestWorkspaceZeroValues(t *testing.T) {
	t.Parallel()

	var ws workspace.Workspace
	if ws.ID != "" {
		t.Fatalf("Workspace.ID = %q, want empty", ws.ID)
	}
	if ws.RootDir != "" {
		t.Fatalf("Workspace.RootDir = %q, want empty", ws.RootDir)
	}
	if ws.AdditionalDirs != nil {
		t.Fatalf("Workspace.AdditionalDirs = %#v, want nil", ws.AdditionalDirs)
	}
	if ws.Name != "" {
		t.Fatalf("Workspace.Name = %q, want empty", ws.Name)
	}
	if ws.DefaultAgent != "" {
		t.Fatalf("Workspace.DefaultAgent = %q, want empty", ws.DefaultAgent)
	}
	if !ws.CreatedAt.IsZero() {
		t.Fatalf("Workspace.CreatedAt = %v, want zero", ws.CreatedAt)
	}
	if !ws.UpdatedAt.IsZero() {
		t.Fatalf("Workspace.UpdatedAt = %v, want zero", ws.UpdatedAt)
	}
}

func TestResolvedWorkspaceZeroValue(t *testing.T) {
	t.Parallel()

	var resolved workspace.ResolvedWorkspace
	if !reflect.DeepEqual(resolved.Workspace, workspace.Workspace{}) {
		t.Fatalf("ResolvedWorkspace.Workspace = %#v, want zero Workspace", resolved.Workspace)
	}
	if !reflect.DeepEqual(resolved.Config, aghconfig.Config{}) {
		t.Fatalf("ResolvedWorkspace.Config = %#v, want zero Config", resolved.Config)
	}
	if resolved.Agents != nil {
		t.Fatalf("ResolvedWorkspace.Agents = %#v, want nil", resolved.Agents)
	}
	if resolved.AgentDiagnostics != nil {
		t.Fatalf("ResolvedWorkspace.AgentDiagnostics = %#v, want nil", resolved.AgentDiagnostics)
	}
	if resolved.Skills != nil {
		t.Fatalf("ResolvedWorkspace.Skills = %#v, want nil", resolved.Skills)
	}
	if !resolved.ResolvedAt.IsZero() {
		t.Fatalf("ResolvedWorkspace.ResolvedAt = %v, want zero", resolved.ResolvedAt)
	}
}

func TestWorkspaceStructSurface(t *testing.T) {
	t.Parallel()

	type fieldSpec struct {
		name      string
		fieldType reflect.Type
		tag       string
		embedded  bool
	}

	tests := []struct {
		name   string
		target reflect.Type
		fields []fieldSpec
	}{
		{
			name:   "Workspace",
			target: reflect.TypeFor[workspace.Workspace](),
			fields: []fieldSpec{
				{name: "ID", fieldType: reflect.TypeFor[string]()},
				{name: "RootDir", fieldType: reflect.TypeFor[string]()},
				{name: "AdditionalDirs", fieldType: reflect.TypeFor[[]string]()},
				{name: "Name", fieldType: reflect.TypeFor[string]()},
				{name: "DefaultAgent", fieldType: reflect.TypeFor[string]()},
				{name: "SandboxRef", fieldType: reflect.TypeFor[string]()},
				{name: "CreatedAt", fieldType: reflect.TypeFor[time.Time]()},
				{name: "UpdatedAt", fieldType: reflect.TypeFor[time.Time]()},
			},
		},
		{
			name:   "ResolvedWorkspace",
			target: reflect.TypeFor[workspace.ResolvedWorkspace](),
			fields: []fieldSpec{
				{name: "Workspace", fieldType: reflect.TypeFor[workspace.Workspace](), embedded: true},
				{name: "WorkspaceID", fieldType: reflect.TypeFor[string]()},
				{name: "Config", fieldType: reflect.TypeFor[aghconfig.Config]()},
				{name: "Agents", fieldType: reflect.TypeFor[[]aghconfig.AgentDef]()},
				{name: "AgentDiagnostics", fieldType: reflect.TypeFor[[]workspace.AgentDiagnostic]()},
				{name: "Skills", fieldType: reflect.TypeFor[[]workspace.SkillPath]()},
				{name: "Sandbox", fieldType: reflect.TypeFor[sandbox.Resolved]()},
				{name: "ResolvedAt", fieldType: reflect.TypeFor[time.Time]()},
			},
		},
		{
			name:   "SkillPath",
			target: reflect.TypeFor[workspace.SkillPath](),
			fields: []fieldSpec{
				{name: "Dir", fieldType: reflect.TypeFor[string]()},
				{name: "Source", fieldType: reflect.TypeFor[string]()},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got, want := tt.target.NumField(), len(tt.fields); got != want {
				t.Fatalf("%s field count = %d, want %d", tt.name, got, want)
			}

			for idx, wantField := range tt.fields {
				field := tt.target.Field(idx)
				if field.Name != wantField.name {
					t.Fatalf("%s field %d name = %q, want %q", tt.name, idx, field.Name, wantField.name)
				}
				if field.Type != wantField.fieldType {
					t.Fatalf("%s field %q type = %v, want %v", tt.name, field.Name, field.Type, wantField.fieldType)
				}
				if field.Tag != reflect.StructTag(wantField.tag) {
					t.Fatalf("%s field %q tag = %q, want %q", tt.name, field.Name, field.Tag, wantField.tag)
				}
				if field.Anonymous != wantField.embedded {
					t.Fatalf(
						"%s field %q embedded = %t, want %t",
						tt.name,
						field.Name,
						field.Anonymous,
						wantField.embedded,
					)
				}
			}
		})
	}
}
