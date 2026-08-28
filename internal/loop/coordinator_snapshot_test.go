package loop

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func TestCoordinatorRunnerShouldExecutePinnedDefinitionSnapshot(t *testing.T) {
	t.Run("Should hydrate the complete Run envelope without mutable runtime resolvers", func(t *testing.T) {
		t.Parallel()

		definition := pinnedSnapshotDefinition()
		resolved, err := NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		effective := EffectiveConfig{
			HumanGateEnabled:  true,
			ReattemptStrategy: ReattemptFullBody,
			EnabledChecks:     new(EffectiveChecksJSON(`{"judge":true}`)),
			IterationCap:      7,
			BudgetTokens:      101,
			BudgetWallSec:     202,
			BudgetOnExceeded:  dsl.BudgetExceededEscalate,
			NoProgressWindow:  6,
			FanOutWidth:       5,
			GateMaxRevisions:  4,
			RuntimeDefaults: RuntimeDefaults{
				Worker: RuntimeSpec{Model: "worker-pinned"},
				Judge:  RuntimeSpec{Model: "judge-pinned"},
			},
			Lifecycle: DefaultLifecycleConfig(),
		}
		snapshotJSON, digest, err := BuildExecutedDefinitionSnapshot(resolved, effective)
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		hydrated, err := LoadExecutedDefinitionSnapshot(snapshotJSON, digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		if !reflect.DeepEqual(hydrated.EffectiveConfig, effective) {
			t.Fatalf("hydrated effective config = %#v, want %#v", hydrated.EffectiveConfig, effective)
		}
		if hydrated.Templates["start[0].input_mapping.slug"] == nil {
			t.Fatal("hydrated trigger template is nil")
		}
		if hydrated.Conditions[contractStopWhenConditionKey] == nil {
			t.Fatal("hydrated contract condition is nil")
		}

		run := Run{
			ID:                "loop-pinned",
			WorkspaceID:       "workspace-pinned",
			LoopName:          "deleted-catalog-name",
			Status:            StatusRunning,
			DefinitionVersion: hydrated.DefinitionVersion,
			DefinitionDigest:  digest,
			IterationCap:      effective.IterationCap,
			Inputs:            map[string]any{"slug": "pinned-value"},
		}
		coordinatorRun := task.Run{
			ID:        "coordinator-pinned",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(run.ID),
		}
		runner, err := NewCoordinatorRunner(
			&coordinatorRunnerTaskRunReader{run: coordinatorRun},
			&coordinatorRunnerLoopStore{
				run: run,
				snapshot: &DefinitionSnapshot{
					WorkspaceID: run.WorkspaceID,
					Digest:      digest,
					Version:     hydrated.DefinitionVersion,
					Definition:  snapshotJSON,
					ByteSize:    len(snapshotJSON),
				},
			},
			coordinatorRunnerOutputs{},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
		if err != nil {
			t.Fatalf("NewCoordinatorRunner() error = %v", err)
		}
		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusDone) {
			t.Fatalf("plan terminal = %#v, want done from pinned input graph", plan.Terminal)
		}
	})

	t.Run("Should reject a snapshot whose digest does not match its complete envelope", func(t *testing.T) {
		t.Parallel()

		resolved, err := NewCompiler().Compile(pinnedSnapshotDefinition())
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		snapshotJSON, _, err := BuildExecutedDefinitionSnapshot(resolved, snapshotEffectiveConfig())
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		if _, err := LoadExecutedDefinitionSnapshot(snapshotJSON, "sha256:tampered"); err == nil {
			t.Fatal("LoadExecutedDefinitionSnapshot(tampered digest) error = nil")
		}
	})

	t.Run(
		"Should hydrate persisted bytes written before an optional effective config field existed",
		func(t *testing.T) {
			t.Parallel()

			resolved, err := NewCompiler().Compile(pinnedSnapshotDefinition())
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			current, _, err := BuildExecutedDefinitionSnapshot(resolved, snapshotEffectiveConfig())
			if err != nil {
				t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
			}
			var persisted map[string]any
			if err := json.Unmarshal(current, &persisted); err != nil {
				t.Fatalf("json.Unmarshal(current snapshot) error = %v", err)
			}
			effective, ok := persisted["effective_config"].(map[string]any)
			if !ok {
				t.Fatalf("effective_config = %#v, want JSON object", persisted["effective_config"])
			}
			delete(effective, "request_expire_after")
			raw, err := json.Marshal(persisted)
			if err != nil {
				t.Fatalf("json.Marshal(previous snapshot) error = %v", err)
			}

			hydrated, err := LoadExecutedDefinitionSnapshot(raw, executedDefinitionDigest(raw))
			if err != nil {
				t.Fatalf("LoadExecutedDefinitionSnapshot(previous bytes) error = %v", err)
			}
			if hydrated.EffectiveConfig.RequestExpireAfter != "" {
				t.Fatalf(
					"hydrated request_expire_after = %q, want zero value",
					hydrated.EffectiveConfig.RequestExpireAfter,
				)
			}
		},
	)

	t.Run("Should hydrate a snapshot with omitted optional definition extensions", func(t *testing.T) {
		t.Parallel()

		definition := pinnedSnapshotDefinition()
		definition.DefinitionExtensionState = nil
		definition.Contract.StopWhen = dsl.StopWhenSpec{}
		resolved, err := NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		snapshotJSON, digest, err := BuildExecutedDefinitionSnapshot(resolved, snapshotEffectiveConfig())
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		hydrated, err := LoadExecutedDefinitionSnapshot(snapshotJSON, digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		if hydrated.Definition.DefinitionExtensionState == nil {
			t.Fatal("hydrated definition extension state = nil, want normalized empty state")
		}
		if got := len(hydrated.Definition.Start); got != 0 {
			t.Fatalf("hydrated start bindings = %d, want 0", got)
		}
		if got := len(hydrated.Templates); got != 2 ||
			hydrated.Templates["contract.goal"] == nil ||
			hydrated.Templates["contract.definition_of_done"] == nil {
			t.Fatalf(
				"hydrated contract narrative templates = %#v, want goal and definition of done",
				hydrated.Templates,
			)
		}
		if got := len(hydrated.Conditions); got != 0 {
			t.Fatalf("hydrated conditions = %d, want 0", got)
		}
	})

	t.Run("Should round trip every execution manifest without consulting mutable registries", func(t *testing.T) {
		t.Parallel()

		const toolID = "compozy__task_read"
		definition := pinnedSnapshotDefinition()
		definition.Graph = dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "watch_tasks",
					Class: dsl.NodeClassSource,
					Kind:  string(dsl.SourceWatchEvents),
					Events: []dsl.EventSubscription{{
						Kind:   string(hooks.HookTaskStatusChanged),
						Filter: "event.task_id == inputs.slug",
					}},
				},
				{
					ID:     "read_task",
					Class:  dsl.NodeClassAction,
					Kind:   toolID,
					Params: dsl.NodeParams{"id": "{{ .inputs.slug }}"},
				},
			},
			Edges: []dsl.Edge{{From: "watch_tasks", To: "read_task"}},
		}
		toolSnapshot := ToolSchemaSnapshot{
			ToolID:             toolID,
			InputSchema:        []byte(`{"type":"object","properties":{"id":{"type":"string"}}}`),
			OutputSchema:       []byte(`{"type":"object","properties":{"status":{"type":"string"}}}`),
			InputSchemaDigest:  "input-v1",
			OutputSchemaDigest: "output-v1",
		}
		resolved, err := NewCompiler(WithCompilerToolSchemaSource(
			snapshotToolSchemas{toolID: toolSnapshot},
		)).Compile(definition)
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		pinnedContract := resolved.WatchEventsContracts[hooks.HookTaskStatusChanged]
		pinnedContract.Stream = "pinned_task_events"
		resolved.WatchEventsContracts[hooks.HookTaskStatusChanged] = pinnedContract
		effective := snapshotEffectiveConfig()
		effective.IterationCap = 27
		effective.RuntimeDefaults = RuntimeDefaults{
			Worker: RuntimeSpec{Model: "worker-v1"},
			Judge:  RuntimeSpec{Model: "judge-v1"},
		}
		effective.Sources = map[string]string{"/iteration_cap": EffectiveConfigSourcePerRun}
		owned := cloneEffectiveConfig(effective)
		owned.Sources["/iteration_cap"] = EffectiveConfigSourceBuiltin
		if effective.Sources["/iteration_cap"] != EffectiveConfigSourcePerRun {
			t.Fatal("cloneEffectiveConfig() aliased Sources")
		}

		snapshotJSON, digest, err := BuildExecutedDefinitionSnapshot(resolved, effective)
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		hydrated, err := LoadExecutedDefinitionSnapshot(snapshotJSON, digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}

		if got, want := len(hydrated.Definition.Graph.Nodes), len(resolved.Definition.Graph.Nodes); got != want {
			t.Fatalf("hydrated graph node count = %d, want %d", got, want)
		}
		if !reflect.DeepEqual(hydrated.Defaults, resolved.Defaults) {
			t.Fatalf("hydrated defaults = %#v, want %#v", hydrated.Defaults, resolved.Defaults)
		}
		if !reflect.DeepEqual(hydrated.EffectiveConfig, effective) {
			t.Fatalf("hydrated effective config = %#v, want %#v", hydrated.EffectiveConfig, effective)
		}
		if !reflect.DeepEqual(hydrated.ToolSchemas[toolID], toolSnapshot) {
			t.Fatalf("hydrated tool snapshot = %#v, want %#v", hydrated.ToolSchemas[toolID], toolSnapshot)
		}
		if got := hydrated.WatchEventsContracts[hooks.HookTaskStatusChanged].Stream; got != "pinned_task_events" {
			t.Fatalf("hydrated watch stream = %q, want pinned_task_events", got)
		}
		for _, key := range []string{
			"start[0].input_mapping.slug",
			"nodes.read_task.params.id",
		} {
			if hydrated.Templates[key] == nil {
				t.Fatalf("hydrated template %q is nil", key)
			}
		}
		for _, key := range []string{
			contractStopWhenConditionKey,
			"nodes.watch_tasks.events.0.filter",
		} {
			if hydrated.Conditions[key] == nil {
				t.Fatalf("hydrated condition %q is nil", key)
			}
		}
	})

	t.Run("Should round trip templated parent and child Loop actions", func(t *testing.T) {
		t.Parallel()

		definition, err := dsl.Parse([]byte(`apiVersion: compozy.loop/v1
kind: Loop
meta:
  name: parent-loop
  version: 1
inputs:
  slug: { type: string, required: true }
contract:
  goal: Deliver the requested change
  definition_of_done: The child Loops complete
  iteration_cap: 1
  no_progress: { window: 1 }
  budget: { on_exceeded: halt }
graph:
  nodes:
    - id: draft
      class: action
      kind: run-agent
      params:
        agent: codex
        prompt: Draft {{ .inputs.slug }}
        output_schema:
          summary: string
    - id: workspace_child
      class: action
      kind: run-loop
      params:
        loop: workspace-child
        inputs:
          slug: "{{ .inputs.slug }}"
          summary: "{{ .nodes.draft.output.summary }}"
    - id: marketplace_child
      class: action
      kind: run-loop
      params:
        loop: review-and-fix
        inputs:
          slug: "{{ .inputs.slug }}"
          summary: "{{ .nodes.draft.output.summary }}"
  edges:
    - { from: draft, to: workspace_child }
    - { from: workspace_child, to: marketplace_child }
start: [{ kind: manual }]
`))
		if err != nil {
			t.Fatalf("Parse(parent and child Loops) error = %v", err)
		}
		resolved, err := NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile(parent and child Loops) error = %v", err)
		}
		raw, digest, err := BuildExecutedDefinitionSnapshot(resolved, snapshotEffectiveConfig())
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		hydrated, err := LoadExecutedDefinitionSnapshot(raw, digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		for _, key := range []string{
			"nodes.workspace_child.params.inputs.slug",
			"nodes.workspace_child.params.inputs.summary",
			"nodes.marketplace_child.params.inputs.slug",
			"nodes.marketplace_child.params.inputs.summary",
		} {
			if hydrated.Templates[key] == nil {
				t.Fatalf("hydrated template %q is nil", key)
			}
		}
	})

	t.Run("Should round trip composed request schemas without manifest drift", func(t *testing.T) {
		t.Parallel()

		definition, err := dsl.Parse([]byte(`apiVersion: compozy.loop/v1
kind: Loop
meta: { name: composed-request, version: 1 }
contract:
  goal: Collect a typed reviewer selection
  definition_of_done: A reviewer is selected
  iteration_cap: 1
  no_progress: { window: 1 }
  budget: { on_exceeded: halt }
graph:
  nodes:
    - id: choose_reviewers
      class: control
      kind: ask
      params:
        prompt: Choose reviewers
        expect:
          allOf:
            - type: object
              required: [reviewers]
              properties:
                reviewers:
                  type: array
                  items:
                    oneOf:
                      - type: string
                        x-compozy-entity-kind: agent
  edges: []
start: [{ kind: manual }]
`))
		if err != nil {
			t.Fatalf("Parse(composed request schema) error = %v", err)
		}
		resolved, err := NewCompiler().Compile(definition)
		if err != nil {
			t.Fatalf("Compile(composed request schema) error = %v", err)
		}
		raw, digest, err := BuildExecutedDefinitionSnapshot(resolved, snapshotEffectiveConfig())
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		if _, err := LoadExecutedDefinitionSnapshot(raw, digest); err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
	})
}

