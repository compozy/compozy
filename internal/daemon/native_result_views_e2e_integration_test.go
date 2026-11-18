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
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtin "github.com/compozy/compozy/internal/tools/builtin"
)

// Native result views own these CLI/runtime journeys; unit projection assertions
// remain in the native contract suite. The optional manifest binds a deliberate QA lab.
func TestDaemonE2ENativeResultViews(t *testing.T) {
	acpmock.RequireDriver(t)
	opts := &e2etest.RuntimeHarnessOptions{MockAgents: []e2etest.MockAgentSpec{{
		FixturePath: mockFixturePath(t, "multi_agent_fixture.json"), FixtureAgent: "alpha", AgentName: "general",
	}}, ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *config.Config) {
		cfg.Roles.AutoTitle.Enabled = false
		cfg.ProviderUsage.Enabled = false
	}}}
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
			var archiveLineage *contract.SessionLineagePayload
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
				before, err := h.GetSession(ctx, child.ID)
				if err != nil {
					t.Fatal(err)
				}
				archiveLineage = before.Lineage
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
			// The hosted MCP projection emits the preview plus the structured JSON as text.
			// Count both, rather than presenting structured bytes as total model-facing text.
			compactChars := utf8.RuneCountInString(compact.Result.Preview) + utf8.RuneCount(compact.Result.Structured)
			fullChars := utf8.RuneCountInString(full.Result.Preview) + utf8.RuneCount(full.Result.Structured)
			t.Logf("HOSTED_TEXT %s full_chars=%d compact_chars=%d", name, fullChars, compactChars)
			if name != "session_create" &&
				(compactChars*5 > fullChars || len(compact.Result.Structured)*5 > len(full.Result.Structured)) {
				t.Fatalf("%s misses 80%% text reduction", name)
			}
			var compactRecord map[string]any
			if err := json.Unmarshal(compact.Result.Structured, &compactRecord); err != nil {
				t.Fatal(err)
			}
			var compactID string
			if name == "session_spawn" {
				compactID, _ = compactRecord["session_id"].(string)
			} else if session, ok := compactRecord["session"].(map[string]any); ok {
				compactID, _ = session["id"].(string)
			}
			if compactID == "" {
				t.Fatal("compact result lost the follow-up session handle")
			}
			if _, err := h.GetSession(ctx, compactID); err != nil {
				t.Fatal(err)
			}
			var record map[string]any
			if err := json.Unmarshal(full.Result.Structured, &record); err != nil {
				t.Fatal(err)
			}
			if name == "session_archive" {
				var archived struct {
					Session contract.SessionPayload `json:"session"`
				}
				if err := json.Unmarshal(full.Result.Structured, &archived); err != nil {
					t.Fatal(err)
				}
				// Archive returns a catalog snapshot, unlike live session detail. Its
				// persisted full lineage must survive the mutation without projection.
				if archived.Session.ArchivedAt == nil || archiveLineage == nil ||
					len(archiveLineage.PermissionPolicy.Tools) == 0 ||
					!reflect.DeepEqual(archiveLineage, archived.Session.Lineage) {
					t.Fatal("full archive lost persisted lineage or archive state")
				}
			}
			if name == "session_status" || name == "session_create" {
				session := record["session"].(map[string]any)
				if name == "session_status" && session["lineage"] == nil {
					t.Fatal("full result lost lineage")
				}
				previous, err := h.GetSession(ctx, session["id"].(string))
				if err != nil {
					t.Fatal(err)
				}
				// Create historically returns SessionPayloadFromInfo, without the
				// profile owners and queue/linked-work decorations used by detail reads.
				if name != "session_status" {
					previous.ProfileName, previous.ProfileColor, previous.ProfileIcon, previous.ProfileEmoji = "", "", "", ""
					previous.ProfileArchived = false
					previous.Queue, previous.LinkedWork = nil, nil
					if name == "session_create" {
						previous.ParentTitle = ""
					}
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
		var info contract.ToolResponse
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
		descriptor := info.Tool.Descriptor
		views := descriptor.OutputViews
		if descriptor.ToolID != toolspkg.ToolID("compozy__session_status") || views == nil {
			t.Fatal("descriptor lost result views")
		}
		if len(views.Compact.Schema) == 0 || len(views.Full.Schema) == 0 || views.Compact.Digest == "" ||
			views.Full.Digest == "" {
			t.Fatal("descriptor must publish both schemas and digests")
		}
		var defaultSchema, compactSchema map[string]any
		if err := json.Unmarshal(descriptor.OutputSchema, &defaultSchema); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(views.Compact.Schema, &compactSchema); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(defaultSchema, compactSchema) || descriptor.OutputSchemaDigest != views.Compact.Digest {
			t.Fatal("default descriptor contract must equal compact")
		}
		full := invoke(t, "compozy__session_status", map[string]any{"session_id": parent.ID, "result_view": "full"})
		var fullRecord struct {
			Session struct {
				Lineage *contract.SessionLineagePayload `json:"lineage"`
			} `json:"session"`
		}
		if err := json.Unmarshal(full.Result.Structured, &fullRecord); err != nil {
			t.Fatal(err)
		}
		if fullRecord.Session.Lineage == nil || len(fullRecord.Session.Lineage.PermissionPolicy.Tools) == 0 {
			t.Fatal("full result lost the session permission policy")
		}
		for _, row := range []struct{ id, input string }{
			{"compozy__session_status", fmt.Sprintf(`{"session_id":%q,"result_view":"summary"}`, parent.ID)},
			{"compozy__session_status", fmt.Sprintf(`{"session_id":%q,"result_view":true}`, parent.ID)},
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
			if err == nil || !strings.Contains(out+stderr, "tool_invalid_input") ||
				!strings.Contains(out+stderr, "schema_invalid") {
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
