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
}
