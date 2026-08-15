package builtin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/santhosh-tekuri/jsonschema/v6"
	jsonschemakind "github.com/santhosh-tekuri/jsonschema/v6/kind"
)

func TestBuiltinNativeDescriptors(t *testing.T) {
	t.Parallel()

	t.Run("Should expose exactly the MVP native tool scope", func(t *testing.T) {
		t.Parallel()

		descriptors := NativeDescriptors()
		got := make(map[toolspkg.ToolID]toolspkg.Descriptor, len(descriptors))
		for _, descriptor := range descriptors {
			if err := descriptor.Validate(); err != nil {
				t.Fatalf("descriptor %q Validate() error = %v", descriptor.ID, err)
			}
			got[descriptor.ID] = descriptor
		}

		expectations := nativeDescriptorExpectations()
		if gotLen, wantLen := len(got), len(expectations); gotLen != wantLen {
			t.Fatalf("len(NativeDescriptors()) = %d, want %d", gotLen, wantLen)
		}
		for _, expectation := range expectations {
			id := expectation.id
			descriptor, ok := got[id]
			if !ok {
				t.Fatalf("descriptor %q missing from MVP native scope", id)
			}
			if descriptor.Backend.Kind != toolspkg.BackendNativeGo {
				t.Fatalf("%s backend kind = %q, want native_go", id, descriptor.Backend.Kind)
			}
			if descriptor.Backend.NativeName == "" {
				t.Fatalf("%s backend native name is empty", id)
			}
			if descriptor.Source != Source() {
				t.Fatalf("%s source = %#v, want builtin source", id, descriptor.Source)
			}
			if descriptor.Visibility != toolspkg.VisibilityModel {
				t.Fatalf("%s visibility = %q, want model", id, descriptor.Visibility)
			}
		}

		excluded := []toolspkg.ToolID{
			"compozy__skill_install",
			"compozy__skill_update",
			"compozy__skill_remove",
			"compozy__task_claim",
			"compozy__task_release",
			"compozy__task_complete",
			"compozy__task_fail",
			"compozy__task_run_start",
			"compozy__task_run_cancel",
			"compozy__mcp_auth_login",
			"compozy__mcp_auth_logout",
			"compozy__memory_read",
			"compozy__memory_history",
			"compozy__memory_write",
			"compozy__memory_edit",
			"compozy__memory_delete",
		}
		for _, id := range excluded {
			if _, ok := got[id]; ok {
				t.Fatalf("descriptor %q is registered but must be excluded from MVP native scope", id)
			}
		}
	})

	t.Run("Should expose provider-compatible top-level input schemas", func(t *testing.T) {
		t.Parallel()

		for _, descriptor := range NativeDescriptors() {
			var schema map[string]json.RawMessage
			if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
				t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
			}
			for _, forbidden := range []string{"oneOf", "anyOf", "allOf"} {
				if _, ok := schema[forbidden]; ok {
					t.Fatalf(
						"%s input schema has top-level %s, want provider-compatible object schema",
						descriptor.ID,
						forbidden,
					)
				}
			}
		}
	})

	t.Run("Should expose typed participation on execution management tools", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDTaskCreate,
			toolspkg.ToolIDTaskChildCreate,
			toolspkg.ToolIDTaskUpdate,
			toolspkg.ToolIDTaskFanOutRuns,
			toolspkg.ToolIDLoopRun,
		} {
			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(descriptors[id].InputSchema, &schema); err != nil {
				t.Fatalf("%s input schema unmarshal error = %v", id, err)
			}
			participationSchema, ok := schema.Properties["network_participation"]
			if !ok {
				t.Fatalf("%s input schema omits network_participation", id)
			}
			assertTypedNetworkParticipationSchema(t, id.String(), participationSchema)
			for _, legacy := range []string{"channel", "network_channel", "coordination_channel_id"} {
				if _, ok := schema.Properties[legacy]; ok {
					t.Fatalf("%s input schema exposes removed %s", id, legacy)
				}
			}
		}

		var profileInput nativeObjectSchema
		profileDescriptor := descriptors[toolspkg.ToolIDTaskExecutionProfileSet]
		if err := json.Unmarshal(profileDescriptor.InputSchema, &profileInput); err != nil {
			t.Fatalf("%s input schema unmarshal error = %v", profileDescriptor.ID, err)
		}
		var profile nativeObjectSchema
		if err := json.Unmarshal(profileInput.Properties["profile"], &profile); err != nil {
			t.Fatalf("%s profile schema unmarshal error = %v", profileDescriptor.ID, err)
		}
		participationSchema, ok := profile.Properties["network_participation"]
		if !ok {
			t.Fatalf("%s profile schema omits network_participation", profileDescriptor.ID)
		}
		assertTypedNetworkParticipationSchema(t, profileDescriptor.ID.String(), participationSchema)
		var worktreePolicy nativeObjectSchema
		if err := json.Unmarshal(profile.Properties["worktree"], &worktreePolicy); err != nil {
			t.Fatalf("%s worktree schema unmarshal error = %v", profileDescriptor.ID, err)
		}
		assertClosedObjectSchema(
			t,
			profileDescriptor.ID.String()+" worktree",
			worktreePolicy,
			[]string{"mode", "worktree_ref"},
		)
		worktreeDescriptor := descriptors[toolspkg.ToolIDTaskWorktreePolicySet]
		var worktreeInput nativeObjectSchema
		if err := json.Unmarshal(worktreeDescriptor.InputSchema, &worktreeInput); err != nil {
			t.Fatalf("%s input schema unmarshal error = %v", worktreeDescriptor.ID, err)
		}
		assertClosedObjectSchema(
			t,
			worktreeDescriptor.ID.String(),
			worktreeInput,
			[]string{"mode", "task_id", "worktree_ref"},
		)

		for _, id := range []toolspkg.ToolID{toolspkg.ToolIDTaskList, toolspkg.ToolIDTaskRunList} {
			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(descriptors[id].InputSchema, &schema); err != nil {
				t.Fatalf("%s input schema unmarshal error = %v", id, err)
			}
			if _, ok := schema.Properties["participation_channel"]; !ok {
				t.Fatalf("%s input schema omits participation_channel", id)
			}
			if id == toolspkg.ToolIDTaskList {
				if _, ok := schema.Properties["worktree"]; !ok {
					t.Fatalf("%s input schema omits worktree", id)
				}
			}
			for _, legacy := range []string{"network_channel", "coordination_channel_id"} {
				if _, ok := schema.Properties[legacy]; ok {
					t.Fatalf("%s input schema exposes removed %s", id, legacy)
				}
			}
		}
	})

	t.Run("Should expose the closed Loop environment on create configure and run", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		configure := decodeNativeObjectSchema(t, descriptors[toolspkg.ToolIDLoopConfigure], "config")
		assertNativeLoopEnvironmentSchema(t, toolspkg.ToolIDLoopConfigure.String(), configure.Properties["environment"])
		run := decodeNativeObjectSchema(t, descriptors[toolspkg.ToolIDLoopRun], "config_overrides")
		assertNativeLoopEnvironmentSchema(t, toolspkg.ToolIDLoopRun.String(), run.Properties["environment"])

		definition := decodeNativeObjectSchema(t, descriptors[toolspkg.ToolIDLoopCreate], "definition")
		graph := decodeRawNativeObjectSchema(
			t,
			toolspkg.ToolIDLoopCreate.String()+" graph",
			definition.Properties["graph"],
		)
		nodes := decodeRawNativeObjectSchema(t, toolspkg.ToolIDLoopCreate.String()+" nodes", graph.Properties["nodes"])
		node := decodeRawNativeObjectSchema(t, toolspkg.ToolIDLoopCreate.String()+" node", nodes.Items)
		params := decodeRawNativeObjectSchema(
			t,
			toolspkg.ToolIDLoopCreate.String()+" params",
			node.Properties["params"],
		)
		assertNativeLoopEnvironmentSchema(t, toolspkg.ToolIDLoopCreate.String(), params.Properties["environment"])
		if _, ok := params.Properties["cwd"]; ok {
			t.Fatal("compozy__loop_create params schema exposes retired cwd")
		}
	})

	t.Run("Should expose closed session mutation contracts", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		assertSessionCreateMutationSchema(t, descriptors[toolspkg.ToolIDSessionCreate])
		assertSessionRenameMutationSchema(t, descriptors[toolspkg.ToolIDSessionRename])
		t.Run("Should validate the session archive schema", func(t *testing.T) {
			t.Parallel()
			assertSessionTargetMutationSchema(t, descriptors[toolspkg.ToolIDSessionArchive])
		})
		t.Run("Should validate the session unarchive schema", func(t *testing.T) {
			t.Parallel()
			assertSessionTargetMutationSchema(t, descriptors[toolspkg.ToolIDSessionUnarchive])
		})
		assertSessionPromptMutationSchema(t, descriptors[toolspkg.ToolIDSessionPrompt])
		assertSessionRuntimeMutationSchemas(t, descriptors)
		assertSessionInputSchemas(t, descriptors)
	})

	t.Run("Should describe canonical agent name constraints on native inputs", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		for _, testCase := range []struct {
			id       toolspkg.ToolID
			property string
			pattern  string
		}{
			{
				id:       toolspkg.ToolIDAgentCreate,
				property: "name",
				pattern:  compozyconfig.AgentNamePattern,
			},
			{
				id:       toolspkg.ToolIDSessionCreate,
				property: "agent",
				pattern:  compozyconfig.AgentNamePattern,
			},
			{
				id:       toolspkg.ToolIDAutomationJobsCreate,
				property: "agent_name",
				pattern:  compozyconfig.OptionalAgentNamePattern,
			},
			{
				id:       toolspkg.ToolIDAutomationTriggersCreate,
				property: "agent_name",
				pattern:  compozyconfig.OptionalAgentNamePattern,
			},
		} {
			t.Run("Should describe "+testCase.id.String()+" "+testCase.property, func(t *testing.T) {
				t.Parallel()

				var input nativeObjectSchema
				descriptor := descriptors[testCase.id]
				if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
					t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
				}
				if !slices.Contains(input.Required, testCase.property) {
					t.Fatalf(
						"%s input required = %#v, want %q retained",
						descriptor.ID,
						input.Required,
						testCase.property,
					)
				}
				assertNativeAgentNameInputSchema(
					t,
					descriptor.ID.String(),
					input.Properties[testCase.property],
					testCase.pattern,
				)
			})
		}
	})

	t.Run("Should expose a closed atomic memory operations batch schema", func(t *testing.T) {
		t.Parallel()

		type schemaField struct {
			Type                 string                 `json:"type"`
			Enum                 []string               `json:"enum"`
			MinItems             int                    `json:"minItems"`
			Required             []string               `json:"required"`
			Properties           map[string]schemaField `json:"properties"`
			Items                *schemaField           `json:"items"`
			AdditionalProperties *bool                  `json:"additionalProperties"`
		}
		descriptor := descriptorMap(NativeDescriptors())[toolspkg.ToolIDMemoryPropose]
		var schema schemaField
		if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
			t.Fatalf("memory_propose input schema unmarshal error = %v", err)
		}
		operations := schema.Properties["operations"]
		if operations.Type != "array" || operations.MinItems != 1 || operations.Items == nil {
			t.Fatalf("memory_propose operations schema = %#v, want non-empty array", operations)
		}
		items := operations.Items
		if !slices.Equal(items.Required, []string{"action"}) {
			t.Fatalf("memory_propose operation required = %#v, want [action]", items.Required)
		}
		if items.AdditionalProperties == nil || *items.AdditionalProperties {
			t.Fatalf(
				"memory_propose operation additionalProperties = %#v, want false",
				items.AdditionalProperties,
			)
		}
		gotActions := items.Properties["action"].Enum
		wantActions := []string{"add", "replace", "remove"}
		if !slices.Equal(gotActions, wantActions) {
			t.Fatalf("memory_propose operation action enum = %#v, want %#v", gotActions, wantActions)
		}
		for _, field := range []string{"content", "old_text"} {
			if got := items.Properties[field].Type; got != "string" {
				t.Fatalf("memory_propose operation %s type = %q, want string", field, got)
			}
		}
	})

	t.Run("Should describe the first public thread send contract", func(t *testing.T) {
		t.Parallel()

		descriptor := descriptorMap(NativeDescriptors())[toolspkg.ToolIDNetworkSend]
		var schema struct {
			Description string `json:"description"`
			Properties  map[string]struct {
				Description string `json:"description"`
				Pattern     string `json:"pattern"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
			t.Fatalf("network_send input schema unmarshal error = %v", err)
		}

		const threadIDPattern = `^thread_[a-z0-9][a-z0-9_-]{2,95}$`
		if got := schema.Properties["thread_id"].Pattern; got != threadIDPattern {
			t.Fatalf("network_send thread_id pattern = %q, want %q", got, threadIDPattern)
		}
		const directIDPattern = `^direct_[a-f0-9]{32}$`
		if got := schema.Properties["direct_id"].Pattern; got != directIDPattern {
			t.Fatalf("network_send direct_id pattern = %q, want %q", got, directIDPattern)
		}
		for field, phrase := range map[string]string{
			"surface":   "required for say, capability, receipt, and trace",
			"thread_id": "first valid send creates the public thread",
			"body":      "say requires a non-empty text field",
			"to":        "Required for capability and for say carrying work_id",
			"work_id":   "Required for capability, receipt, and trace",
		} {
			if got := schema.Properties[field].Description; !strings.Contains(got, phrase) {
				t.Fatalf("network_send %s description = %q, want phrase %q", field, got, phrase)
			}
		}
		if !strings.Contains(schema.Description, "surface=thread requires thread_id") {
			t.Fatalf("network_send schema description = %q, want conditional thread contract", schema.Description)
		}
		if !strings.Contains(schema.Description, "greet and whois omit conversation and work fields") {
			t.Fatalf("network_send schema description = %q, want discovery omission contract", schema.Description)
		}
	})

	t.Run("Should describe recurring schedule catch up fields for automation mutations", func(t *testing.T) {
		t.Parallel()

		type schemaField struct {
			Type                 string                 `json:"type"`
			Enum                 []string               `json:"enum"`
			Minimum              *int                   `json:"minimum"`
			Properties           map[string]schemaField `json:"properties"`
			AdditionalProperties *bool                  `json:"additionalProperties"`
		}
		descriptors := descriptorMap(NativeDescriptors())
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDAutomationJobsCreate,
			toolspkg.ToolIDAutomationJobsUpdate,
		} {
			var schema schemaField
			if err := json.Unmarshal(descriptors[id].InputSchema, &schema); err != nil {
				t.Fatalf("%s input schema unmarshal error = %v", id, err)
			}
			schedule := schema.Properties["schedule"]
			policy := schedule.Properties["catch_up_policy"]
			wantPolicy := []string{"skip_missed", "coalesce", "replay", "run_once_on_catchup"}
			if !slices.Equal(policy.Enum, wantPolicy) {
				t.Fatalf("%s catch_up_policy enum = %#v, want %#v", id, policy.Enum, wantPolicy)
			}
			grace := schedule.Properties["misfire_grace_seconds"]
			if grace.Minimum == nil || *grace.Minimum != 0 {
				t.Fatalf("%s misfire_grace_seconds minimum = %#v, want 0", id, grace.Minimum)
			}
			if schedule.AdditionalProperties == nil || *schedule.AdditionalProperties {
				t.Fatalf("%s schedule additionalProperties = %#v, want false", id, schedule.AdditionalProperties)
			}
		}
	})

	t.Run("Should keep provider model refresh out of the read-only list schema", func(t *testing.T) {
		t.Parallel()

		descriptor := descriptorMap(NativeDescriptors())[toolspkg.ToolIDProviderModelsList]
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
			t.Fatalf("provider_models_list input schema unmarshal error = %v", err)
		}
		if _, ok := schema.Properties["refresh"]; ok {
			t.Fatalf("provider_models_list input schema exposes refresh: %s", string(descriptor.InputSchema))
		}
		if _, ok := schema.Properties["view"]; !ok {
			t.Fatalf("provider_models_list input schema omits view: %s", string(descriptor.InputSchema))
		}

		curate := descriptorMap(NativeDescriptors())[toolspkg.ToolIDProviderModelsCurate]
		var curateSchema struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(curate.InputSchema, &curateSchema); err != nil {
			t.Fatalf("provider_models_curate input schema unmarshal error = %v", err)
		}
		if !slices.Contains(curateSchema.Required, "provider_id") ||
			!slices.Contains(curateSchema.Required, "model_id") ||
			curateSchema.Properties["default_effort"] == nil {
			t.Fatalf("provider_models_curate schema = %#v, want required identity and default_effort", curateSchema)
		}
	})

	t.Run("Should derive descriptor presence and risk from one expectation table", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		expectations := nativeDescriptorExpectations()
		if got, want := len(descriptors), len(expectations); got != want {
			t.Fatalf("descriptor count = %d, want %d", got, want)
		}
		for _, expectation := range expectations {
			descriptor, ok := descriptors[expectation.id]
			if !ok {
				t.Fatalf("descriptor %q missing from native scope", expectation.id)
			}
			requireDescriptorRisk(
				t,
				descriptor,
				expectation.risk,
				expectation.readOnly,
				expectation.destructive,
				expectation.openWorld,
			)
		}

		sessionList := descriptors[toolspkg.ToolIDSessionList]
		if !strings.Contains(string(sessionList.InputSchema), `"cursor"`) ||
			!strings.Contains(string(sessionList.InputSchema), `"include_health"`) ||
			!strings.Contains(string(sessionList.InputSchema), `"archive"`) ||
			!strings.Contains(string(sessionList.InputSchema), `"type"`) ||
			!strings.Contains(string(sessionList.InputSchema), `"last_activity"`) ||
			!strings.Contains(string(sessionList.OutputSchema), `"page"`) ||
			!strings.Contains(string(sessionList.OutputSchema), `"next_cursor"`) {
			t.Fatalf(
				"session list schemas = input %s output %s, want paged filters and continuation",
				sessionList.InputSchema,
				sessionList.OutputSchema,
			)
		}
		if strings.Contains(string(sessionList.InputSchema), `"dream"`) {
			t.Fatalf("session list input schema exposes internal dream type: %s", sessionList.InputSchema)
		}

		bridgeList := descriptors[toolspkg.ToolIDBridgesList]
		if !strings.Contains(string(bridgeList.InputSchema), `"workspace"`) ||
			!strings.Contains(string(bridgeList.InputSchema), `"cursor"`) ||
			!strings.Contains(string(bridgeList.OutputSchema), `"facets"`) ||
			!strings.Contains(string(bridgeList.OutputSchema), `"next_cursor"`) {
			t.Fatalf(
				"bridge list schemas = input %s output %s, want workspace-safe paged filters",
				bridgeList.InputSchema,
				bridgeList.OutputSchema,
			)
		}

		if got, want := descriptors[toolspkg.ToolIDToolArtifactRead].MaxResultBytes,
			toolArtifactReadMaxResultBytes; got != want {
			t.Fatalf("tool artifact read max result bytes = %d, want %d", got, want)
		}
		publish := descriptors[toolspkg.ToolIDExtensionsPublish]
		if !publish.RequiresInteraction {
			t.Fatal("extensions_publish RequiresInteraction = false, want true")
		}
		for _, forbidden := range []string{"credential", "secret", "token"} {
			if strings.Contains(strings.ToLower(string(publish.InputSchema)), forbidden) {
				t.Fatalf("extensions_publish schema contains forbidden credential field %q", forbidden)
			}
		}
		if got := descriptors[toolspkg.ToolIDTaskRunReviewSubmit].Backend.NativeName; got != "submit_run_review" {
			t.Fatalf("submit review native name = %q, want submit_run_review", got)
		}
	})
	t.Run("Should expose the closed clarification contract without approval recursion", func(t *testing.T) {
		t.Parallel()

		descriptor := descriptorMap(NativeDescriptors())[toolspkg.ToolIDClarify]
		if descriptor.RequiresInteraction {
			t.Fatal("clarify RequiresInteraction = true, want false")
		}
		var input struct {
			Required             []string `json:"required"`
			AdditionalProperties bool     `json:"additionalProperties"`
			Properties           map[string]struct {
				MaxItems int `json:"maxItems"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
			t.Fatalf("clarify input schema unmarshal error = %v", err)
		}
		if !slices.Equal(input.Required, []string{"question"}) || input.AdditionalProperties {
			t.Fatalf("clarify input schema = %s, want closed question contract", descriptor.InputSchema)
		}
		if got, want := input.Properties["choices"].MaxItems, toolspkg.MaxClarifyChoices; got != want {
			t.Fatalf("clarify choices maxItems = %d, want %d", got, want)
		}
	})

	t.Run("Should validate exact Loop lifecycle output schemas", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		status := descriptors[toolspkg.ToolIDLoopStatus]
		runs := descriptors[toolspkg.ToolIDLoopRuns]
		cancel := descriptors[toolspkg.ToolIDLoopCancel]
		nodes := descriptors[toolspkg.ToolIDLoopNodes]
		assertNativeOutputSchemaAccepts(t, status, `{
			"run":{"id":"run-1","best_generation":2,"best_score":0.91},
			"materialized_contract":{"goal":"Ship weather-app","definition_of_done":"All tasks complete"},
			"node_controls":[],
			"waits":[],
			"generations":[{
				"generation":2,"parent_generation":1,"origin":"gate_revise",
				"verdicts":[{"gate_id":"quality","outcome":"rejected","score":0.7,"route_cause_rank":0}],
				"outputs":[{
					"node_id":"draft","status":"failed","attempt":3,
					"next_attempt_at":"2026-08-02T18:00:00Z",
					"failure_class":"transient","disposition":"retry"
				}]
			}]
		}`)
		assertNativeOutputSchemaAccepts(t, runs, `{
			"runs":[{"id":"run-1","best_generation":2,"best_score":0.91}],
			"aggregates":{"total":1,"live":0,"terminal":1,"succeeded":1,"failed":0}
		}`)
		assertNativeOutputSchemaRejects(t, runs, `{
			"runs":[{"id":"run-1","generations":[]}],
			"aggregates":{"total":1,"live":0,"terminal":1,"succeeded":1,"failed":0}
		}`)
		assertNativeOutputSchemaRejects(t, status, `{
			"run":{},
			"generations":[]
		}`)
		assertNativeOutputSchemaRejects(t, runs, `{
			"runs":[{"best_generation":2}],
			"aggregates":{"total":1,"live":0,"terminal":1,"succeeded":1,"failed":0}
		}`)
		assertNativeOutputSchemaAccepts(t, cancel, `{
			"ok":true,
			"run_id":"run-1",
			"status":"canceled",
			"provenance":{
				"actor_kind":"cli",
				"actor_id":"operator-1",
				"reason":"operator request",
				"requested_at":"2026-08-02T18:00:00Z"
			}
		}`)
		assertNativeOutputSchemaAccepts(t, nodes, `{
			"items":[{
				"state":"waiting",
				"loop_run_id":"run-1",
				"loop_name":"review",
				"generation":2,
				"node_id":"approval",
				"item_index":0,
				"state_at":"2026-08-02T18:00:00Z",
				"wait":{"kind":"human"}
			}],
			"next_cursor":"cursor-2"
		}`)
		if _, exists := descriptors[toolspkg.ToolID("compozy__loop_stop")]; exists {
			t.Fatal("deleted compozy__loop_stop descriptor is still registered")
		}
		for _, descriptor := range []toolspkg.Descriptor{status, runs} {
			if strings.Contains(string(descriptor.OutputSchema), "confidence") {
				t.Fatalf(
					"%s output schema contains deleted confidence field: %s",
					descriptor.ID,
					descriptor.OutputSchema,
				)
			}
		}
	})

	t.Run("Should publish closed extension inspection schemas and network digest inputs", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		inventory := descriptors[toolspkg.ToolIDExtensionsInventory]
		logs := descriptors[toolspkg.ToolIDExtensionsLogs]
		preview := descriptors[toolspkg.ToolIDExtensionsPreview]
		assertNativeOutputSchemaAccepts(t, logs, `{
			"stream_epoch":"epoch-a",
			"logs":[{
				"sequence":1,"timestamp":"2026-08-12T10:00:00Z","message":"ready",
				"generation_hash":"generation-a","stream_epoch":"epoch-a"
			}]
		}`)
		assertNativeOutputSchemaRejects(t, logs, `{
			"logs":[{
				"sequence":1,"timestamp":"2026-08-12T10:00:00Z","message":"ready",
				"stream_epoch":"epoch-a"
			}]
		}`)
		assertNativeOutputSchemaAccepts(t, inventory, `{
			"extension":"kit","enabled":true,
			"items":[{"kind":"skill","id":"review","name":"Review","live":true}]
		}`)
		assertNativeOutputSchemaRejects(t, inventory, `{
			"extension":"kit","enabled":true,
			"items":[{"kind":"skill","id":"review","name":"Review","live":true,"extra":1}]
		}`)
		assertNativeOutputSchemaAccepts(t, preview, `{
			"extension":"kit",
			"changes":[{"kind":"skill","id":"review","name":"Review","change":"changed"}],
			"agent_conflicts":[],"missing_env":["API_KEY"],"automation_starting":["daily"],
			"network_requirement_digest":"","network_confirmation_required":false
		}`)
		assertNativeOutputSchemaRejects(t, preview, `{
			"extension":"kit",
			"changes":[{"kind":"skill","id":"review","name":"Review","change":"updated"}],
			"agent_conflicts":[],"missing_env":[],"automation_starting":[],
			"network_requirement_digest":"","network_confirmation_required":false
		}`)

		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDExtensionsEnable,
			toolspkg.ToolIDExtensionsUpdate,
		} {
			var schema struct {
				Properties map[string]struct {
					Pattern string `json:"pattern"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(descriptors[id].InputSchema, &schema); err != nil {
				t.Fatalf("%s input schema unmarshal error = %v", id, err)
			}
			if got, want := schema.Properties["confirm_network_digest"].Pattern,
				"^[a-f0-9]{64}$"; got != want {
				t.Fatalf("%s confirm_network_digest pattern = %q, want %q", id, got, want)
			}
		}
	})

	t.Run("Should publish native schema digests and capability roster", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		cases := []struct {
			id         toolspkg.ToolID
			capability string
		}{
			{id: toolspkg.ToolIDResourcesList, capability: "resources.read"},
			{id: toolspkg.ToolIDResourcesSnapshot, capability: "resources.read"},
			{id: toolspkg.ToolIDProviderModelsCurate, capability: "providers.models.write"},
		}
		for _, tc := range cases {
			descriptor, ok := descriptors[tc.id]
			if !ok {
				t.Fatalf("descriptor %q missing", tc.id)
			}
			withDigests, err := toolspkg.DescriptorWithSchemaDigests(descriptor)
			if err != nil {
				t.Fatalf("DescriptorWithSchemaDigests(%s) error = %v", tc.id, err)
			}
			if strings.TrimSpace(withDigests.InputSchemaDigest) == "" {
				t.Fatalf("%s input schema digest is empty", tc.id)
			}
			if !slices.Contains(withDigests.Backend.RequiresCapabilities, tc.capability) {
				t.Fatalf(
					"%s capabilities = %#v, want %q",
					tc.id,
					withDigests.Backend.RequiresCapabilities,
					tc.capability,
				)
			}
		}
		lifecycleIDs := []toolspkg.ToolID{
			toolspkg.ToolIDLoopCancel,
			toolspkg.ToolIDLoopKill,
			toolspkg.ToolIDLoopNodes,
			toolspkg.ToolIDLoopNodePause,
			toolspkg.ToolIDLoopNodeResume,
			toolspkg.ToolIDLoopNodeCancel,
			toolspkg.ToolIDLoopNodeKill,
			toolspkg.ToolIDLoopNodeRequeue,
		}
		for _, id := range lifecycleIDs {
			descriptor, ok := descriptors[id]
			if !ok {
				t.Fatalf("descriptor %q missing", id)
			}
			withDigests, err := toolspkg.DescriptorWithSchemaDigests(descriptor)
			if err != nil {
				t.Fatalf("DescriptorWithSchemaDigests(%s) error = %v", id, err)
			}
			if strings.TrimSpace(withDigests.InputSchemaDigest) == "" ||
				strings.TrimSpace(withDigests.OutputSchemaDigest) == "" {
				t.Fatalf("%s lifecycle schema digests are incomplete", id)
			}
			if len(withDigests.Backend.RequiresCapabilities) != 0 {
				t.Fatalf(
					"%s capabilities = %#v, want no new capability gate",
					id,
					withDigests.Backend.RequiresCapabilities,
				)
			}
		}
	})

	t.Run("Should publish the closed window manager contract with risk and capability gates", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		type expectation struct {
			risk        toolspkg.RiskClass
			readOnly    bool
			destructive bool
			capability  string
		}
		expectations := map[toolspkg.ToolID]expectation{
			toolspkg.ToolIDDesktopList:    {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDDesktopCreate:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDDesktopUpdate:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDDesktopReorder: {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDDesktopSwitch:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDDesktopDelete:  {toolspkg.RiskDestructive, false, true, windowManagerWriteCapability},
			toolspkg.ToolIDDesktopClients: {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDWindowList:     {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDWindowOpen:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowNavigate: {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowClose:    {toolspkg.RiskDestructive, false, true, windowManagerWriteCapability},
			toolspkg.ToolIDWindowGroup:    {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowReorder:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowActivate: {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowPin:      {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowReopen:   {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowFocus:    {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowMove:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowResize:   {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowSwap:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowFloat:    {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowZoom:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutGet:      {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDLayoutPreview:  {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDLayoutArrange:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutResize:   {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutFrameResize: {
				toolspkg.RiskMutating,
				false,
				false,
				windowManagerWriteCapability,
			},
			toolspkg.ToolIDLayoutBalance:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutUndo:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutRedo:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutExport:   {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDLayoutValidate: {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDLayoutApply:    {toolspkg.RiskDestructive, false, true, windowManagerWriteCapability},
		}
		for _, id := range windowManagerExpectedToolIDs() {
			descriptor, ok := descriptors[id]
			if !ok {
				t.Fatalf("window-manager descriptor %q missing", id)
			}
			var inputSchema struct {
				AdditionalProperties *bool `json:"additionalProperties"`
			}
			if err := json.Unmarshal(descriptor.InputSchema, &inputSchema); err != nil {
				t.Fatalf("%s input schema unmarshal error = %v", id, err)
			}
			if inputSchema.AdditionalProperties == nil || *inputSchema.AdditionalProperties {
				t.Fatalf(
					"%s input schema additionalProperties = %#v, want explicit false",
					id,
					inputSchema.AdditionalProperties,
				)
			}
			want, ok := expectations[id]
			if !ok {
				t.Fatalf("window-manager descriptor %q has no independent contract expectation", id)
			}
			requireDescriptorRisk(t, descriptor, want.risk, want.readOnly, want.destructive, false)
			if !slices.Equal(descriptor.Backend.RequiresCapabilities, []string{want.capability}) {
				t.Fatalf(
					"%s capabilities = %#v, want [%q]",
					id,
					descriptor.Backend.RequiresCapabilities,
					want.capability,
				)
			}
			if !bytes.Contains(descriptor.OutputSchema, []byte(`"revision"`)) {
				t.Fatalf("%s output schema omits revision: %s", id, descriptor.OutputSchema)
			}
		}

		assertWindowManagerMoveSchema(t, descriptors[toolspkg.ToolIDWindowMove].InputSchema)
		assertWindowManagerPreviewSchema(t, descriptors[toolspkg.ToolIDLayoutPreview].InputSchema)
		assertWindowManagerResultSchemas(
			t,
			descriptors[toolspkg.ToolIDDesktopCreate].OutputSchema,
			descriptors[toolspkg.ToolIDLayoutPreview].OutputSchema,
		)
		assertWindowManagerLayoutDocumentSchemas(
			t,
			descriptors[toolspkg.ToolIDLayoutValidate].InputSchema,
			descriptors[toolspkg.ToolIDLayoutApply].InputSchema,
		)
	})

	t.Run("Should keep network schemas closed and hard-cut vocabulary out of descriptors", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		networkIDs := []toolspkg.ToolID{
			toolspkg.ToolIDNetworkSend,
			toolspkg.ToolIDNetworkChannelCreate,
			toolspkg.ToolIDNetworkThreads,
			toolspkg.ToolIDNetworkThreadMessages,
			toolspkg.ToolIDNetworkDirects,
			toolspkg.ToolIDNetworkDirectResolve,
			toolspkg.ToolIDNetworkDirectMessages,
			toolspkg.ToolIDNetworkWork,
		}
		for _, id := range networkIDs {
			descriptor := descriptors[id]
			var schema map[string]json.RawMessage
			if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
				t.Fatalf("%s input schema is invalid JSON: %v", id, err)
			}
			var additionalProperties bool
			if err := json.Unmarshal(schema["additionalProperties"], &additionalProperties); err != nil {
				t.Fatalf("%s additionalProperties = %s: %v", id, schema["additionalProperties"], err)
			}
			if additionalProperties {
				t.Fatalf("%s additionalProperties = true, want false", id)
			}
			schemaText := string(descriptor.InputSchema)
			if strings.Contains(schemaText, "interaction_id") {
				t.Fatalf("%s schema includes deleted interaction_id field: %s", id, schemaText)
			}
			if strings.Contains(schemaText, `"kind":"direct"`) ||
				strings.Contains(descriptor.Description, `kind:"direct"`) {
				t.Fatalf("%s descriptor teaches legacy direct message kind", id)
			}
		}

		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDNetworkSend,
			toolspkg.ToolIDNetworkDirects,
			toolspkg.ToolIDNetworkDirectResolve,
			toolspkg.ToolIDNetworkDirectMessages,
		} {
			description := strings.ToLower(descriptors[id].Description)
			if !strings.Contains(description, "runtime/audit access") ||
				!strings.Contains(description, "not cryptographic privacy") {
				t.Fatalf("%s description = %q, want explicit direct-room visibility boundary", id, description)
			}
			if strings.Contains(description, "encrypted") {
				t.Fatalf("%s description = %q, must not imply encrypted direct rooms", id, description)
			}
		}
	})

	t.Run("Should return cloned descriptors", func(t *testing.T) {
		t.Parallel()

		first := NativeDescriptors()
		first[0].ID = "compozy__mutated"
		first[0].InputSchema[0] = '['

		second := NativeDescriptors()
		if second[0].ID == "compozy__mutated" {
			t.Fatal("NativeDescriptors() reused descriptor slice")
		}
		if len(second[0].InputSchema) == 0 || second[0].InputSchema[0] == '[' {
			t.Fatal("NativeDescriptors() reused input schema bytes")
		}
	})

	t.Run("Should expose the closed gateway action contract", func(t *testing.T) {
		t.Parallel()

		descriptor := descriptorMap(NativeDescriptors())[toolspkg.ToolIDGateway]
		var input struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
			t.Fatalf("gateway input schema unmarshal error = %v", err)
		}
		wantActions := []string{
			"status", "audit", "device_list", "surface_set", "provider_enable", "provider_disable",
			"device_rename", "device_revoke", "ingress_bind", "ingress_unbind",
		}
		if got := input.Properties["action"].Enum; !slices.Equal(got, wantActions) {
			t.Fatalf("gateway action enum = %#v, want %#v", got, wantActions)
		}
		if len(descriptor.OutputSchema) == 0 || !json.Valid(descriptor.OutputSchema) {
			t.Fatal("gateway output schema is missing or invalid")
		}
	})
}

type nativeDescriptorExpectation struct {
	id          toolspkg.ToolID
	risk        toolspkg.RiskClass
	readOnly    bool
	destructive bool
	openWorld   bool
}

func nativeDescriptorExpectations() []nativeDescriptorExpectation {
	return []nativeDescriptorExpectation{
		{id: "compozy__agent_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__agent_heartbeat_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__agent_heartbeat_wake", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__automation_jobs_disable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_enable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_history", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_trigger", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_jobs_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_runs_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_runs_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_suggestions_accept", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_suggestions_dismiss", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__automation_suggestions_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__automation_triggers_disable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_enable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_history", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__automation_triggers_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__bridges_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__bridges_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__clarify", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__command_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__config_diff", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__config_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__config_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__config_path", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__config_set", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__config_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__config_unset", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__desktop_clients", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__desktop_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__desktop_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__desktop_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__desktop_reorder", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__desktop_switch", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__desktop_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_build", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_dev", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_disable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_enable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_info", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_init", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_install", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_inventory", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_logs", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_preview", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_provenance", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_publish", risk: toolspkg.RiskOpenWorld,
			readOnly: false, destructive: false, openWorld: true},
		{id: "compozy__extensions_reload", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_remove", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__extensions_search", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__extensions_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__extensions_validate", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__gateway", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__goal_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__goal_report", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__hooks_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__hooks_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__hooks_disable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__hooks_enable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__hooks_events", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__hooks_info", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__hooks_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__hooks_runs", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__hooks_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_apply", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__layout_arrange", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_balance", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_export", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__layout_frame_resize", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__layout_preview", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__layout_redo", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_resize", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_undo", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__layout_validate", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__logs", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_approve", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_cancel", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_configure", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__loop_inspect", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_kill", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__loop_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_node_cancel", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_node_kill", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__loop_node_pause", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_node_requeue", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_node_resume", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_nodes", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_pause", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_resume", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_run", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__loop_runs", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_turns", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__loop_validate", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__marketplace_search", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__mcp_auth_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__mcp_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_admin_history", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_daily_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_decisions_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_decisions_revert", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__memory_decisions_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_dream_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_dream_retry", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_dream_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_dream_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_dream_trigger", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_extractor_drain", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_extractor_failures", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_extractor_retry", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_extractor_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_health", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_note", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_promote", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_propose", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_provider_disable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_provider_enable", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_provider_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_provider_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_provider_select", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_recall_trace", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_reindex", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_reload", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_reset", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__memory_scope_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_search", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_session_ledger", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__memory_session_replay", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_sessions_prune", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__memory_sessions_repair", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__memory_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_channel_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__network_channel_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__network_channels", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_direct_messages", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_direct_resolve", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__network_directs", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_inbox", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_mute", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__network_peers", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_send", risk: toolspkg.RiskOpenWorld,
			readOnly: false, destructive: false, openWorld: true},
		{id: "compozy__network_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_subscribe", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__network_subscriptions", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_thread_messages", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_threads", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_unmute", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__network_usage", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__network_work", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__observe_metrics", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__observe_search", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__provider_models_curate", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__provider_models_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__provider_models_refresh", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__provider_models_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__resources_info", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__resources_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__resources_snapshot", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_archive", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_describe", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_events", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_health", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_history", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_input_cancel", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__session_input_promote", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_input_replace", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_inputs_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_rename", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_prompt", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_rewind", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__session_runtime_clear", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_runtime_set", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__session_status", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__session_unarchive", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__skill_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__skill_search", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__skill_view", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_block", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_blocks", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_cancel", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__task_child_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_execution_profile_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__task_execution_profile_get", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_execution_profile_set", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_worktree_policy_set", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_fanout_runs", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_notification_delete", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__task_notification_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_notification_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_notification_subscribe", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_promote_from_thread", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_read", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_recover", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_claim_next", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_complete", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_fail", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_heartbeat", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_run_release", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_review_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_run_review_request", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_run_review_show", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__task_run_review_submit", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_unblock", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__task_update", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__tool_approvals_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__tool_approvals_revoke", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__tool_approvals_set", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__tool_artifact_read", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__tool_info", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__tool_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__tool_search", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__window_activate", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_close", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__window_float", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_focus", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_group", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__window_move", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_navigate", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_open", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_pin", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_reopen", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_reorder", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_resize", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_swap", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__window_zoom", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__worktree_create", risk: toolspkg.RiskMutating,
			readOnly: false, destructive: false, openWorld: false},
		{id: "compozy__worktree_inspect", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__worktree_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__worktree_remove", risk: toolspkg.RiskDestructive,
			readOnly: false, destructive: true, openWorld: false},
		{id: "compozy__workspace_describe", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__workspace_info", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
		{id: "compozy__workspace_list", risk: toolspkg.RiskRead,
			readOnly: true, destructive: false, openWorld: false},
	}
}

func assertNativeOutputSchemaAccepts(t *testing.T, descriptor toolspkg.Descriptor, payload string) {
	t.Helper()
	compiled := compileNativeSchema(t, descriptor, descriptor.OutputSchema, "output")
	instance, err := jsonschema.UnmarshalJSON(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("%s valid output parse error = %v", descriptor.ID, err)
	}
	if err := compiled.Validate(instance); err != nil {
		t.Fatalf("%s output schema rejected %s: %v", descriptor.ID, payload, err)
	}
}

func assertNativeOutputSchemaRejects(t *testing.T, descriptor toolspkg.Descriptor, payload string) {
	t.Helper()
	compiled := compileNativeSchema(t, descriptor, descriptor.OutputSchema, "output")
	instance, err := jsonschema.UnmarshalJSON(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("%s invalid output parse error = %v", descriptor.ID, err)
	}
	if err := compiled.Validate(instance); err == nil {
		t.Fatalf("%s output schema accepted forbidden payload %s", descriptor.ID, payload)
	}
}

func compileNativeSchema(
	t *testing.T,
	descriptor toolspkg.Descriptor,
	schema json.RawMessage,
	schemaKind string,
) *jsonschema.Schema {
	t.Helper()
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		t.Fatalf("%s %s schema parse error = %v", descriptor.ID, schemaKind, err)
	}
	compiler := jsonschema.NewCompiler()
	resource := descriptor.ID.String() + "-" + schemaKind + ".json"
	if err := compiler.AddResource(resource, schemaValue); err != nil {
		t.Fatalf("%s %s schema add error = %v", descriptor.ID, schemaKind, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		t.Fatalf("%s %s schema compile error = %v", descriptor.ID, schemaKind, err)
	}
	return compiled
}

type nativeObjectSchema struct {
	Type                 string                     `json:"type"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	Enum                 []string                   `json:"enum"`
	OneOf                []json.RawMessage          `json:"oneOf"`
	Pattern              string                     `json:"pattern"`
	Minimum              *float64                   `json:"minimum"`
	MinLength            int                        `json:"minLength"`
	MaxLength            int                        `json:"maxLength"`
	Items                json.RawMessage            `json:"items"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
}

func assertNativeAgentNameInputSchema(
	t *testing.T,
	owner string,
	raw json.RawMessage,
	pattern string,
) {
	t.Helper()

	var schema nativeObjectSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s agent name schema unmarshal error = %v", owner, err)
	}
	if schema.Type != "string" {
		t.Fatalf("%s agent name type = %q, want string", owner, schema.Type)
	}
	if schema.Pattern != pattern {
		t.Fatalf("%s agent name pattern = %q, want %q", owner, schema.Pattern, pattern)
	}
	if schema.MaxLength != compozyconfig.AgentNameMaxLength {
		t.Fatalf(
			"%s agent name maxLength = %d, want %d",
			owner,
			schema.MaxLength,
			compozyconfig.AgentNameMaxLength,
		)
	}
}

func assertTypedNetworkParticipationSchema(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()
	var schema nativeObjectSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s network_participation schema unmarshal error = %v", owner, err)
	}
	assertClosedObjectSchema(t, owner+" network_participation", schema, []string{
		"bounds", "channel_id", "channel_strategy", "mode",
	})
	if got, want := len(schema.OneOf), 4; got != want {
		t.Fatalf("%s network_participation oneOf branches = %d, want %d", owner, got, want)
	}
	assertStringEnumSchema(
		t,
		owner+" network_participation.mode",
		schema.Properties["mode"],
		[]string{"local", "live"},
	)
	assertStringEnumSchema(
		t,
		owner+" network_participation.channel_strategy",
		schema.Properties["channel_strategy"],
		[]string{"named", "run", "loop_run"},
	)
	var channel nativeObjectSchema
	if err := json.Unmarshal(schema.Properties["channel_id"], &channel); err != nil ||
		channel.Type != "string" || channel.Pattern != networkParticipationChannelPattern {
		t.Fatalf(
			"%s network_participation.channel_id = %#v, error=%v, want patterned string",
			owner,
			channel,
			err,
		)
	}
	var bounds nativeObjectSchema
	if err := json.Unmarshal(schema.Properties["bounds"], &bounds); err != nil {
		t.Fatalf("%s network_participation.bounds unmarshal error = %v", owner, err)
	}
	assertClosedObjectSchema(t, owner+" network_participation.bounds", bounds, []string{
		"coalesce_window",
		"max_input_tokens",
		"max_output_tokens",
		"max_total_wall_time",
		"max_wake_depth",
		"max_wake_wall_time",
		"max_wakes",
	})
	for key, wantType := range map[string]string{
		"coalesce_window":     "string",
		"max_input_tokens":    "integer",
		"max_output_tokens":   "integer",
		"max_total_wall_time": "string",
		"max_wake_depth":      "integer",
		"max_wake_wall_time":  "string",
		"max_wakes":           "integer",
	} {
		var property nativeObjectSchema
		if err := json.Unmarshal(bounds.Properties[key], &property); err != nil || property.Type != wantType {
			t.Fatalf("%s network_participation.bounds.%s = %#v, error=%v, want %s", owner, key, property, err, wantType)
		}
		if wantType == "integer" && (property.Minimum == nil || *property.Minimum != 1) {
			t.Fatalf("%s network_participation.bounds.%s minimum = %#v, want 1", owner, key, property.Minimum)
		}
		if wantType == "string" && property.MinLength != 1 {
			t.Fatalf("%s network_participation.bounds.%s minLength = %d, want 1", owner, key, property.MinLength)
		}
	}
	assertNetworkParticipationSchemaMatchesRuntime(t, owner, raw)
}

func assertSessionCreateMutationSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, []string{
		"agent", "name", "network_participation", "new_worktree", "workspace", "worktree",
	})
	var newWorktree nativeObjectSchema
	if err := json.Unmarshal(input.Properties["new_worktree"], &newWorktree); err != nil {
		t.Fatalf("%s new_worktree schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" new_worktree", newWorktree, []string{"name"})
	assertTypedNetworkParticipationSchema(
		t,
		descriptor.ID.String(),
		input.Properties["network_participation"],
	)
	assertSessionMutationEnvelopeSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema, "session")
}

func assertSessionTargetMutationSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, []string{"session_id", "workspace"})
	if !slices.Equal(input.Required, []string{"session_id"}) {
		t.Fatalf("%s input required = %#v, want [session_id]", descriptor.ID, input.Required)
	}
	assertSessionMutationEnvelopeSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema, "session")
	var output nativeObjectSchema
	if err := json.Unmarshal(descriptor.OutputSchema, &output); err != nil {
		t.Fatalf("%s output schema unmarshal error = %v", descriptor.ID, err)
	}
	var sessionSchema nativeObjectSchema
	if err := json.Unmarshal(output.Properties["session"], &sessionSchema); err != nil {
		t.Fatalf("%s session output schema unmarshal error = %v", descriptor.ID, err)
	}
	if _, ok := sessionSchema.Properties["archived_at"]; !ok {
		t.Fatalf("%s session output schema omits archived_at", descriptor.ID)
	}
}

func assertSessionRenameMutationSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, []string{"name", "session_id", "workspace"})
	if !slices.Equal(input.Required, []string{"session_id", "name"}) {
		t.Fatalf("%s input required = %#v, want [session_id name]", descriptor.ID, input.Required)
	}
	var name nativeObjectSchema
	if err := json.Unmarshal(input.Properties["name"], &name); err != nil {
		t.Fatalf("%s name schema unmarshal error = %v", descriptor.ID, err)
	}
	if name.Type != "string" || name.MinLength != 1 || name.MaxLength != session.SessionNameMaxRunes {
		t.Fatalf("%s name schema = %#v, want 1..%d characters", descriptor.ID, name, session.SessionNameMaxRunes)
	}
	assertSessionMutationEnvelopeSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema, "session")
}

func assertSessionPromptMutationSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, []string{
		"attachments",
		"expected_turn_id",
		"idempotency_key",
		"message",
		"message_id",
		"mode",
		"runtime",
		"session_id",
		"workspace",
	})
	if !slices.Equal(input.Required, []string{
		"session_id", "message_id", "idempotency_key",
	}) {
		t.Fatalf(
			"%s input required = %#v, want session_id/message_id/idempotency_key",
			descriptor.ID,
			input.Required,
		)
	}
	var runtime nativeObjectSchema
	if err := json.Unmarshal(input.Properties["runtime"], &runtime); err != nil {
		t.Fatalf("%s runtime schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" runtime", runtime, []string{
		"model", "provider", "reasoning_effort", "speed",
	})
	if !slices.Equal(runtime.Required, []string{"provider"}) {
		t.Fatalf("%s runtime required = %#v, want [provider]", descriptor.ID, runtime.Required)
	}
	assertStringEnumSchema(
		t,
		descriptor.ID.String()+" mode",
		input.Properties["mode"],
		[]string{"queue", "interrupt", "steer"},
	)
	var expectedTurn nativeObjectSchema
	if err := json.Unmarshal(input.Properties["expected_turn_id"], &expectedTurn); err != nil {
		t.Fatalf("%s expected_turn_id schema unmarshal error = %v", descriptor.ID, err)
	}
	if expectedTurn.Type != "string" || expectedTurn.MinLength != 1 {
		t.Fatalf("%s expected_turn_id schema = %#v, want non-empty string", descriptor.ID, expectedTurn)
	}
	compiled := compileNativeSchema(t, descriptor, descriptor.InputSchema, "input")
	for _, tc := range []struct {
		name         string
		payload      string
		valid        bool
		matchesKind  func(jsonschema.ErrorKind) bool
		expectedKind string
	}{
		{
			name:    "Should accept a text prompt",
			payload: `{"session_id":"s","message_id":"m","idempotency_key":"k","message":"hello"}`,
			valid:   true,
		},
		{
			name:    "Should accept an attachment-only prompt",
			payload: `{"session_id":"s","message_id":"m","idempotency_key":"k","attachments":["att"]}`,
			valid:   true,
		},
		{
			name:         "Should reject missing prompt content",
			payload:      `{"session_id":"s","message_id":"m","idempotency_key":"k"}`,
			matchesKind:  validationErrorKindIsNot,
			expectedKind: "not",
		},
		{
			name:         "Should reject an empty message",
			payload:      `{"session_id":"s","message_id":"m","idempotency_key":"k","message":""}`,
			matchesKind:  validationErrorKindIsNot,
			expectedKind: "not",
		},
		{
			name:         "Should reject empty attachments",
			payload:      `{"session_id":"s","message_id":"m","idempotency_key":"k","attachments":[]}`,
			matchesKind:  validationErrorKindIsNot,
			expectedKind: "not",
		},
		{
			name:         "Should reject an attachment with an empty ID",
			payload:      `{"session_id":"s","message_id":"m","idempotency_key":"k","attachments":[""]}`,
			matchesKind:  validationErrorKindIsMinLength,
			expectedKind: "minLength",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			instance, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.payload))
			if err != nil {
				t.Fatalf("%s payload parse error = %v", descriptor.ID, err)
			}
			err = compiled.Validate(instance)
			if tc.valid {
				if err != nil {
					t.Fatalf("%s input schema rejected payload: %v", descriptor.ID, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s input schema accepted rejected payload", descriptor.ID)
			}
			var validationErr *jsonschema.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("%s validation error = %T, want *jsonschema.ValidationError", descriptor.ID, err)
			}
			if !validationErrorContainsKind(validationErr, tc.matchesKind) {
				t.Fatalf("%s validation cause = %#v, want %s", descriptor.ID, validationErr, tc.expectedKind)
			}
		})
	}
	assertSessionPromptMutationOutputSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema)
}

func validationErrorContainsKind(
	err *jsonschema.ValidationError,
	matches func(jsonschema.ErrorKind) bool,
) bool {
	if matches(err.ErrorKind) {
		return true
	}
	return slices.ContainsFunc(err.Causes, func(cause *jsonschema.ValidationError) bool {
		return validationErrorContainsKind(cause, matches)
	})
}

func validationErrorKindIsNot(errorKind jsonschema.ErrorKind) bool {
	_, ok := errorKind.(*jsonschemakind.Not)
	return ok
}

func validationErrorKindIsMinLength(errorKind jsonschema.ErrorKind) bool {
	_, ok := errorKind.(*jsonschemakind.MinLength)
	return ok
}

func assertSessionRuntimeMutationSchemas(
	t *testing.T,
	descriptors map[toolspkg.ToolID]toolspkg.Descriptor,
) {
	t.Helper()
	for _, id := range []toolspkg.ToolID{
		toolspkg.ToolIDSessionRuntimeSet,
		toolspkg.ToolIDSessionRuntimeClear,
	} {
		descriptor := descriptors[id]
		var input nativeObjectSchema
		if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
			t.Fatalf("%s input schema unmarshal error = %v", id, err)
		}
		wantFields := []string{"expected_revision", "session_id", "workspace"}
		if id == toolspkg.ToolIDSessionRuntimeSet {
			wantFields = append(wantFields, "runtime")
		}
		slices.Sort(wantFields)
		assertClosedObjectSchema(t, id.String()+" input", input, wantFields)
		if !slices.Contains(input.Required, "session_id") ||
			(id == toolspkg.ToolIDSessionRuntimeSet && !slices.Contains(input.Required, "runtime")) {
			t.Fatalf("%s required = %#v, want session_id and set runtime", id, input.Required)
		}
		assertSessionMutationEnvelopeSchema(t, id.String()+" output", descriptor.OutputSchema, "session")
	}
}

func assertSessionPromptMutationOutputSchema(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()
	var envelope nativeObjectSchema
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("%s schema unmarshal error = %v", owner, err)
	}
	assertClosedObjectSchema(t, owner, envelope, []string{"prompt"})
	var prompt nativeObjectSchema
	if err := json.Unmarshal(envelope.Properties["prompt"], &prompt); err != nil {
		t.Fatalf("%s prompt schema unmarshal error = %v", owner, err)
	}
	if prompt.Type != "object" {
		t.Fatalf("%s prompt schema = %#v, want object", owner, prompt)
	}
	assertClosedObjectSchema(t, owner+" prompt", prompt, []string{
		"canceled_queued_entries", "delivery", "estimated_send_at", "goal", "idempotency_key", "message_id",
		"mode", "new_turn_id", "previous_turn_id", "queue_entry_id", "queue_generation", "queue_position",
		"replayed", "status",
	})
	if !slices.Equal(prompt.Required, []string{"status", "delivery", "message_id", "idempotency_key", "replayed"}) {
		t.Fatalf(
			"%s prompt required = %#v, want status/delivery/message_id/idempotency_key/replayed",
			owner,
			prompt.Required,
		)
	}
	assertStringEnumSchema(
		t,
		owner+" prompt.delivery",
		prompt.Properties["delivery"],
		[]string{"none", "direct", "after_turn", "interrupt_then_prompt"},
	)
	assertStringEnumSchema(
		t,
		owner+" prompt.mode",
		prompt.Properties["mode"],
		[]string{"queue", "interrupt", "steer"},
	)
	for _, field := range []string{"message_id", "idempotency_key"} {
		var identity nativeObjectSchema
		if err := json.Unmarshal(prompt.Properties[field], &identity); err != nil {
			t.Fatalf("%s prompt.%s schema unmarshal error = %v", owner, field, err)
		}
		if identity.Type != "string" || identity.MinLength != 0 {
			t.Fatalf(
				"%s prompt.%s schema = %#v, want string without a minLength constraint",
				owner,
				field,
				identity,
			)
		}
	}
}

func assertSessionInputSchemas(t *testing.T, descriptors map[toolspkg.ToolID]toolspkg.Descriptor) {
	t.Helper()
	assertSessionInputsListSchema(t, descriptors[toolspkg.ToolIDSessionInputsList])
	assertSessionInputReplaceSchema(t, descriptors[toolspkg.ToolIDSessionInputReplace])
	assertSessionInputCancelSchema(t, descriptors[toolspkg.ToolIDSessionInputCancel])
	assertSessionInputPromoteSchema(t, descriptors[toolspkg.ToolIDSessionInputPromote])
}

func assertSessionInputsListSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, []string{"session_id", "workspace"})
	if !slices.Equal(input.Required, []string{"session_id"}) {
		t.Fatalf("%s input required = %#v, want [session_id]", descriptor.ID, input.Required)
	}
	var output nativeObjectSchema
	if err := json.Unmarshal(descriptor.OutputSchema, &output); err != nil {
		t.Fatalf("%s output schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" output", output, []string{"inputs"})
	var inputs nativeObjectSchema
	if err := json.Unmarshal(output.Properties["inputs"], &inputs); err != nil {
		t.Fatalf("%s outputs schema unmarshal error = %v", descriptor.ID, err)
	}
	if inputs.Type != "array" {
		t.Fatalf("%s outputs schema = %#v, want array", descriptor.ID, inputs)
	}
	var item nativeObjectSchema
	if err := json.Unmarshal(inputs.Items, &item); err != nil {
		t.Fatalf("%s output items schema unmarshal error = %v", descriptor.ID, err)
	}
	assertSessionInputPayloadSchema(t, descriptor.ID.String()+" output input", item)
}

func assertSessionInputReplaceSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	assertSessionInputMutationSchema(t, descriptor, []string{
		"idempotency_key", "message_id", "queue_entry_id", "session_id", "text", "workspace",
	}, []string{"session_id", "queue_entry_id", "text", "message_id", "idempotency_key"}, false)
	assertSessionInputOutputSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema)
}

func assertSessionInputCancelSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, []string{
		"queue_entry_id", "session_id", "workspace",
	})
	if !slices.Equal(input.Required, []string{"session_id", "queue_entry_id"}) {
		t.Fatalf("%s input required = %#v, want [session_id queue_entry_id]", descriptor.ID, input.Required)
	}
	assertSessionPromptMutationOutputSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema)
}

func assertSessionInputPromoteSchema(t *testing.T, descriptor toolspkg.Descriptor) {
	t.Helper()
	assertSessionInputMutationSchema(t, descriptor, []string{
		"expected_turn_id", "idempotency_key", "message_id", "queue_entry_id", "session_id", "text", "workspace",
	}, []string{
		"session_id", "queue_entry_id", "text", "message_id", "idempotency_key", "expected_turn_id",
	}, true)
	assertSessionPromptMutationOutputSchema(t, descriptor.ID.String()+" output", descriptor.OutputSchema)
}

func assertSessionInputMutationSchema(
	t *testing.T,
	descriptor toolspkg.Descriptor,
	wantProperties []string,
	wantRequired []string,
	requiresExpectedTurn bool,
) {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	assertClosedObjectSchema(t, descriptor.ID.String()+" input", input, wantProperties)
	if !slices.Equal(input.Required, wantRequired) {
		t.Fatalf("%s input required = %#v, want %#v", descriptor.ID, input.Required, wantRequired)
	}
	for _, field := range []string{"session_id", "queue_entry_id", "text", "message_id", "idempotency_key"} {
		var property nativeObjectSchema
		if err := json.Unmarshal(input.Properties[field], &property); err != nil {
			t.Fatalf("%s %s schema unmarshal error = %v", descriptor.ID, field, err)
		}
		if property.Type != "string" || property.MinLength != 1 {
			t.Fatalf("%s %s schema = %#v, want non-empty string", descriptor.ID, field, property)
		}
	}
	if requiresExpectedTurn {
		var expectedTurn nativeObjectSchema
		if err := json.Unmarshal(input.Properties["expected_turn_id"], &expectedTurn); err != nil {
			t.Fatalf("%s expected_turn_id schema unmarshal error = %v", descriptor.ID, err)
		}
		if expectedTurn.Type != "string" || expectedTurn.MinLength != 1 {
			t.Fatalf("%s expected_turn_id schema = %#v, want non-empty string", descriptor.ID, expectedTurn)
		}
	}
}

func assertSessionInputOutputSchema(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()
	var envelope nativeObjectSchema
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("%s schema unmarshal error = %v", owner, err)
	}
	assertClosedObjectSchema(t, owner, envelope, []string{"input"})
	var input nativeObjectSchema
	if err := json.Unmarshal(envelope.Properties["input"], &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", owner, err)
	}
	assertSessionInputPayloadSchema(t, owner+" input", input)
}

func assertSessionInputPayloadSchema(t *testing.T, owner string, input nativeObjectSchema) {
	t.Helper()
	assertClosedObjectSchema(t, owner, input, []string{
		"delivery", "enqueued_at", "id", "idempotency_key", "message_id", "mode", "queue_generation",
		"runtime", "session_id", "status", "target_turn_id", "text",
	})
	if !slices.Equal(input.Required, []string{
		"id", "session_id", "status", "mode", "delivery", "text", "queue_generation", "enqueued_at",
	}) {
		t.Fatalf("%s required = %#v, want durable session input fields", owner, input.Required)
	}
	assertStringEnumSchema(t, owner+" mode", input.Properties["mode"], []string{"queue", "interrupt", "steer"})
	assertStringEnumSchema(
		t,
		owner+" delivery",
		input.Properties["delivery"],
		[]string{"none", "direct", "after_turn", "interrupt_then_prompt"},
	)
}

func assertSessionMutationEnvelopeSchema(
	t *testing.T,
	owner string,
	raw json.RawMessage,
	key string,
) {
	t.Helper()
	var envelope nativeObjectSchema
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("%s schema unmarshal error = %v", owner, err)
	}
	assertClosedObjectSchema(t, owner, envelope, []string{key})
	var payload nativeObjectSchema
	if err := json.Unmarshal(envelope.Properties[key], &payload); err != nil {
		t.Fatalf("%s payload schema unmarshal error = %v", owner, err)
	}
	if payload.Type != "object" {
		t.Fatalf("%s payload = %#v, want object", owner, payload)
	}
}

func assertNetworkParticipationSchemaMatchesRuntime(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s network_participation schema parse error = %v", owner, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("network_participation.json", schemaValue); err != nil {
		t.Fatalf("%s network_participation schema add error = %v", owner, err)
	}
	compiled, err := compiler.Compile("network_participation.json")
	if err != nil {
		t.Fatalf("%s network_participation schema compile error = %v", owner, err)
	}

	for _, payload := range []string{
		`{}`,
		`{"mode":"local"}`,
		`{"mode":"local","bounds":{"max_wakes":1}}`,
		`{"mode":"live"}`,
		`{"mode":"live","channel_strategy":"named"}`,
		`{"mode":"live","channel_strategy":"named","channel_id":"builders"}`,
		`{"mode":"live","channel_strategy":"named","channel_id":"Invalid channel"}`,
		`{"mode":"live","channel_strategy":"run"}`,
		`{"mode":"live","channel_strategy":"run","channel_id":"builders"}`,
		`{"mode":"live","channel_strategy":"loop_run","bounds":{"max_wakes":2}}`,
		`{"mode":"live","channel_strategy":"loop_run","bounds":{"max_wakes":0}}`,
	} {
		var request participation.Request
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			t.Fatalf("%s runtime participation unmarshal error = %v", owner, err)
		}
		_, runtimeErr := participation.NormalizeIntent(request)
		instance, err := jsonschema.UnmarshalJSON(strings.NewReader(payload))
		if err != nil {
			t.Fatalf("%s schema instance parse error = %v", owner, err)
		}
		schemaErr := compiled.Validate(instance)
		if (runtimeErr == nil) != (schemaErr == nil) {
			t.Fatalf(
				"%s payload %s runtime error=%v schema error=%v, want matching validity",
				owner,
				payload,
				runtimeErr,
				schemaErr,
			)
		}
	}
	unknown, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"mode":"local","legacy":true}`))
	if err != nil {
		t.Fatalf("%s unknown-field instance parse error = %v", owner, err)
	}
	if err := compiled.Validate(unknown); err == nil {
		t.Fatalf("%s network_participation schema accepted an unknown field", owner)
	}
}