func TestExecutedDefinitionSnapshotShouldCanonicalizeTypedNodeParams(t *testing.T) {
	t.Parallel()

	definition := pinnedSnapshotDefinition()
	definition.Graph = dsl.Graph{Nodes: []dsl.Node{{
		ID: "goal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionGoal),
		Session: &dsl.SessionSpec{Mode: dsl.SessionModeContinuous},
		Params: dsl.NodeParams{
			"agent": "codex", "objective": "Confirm the objective.", "max_turns": 3,
			"judge": []dsl.GateCriterion{{
				ID: "objective_satisfied", Type: dsl.CriterionAgentJudge, Rubric: "Confirm the objective.",
			}},
			"output_schema": map[string]any{
				"status": map[string]any{"type": "string", "enum": []string{"complete", "blocked"}},
			},
		},
	}}, Edges: []dsl.Edge{}}
	resolved, err := NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compile(typed params) error = %v", err)
	}
	effective := EffectiveConfig{
		ReattemptStrategy: ReattemptFailedOnly,
		EnabledChecks:     new(EffectiveChecksJSON(`{}`)),
		BudgetOnExceeded:  dsl.BudgetExceededHalt,
		RuntimeDefaults: RuntimeDefaults{
			Judge: RuntimeSpec{Model: "judge-v1"},
		},
	}
	raw, digest, err := BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	if _, err := LoadExecutedDefinitionSnapshot(raw, digest); err != nil {
		t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
	}
}

