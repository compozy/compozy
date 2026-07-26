package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestHooksListCommandPassesFiltersAndRendersJSON(t *testing.T) {
	t.Parallel()

	var seenQuery HookCatalogQuery
	deps := newTestDeps(t, &stubClient{
		hookCatalogFn: func(_ context.Context, query HookCatalogQuery) ([]HookCatalogRecord, error) {
			seenQuery = query
			return []HookCatalogRecord{{
				Order:        1,
				Name:         "permission-guard",
				Event:        "tool.pre_call",
				Source:       "config",
				SkillSource:  "review-skill",
				Mode:         "sync",
				Required:     true,
				Priority:     10,
				ExecutorKind: "subprocess",
			}}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps,
		"hooks", "list",
		"--workspace", "alpha",
		"--agent", "coder",
		"--event", "tool.pre_call",
		"--source", "config",
		"--mode", "sync",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(hooks list) error = %v", err)
	}

	if seenQuery != (HookCatalogQuery{
		Workspace: "alpha",
		Agent:     "coder",
		Event:     "tool.pre_call",
		Source:    "config",
		Mode:      "sync",
	}) {
		t.Fatalf("HookCatalog() query = %#v, want expected filters", seenQuery)
	}

	var decoded []HookCatalogRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(hooks list) error = %v", err)
	}
	if got, want := len(decoded), 1; got != want {
		t.Fatalf("len(decoded) = %d, want %d", got, want)
	}
	if decoded[0].Name != "permission-guard" || decoded[0].ExecutorKind != "subprocess" {
		t.Fatalf("decoded[0] = %#v, want hook catalog payload", decoded[0])
	}
}

func TestHooksListCommandRendersHumanAndToon(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		hookCatalogFn: func(context.Context, HookCatalogQuery) ([]HookCatalogRecord, error) {
			return []HookCatalogRecord{{
				Order:       1,
				Name:        "permission-guard",
				Event:       "tool.pre_call",
				Source:      "config",
				SkillSource: "review-skill",
				Mode:        "sync",
				Required:    true,
				Priority:    10,
			}}, nil
		},
	})

	humanOut, _, err := executeRootCommand(t, deps, "hooks", "list", "-o", "human")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks list human) error = %v", err)
	}
	if !strings.Contains(humanOut, "Hooks") || !strings.Contains(humanOut, "permission-guard") {
		t.Fatalf("human output = %q, want hooks table", humanOut)
	}

	toonOut, _, err := executeRootCommand(t, deps, "hooks", "list", "-o", "toon")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks list toon) error = %v", err)
	}
	if !strings.Contains(toonOut, "hooks[1]{order,name,event,source,skill_source,mode,required,priority}:") {
		t.Fatalf("toon output = %q, want TOON header", toonOut)
	}
}