func assertWindowManagerMoveSchema(t *testing.T, raw json.RawMessage) {
	t.Helper()
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("window move schema parse error = %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("window_move.json", schemaValue); err != nil {
		t.Fatalf("window move schema add error = %v", err)
	}
	compiled, err := compiler.Compile("window_move.json")
	if err != nil {
		t.Fatalf("window move schema compile error = %v", err)
	}

	cases := []struct {
		name    string
		payload string
		valid   bool
	}{
		{
			name: "Should accept structural placement",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","placement":"right"}`,
			valid: true,
		},
		{
			name: "Should accept exclusive group relocation",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true}`,
			valid: true,
		},
		{
			name: "Should reject missing placement outside group mode",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b"}`,
		},
		{
			name: "Should accept structural group relocation onto a target",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true,` +
				`"target_window_id":"window-b","placement":"right"}`,
			valid: true,
		},
		{
			name: "Should accept a frame relocation with a floating rectangle",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true,` +
				`"floating_rect":{"x":0,"y":0,"width":0.5,"height":0.5}}`,
			valid: true,
		},
		{
			name: "Should reject a structural group relocation with a floating rectangle",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true,` +
				`"target_window_id":"window-b",` +
				`"floating_rect":{"x":0,"y":0,"width":0.5,"height":0.5}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			instance, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.payload))
			if err != nil {
				t.Fatalf("window move instance parse error = %v", err)
			}
			err = compiled.Validate(instance)
			if tc.valid && err != nil {
				t.Fatalf("window move schema rejected %s: %v", tc.payload, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("window move schema accepted invalid payload %s", tc.payload)
			}
		})
	}
}

