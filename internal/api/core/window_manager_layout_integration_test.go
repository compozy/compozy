//go:build integration

package core

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

// Invariant: legacy documents cross the shared HTTP/UDS boundary as v6 topology.
func TestWindowManagerLegacyLayoutDocuments(t *testing.T) {
	for _, version := range []int{4, 5} {
		t.Run(fmt.Sprintf("Should validate apply and export a v%d floating layout as v6 tabs (IT-012)", version), func(t *testing.T) {
			t.Parallel()
			fixture := newWindowManagerHandlerFixture(t)
			path := windowManagerTestPath("workspace-a")
			document := fmt.Sprintf(`{
				"version":%d,"workspace_id":"workspace-a",
				"desktops":[{"id":"legacy","name":"Saved","order":0,"groups":[],"floating":["one","two"],"floating_stacks":[]}],
				"windows":{
					"one":{"id":"one","app":"session","desktop_id":"legacy","placement":"floating","route":{"pathname":"/sessions/one","search":{}},"nav_stack":[{"pathname":"/sessions","search":{}}],"pinned":true,"floating_rect":{"x":0.1,"y":0.1,"width":0.4,"height":0.6}},
					"two":{"id":"two","app":"terminal","desktop_id":"legacy","placement":"floating","route":{"pathname":"/terminal/two","search":{}},"nav_stack":[],"minimized":true,"floating_rect":{"x":0.5,"y":0.1,"width":0.4,"height":0.6}}
				},"overrides":{"focus_wrap":true}
			}`, version)
			validated := performWindowManagerTestRequest(t, fixture.router, http.MethodPost, path+"/layout/validate",
				`{"workspace_id":"workspace-a","document":`+document+`}`)
			requireWindowManagerStatus(t, validated, http.StatusOK)
			var validation contract.WindowManagerLayoutValidationResponse
			decodeWindowManagerTestBody(t, validated, &validation)
			if !validation.Valid || validation.WorkspaceID != "workspace-a" {
				t.Fatalf("validation = %+v", validation)
			}
			applied := performWindowManagerTestRequest(t, fixture.router, http.MethodPut, path+"/layout",
				`{"workspace_id":"workspace-a","expected_revision":0,"document":`+document+`}`)
			requireWindowManagerStatus(t, applied, http.StatusOK)
			var result contract.WindowManagerResult
			decodeWindowManagerTestBody(t, applied, &result)
			if !result.Applied || result.Snapshot.Version != 6 || len(result.Snapshot.Desktops[0].Tabs) != 2 ||
				len(result.Snapshot.Windows) != 2 || !result.Snapshot.Desktops[0].Tabs[0].Pinned {
				t.Fatalf("apply = %+v", result)
			}
			exported := performWindowManagerTestRequest(t, fixture.router, http.MethodGet, path+"/layout", "")
			requireWindowManagerStatus(t, exported, http.StatusOK)
			var current contract.WindowManagerLayoutDocument
			decodeWindowManagerTestBody(t, exported, &current)
			if current.Version != 6 || len(current.Desktops[0].Tabs) != 2 ||
				current.Windows["one"].Route.Pathname != "/sessions/one" || len(current.Windows["one"].NavStack) != 1 ||
				current.Windows["two"].Route.Pathname != "/terminal/two" ||
				current.Overrides.FocusWrap == nil || !*current.Overrides.FocusWrap {
				t.Fatalf("export = %+v", current)
			}
		})
	}

	// Invariant: every shared transport decoder, including Settings preview, upgrades v4 apps before use.
	t.Run("Should upgrade retired v4 apps through validate preview apply and export (IT-012)", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		path := windowManagerTestPath("workspace-a")
		document := `{
			"version":4,"workspace_id":"workspace-a",
			"desktops":[{"id":"saved","name":"Saved","order":0,"groups":[],"floating":["job","trigger"],"floating_stacks":[]}],
			"windows":{
				"job":{"id":"job","app":"jobs","desktop_id":"saved","placement":"floating","route":{"pathname":"/jobs/morning-digest","search":{}},"nav_stack":[{"pathname":"/jobs","search":{}}]},
				"trigger":{"id":"trigger","app":"triggers","desktop_id":"saved","placement":"floating","route":{"pathname":"/triggers","search":{"event":"session.stopped"}},"nav_stack":[{"pathname":"/triggers/notify","search":{}}]}
			},"overrides":{}
		}`
		assertUpgraded := func(version uint32, windows map[string]contract.WindowManagerWindow) {
			t.Helper()
			job, trigger := windows["job"], windows["trigger"]
			if version != 6 || len(windows) != 2 ||
				job.App != "automations" || job.Route.Pathname != "/automations/jobs/morning-digest" ||
				trigger.App != "automations" || trigger.Route.Pathname != "/automations" ||
				string(trigger.Route.Search["q"]) != `"session.stopped"` ||
				string(trigger.Route.Search["start"]) != `"event"` || trigger.Route.Search["event"] != nil ||
				len(job.NavStack) != 1 || len(trigger.NavStack) != 1 {
				t.Fatalf("version = %d, upgraded windows = %+v", version, windows)
			}
			if job.NavStack[0].Pathname != "/automations" ||
				string(job.NavStack[0].Search["start"]) != `"schedule"` ||
				trigger.NavStack[0].Pathname != "/automations/triggers/notify" {
				t.Fatalf("upgraded navigation stacks = %+v / %+v", job.NavStack, trigger.NavStack)
			}
		}
		validated := performWindowManagerTestRequest(t, fixture.router, http.MethodPost, path+"/layout/validate",
			`{"workspace_id":"workspace-a","document":`+document+`}`)
		requireWindowManagerStatus(t, validated, http.StatusOK)
		var validation contract.WindowManagerLayoutValidationResponse
		decodeWindowManagerTestBody(t, validated, &validation)
		if !validation.Valid || validation.WorkspaceID != "workspace-a" {
			t.Fatalf("validation = %+v", validation)
		}
		previewed := performWindowManagerTestRequest(t, fixture.router, http.MethodPost, path+"/preview",
			windowManagerTestCommandBody("workspace-a", 0, "layout.replace", `{"document":`+document+`}`))
		requireWindowManagerStatus(t, previewed, http.StatusOK)
		var preview contract.WindowManagerPreview
		decodeWindowManagerTestBody(t, previewed, &preview)
		if !preview.Changed {
			t.Fatalf("preview = %+v", preview)
		}
		assertUpgraded(preview.Snapshot.Version, preview.Snapshot.Windows)
		unchanged, err := fixture.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil || unchanged.Revision != 0 || len(unchanged.Windows) != 1 {
			t.Fatalf("snapshot after preview = %+v, error = %v", unchanged, err)
		}
		applied := performWindowManagerTestRequest(t, fixture.router, http.MethodPut, path+"/layout",
			`{"workspace_id":"workspace-a","expected_revision":0,"document":`+document+`}`)
		requireWindowManagerStatus(t, applied, http.StatusOK)
		var result contract.WindowManagerResult
		decodeWindowManagerTestBody(t, applied, &result)
		if !result.Applied {
			t.Fatalf("apply = %+v", result)
		}
		assertUpgraded(result.Snapshot.Version, result.Snapshot.Windows)
		exported := performWindowManagerTestRequest(t, fixture.router, http.MethodGet, path+"/layout", "")
		requireWindowManagerStatus(t, exported, http.StatusOK)
		var current contract.WindowManagerLayoutDocument
		decodeWindowManagerTestBody(t, exported, &current)
		assertUpgraded(current.Version, current.Windows)
	})
}