func TestHooksInfoCommandReturnsAllMatchesAcrossFormats(t *testing.T) {
	t.Parallel()

	var seenQuery HookCatalogQuery
	deps := newTestDeps(t, &stubClient{
		hookCatalogFn: func(_ context.Context, query HookCatalogQuery) ([]HookCatalogRecord, error) {
			seenQuery = query
			return []HookCatalogRecord{
				{
					Order:        1,
					Name:         "permission-guard",
					Event:        "permission.request",
					Source:       "config",
					Mode:         "sync",
					Required:     true,
					Priority:     10,
					TimeoutMS:    500,
					ExecutorKind: "subprocess",
					Matcher: hookspkg.HookMatcher{
						ToolName: "shell",
					},
					Metadata: map[string]string{"origin": "config"},
				},
				{
					Order:        1,
					Name:         "permission-guard",
					Event:        "tool.pre_call",
					Source:       "skill",
					SkillSource:  "review-skill",
					Mode:         "async",
					Priority:     20,
					ExecutorKind: "native",
					Matcher: hookspkg.HookMatcher{
						AgentName: "coder",
					},
					Metadata: map[string]string{"owner": "team"},
				},
				{
					Order: 1,
					Name:  "other-hook",
					Event: "tool.pre_call",
					Mode:  "sync",
				},
			}, nil
		},
	})

	jsonOut, _, err := executeRootCommand(
		t,
		deps,
		"hooks",
		"info",
		"permission-guard",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(hooks info json) error = %v", err)
	}
	if seenQuery.Workspace != "alpha" {
		t.Fatalf("HookCatalog() workspace query = %q, want alpha", seenQuery.Workspace)
	}

	var decoded []HookCatalogRecord
	if err := json.Unmarshal([]byte(jsonOut), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(hooks info) error = %v", err)
	}
	if got, want := len(decoded), 2; got != want {
		t.Fatalf("len(decoded) = %d, want %d", got, want)
	}

	humanOut, _, err := executeRootCommand(t, deps, "hooks", "info", "permission-guard", "-o", "human")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks info human) error = %v", err)
	}
	if !strings.Contains(humanOut, "Matcher") || !strings.Contains(humanOut, "Metadata") ||
		!strings.Contains(humanOut, "Executor Kind") {
		t.Fatalf("human output = %q, want detail sections", humanOut)
	}

	toonOut, _, err := executeRootCommand(t, deps, "hooks", "info", "permission-guard", "-o", "toon")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks info toon) error = %v", err)
	}
	if !strings.Contains(
		toonOut,
		"hooks[2]{name,order,event,source,skill_source,mode,required,priority,timeout_ms,executor_kind}:",
	) {
		t.Fatalf("toon output = %q, want hooks array", toonOut)
	}
	if !strings.Contains(toonOut, "matcher[1]{field,value}:") || !strings.Contains(toonOut, "metadata[1]{key,value}:") {
		t.Fatalf("toon output = %q, want matcher/metadata blocks", toonOut)
	}
}

func TestHookMatcherRowsIncludesNetworkFields(t *testing.T) {
	t.Parallel()

	t.Run("Should include network matcher fields", func(t *testing.T) {
		t.Parallel()

		rows := hookMatcherRows(hookspkg.HookMatcher{
			NetworkMatcher: &hookspkg.NetworkMatcher{
				Channel:   "builders",
				Surface:   "direct",
				Kind:      "say",
				Direction: "sent",
				WorkState: "working",
			},
		})
		got := map[string]string{}
		for _, row := range rows {
			if len(row) != 2 {
				t.Fatalf("hookMatcherRows() row = %#v, want field/value pair", row)
			}
			got[row[0]] = row[1]
		}
		if got["channel"] != "builders" ||
			got["surface"] != "direct" ||
			got["kind"] != "say" ||
			got["direction"] != "sent" ||
			got["work_state"] != "working" {
			t.Fatalf("hookMatcherRows() = %#v, want network matcher fields", got)
		}
	})
}

func TestHooksEventsCommandPassesFiltersAndRendersFormats(t *testing.T) {
	t.Parallel()

	var seenQuery HookEventsQuery
	deps := newTestDeps(t, &stubClient{
		hookEventsFn: func(_ context.Context, query HookEventsQuery) ([]HookEventRecord, error) {
			seenQuery = query
			return []HookEventRecord{{
				Event:         "tool.pre_call",
				Family:        "tool",
				SyncEligible:  true,
				PayloadSchema: "ToolPreCallPayload",
				PatchSchema:   "ToolCallPatch",
			}}, nil
		},
	})

	jsonOut, _, err := executeRootCommand(t, deps, "hooks", "events", "--family", "tool", "--sync-only", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks events json) error = %v", err)
	}
	if seenQuery != (HookEventsQuery{Family: "tool", SyncOnly: true}) {
		t.Fatalf("HookEvents() query = %#v, want expected filters", seenQuery)
	}

	var decoded []HookEventRecord
	if err := json.Unmarshal([]byte(jsonOut), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(hooks events) error = %v", err)
	}
	if got, want := len(decoded), 1; got != want {
		t.Fatalf("len(decoded) = %d, want %d", got, want)
	}

	humanOut, _, err := executeRootCommand(t, deps, "hooks", "events", "-o", "human")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks events human) error = %v", err)
	}
	if !strings.Contains(humanOut, "Hook Events") || !strings.Contains(humanOut, "tool.pre_call") {
		t.Fatalf("human output = %q, want events table", humanOut)
	}

	toonOut, _, err := executeRootCommand(t, deps, "hooks", "events", "-o", "toon")
	if err != nil {
		t.Fatalf("executeRootCommand(hooks events toon) error = %v", err)
	}
	if !strings.Contains(toonOut, "events[1]{event,family,sync_eligible,payload_schema,patch_schema}:") {
		t.Fatalf("toon output = %q, want TOON header", toonOut)
	}
}