func assertWindowManagerPreviewSchema(t *testing.T, raw json.RawMessage) {
	t.Helper()
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("layout preview schema parse error = %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("layout_preview.json", schemaValue); err != nil {
		t.Fatalf("layout preview schema add error = %v", err)
	}
	compiled, err := compiler.Compile("layout_preview.json")
	if err != nil {
		t.Fatalf("layout preview schema compile error = %v", err)
	}

	cases := []struct {
		name      string
		commandID string
		clientID  string
		valid     bool
	}{
		{name: "Should accept a durable command without a client", commandID: "desktop.update", valid: true},
		{
			name:      "Should accept a client-local desktop switch with a client",
			commandID: "desktop.switch",
			clientID:  "client-a",
			valid:     true,
		},
		{name: "Should reject a client-local desktop switch without a client", commandID: "desktop.switch"},
		{name: "Should reject a client-local window focus without a client", commandID: "window.focus"},
		{name: "Should reject a client-local window zoom without a client", commandID: "window.zoom"},
		{name: "Should accept window stack group", commandID: "window.stack.group", valid: true},
		{name: "Should accept window stack reorder", commandID: "window.stack.reorder", valid: true},
		{name: "Should accept window stack activation", commandID: "window.stack.set_active", valid: true},
		{name: "Should accept window pin", commandID: "window.pin", valid: true},
		{name: "Should accept window reopen", commandID: "window.reopen", valid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := map[string]any{
				"expected_revision": 1,
				"command_id":        tc.commandID,
				"payload":           map[string]any{},
			}
			if tc.clientID != "" {
				payload["client_id"] = tc.clientID
			}
			rawPayload, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal(layout preview input) error = %v", err)
			}
			instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(rawPayload))
			if err != nil {
				t.Fatalf("layout preview instance parse error = %v", err)
			}
			err = compiled.Validate(instance)
			if tc.valid && err != nil {
				t.Fatalf("layout preview schema rejected %s: %v", rawPayload, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("layout preview schema accepted invalid payload %s", rawPayload)
			}
		})
	}
}

