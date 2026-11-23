package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Invariant: `agent instance skill-auto-approve on|off` sends exactly one PATCH field,
// skill_auto_approve, prints the resulting state, keeps `-o json` equal to the HTTP payload, and
// surfaces the daemon's operator-only refusal.
// Owning layer: CLI command over a stub daemon.
func TestAgentInstanceSkillAutoApproveCLI(t *testing.T) {
	t.Parallel()
	const instanceJSON = `{"instance":{"id":"ainst-test","agent_name":"triage","label":"triage",` +
		`"checks_enabled":true,"skill_auto_approve":%s}}`
	for _, tc := range []struct {
		name, state, output string
		refuse              bool
		want                string
	}{
		{name: "Should turn it on and say so", state: "on", want: "Skill auto-approve is on for triage.\n"},
		{
			name: "Should print the HTTP payload for off with -o json [IT-011]", state: "off", output: "json",
			want: strings.Replace(instanceJSON, "%s", "false", 1),
		},
		{name: "Should surface the operator-only refusal", state: "on", refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			patches := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/api/agent-instances":
					_, _ = io.WriteString(w, `{"instances":[{"id":"ainst-test","agent_name":"triage"}]}`)
				case r.URL.Path == "/api/agent-instances/ainst-test" && r.Method == http.MethodPatch:
					patches++
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					var body map[string]any
					if err := json.Unmarshal(raw, &body); err != nil || len(body) != 1 ||
						body["skill_auto_approve"] != (tc.state == "on") {
						t.Errorf("PATCH body = %s, want only skill_auto_approve", raw)
					}
					if tc.refuse {
						w.WriteHeader(http.StatusForbidden)
						_, _ = io.WriteString(w, `{"code":"skill_auto_approve_operator_only",`+
							`"error":"only an operator can turn on skill auto-approve"}`)
						return
					}
					_, _ = io.WriteString(w, strings.Replace(instanceJSON, "%s", map[bool]string{true: "true",
						false: "false"}[tc.state == "on"], 1))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
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
			args := []string{"agent", "instance", "skill-auto-approve", tc.state, "triage", "--global"}
			if tc.output != "" {
				args = append(args, "-o", tc.output)
			}
			stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), args...)
			if tc.refuse {
				if err == nil || !strings.Contains(err.Error(), "skill_auto_approve_operator_only") {
					t.Fatalf("error = %v, want skill_auto_approve_operator_only", err)
				}
				return
			}
			if err != nil || patches != 1 {
				t.Fatalf("command err=%v patches=%d, want one PATCH", err, patches)
			}
			if tc.output == "json" {
				var got, want any
				if json.Unmarshal([]byte(stdout), &got) != nil || json.Unmarshal([]byte(tc.want), &want) != nil ||
					!jsonEqual(got, want) {
					t.Fatalf("stdout = %s, want %s", stdout, tc.want)
				}
				return
			}
			if stdout != tc.want {
				t.Fatalf("stdout = %q, want %q", stdout, tc.want)
			}
		})
	}
}

func jsonEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && string(left) == string(right)
}