func TestExecutedDefinitionSnapshotShouldPreserveRunAgentRuntimeTemplateManifest(t *testing.T) {
	t.Parallel()

	definition := pinnedSnapshotDefinition()
	definition.Graph = dsl.Graph{
		Nodes: []dsl.Node{
			{
				ID: "items", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
				Params: dsl.NodeParams{"map": map[string]any{"tasks": map[string]any{"value": []any{
					map[string]any{
						"id": "task-1", "path": ".compozy/tasks/task-1.md",
						"runtime": map[string]any{"model": "frontmatter-model"},
					},
				}}}},
			},
			{
				ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut),
				Collection: "{{ .nodes.items.output.tasks }}", BatchSize: 1, MaxFanOut: 1,
			},
			{
				ID: "worker", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
				Params: dsl.NodeParams{
					"agent": "codex", "prompt": "Complete the task.",
					"runtime": map[string]any{"model": "node-model"},
				},
			},
			{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
		},
		Edges: []dsl.Edge{
			{From: "items", To: "fan"},
			{From: "fan", To: "worker"},
			{From: "worker", To: "collect"},
		},
	}
	serialized, err := dsl.Serialize(definition)
	if err != nil {
		t.Fatalf("Serialize(run-agent runtime) error = %v", err)
	}
	parsed, err := dsl.Parse(serialized)
	if err != nil {
		t.Fatalf("Parse(run-agent runtime) error = %v", err)
	}
	resolved, err := NewCompiler().Compile(parsed)
	if err != nil {
		t.Fatalf("Compile(run-agent runtime) error = %v", err)
	}
	raw, digest, err := BuildExecutedDefinitionSnapshot(resolved, snapshotEffectiveConfig())
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	hydrated, err := LoadExecutedDefinitionSnapshot(raw, digest)
	if err != nil {
		t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
	}
	if hydrated.Templates["nodes.worker.params.runtime.model"] == nil {
		t.Fatal("hydrated run-agent runtime model template is nil")
	}
}