func assertWindowManagerResultSchemas(t *testing.T, commandRaw, previewRaw json.RawMessage) {
	t.Helper()
	type resultSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	var command resultSchema
	if err := json.Unmarshal(commandRaw, &command); err != nil {
		t.Fatalf("window manager command output schema unmarshal error = %v", err)
	}
	assertSchemaFields(t, "window manager command output", command, []string{
		"applied", "changes", "client", "command_id", "diagnostics", "rebased_from", "revision", "workspace_id",
	}, []string{"applied", "changes", "command_id", "diagnostics", "revision", "workspace_id"})
	assertWindowManagerChangesSchema(t, "window manager command output", command.Properties["changes"])

	var preview resultSchema
	if err := json.Unmarshal(previewRaw, &preview); err != nil {
		t.Fatalf("window manager preview output schema unmarshal error = %v", err)
	}
	assertSchemaFields(t, "window manager preview output", preview, []string{
		"changed", "changes", "client", "command_id", "diagnostics", "revision", "snapshot", "workspace_id",
	}, []string{"changed", "changes", "command_id", "diagnostics", "revision", "snapshot", "workspace_id"})
	assertWindowManagerChangesSchema(t, "window manager preview output", preview.Properties["changes"])
}

func assertWindowManagerChangesSchema(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()
	var changes struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &changes); err != nil {
		t.Fatalf("%s changes schema unmarshal error = %v", owner, err)
	}
	assertSchemaFields(t, owner+" changes", changes, []string{
		"client_ids", "desktop_ids", "group_ids", "node_ids", "stack_grouped", "stack_ungrouped", "window_ids",
	}, nil)
}

