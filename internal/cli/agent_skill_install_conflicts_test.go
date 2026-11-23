package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

// Invariant: `agent skill install-conflict` forwards list and resolve to the operator routes and
// prints what was kept. Owning layer: CLI transport; canonical suite: TestAgentSkillInstallConflictCommands.
func TestAgentSkillInstallConflictCommands(t *testing.T) {
	run := func(t *testing.T, handler http.HandlerFunc, args ...string) (string, error) {
		t.Helper()
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		target, err := url.Parse(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		client := &daemonClient{
			target: LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
				return server.Client().Transport.RoundTrip(r)
			})},
		}
		output, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client),
			append([]string{"agent", "skill", "install-conflict"}, args...)...)
		return output, err
	}
	t.Run("Should list conflicts as a table [IT-019]", func(t *testing.T) {
		t.Parallel()
		output, err := run(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/agent-skill-install-conflicts" {
				t.Errorf("list request: %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(w, `{"conflicts":[{"op_id":"sop_01JCA2W8NQ","skill":"release-checklist",`+
				`"action":"install","state":"conflict","since":"2026-11-05T15:20:11Z"}]}`); err != nil {
				t.Error(err)
			}
		}, "list", "--workspace", "@user")
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"OP", "SINCE", "sop_01JCA2W8NQ", "release-checklist", "conflict",
			"2026-11-05T15:20:11Z"} {
			if !strings.Contains(output, want) {
				t.Fatalf("list output missing %q:\n%s", want, output)
			}
		}
	})
	t.Run("Should resolve with the chosen keep and say where the edited copy went [IT-019]", func(t *testing.T) {
		t.Parallel()
		output, err := run(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost ||
				r.URL.Path != "/api/agent-skill-install-conflicts/sop_01JCA2W8NQ/resolve" {
				t.Errorf("resolve request: %s %s", r.Method, r.URL)
			}
			var input contract.AgentSkillInstallConflictResolveRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Keep != "previous" {
				t.Errorf("resolve body = %+v, %v", input, err)
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(w, `{"op_id":"sop_01JCA2W8NQ","skill":"release-checklist","keep":"previous",`+
				`"kept_copy":"/work/.compozy/skills/.skill-conflict-sop_01JCA2W8NQ"}`); err != nil {
				t.Error(err)
			}
		}, "resolve", "sop_01JCA2W8NQ", "--keep", "previous", "--workspace", "@user")
		if err != nil {
			t.Fatal(err)
		}
		want := "Restored the previous release-checklist. The edited copy is in " +
			"/work/.compozy/skills/.skill-conflict-sop_01JCA2W8NQ/.\n"
		if output != want {
			t.Fatalf("resolve output = %q, want %q", output, want)
		}
	})
	t.Run("Should refuse a missing keep before calling the daemon", func(t *testing.T) {
		t.Parallel()
		_, err := run(t, func(_ http.ResponseWriter, r *http.Request) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}, "resolve", "sop_01JCA2W8NQ")
		if err == nil || !strings.Contains(err.Error(), "--keep must be current or previous") {
			t.Fatalf("resolve without --keep = %v", err)
		}
	})
}