type snapshotToolSchemas map[string]ToolSchemaSnapshot

func (s snapshotToolSchemas) Snapshot(toolID string) (ToolSchemaSnapshot, bool) {
	snapshot, ok := s[toolID]
	return snapshot, ok
}

func pinnedSnapshotDefinition() dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "pinned", Version: 1},
		Contract: dsl.Contract{
			Goal:             "Use the pinned graph",
			DefinitionOfDone: "The pinned graph completes",
			StopWhen:         dsl.StopWhenSpec{Expr: "generation > 5"},
			IterationCap:     1,
			NoProgress:       dsl.NoProgress{Window: 1},
			Budget:           dsl.Budget{OnExceeded: dsl.BudgetExceededHalt},
		},
		Inputs: map[string]dsl.Input{
			"slug": {Type: dsl.InputTypeString, Required: true},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{
				Kind: dsl.StartTrigger,
				InputMapping: map[string]string{
					"slug": "{{ .trigger.payload.slug }}",
				},
			}},
		},
		Graph: dsl.Graph{Nodes: []dsl.Node{{
			ID:       "slug_input",
			Class:    dsl.NodeClassSource,
			Kind:     string(dsl.SourceInput),
			InputRef: "slug",
		}}},
	}
}

func snapshotEffectiveConfig() EffectiveConfig {
	return EffectiveConfig{
		ReattemptStrategy: ReattemptFailedOnly,
		EnabledChecks:     new(EffectiveChecksJSON(`{}`)),
		IterationCap:      1,
		BudgetOnExceeded:  dsl.BudgetExceededHalt,
		NoProgressWindow:  1,
		FanOutWidth:       1,
		GateMaxRevisions:  1,
		Lifecycle:         DefaultLifecycleConfig(),
	}
}