func assertWindowManagerLayoutDocumentSchemas(t *testing.T, schemas ...json.RawMessage) {
	t.Helper()
	for _, raw := range schemas {
		var input struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			t.Fatalf("window manager layout input schema unmarshal error = %v", err)
		}
		var document struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(input.Properties["document"], &document); err != nil {
			t.Fatalf("window manager layout document schema unmarshal error = %v", err)
		}
		var version struct {
			Const int `json:"const"`
		}
		if err := json.Unmarshal(document.Properties["version"], &version); err != nil {
			t.Fatalf("window manager layout version schema unmarshal error = %v", err)
		}
		if version.Const != int(windowmanager.SnapshotVersion) {
			t.Fatalf("window manager layout version const = %d, want %d", version.Const, windowmanager.SnapshotVersion)
		}
	}
}

func assertSchemaFields(
	t *testing.T,
	owner string,
	schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	},
	wantProperties []string,
	wantRequired []string,
) {
	t.Helper()
	properties := make([]string, 0, len(schema.Properties))
	for property := range schema.Properties {
		properties = append(properties, property)
	}
	sort.Strings(properties)
	sort.Strings(schema.Required)
	if !slices.Equal(properties, wantProperties) {
		t.Fatalf("%s properties = %#v, want %#v", owner, properties, wantProperties)
	}
	if !slices.Equal(schema.Required, wantRequired) {
		t.Fatalf("%s required = %#v, want %#v", owner, schema.Required, wantRequired)
	}
}

