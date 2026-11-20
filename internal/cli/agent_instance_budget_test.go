package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestAgentInstanceBudgetCLI(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		flags    []string
		previous contract.AgentInstanceDailyBudgetLimits
		want     contract.AgentInstanceDailyBudgetInput
		wantErr  string
	}{
		{name: "Should configure two independent ceilings atomically", flags: []string{"--tokens", "100", "--usd", "1.25"},
			previous: contract.AgentInstanceDailyBudgetLimits{USD: new(float64(1.25))},
			want:     contract.AgentInstanceDailyBudgetInput{Tokens: new(int64(100)), USD: new(float64(1.25))}},
		{name: "Should set one ceiling with the other explicitly unlimited", flags: []string{"--tokens", "50", "--usd", "off"},
			previous: contract.AgentInstanceDailyBudgetLimits{Tokens: new(int64(50)), USD: new(float64(1.25))},
			want:     contract.AgentInstanceDailyBudgetInput{Tokens: new(int64(50))}},
		{name: "Should clear all ceilings", flags: []string{"--clear"},
			previous: contract.AgentInstanceDailyBudgetLimits{Tokens: new(int64(50)), USD: new(float64(1.25))}},
		{name: "Should reject missing change", wantErr: "specify both --tokens and --usd"},
		{name: "Should reject partial stale update", flags: []string{"--tokens", "100"}, wantErr: "specify both --tokens and --usd"},
		{name: "Should reject negative token cap", flags: []string{"--tokens", "-2", "--usd", "off"}, wantErr: "non-negative"},
		{name: "Should reject unknown USD", flags: []string{"--usd", "abc", "--tokens", "off"}, wantErr: "invalid USD"},
		{name: "Should reject clear with updates", flags: []string{"--clear", "--usd", "1"}, wantErr: "cannot combine"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			patches := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/agent-instances":
					if _, err := io.WriteString(
						w,
						`{"instances":[{"id":"ainst-test","agent_name":"triage"}]}`,
					); err != nil {
						t.Error(err)
					}
				case "/api/agent-instances/ainst-test":
					if r.Method == http.MethodPatch {
						patches++
						var request contract.AgentInstanceUpdateRequest
						if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
							t.Error(err)
						}
						if request.DailyBudget == nil || !reflect.DeepEqual(*request.DailyBudget, tc.want) {
							t.Errorf("daily budget request=%+v, want %+v", request.DailyBudget, tc.want)
						}
					}
					body, err := json.Marshal(contract.AgentInstanceResponse{Instance: contract.AgentInstancePayload{
						ID: "ainst-test", AgentName: "triage", DailyBudget: &tc.previous,
					}})
					if err != nil {
						t.Error(err)
					}
					if _, err := w.Write(body); err != nil {
						t.Error(err)
					}
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
			args := append([]string{"agent", "instance", "budget", "triage", "--global"}, tc.flags...)
			_, _, err = executeRootCommand(t, newWorkspaceTestDeps(t, client), args...)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error=%v, want %q", err, tc.wantErr)
				}
				if patches != 0 {
					t.Fatalf("invalid budget request mutated instance %d times", patches)
				}
				return
			}
			if err != nil || patches != 1 {
				t.Fatalf("budget command err=%v patches=%d, want one PATCH", err, patches)
			}
		})
	}
}
