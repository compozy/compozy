package core_test

import (
	"context"
	"encoding/json/v2"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestAgentInstanceReports(t *testing.T) {
	t.Run("Should page durable reports and reject a cursor from another instance", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instances := db.Instances()
		input := agentinstance.Instance{AgentName: "coder", ProfileID: store.DefaultProfileID, CreatedByKind: "human", PermissionCeiling: "{}"}
		first, _, err := instances.AdmitCreate(t.Context(), input, 10)
		if err != nil {
			t.Fatal(err)
		}
		input.AgentName = "other"
		other, _, err := instances.AdmitCreate(t.Context(), input, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"occ:first", "occ:second"} {
			if _, err := db.InstanceReports().AppendReport(t.Context(), first.ID, agentinstance.ReportInput{SourceKey: key, Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"provider exited"}`)}); err != nil {
				t.Fatal(err)
			}
		}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: instances, Reports: db.InstanceReports(), Authority: agentinstance.Authority{Callers: instances}}
		fixture.Engine.GET("/instances/:id/reports", fixture.Handlers.ListAgentInstanceReports)
		read := func(path string) (int, contract.AgentInstanceReportsResponse) {
			t.Helper()
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), "GET", path, nil))
			var body contract.AgentInstanceReportsResponse
			if response.Code == 200 {
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
			}
			if response.Code >= 400 {
				var failure contract.ErrorPayload
				if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
					t.Fatal(err)
				}
				if failure.Error == "" {
					t.Fatalf("missing error body: %s", response.Body.String())
				}
			}
			return response.Code, body
		}
		status, filtered := read("/instances/" + first.ID + "/reports?state=delivered")
		if status != 200 || len(filtered.Reports) != 0 {
			t.Fatalf("delivered filter: %d %#v", status, filtered)
		}
		status, filtered = read("/instances/" + first.ID + "/reports?state=pending")
		if status != 200 || len(filtered.Reports) != 2 {
			t.Fatalf("pending filter: %d %#v", status, filtered)
		}
		status, _ = read("/instances/" + first.ID + "/reports?state=unknown")
		if status != 422 {
			t.Fatalf("invalid state: %d", status)
		}
		status, page := read("/instances/" + first.ID + "/reports?limit=1")
		if status != 200 || len(page.Reports) != 1 || page.NextCursor == "" {
			t.Fatalf("first page: %d %#v", status, page)
		}
		if page.Reports[0].Detail != "provider exited" {
			t.Fatalf("payload: %#v", page.Reports[0])
		}
		status, _ = read("/instances/" + first.ID + "/reports?state=pending&cursor=" + page.NextCursor)
		if status != 422 {
			t.Fatalf("changed cursor filter: %d", status)
		}
		status, next := read("/instances/" + first.ID + "/reports?limit=1&cursor=" + page.NextCursor)
		if status != 200 || len(next.Reports) != 1 || next.Reports[0].ID == page.Reports[0].ID {
			t.Fatalf("next page: %d %#v", status, next)
		}
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), "GET", "/instances/"+other.ID+"/reports?cursor="+page.NextCursor, nil))
		var denial contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &denial); err != nil {
			t.Fatal(err)
		}
		if response.Code != 422 || denial.Error == "" {
			t.Fatalf("cursor denial: %d %#v", response.Code, denial)
		}
	})
}