func assertClosedObjectSchema(t *testing.T, owner string, schema nativeObjectSchema, wantKeys []string) {
	t.Helper()
	if schema.Type != "object" || schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Fatalf("%s = %#v, want closed object", owner, schema)
	}
	gotKeys := make([]string, 0, len(schema.Properties))
	for key := range schema.Properties {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !slices.Equal(gotKeys, wantKeys) {
		t.Fatalf("%s properties = %#v, want %#v", owner, gotKeys, wantKeys)
	}
}

func decodeNativeObjectSchema(
	t *testing.T,
	descriptor toolspkg.Descriptor,
	property string,
) nativeObjectSchema {
	t.Helper()
	var input nativeObjectSchema
	if err := json.Unmarshal(descriptor.InputSchema, &input); err != nil {
		t.Fatalf("%s input schema unmarshal error = %v", descriptor.ID, err)
	}
	raw, ok := input.Properties[property]
	if !ok {
		t.Fatalf("%s input schema omits %s", descriptor.ID, property)
	}
	return decodeRawNativeObjectSchema(t, descriptor.ID.String()+" "+property, raw)
}

func decodeRawNativeObjectSchema(t *testing.T, owner string, raw json.RawMessage) nativeObjectSchema {
	t.Helper()
	var schema nativeObjectSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s schema unmarshal error = %v", owner, err)
	}
	return schema
}