func TestHooksRunsCommandRequiresSession(t *testing.T) {
	t.Parallel()

	code, _, stderr := executeRootCommandWithExit(t, newTestDeps(t, &stubClient{}), "hooks", "runs")
	if code != 1 {
		t.Fatalf("executeRootCommandWithExit() code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "--session is required") {
		t.Fatalf("stderr = %q, want session validation message", stderr)
	}
}

func TestHooksRunsCommandParsesSinceAndRendersFormats(t *testing.T) {
	t.Parallel()

	var seenQuery HookRunsQuery
	var seenWorkspace string
	deps := newTestDeps(t, &stubClient{
		hookRunsFn: func(_ context.Context, workspaceRef string, query HookRunsQuery) ([]HookRunRecord, error) {
			seenWorkspace = workspaceRef
			seenQuery = query
			return []HookRunRecord{{
				HookName:   "permission-guard",
				Event:      "permission.request",
				Outcome:    "failed",
				DurationMS: 12,
				Error:      "boom",
				RecordedAt: time.Date(2026, 4, 3, 11, 59, 0, 0, time.UTC),
			}}, nil
		},
	})

	jsonOut, _, err := executeRootCommand(t, deps,
		"hooks", "runs",
		"--workspace", "ws-workspace",
		"--session", "sess-1",
		"--event", "permission.request",
		"--outcome", "failed",
		"--since", "5m",
		"--last", "2",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(hooks runs json) error = %v", err)
	}
	if seenQuery.Session != "sess-1" || seenQuery.Event != "permission.request" || seenQuery.Outcome != "failed" ||
		seenQuery.Last != 2 {
		t.Fatalf("HookRuns() query = %#v, want session/event/outcome/last", seenQuery)
	}
	if seenWorkspace != "ws-workspace" {
		t.Fatalf("HookRuns() workspace = %q, want ws-workspace", seenWorkspace)
	}
	if want := fixedTestNow.Add(-5 * time.Minute).UTC().Format(time.RFC3339Nano); seenQuery.Since != want {
		t.Fatalf("HookRuns() since = %q, want %q", seenQuery.Since, want)
	}

	var decoded []HookRunRecord
	if err := json.Unmarshal([]byte(jsonOut), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(hooks runs) error = %v", err)
	}
	if got, want := len(decoded), 1; got != want {
		t.Fatalf("len(decoded) = %d, want %d", got, want)
	}

	humanOut, _, err := executeRootCommand(
		t,
		deps,
		"hooks",
		"runs",
		"--workspace",
		"ws-workspace",
		"--session",
		"sess-1",
		"-o",
		"human",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(hooks runs human) error = %v", err)
	}
	if !strings.Contains(humanOut, "Hook Runs") || !strings.Contains(humanOut, "permission-guard") ||
		!strings.Contains(humanOut, "12ms") {
		t.Fatalf("human output = %q, want runs table", humanOut)
	}

	toonOut, _, err := executeRootCommand(
		t,
		deps,
		"hooks",
		"runs",
		"--workspace",
		"ws-workspace",
		"--session",
		"sess-1",
		"-o",
		"toon",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(hooks runs toon) error = %v", err)
	}
	if !strings.Contains(toonOut, "runs[1]{hook_name,event,outcome,duration_ms,error,recorded_at}:") {
		t.Fatalf("toon output = %q, want TOON header", toonOut)
	}
}
