//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/contract"
	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	builtin "github.com/compozy/compozy/internal/tools/builtin"
)

// Native result views own these CLI/runtime journeys; unit projection assertions
// remain in the native contract suite. The optional manifest binds a deliberate QA lab.
func TestDaemonE2ENativeResultViews(t *testing.T) {
	acpmock.RequireDriver(t)
	opts := &e2etest.RuntimeHarnessOptions{MockAgents: []e2etest.MockAgentSpec{{
		FixturePath: mockFixturePath(t, "multi_agent_fixture.json"), FixtureAgent: "alpha", AgentName: "general",
	}}, ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *config.Config) { cfg.Roles.AutoTitle.Enabled = false; cfg.ProviderUsage.Enabled = false }}}
	for _, d := range builtin.NativeDescriptors() {
		opts.MockAgents[0].Tools = append(opts.MockAgents[0].Tools, d.ID.String())
	}
	if path := os.Getenv("COMPOZY_S10_LAB_MANIFEST"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Env map[string]string `json:"env"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		opts.HomePaths, err = config.ResolveHomePathsFrom(manifest.Env["COMPOZY_HOME"])
		if err != nil {
			t.Fatal(err)
		}
		opts.Workspace.Root = manifest.Env["RUNTIME_WORKSPACE_PATH"]
		opts.ConfigSeed.HTTPPort, err = strconv.Atoi(manifest.Env["COMPOZY_HTTP_PORT"])
		if err != nil {
			t.Fatal(err)
		}
		opts.ConfigSeed.SocketPath = manifest.Env["COMPOZY_UDS_PATH"]
	}
	h := e2etest.StartRuntimeHarness(t, opts)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	parent, err := h.CreateSession(ctx, contract.CreateSessionRequest{AgentName: "general", Workspace: h.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.PromptSession(ctx, parent.ID, "hello alpha"); err != nil {
		t.Fatal(err)
	}
	invoke := func(t *testing.T, id string, input map[string]any) contract.ToolInvokeResponse {
		t.Helper()
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		var response contract.ToolInvokeResponse
		if err := h.CLI.RunJSONInDir(
			ctx,
			h.WorkspaceRoot,
			&response,
			"tool",
			"invoke",
			id,
			"--session",
			parent.ID,
			"--agent",
			"general",
			"--input",
			string(data),
			"-o",
			"json",
		); err != nil {
			t.Fatal(err)
		}
		if response.Status != "completed" {
			t.Fatalf("tool status=%s", response.Status)
		}
		return response
	}
	t.Run("Should measure both session views through public CLI E2E-001", func(t *testing.T) {
		for _, name := range []string{"session_status", "session_create", "session_spawn", "session_archive"} {
			input := map[string]any{}
			switch name {
			case "session_status":
				input["session_id"] = parent.ID
			case "session_create":
				input["agent"] = "general"
			case "session_spawn":
				input["agent_name"] = "general"
				input["notify_creator"] = false
				input["ttl_seconds"] = 3600
				input["inherit_permissions"] = true
			case "session_archive":
				child, err := h.CreateSession(
					ctx,
					contract.CreateSessionRequest{AgentName: "general", Workspace: h.WorkspaceID},
				)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := h.PromptSession(ctx, child.ID, "hello alpha"); err != nil {
					t.Fatal(err)
				}
				if err := h.StopSession(ctx, child.ID); err != nil {
					t.Fatal(err)
				}
				input["session_id"] = child.ID
			}
			compact := invoke(t, "compozy__"+name, input)
			input["result_view"] = "full"
			full := invoke(t, "compozy__"+name, input)
			t.Logf(
				"REAL %s full=%d compact=%d reduction=%.2f%% full_chars=%d compact_chars=%d",
				name,
				len(full.Result.Structured),
				len(compact.Result.Structured),
				100*(1-float64(len(compact.Result.Structured))/float64(len(full.Result.Structured))),
				utf8.RuneCount(full.Result.Structured), utf8.RuneCount(compact.Result.Structured),
			)
			if name != "session_create" && len(compact.Result.Structured)*5 > len(full.Result.Structured) {
				t.Fatalf("%s misses 80%% reduction", name)
			}
			var record map[string]any
			if err := json.Unmarshal(full.Result.Structured, &record); err != nil {
				t.Fatal(err)
			}
			if name == "session_status" {
				session := record["session"].(map[string]any)
				if session["lineage"] == nil {
					t.Fatal("full result lost lineage")
				}
				previous, err := h.GetSession(ctx, parent.ID)
				if err != nil {
					t.Fatal(err)
				}
				previousJSON, err := json.Marshal(previous)
				if err != nil {
					t.Fatal(err)
				}
				var previousRecord map[string]any
				if err := json.Unmarshal(previousJSON, &previousRecord); err != nil {
					t.Fatal(err)
				}
				// These fields are added by the session API, not by the pre-view native handler.
				for _, key := range []string{"worktree_binding", "working_dir", "process_summary"} {
					delete(previousRecord, key)
				}
				if !reflect.DeepEqual(previousRecord, session) {
					for key, expected := range previousRecord {
						if !reflect.DeepEqual(expected, session[key]) {
							t.Errorf("full/API field %s differs", key)
						}
					}
					t.Fatal("full record differs from the unchanged native record")
				}
			}
		}
	})
	t.Run("Should discover both contracts and reject invalid selectors E2E-002", func(t *testing.T) {
		var info map[string]any
		if err := h.CLI.RunJSONInDir(
			ctx,
			h.WorkspaceRoot,
			&info,
			"tool",
			"info",
			"compozy__session_status",
			"-o",
			"json",
		); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(encoded), `"output_views"`) || !strings.Contains(string(encoded), `"compact"`) ||
			!strings.Contains(string(encoded), `"full"`) {
			t.Fatal("invalid descriptor JSON")
		}
		full := invoke(t, "compozy__session_status", map[string]any{"session_id": parent.ID, "result_view": "full"})
		if len(full.Result.Structured) == 0 {
			t.Fatal("empty full result")
		}
		for _, row := range []struct{ id, input string }{
			{"compozy__session_status", fmt.Sprintf(`{"session_id":%q,"result_view":"summary"}`, parent.ID)},
			{"compozy__skill_view", `{"name":"compozy","result_view":"full"}`},
		} {
			out, stderr, err := h.CLI.RunInDir(
				ctx,
				h.WorkspaceRoot,
				"tool",
				"invoke",
				row.id,
				"--input",
				row.input,
				"-o",
				"json",
			)
			if err == nil || !strings.Contains(out+stderr, "tool_invalid_input") {
				t.Fatalf("invalid selector outcome: %v %s %s", err, out, stderr)
			}
			t.Logf("selector rejection: %s %s", out, stderr)
		}
	})
	t.Run("Should lint a moved field then run a full-view node E2E-003", func(t *testing.T) {
		definition := loopEventsDefinition()
		definition.Meta.Name = "native-view-" + parent.ID
		definition.Graph.Nodes = []dsl.Node{
			{
				ID:     "inspect",
				Class:  dsl.NodeClassAction,
				Kind:   "compozy__session_status",
				Params: map[string]any{"session_id": parent.ID},
			},
			{
				ID:    "report",
				Class: dsl.NodeClassAction,
				Kind:  "run-agent",
				Params: map[string]any{
					"agent":  "general",
					"prompt": "Tools allowed: {{ .nodes.inspect.output.session.lineage.permission_policy.tools }}",
				},
			},
		}
		definition.Graph.Edges = []dsl.Edge{{From: "inspect", To: "report"}}
		definition.Contract.StopWhen.Expr = "nodes.report.status == 'succeeded'"
		result := invoke(t, "compozy__loop_validate", map[string]any{"definition": definition})
		if !strings.Contains(string(result.Result.Structured), "result_view_field_requires_full") ||
			!strings.Contains(string(result.Result.Structured), "inspect") {
			t.Fatalf("compact validation: %s", result.Result.Structured)
		}
		definition.Graph.Nodes[0].Params["result_view"] = "full"
		result = invoke(t, "compozy__loop_validate", map[string]any{"definition": definition})
		var validation struct {
			Validation struct {
				Valid bool `json:"valid"`
			} `json:"validation"`
		}
		if err := json.Unmarshal(result.Result.Structured, &validation); err != nil {
			t.Fatal(err)
		}
		if !validation.Validation.Valid {
			t.Fatalf("full validation: %s", result.Result.Structured)
		}
		createLoopViaHTTP(t, ctx, h, definition)
		run := runLoopViaHTTP(t, ctx, h, definition.Meta.Name)
		waitForLoopRunStatus(t, ctx, h, run.ID, contract.LoopRunStatusDone)
	})
}