func assertNativeLoopEnvironmentSchema(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()
	environment := decodeRawNativeObjectSchema(t, owner+" environment", raw)
	assertClosedObjectSchema(
		t,
		owner+" environment",
		environment,
		[]string{"directory", "mode", "worktree_ref"},
	)
	if !slices.Equal(environment.Required, []string{"mode"}) {
		t.Fatalf("%s environment required = %#v, want mode", owner, environment.Required)
	}
	assertStringEnumSchema(
		t,
		owner+" environment.mode",
		environment.Properties["mode"],
		[]string{"root", "worktree", "per_run", "directory"},
	)
}

func assertStringEnumSchema(t *testing.T, owner string, raw json.RawMessage, want []string) {
	t.Helper()
	var schema nativeObjectSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s schema unmarshal error = %v", owner, err)
	}
	if schema.Type != "string" || !slices.Equal(schema.Enum, want) {
		t.Fatalf("%s schema = %#v, want string enum %#v", owner, schema, want)
	}
}

func TestBuiltinNativeWorkspaceInputContract(t *testing.T) {
	t.Parallel()

	descriptors := descriptorMap(NativeDescriptors())
	for _, id := range []toolspkg.ToolID{
		toolspkg.ToolIDTaskNotificationSubscribe,
		toolspkg.ToolIDTaskNotificationList,
	} {
		descriptor, ok := descriptors[id]
		if !ok {
			t.Fatalf("descriptor %q missing", id)
		}
		var schema nativeObjectSchema
		if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
			t.Fatalf("json.Unmarshal(%s schema) error = %v", id, err)
		}
		if _, ok := schema.Properties["workspace_id"]; !ok {
			t.Fatalf("%s properties omit opaque workspace_id", id)
		}
		if _, ok := schema.Properties["workspace"]; ok {
			t.Fatalf("%s properties retain removed workspace alias", id)
		}
	}

	for _, descriptor := range NativeDescriptors() {
		t.Run("Should preserve the canonical workspace input contract for "+descriptor.ID.String(), func(t *testing.T) {
			t.Parallel()

			assertNativeWorkspaceInputSchema(t, descriptor.ID.String(), descriptor.InputSchema)
		})
	}
}

func assertNativeWorkspaceInputSchema(t *testing.T, owner string, raw json.RawMessage) {
	t.Helper()

	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) {
		return
	}
	var schema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
		AllOf      []json.RawMessage          `json:"allOf"`
		AnyOf      []json.RawMessage          `json:"anyOf"`
		OneOf      []json.RawMessage          `json:"oneOf"`
		Items      json.RawMessage            `json:"items"`
	}
	if err := json.Unmarshal(trimmed, &schema); err != nil {
		t.Fatalf("json.Unmarshal(%s schema) error = %v", owner, err)
	}
	for _, legacy := range []string{"workspace_id", "workspace_root"} {
		if _, ok := schema.Properties[legacy]; ok && !nativeWorkspaceContractFieldAllowed(owner, legacy) {
			t.Fatalf("%s properties include legacy workspace field %q", owner, legacy)
		}
	}
	if slices.Contains(schema.Required, "workspace") {
		t.Fatalf("%s requires workspace even though dispatch can bind caller scope", owner)
	}
	for name, property := range schema.Properties {
		assertNativeWorkspaceInputSchema(t, owner+"."+name, property)
	}
	for keyword, branches := range map[string][]json.RawMessage{
		"allOf": schema.AllOf,
		"anyOf": schema.AnyOf,
		"oneOf": schema.OneOf,
	} {
		for index, branch := range branches {
			assertNativeWorkspaceInputSchema(t, fmt.Sprintf("%s.%s[%d]", owner, keyword, index), branch)
		}
	}
	if len(bytes.TrimSpace(schema.Items)) > 0 {
		assertNativeWorkspaceInputSchema(t, owner+".items", schema.Items)
	}
}

func nativeWorkspaceContractFieldAllowed(owner string, field string) bool {
	if field != "workspace_id" {
		return false
	}
	switch owner {
	case "compozy__layout_apply.document",
		"compozy__layout_validate.document",
		toolspkg.ToolIDTaskNotificationList.String(),
		toolspkg.ToolIDTaskNotificationSubscribe.String():
		return true
	default:
		return false
	}
}

func TestBuiltinToolsetCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should expand built-in toolsets into canonical MVP tools", func(t *testing.T) {
		t.Parallel()

		descriptors := NativeDescriptors()
		universe := make([]toolspkg.ToolID, 0, len(descriptors))
		for _, descriptor := range descriptors {
			universe = append(universe, descriptor.ID)
		}
		catalog, err := ToolsetCatalog()
		if err != nil {
			t.Fatalf("ToolsetCatalog() error = %v", err)
		}

		bootstrap, err := catalog.Expand(toolspkg.ToolsetIDBootstrap, universe)
		if err != nil {
			t.Fatalf("Expand(bootstrap) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDToolArtifactRead,
			toolspkg.ToolIDToolInfo,
			toolspkg.ToolIDToolList,
			toolspkg.ToolIDToolSearch,
		}; !slices.Equal(
			bootstrap,
			want,
		) {
			t.Fatalf("bootstrap expansion = %#v, want %#v", bootstrap, want)
		}

		artifacts, err := catalog.Expand(toolspkg.ToolsetIDToolArtifacts, universe)
		if err != nil {
			t.Fatalf("Expand(tool artifacts) error = %v", err)
		}
		if want := []toolspkg.ToolID{toolspkg.ToolIDToolArtifactRead}; !slices.Equal(artifacts, want) {
			t.Fatalf("tool artifact expansion = %#v, want %#v", artifacts, want)
		}

		approvals, err := catalog.Expand(toolspkg.ToolsetIDToolApprovals, universe)
		if err != nil {
			t.Fatalf("Expand(tool approvals) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDToolApprovalsList,
			toolspkg.ToolIDToolApprovalsRevoke,
			toolspkg.ToolIDToolApprovalsSet,
		}; !slices.Equal(approvals, want) {
			t.Fatalf("tool approval expansion = %#v, want %#v", approvals, want)
		}

		clarify, err := catalog.Expand(toolspkg.ToolsetIDClarify, universe)
		if err != nil {
			t.Fatalf("Expand(clarify) error = %v", err)
		}
		if want := []toolspkg.ToolID{toolspkg.ToolIDClarify}; !slices.Equal(clarify, want) {
			t.Fatalf("clarify expansion = %#v, want %#v", clarify, want)
		}

		tasks, err := catalog.Expand(toolspkg.ToolsetIDTasks, universe)
		if err != nil {
			t.Fatalf("Expand(tasks) error = %v", err)
		}
		if !slices.Contains(tasks, toolspkg.ToolIDTaskChildCreate) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskBlock) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskUnblock) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskBlocks) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskRecover) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskRunReviewRequest) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskRunReviewList) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskRunReviewShow) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskExecutionProfileGet) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskExecutionProfileSet) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskWorktreePolicySet) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskExecutionProfileDelete) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskNotificationSubscribe) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskNotificationList) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskNotificationShow) ||
			!slices.Contains(tasks, toolspkg.ToolIDTaskNotificationDelete) ||
			slices.Contains(tasks, toolspkg.ToolIDTaskRunClaimNext) {
			t.Fatalf("task toolset expansion = %#v, want bounded task scope", tasks)
		}
		autonomy, err := catalog.Expand(toolspkg.ToolsetIDAutonomy, universe)
		if err != nil {
			t.Fatalf("Expand(autonomy) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDTaskRunClaimNext,
			toolspkg.ToolIDTaskRunComplete,
			toolspkg.ToolIDTaskRunFail,
			toolspkg.ToolIDTaskRunHeartbeat,
			toolspkg.ToolIDTaskRunRelease,
			toolspkg.ToolIDTaskRunReviewSubmit,
		}; !slices.Equal(autonomy, want) {
			t.Fatalf("autonomy expansion = %#v, want %#v", autonomy, want)
		}

		coordination, err := catalog.Expand(toolspkg.ToolsetIDCoordination, universe)
		if err != nil {
			t.Fatalf("Expand(coordination) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDNetworkChannelCreate,
			toolspkg.ToolIDNetworkChannelUpdate,
			toolspkg.ToolIDNetworkChannels,
			toolspkg.ToolIDNetworkDirectMessages,
			toolspkg.ToolIDNetworkDirectResolve,
			toolspkg.ToolIDNetworkDirects,
			toolspkg.ToolIDNetworkInbox,
			toolspkg.ToolIDNetworkMute,
			toolspkg.ToolIDNetworkPeers,
			toolspkg.ToolIDNetworkSend,
			toolspkg.ToolIDNetworkStatus,
			toolspkg.ToolIDNetworkSubscribe,
			toolspkg.ToolIDNetworkSubscriptions,
			toolspkg.ToolIDNetworkThreadMessages,
			toolspkg.ToolIDNetworkThreads,
			toolspkg.ToolIDNetworkUnmute,
			toolspkg.ToolIDNetworkUsage,
			toolspkg.ToolIDNetworkWork,
		}; !slices.Equal(coordination, want) {
			t.Fatalf("coordination expansion = %#v, want %#v", coordination, want)
		}

		sessions, err := catalog.Expand(toolspkg.ToolsetIDSessions, universe)
		if err != nil {
			t.Fatalf("Expand(sessions) error = %v", err)
		}
		if !slices.Contains(sessions, toolspkg.ToolIDSessionList) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionArchive) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionUnarchive) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionRename) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionCreate) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionPrompt) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionRewind) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionInputsList) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionInputReplace) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionInputCancel) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionInputPromote) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionDescribe) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionHealth) ||
			slices.Contains(sessions, toolspkg.ToolID("compozy__session_stop")) {
			t.Fatalf("sessions toolset expansion = %#v, want session read and mutation tools", sessions)
		}

		authoredContext, err := catalog.Expand(toolspkg.ToolsetIDAuthoredContext, universe)
		if err != nil {
			t.Fatalf("Expand(authored_context) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDAgentHeartbeatStatus,
			toolspkg.ToolIDAgentHeartbeatWake,
			toolspkg.ToolIDSessionHealth,
		}; !slices.Equal(authoredContext, want) {
			t.Fatalf("authored context expansion = %#v, want %#v", authoredContext, want)
		}

		workspace, err := catalog.Expand(toolspkg.ToolsetIDWorkspace, universe)
		if err != nil {
			t.Fatalf("Expand(workspace) error = %v", err)
		}
		if !slices.Contains(workspace, toolspkg.ToolIDWorkspaceList) ||
			!slices.Contains(workspace, toolspkg.ToolIDWorkspaceDescribe) ||
			!slices.Contains(workspace, toolspkg.ToolIDAgentCreate) ||
			slices.Contains(workspace, toolspkg.ToolID("compozy__workspace_remove")) {
			t.Fatalf("workspace toolset expansion = %#v, want workspace read + agent authoring tools", workspace)
		}

		providerModels, err := catalog.Expand(toolspkg.ToolsetIDProviderModels, universe)
		if err != nil {
			t.Fatalf("Expand(provider_models) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDProviderModelsCurate,
			toolspkg.ToolIDProviderModelsList,
			toolspkg.ToolIDProviderModelsRefresh,
			toolspkg.ToolIDProviderModelsStatus,
		}; !slices.Equal(providerModels, want) {
			t.Fatalf("provider models expansion = %#v, want %#v", providerModels, want)
		}

		memory, err := catalog.Expand(toolspkg.ToolsetIDMemory, universe)
		if err != nil {
			t.Fatalf("Expand(memory) error = %v", err)
		}
		if !slices.Contains(memory, toolspkg.ToolIDMemoryShow) ||
			!slices.Contains(memory, toolspkg.ToolIDMemoryPropose) ||
			!slices.Contains(memory, toolspkg.ToolIDMemoryNote) ||
			slices.Contains(memory, toolspkg.ToolIDMemoryHealth) ||
			slices.Contains(memory, toolspkg.ToolIDMemoryReset) ||
			slices.Contains(memory, toolspkg.ToolID("compozy__memory_read")) ||
			slices.Contains(memory, toolspkg.ToolID("compozy__memory_history")) ||
			slices.Contains(memory, toolspkg.ToolID("compozy__memory_write")) {
			t.Fatalf("memory toolset expansion = %#v, want Memory v2 Slice 1 tools", memory)
		}

		memoryAdmin, err := catalog.Expand(toolspkg.ToolsetIDMemoryAdmin, universe)
		if err != nil {
			t.Fatalf("Expand(memory_admin) error = %v", err)
		}
		if want := []toolspkg.ToolID{
			toolspkg.ToolIDMemoryAdminHistory,
			toolspkg.ToolIDMemoryDailyList,
			toolspkg.ToolIDMemoryDecisionsList,
			toolspkg.ToolIDMemoryDecisionsRevert,
			toolspkg.ToolIDMemoryDecisionsShow,
			toolspkg.ToolIDMemoryDreamList,
			toolspkg.ToolIDMemoryDreamRetry,
			toolspkg.ToolIDMemoryDreamShow,
			toolspkg.ToolIDMemoryDreamStatus,
			toolspkg.ToolIDMemoryDreamTrigger,
			toolspkg.ToolIDMemoryExtractorDrain,
			toolspkg.ToolIDMemoryExtractorFailures,
			toolspkg.ToolIDMemoryExtractorRetry,
			toolspkg.ToolIDMemoryExtractorStatus,
			toolspkg.ToolIDMemoryHealth,
			toolspkg.ToolIDMemoryPromote,
			toolspkg.ToolIDMemoryProviderDisable,
			toolspkg.ToolIDMemoryProviderEnable,
			toolspkg.ToolIDMemoryProviderGet,
			toolspkg.ToolIDMemoryProviderList,
			toolspkg.ToolIDMemoryProviderSelect,
			toolspkg.ToolIDMemoryRecallTrace,
			toolspkg.ToolIDMemoryReindex,
			toolspkg.ToolIDMemoryReload,
			toolspkg.ToolIDMemoryReset,
			toolspkg.ToolIDMemoryScopeShow,
			toolspkg.ToolIDMemorySessionLedger,
			toolspkg.ToolIDMemorySessionReplay,
			toolspkg.ToolIDMemorySessionsPrune,
			toolspkg.ToolIDMemorySessionsRepair,
		}; !slices.Equal(memoryAdmin, want) {
			t.Fatalf("memory admin toolset expansion = %#v, want %#v", memoryAdmin, want)
		}

		observe, err := catalog.Expand(toolspkg.ToolsetIDObserve, universe)
		if err != nil {
			t.Fatalf("Expand(observe) error = %v", err)
		}
		if !slices.Contains(observe, toolspkg.ToolIDListLogs) ||
			!slices.Contains(observe, toolspkg.ToolIDObserveMetrics) ||
			slices.Contains(observe, toolspkg.ToolID("compozy__observe_delete")) {
			t.Fatalf("observe toolset expansion = %#v, want read-only observe tools", observe)
		}

		bridges, err := catalog.Expand(toolspkg.ToolsetIDBridges, universe)
		if err != nil {
			t.Fatalf("Expand(bridges) error = %v", err)
		}
		if !slices.Contains(bridges, toolspkg.ToolIDBridgesList) ||
			!slices.Contains(bridges, toolspkg.ToolIDBridgesStatus) ||
			slices.Contains(bridges, toolspkg.ToolID("compozy__bridges_update")) {
			t.Fatalf("bridges toolset expansion = %#v, want read-only bridge tools", bridges)
		}

		config, err := catalog.Expand(toolspkg.ToolsetIDConfig, universe)
		if err != nil {
			t.Fatalf("Expand(config) error = %v", err)
		}
		if !slices.Contains(config, toolspkg.ToolIDConfigSet) ||
			!slices.Contains(config, toolspkg.ToolIDConfigUnset) {
			t.Fatalf("config toolset expansion = %#v, want mutable config tools", config)
		}

		hooks, err := catalog.Expand(toolspkg.ToolsetIDHooks, universe)
		if err != nil {
			t.Fatalf("Expand(hooks) error = %v", err)
		}
		if !slices.Contains(hooks, toolspkg.ToolIDHooksCreate) ||
			!slices.Contains(hooks, toolspkg.ToolIDHooksDisable) {
			t.Fatalf("hooks toolset expansion = %#v, want mutable hook tools", hooks)
		}

		automation, err := catalog.Expand(toolspkg.ToolsetIDAutomation, universe)
		if err != nil {
			t.Fatalf("Expand(automation) error = %v", err)
		}
		if !slices.Contains(automation, toolspkg.ToolIDAutomationJobsCreate) ||
			!slices.Contains(automation, toolspkg.ToolIDAutomationRunsGet) ||
			slices.Contains(automation, toolspkg.ToolID("compozy__automation_webhook_secret_set")) {
			t.Fatalf("automation toolset expansion = %#v, want bounded automation tools", automation)
		}

		loops, err := catalog.Expand(toolspkg.ToolsetIDLoops, universe)
		if err != nil {
			t.Fatalf("Expand(loops) error = %v", err)
		}
		if !slices.Contains(loops, toolspkg.ToolIDLoopRun) ||
			!slices.Contains(loops, toolspkg.ToolIDLoopApprove) ||
			!slices.Contains(loops, toolspkg.ToolIDGoalGet) ||
			!slices.Contains(loops, toolspkg.ToolIDGoalReport) ||
			!slices.Contains(loops, toolspkg.ToolIDLoopTurns) ||
			slices.Contains(loops, toolspkg.ToolID("compozy__loop_edit")) {
			t.Fatalf("loops toolset expansion = %#v, want bounded loop tools without edit", loops)
		}

		extensions, err := catalog.Expand(toolspkg.ToolsetIDExtensions, universe)
		if err != nil {
			t.Fatalf("Expand(extensions) error = %v", err)
		}
		if !slices.Contains(extensions, toolspkg.ToolIDExtensionsInstall) ||
			!slices.Contains(extensions, toolspkg.ToolIDExtensionsRemove) ||
			slices.Contains(extensions, toolspkg.ToolID("compozy__extensions_trust_root_set")) {
			t.Fatalf("extensions toolset expansion = %#v, want bounded extension lifecycle tools", extensions)
		}

		marketplace, err := catalog.Expand(toolspkg.ToolsetIDMarketplace, universe)
		if err != nil {
			t.Fatalf("Expand(marketplace) error = %v", err)
		}
		if !slices.Equal(marketplace, []toolspkg.ToolID{toolspkg.ToolIDMarketplaceSearch}) {
			t.Fatalf("marketplace toolset expansion = %#v, want marketplace search", marketplace)
		}

		resourceTools, err := catalog.Expand(toolspkg.ToolsetIDResources, universe)
		if err != nil {
			t.Fatalf("Expand(resources) error = %v", err)
		}
		if !slices.Contains(resourceTools, toolspkg.ToolIDResourcesList) ||
			!slices.Contains(resourceTools, toolspkg.ToolIDResourcesSnapshot) ||
			slices.Contains(resourceTools, toolspkg.ToolID("compozy__resource_list")) {
			t.Fatalf("resources toolset expansion = %#v, want plural desired-state resource tools", resourceTools)
		}

		windowManagerTools, err := catalog.Expand(toolspkg.ToolsetIDWindowManager, universe)
		if err != nil {
			t.Fatalf("Expand(window_manager) error = %v", err)
		}
		if want := windowManagerExpectedToolIDs(); !slices.Equal(windowManagerTools, want) {
			t.Fatalf("window-manager expansion = %#v, want %#v", windowManagerTools, want)
		}

		mcp, err := catalog.Expand(toolspkg.ToolsetIDMCP, universe)
		if err != nil {
			t.Fatalf("Expand(mcp) error = %v", err)
		}
		if want := []toolspkg.ToolID{toolspkg.ToolIDMCPStatus}; !slices.Equal(mcp, want) {
			t.Fatalf("mcp expansion = %#v, want %#v", mcp, want)
		}

		mcpAuth, err := catalog.Expand(toolspkg.ToolsetIDMCPAuth, universe)
		if err != nil {
			t.Fatalf("Expand(mcp_auth) error = %v", err)
		}
		if want := []toolspkg.ToolID{toolspkg.ToolIDMCPAuthStatus}; !slices.Equal(mcpAuth, want) {
			t.Fatalf("mcp auth expansion = %#v, want %#v", mcpAuth, want)
		}

		gatewayTools, err := catalog.Expand(toolspkg.ToolsetIDGateway, universe)
		if err != nil {
			t.Fatalf("Expand(gateway) error = %v", err)
		}
		if want := []toolspkg.ToolID{toolspkg.ToolIDGateway}; !slices.Equal(gatewayTools, want) {
			t.Fatalf("gateway expansion = %#v, want %#v", gatewayTools, want)
		}
	})
}

func descriptorMap(descriptors []toolspkg.Descriptor) map[toolspkg.ToolID]toolspkg.Descriptor {
	values := make(map[toolspkg.ToolID]toolspkg.Descriptor, len(descriptors))
	for _, descriptor := range descriptors {
		values[descriptor.ID] = descriptor
	}
	return values
}

func windowManagerExpectedToolIDs() []toolspkg.ToolID {
	return []toolspkg.ToolID{
		toolspkg.ToolIDDesktopClients,
		toolspkg.ToolIDDesktopCreate,
		toolspkg.ToolIDDesktopDelete,
		toolspkg.ToolIDDesktopList,
		toolspkg.ToolIDDesktopReorder,
		toolspkg.ToolIDDesktopSwitch,
		toolspkg.ToolIDDesktopUpdate,
		toolspkg.ToolIDLayoutApply,
		toolspkg.ToolIDLayoutArrange,
		toolspkg.ToolIDLayoutBalance,
		toolspkg.ToolIDLayoutExport,
		toolspkg.ToolIDLayoutFrameResize,
		toolspkg.ToolIDLayoutGet,
		toolspkg.ToolIDLayoutPreview,
		toolspkg.ToolIDLayoutRedo,
		toolspkg.ToolIDLayoutResize,
		toolspkg.ToolIDLayoutUndo,
		toolspkg.ToolIDLayoutValidate,
		toolspkg.ToolIDWindowActivate,
		toolspkg.ToolIDWindowClose,
		toolspkg.ToolIDWindowFloat,
		toolspkg.ToolIDWindowFocus,
		toolspkg.ToolIDWindowGroup,
		toolspkg.ToolIDWindowList,
		toolspkg.ToolIDWindowMove,
		toolspkg.ToolIDWindowNavigate,
		toolspkg.ToolIDWindowOpen,
		toolspkg.ToolIDWindowPin,
		toolspkg.ToolIDWindowReopen,
		toolspkg.ToolIDWindowReorder,
		toolspkg.ToolIDWindowResize,
		toolspkg.ToolIDWindowSwap,
		toolspkg.ToolIDWindowZoom,
	}
}

func requireDescriptorRisk(
	t *testing.T,
	descriptor toolspkg.Descriptor,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	openWorld bool,
) {
	t.Helper()

	if descriptor.Risk != risk ||
		descriptor.ReadOnly != readOnly ||
		descriptor.Destructive != destructive ||
		descriptor.OpenWorld != openWorld {
		t.Fatalf(
			"%s risk flags = (%s, read=%v, destructive=%v, open_world=%v), want (%s, read=%v, destructive=%v, open_world=%v)",
			descriptor.ID,
			descriptor.Risk,
			descriptor.ReadOnly,
			descriptor.Destructive,
			descriptor.OpenWorld,
			risk,
			readOnly,
			destructive,
			openWorld,
		)
	}
}
