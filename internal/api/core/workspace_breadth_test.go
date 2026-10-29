package core

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/gin-gonic/gin"
)

func TestWorkspaceBreadth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, query, route, selector, code string
		omission                           bool
		want                               WorkspaceBreadth
	}{
		{name: "Should require an explicit selection", selector: "workspace_id", code: "workspace_breadth_invalid"},
		{name: "Should keep omission population", selector: "workspace", omission: true, want: WorkspaceBreadth{AllWorkspaces: true}},
		{name: "Should select a project", query: "workspace=ws-a", selector: "workspace", want: WorkspaceBreadth{WorkspaceID: "ws-a"}},
		{name: "Should reject conflicting selectors", query: "all_workspaces=true&workspace=ws-a", selector: "workspace", code: "workspace_breadth_invalid"},
		{name: "Should reject a path aggregate", query: "all_workspaces=true", route: "ws-a", selector: "workspace_id", code: "workspace_breadth_invalid"},
		{name: "Should reject mismatched path selection", query: "workspace_id=ws-b", route: "ws-a", selector: "workspace_id", code: "workspace_breadth_invalid"},
		{name: "Should reject reserved selector", query: "workspace=global", selector: "workspace", code: "workspace_reserved"},
		{name: "Should reject reserved filter", query: "all_workspaces=true&workspace_ids=global", selector: "workspace", code: "workspace_reserved"},
		{name: "Should reject filters without explicit all", query: "workspace_ids=ws-a", selector: "workspace", omission: true, code: "workspace_breadth_invalid"},
		{name: "Should reject empty filter", query: "all_workspaces=true&workspace_ids=", selector: "workspace", code: "workspace_breadth_invalid"},
		{name: "Should normalize repeated filters", query: "all_workspaces=true&workspace_ids=b&workspace_ids=+a+&workspace_ids=b&workspace_ids=@user", selector: "workspace", want: WorkspaceBreadth{AllWorkspaces: true, WorkspaceIDs: []string{workspaceaccess.UserSelector, "a", "b"}}},
		{name: "Should reject malformed boolean", query: "all_workspaces=garbage", selector: "workspace", code: "workspace_breadth_invalid"},
		{name: "Should reject repeated boolean", query: "all_workspaces=true&all_workspaces=false", selector: "workspace", code: "workspace_breadth_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequestWithContext(t.Context(), "GET", "/?"+tc.query, nil)
			got, err := parseWorkspaceBreadth(
				c,
				tc.route,
				breadthEncoding{Selector: tc.selector, OmissionIsAll: tc.omission},
			)
			if tc.code != "" {
				typed, ok := errors.AsType[*workspaceBreadthError](err)
				if !ok || typed.code != tc.code {
					t.Fatalf("error = %v, want %s", err, tc.code)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got.WorkspaceIDs) == 0 {
				got.WorkspaceIDs = nil
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("breadth = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestWorkspaceBreadthLegacyGlobalAlias(t *testing.T) {
	t.Run("Should resolve the migration alias before applying data filters", func(t *testing.T) {
		t.Parallel()
		getter := workspaceGetterStub{get: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref != "global" {
				t.Fatalf("ref=%q", ref)
			}
			return workspacepkg.Workspace{ID: "ws-renamed", Name: "global-project"}, nil
		}}
		for _, query := range []string{"workspace=global", "all_workspaces=true&workspace_ids=global"} {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequestWithContext(t.Context(), "GET", "/?"+query, nil)
			got, err := parseWorkspaceBreadth(c, "", breadthEncoding{Selector: "workspace"}, getter)
			if err != nil {
				t.Fatal(err)
			}
			if got.AllWorkspaces {
				if len(got.WorkspaceIDs) != 1 || got.WorkspaceIDs[0] != "ws-renamed" {
					t.Fatalf("breadth=%#v", got)
				}
			} else if got.WorkspaceID != "ws-renamed" {
				t.Fatalf("breadth=%#v", got)
			}
		}
	})
}

func TestWorkspaceBreadthArchivedProjects(t *testing.T) {
	t.Parallel()
	now := time.Now()
	catalog := workspaceArchiveCatalogStub{items: []workspacepkg.Workspace{
		{ID: "ws-active"}, {ID: "ws-archived", ArchivedAt: &now},
	}}
	for _, tc := range []struct {
		name  string
		query string
		want  []string
	}{
		{"Should exclude archived projects from explicit all scope", "all_workspaces=true", []string{"@user", "ws-active"}},
		{"Should exclude archived projects from implicit all scope", "", []string{"@user", "ws-active"}},
		{"Should retain explicitly selected archived history", "all_workspaces=true&workspace_ids=ws-archived", []string{"ws-archived"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequestWithContext(t.Context(), "GET", "/?"+tc.query, nil)
			got, err := parseWorkspaceBreadth(
				c,
				"",
				breadthEncoding{Selector: "workspace", OmissionIsAll: true},
				catalog,
			)
			if err != nil || !reflect.DeepEqual(got.WorkspaceIDs, tc.want) {
				t.Fatalf("breadth=%#v err=%v", got, err)
			}
		})
	}
}

type workspaceArchiveCatalogStub struct {
	workspaceGetterStub
	items []workspacepkg.Workspace
}

func (s workspaceArchiveCatalogStub) List(context.Context) ([]workspacepkg.Workspace, error) {
	return s.items, nil
}
