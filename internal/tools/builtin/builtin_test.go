package builtin

import (
	"bytes"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/network/participation"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/santhosh-tekuri/jsonschema/v6"
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

		want := []toolspkg.ToolID{
			toolspkg.ToolIDToolList,
			toolspkg.ToolIDToolSearch,
			toolspkg.ToolIDToolInfo,
			toolspkg.ToolIDToolArtifactRead,
			toolspkg.ToolIDToolApprovalsSet,
			toolspkg.ToolIDToolApprovalsList,
			toolspkg.ToolIDToolApprovalsRevoke,
			toolspkg.ToolIDClarify,
			toolspkg.ToolIDSkillList,
			toolspkg.ToolIDSkillSearch,
			toolspkg.ToolIDSkillView,
			toolspkg.ToolIDNetworkStatus,
			toolspkg.ToolIDNetworkUsage,
			toolspkg.ToolIDNetworkChannels,
			toolspkg.ToolIDNetworkInbox,
			toolspkg.ToolIDNetworkPeers,
			toolspkg.ToolIDNetworkSend,
			toolspkg.ToolIDNetworkChannelCreate,
			toolspkg.ToolIDNetworkChannelUpdate,
			toolspkg.ToolIDNetworkSubscriptions,
			toolspkg.ToolIDNetworkSubscribe,
			toolspkg.ToolIDNetworkMute,
			toolspkg.ToolIDNetworkUnmute,
			toolspkg.ToolIDNetworkThreads,
			toolspkg.ToolIDNetworkThreadMessages,
			toolspkg.ToolIDNetworkDirects,
			toolspkg.ToolIDNetworkDirectResolve,
			toolspkg.ToolIDNetworkDirectMessages,
			toolspkg.ToolIDNetworkWork,
			toolspkg.ToolIDSessionList,
			toolspkg.ToolIDSessionStatus,
			toolspkg.ToolIDSessionHistory,
			toolspkg.ToolIDSessionEvents,
			toolspkg.ToolIDSessionDescribe,
			toolspkg.ToolIDSessionHealth,
			toolspkg.ToolIDAgentHeartbeatStatus,
			toolspkg.ToolIDAgentHeartbeatWake,
			toolspkg.ToolIDWorkspaceList,
			toolspkg.ToolIDWorkspaceInfo,
			toolspkg.ToolIDWorkspaceDescribe,
			toolspkg.ToolIDAgentCreate,
			toolspkg.ToolIDProviderModelsList,
			toolspkg.ToolIDProviderModelsRefresh,
			toolspkg.ToolIDProviderModelsStatus,
			toolspkg.ToolIDProviderModelsCurate,
			toolspkg.ToolIDMemoryList,
			toolspkg.ToolIDMemoryShow,
			toolspkg.ToolIDMemorySearch,
			toolspkg.ToolIDMemoryPropose,
			toolspkg.ToolIDMemoryNote,
			toolspkg.ToolIDMemoryHealth,
			toolspkg.ToolIDMemoryScopeShow,
			toolspkg.ToolIDMemoryAdminHistory,
			toolspkg.ToolIDMemoryReindex,
			toolspkg.ToolIDMemoryPromote,
			toolspkg.ToolIDMemoryReset,
			toolspkg.ToolIDMemoryReload,
			toolspkg.ToolIDMemoryDecisionsList,
			toolspkg.ToolIDMemoryDecisionsShow,
			toolspkg.ToolIDMemoryDecisionsRevert,
			toolspkg.ToolIDMemoryRecallTrace,
			toolspkg.ToolIDMemoryDreamStatus,
			toolspkg.ToolIDMemoryDreamList,
			toolspkg.ToolIDMemoryDreamShow,
			toolspkg.ToolIDMemoryDreamTrigger,
			toolspkg.ToolIDMemoryDreamRetry,
			toolspkg.ToolIDMemoryDailyList,
			toolspkg.ToolIDMemoryExtractorStatus,
			toolspkg.ToolIDMemoryExtractorFailures,
			toolspkg.ToolIDMemoryExtractorRetry,
			toolspkg.ToolIDMemoryExtractorDrain,
			toolspkg.ToolIDMemoryProviderList,
			toolspkg.ToolIDMemoryProviderGet,
			toolspkg.ToolIDMemoryProviderSelect,
			toolspkg.ToolIDMemoryProviderEnable,
			toolspkg.ToolIDMemoryProviderDisable,
			toolspkg.ToolIDMemorySessionLedger,
			toolspkg.ToolIDMemorySessionReplay,
			toolspkg.ToolIDMemorySessionsPrune,
			toolspkg.ToolIDMemorySessionsRepair,
			toolspkg.ToolIDListLogs,
			toolspkg.ToolIDObserveMetrics,
			toolspkg.ToolIDObserveSearch,
			toolspkg.ToolIDBridgesList,
			toolspkg.ToolIDBridgesStatus,
			toolspkg.ToolIDTaskList,
			toolspkg.ToolIDTaskRead,
			toolspkg.ToolIDTaskCreate,
			toolspkg.ToolIDTaskChildCreate,
			toolspkg.ToolIDTaskUpdate,
			toolspkg.ToolIDTaskCancel,
			toolspkg.ToolIDTaskBlock,
			toolspkg.ToolIDTaskUnblock,
			toolspkg.ToolIDTaskBlocks,
			toolspkg.ToolIDTaskRecover,
			toolspkg.ToolIDTaskRunList,
			toolspkg.ToolIDTaskRunReviewRequest,
			toolspkg.ToolIDTaskRunReviewList,
			toolspkg.ToolIDTaskRunReviewShow,
			toolspkg.ToolIDTaskExecutionProfileGet,
			toolspkg.ToolIDTaskExecutionProfileSet,
			toolspkg.ToolIDTaskExecutionProfileDelete,
			toolspkg.ToolIDTaskNotificationSubscribe,
			toolspkg.ToolIDTaskNotificationList,
			toolspkg.ToolIDTaskNotificationShow,
			toolspkg.ToolIDTaskNotificationDelete,
			toolspkg.ToolIDTaskPromoteFromThread,
			toolspkg.ToolIDTaskFanOutRuns,
			toolspkg.ToolIDTaskRunClaimNext,
			toolspkg.ToolIDTaskRunHeartbeat,
			toolspkg.ToolIDTaskRunComplete,
			toolspkg.ToolIDTaskRunFail,
			toolspkg.ToolIDTaskRunRelease,
			toolspkg.ToolIDTaskRunReviewSubmit,
			toolspkg.ToolIDConfigShow,
			toolspkg.ToolIDConfigList,
			toolspkg.ToolIDConfigGet,
			toolspkg.ToolIDConfigSet,
			toolspkg.ToolIDConfigUnset,
			toolspkg.ToolIDConfigDiff,
			toolspkg.ToolIDConfigPath,
			toolspkg.ToolIDHooksList,
			toolspkg.ToolIDHooksInfo,
			toolspkg.ToolIDHooksEvents,
			toolspkg.ToolIDHooksRuns,
			toolspkg.ToolIDHooksCreate,
			toolspkg.ToolIDHooksUpdate,
			toolspkg.ToolIDHooksDelete,
			toolspkg.ToolIDHooksEnable,
			toolspkg.ToolIDHooksDisable,
			toolspkg.ToolIDAutomationJobsList,
			toolspkg.ToolIDAutomationJobsGet,
			toolspkg.ToolIDAutomationJobsCreate,
			toolspkg.ToolIDAutomationJobsUpdate,
			toolspkg.ToolIDAutomationJobsDelete,
			toolspkg.ToolIDAutomationJobsEnable,
			toolspkg.ToolIDAutomationJobsDisable,
			toolspkg.ToolIDAutomationJobsTrigger,
			toolspkg.ToolIDAutomationJobsHistory,
			toolspkg.ToolIDAutomationSuggestionsList,
			toolspkg.ToolIDAutomationSuggestionsAccept,
			toolspkg.ToolIDAutomationSuggestionsDismiss,
			toolspkg.ToolIDAutomationTriggersList,
			toolspkg.ToolIDAutomationTriggersGet,
			toolspkg.ToolIDAutomationTriggersCreate,
			toolspkg.ToolIDAutomationTriggersUpdate,
			toolspkg.ToolIDAutomationTriggersDelete,
			toolspkg.ToolIDAutomationTriggersEnable,
			toolspkg.ToolIDAutomationTriggersDisable,
			toolspkg.ToolIDAutomationTriggersHistory,
			toolspkg.ToolIDAutomationRunsList,
			toolspkg.ToolIDAutomationRunsGet,
			toolspkg.ToolIDGoalGet,
			toolspkg.ToolIDGoalReport,
			toolspkg.ToolIDLoopList,
			toolspkg.ToolIDLoopInspect,
			toolspkg.ToolIDLoopValidate,
			toolspkg.ToolIDLoopCreate,
			toolspkg.ToolIDLoopRun,
			toolspkg.ToolIDLoopStatus,
			toolspkg.ToolIDLoopRuns,
			toolspkg.ToolIDLoopTurns,
			toolspkg.ToolIDLoopStop,
			toolspkg.ToolIDLoopPause,
			toolspkg.ToolIDLoopResume,
			toolspkg.ToolIDLoopConfigure,
			toolspkg.ToolIDLoopApprove,
			toolspkg.ToolIDLoopDelete,
			toolspkg.ToolIDMarketplaceSearch,
			toolspkg.ToolIDExtensionsList,
			toolspkg.ToolIDExtensionsInfo,
			toolspkg.ToolIDExtensionsInstall,
			toolspkg.ToolIDExtensionsUpdate,
			toolspkg.ToolIDExtensionsRemove,
			toolspkg.ToolIDExtensionsEnable,
			toolspkg.ToolIDExtensionsDisable,
			toolspkg.ToolIDBundlesList,
			toolspkg.ToolIDBundlesInfo,
			toolspkg.ToolIDBundlesActivate,
			toolspkg.ToolIDBundlesDeactivate,
			toolspkg.ToolIDBundlesStatus,
			toolspkg.ToolIDResourcesList,
			toolspkg.ToolIDResourcesInfo,
			toolspkg.ToolIDResourcesSnapshot,
		}
		want = append(want, windowManagerExpectedToolIDs()...)
		want = append(
			want,
			toolspkg.ToolIDMCPStatus,
			toolspkg.ToolIDMCPAuthStatus,
		)
		if gotLen, wantLen := len(got), len(want); gotLen != wantLen {
			t.Fatalf("len(NativeDescriptors()) = %d, want %d", gotLen, wantLen)
		}
		for _, id := range want {
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
			"agh__skill_install",
			"agh__skill_update",
			"agh__skill_remove",
			"agh__task_claim",
			"agh__task_release",
			"agh__task_complete",
			"agh__task_fail",
			"agh__task_run_start",
			"agh__task_run_cancel",
			"agh__mcp_auth_login",
			"agh__mcp_auth_logout",
			"agh__memory_read",
			"agh__memory_history",
			"agh__memory_write",
			"agh__memory_edit",
			"agh__memory_delete",
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
			for _, legacy := range []string{"network_channel", "coordination_channel_id"} {
				if _, ok := schema.Properties[legacy]; ok {
					t.Fatalf("%s input schema exposes removed %s", id, legacy)
				}
			}
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

	t.Run("Should classify read mutating open world and destructive risk flags", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())

		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDToolList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDClarify], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDToolApprovalsSet],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDToolApprovalsList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDToolApprovalsRevoke],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSkillView], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkStatus], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkUsage], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkChannels], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkInbox], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkPeers], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkThreads], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkThreadMessages],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkDirects], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkDirectMessages],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkWork], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkChannelCreate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkChannelUpdate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkSubscriptions],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkSubscribe],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkMute],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkUnmute],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSessionList], toolspkg.RiskRead, true, false, false)
		sessionListDescriptor := descriptors[toolspkg.ToolIDSessionList]
		if !strings.Contains(string(sessionListDescriptor.InputSchema), `"cursor"`) ||
			!strings.Contains(string(sessionListDescriptor.InputSchema), `"include_health"`) ||
			!strings.Contains(string(sessionListDescriptor.InputSchema), `"type"`) ||
			!strings.Contains(string(sessionListDescriptor.InputSchema), `"last_activity"`) ||
			!strings.Contains(string(sessionListDescriptor.OutputSchema), `"page"`) ||
			!strings.Contains(string(sessionListDescriptor.OutputSchema), `"next_cursor"`) {
			t.Fatalf(
				"session list schemas = input %s output %s, want paged filters and continuation",
				sessionListDescriptor.InputSchema,
				sessionListDescriptor.OutputSchema,
			)
		}
		if strings.Contains(string(sessionListDescriptor.InputSchema), `"dream"`) {
			t.Fatalf("session list input schema exposes internal dream type: %s", sessionListDescriptor.InputSchema)
		}
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSessionStatus], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSessionHistory], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSessionEvents], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSessionDescribe], toolspkg.RiskRead, true, false, false)
		bridgeListDescriptor := descriptors[toolspkg.ToolIDBridgesList]
		if !strings.Contains(string(bridgeListDescriptor.InputSchema), `"workspace_id"`) ||
			!strings.Contains(string(bridgeListDescriptor.InputSchema), `"cursor"`) ||
			!strings.Contains(string(bridgeListDescriptor.OutputSchema), `"facets"`) ||
			!strings.Contains(string(bridgeListDescriptor.OutputSchema), `"next_cursor"`) {
			t.Fatalf(
				"bridge list schemas = input %s output %s, want workspace-safe paged filters",
				bridgeListDescriptor.InputSchema,
				bridgeListDescriptor.OutputSchema,
			)
		}
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDSessionHealth], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAgentHeartbeatStatus],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAgentHeartbeatWake],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAgentCreate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDWorkspaceList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDWorkspaceInfo], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDWorkspaceDescribe], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDProviderModelsList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDProviderModelsRefresh],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDProviderModelsStatus],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDProviderModelsCurate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMemoryList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMemoryShow], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMemorySearch], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMemoryPropose], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMemoryNote], toolspkg.RiskMutating, false, false, false)
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDMemoryHealth,
			toolspkg.ToolIDMemoryScopeShow,
			toolspkg.ToolIDMemoryAdminHistory,
			toolspkg.ToolIDMemoryDecisionsList,
			toolspkg.ToolIDMemoryDecisionsShow,
			toolspkg.ToolIDMemoryRecallTrace,
			toolspkg.ToolIDMemoryDreamStatus,
			toolspkg.ToolIDMemoryDreamList,
			toolspkg.ToolIDMemoryDreamShow,
			toolspkg.ToolIDMemoryDailyList,
			toolspkg.ToolIDMemoryExtractorStatus,
			toolspkg.ToolIDMemoryExtractorFailures,
			toolspkg.ToolIDMemoryProviderList,
			toolspkg.ToolIDMemoryProviderGet,
			toolspkg.ToolIDMemorySessionLedger,
		} {
			requireDescriptorRisk(t, descriptors[id], toolspkg.RiskRead, true, false, false)
		}
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDMemoryReindex,
			toolspkg.ToolIDMemoryPromote,
			toolspkg.ToolIDMemoryReload,
			toolspkg.ToolIDMemoryDreamTrigger,
			toolspkg.ToolIDMemoryDreamRetry,
			toolspkg.ToolIDMemoryExtractorRetry,
			toolspkg.ToolIDMemoryExtractorDrain,
			toolspkg.ToolIDMemoryProviderSelect,
			toolspkg.ToolIDMemoryProviderEnable,
			toolspkg.ToolIDMemoryProviderDisable,
			toolspkg.ToolIDMemorySessionReplay,
			toolspkg.ToolIDMemorySessionsRepair,
		} {
			requireDescriptorRisk(t, descriptors[id], toolspkg.RiskMutating, false, false, false)
		}
		for _, id := range []toolspkg.ToolID{
			toolspkg.ToolIDMemoryReset,
			toolspkg.ToolIDMemoryDecisionsRevert,
			toolspkg.ToolIDMemorySessionsPrune,
		} {
			requireDescriptorRisk(t, descriptors[id], toolspkg.RiskDestructive, false, true, false)
		}
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDListLogs], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDToolArtifactRead], toolspkg.RiskRead, true, false, false)
		if got, want := descriptors[toolspkg.ToolIDToolArtifactRead].MaxResultBytes,
			toolArtifactReadMaxResultBytes; got != want {
			t.Fatalf("tool artifact read max result bytes = %d, want %d", got, want)
		}
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDObserveMetrics], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDObserveSearch], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDBridgesList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDBridgesStatus], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskRead], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskRunList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunReviewRequest],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunReviewList],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunReviewShow],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDNetworkSend], toolspkg.RiskOpenWorld, false, false, true)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDNetworkDirectResolve],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskCreate], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskChildCreate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskUpdate], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskCancel], toolspkg.RiskDestructive, false, true, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskBlock], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskUnblock], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskBlocks], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskRecover], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskExecutionProfileGet],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskExecutionProfileSet],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskExecutionProfileDelete],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskNotificationSubscribe],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskNotificationList],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskNotificationShow],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskNotificationDelete],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskPromoteFromThread],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskFanOutRuns],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunClaimNext],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunHeartbeat],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunComplete],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskRunFail], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDTaskRunRelease], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDTaskRunReviewSubmit],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		if got := descriptors[toolspkg.ToolIDTaskRunReviewSubmit].Backend.NativeName; got != "submit_run_review" {
			t.Fatalf("submit review native name = %q, want submit_run_review", got)
		}
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDConfigShow], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDConfigSet], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDConfigUnset], toolspkg.RiskDestructive, false, true, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDHooksList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDHooksCreate], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDHooksDelete], toolspkg.RiskDestructive, false, true, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDHooksDisable], toolspkg.RiskMutating, false, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDAutomationJobsList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAutomationJobsCreate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAutomationJobsDelete],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAutomationJobsTrigger],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDAutomationRunsGet], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAutomationTriggersCreate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDAutomationTriggersDelete],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMarketplaceSearch], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDExtensionsList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDExtensionsInfo], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDExtensionsInstall],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDExtensionsUpdate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDExtensionsRemove],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDExtensionsEnable],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDExtensionsDisable],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDBundlesList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDBundlesInfo], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDBundlesActivate],
			toolspkg.RiskMutating,
			false,
			false,
			false,
		)
		if !strings.Contains(
			string(descriptors[toolspkg.ToolIDBundlesActivate].InputSchema),
			`"confirm_network_requirement":{"type":"boolean"}`,
		) {
			t.Fatalf(
				"bundles_activate schema = %s, want operator confirmation input",
				descriptors[toolspkg.ToolIDBundlesActivate].InputSchema,
			)
		}
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDBundlesDeactivate],
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDBundlesStatus], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDResourcesList], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDResourcesInfo], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDResourcesSnapshot],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
		requireDescriptorRisk(t, descriptors[toolspkg.ToolIDMCPStatus], toolspkg.RiskRead, true, false, false)
		requireDescriptorRisk(
			t,
			descriptors[toolspkg.ToolIDMCPAuthStatus],
			toolspkg.RiskRead,
			true,
			false,
			false,
		)
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

	t.Run("Should publish native schema digests and capability roster", func(t *testing.T) {
		t.Parallel()

		descriptors := descriptorMap(NativeDescriptors())
		cases := []struct {
			id         toolspkg.ToolID
			capability string
		}{
			{id: toolspkg.ToolIDBundlesList, capability: "bundles.read"},
			{id: toolspkg.ToolIDBundlesActivate, capability: "bundles.write"},
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
			toolspkg.ToolIDWindowFocus:    {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowMove:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowSwap:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowFloat:    {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDWindowZoom:     {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutGet:      {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDLayoutPreview:  {toolspkg.RiskRead, true, false, windowManagerReadCapability},
			toolspkg.ToolIDLayoutArrange:  {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
			toolspkg.ToolIDLayoutResize:   {toolspkg.RiskMutating, false, false, windowManagerWriteCapability},
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
		first[0].ID = "agh__mutated"
		first[0].InputSchema[0] = '['

		second := NativeDescriptors()
		if second[0].ID == "agh__mutated" {
			t.Fatal("NativeDescriptors() reused descriptor slice")
		}
		if len(second[0].InputSchema) == 0 || second[0].InputSchema[0] == '[' {
			t.Fatal("NativeDescriptors() reused input schema bytes")
		}
	})
}

type nativeObjectSchema struct {
	Type                 string                     `json:"type"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Enum                 []string                   `json:"enum"`
	OneOf                []json.RawMessage          `json:"oneOf"`
	Pattern              string                     `json:"pattern"`
	Minimum              *float64                   `json:"minimum"`
	MinLength            int                        `json:"minLength"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
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
			name: "Should reject group relocation with a target",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true,"target_window_id":"window-b"}`,
		},
		{
			name: "Should reject group relocation with placement",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true,"placement":"right"}`,
		},
		{
			name: "Should reject group relocation with a floating rectangle",
			payload: `{"expected_revision":1,"window_id":"window-a",` +
				`"destination_desktop_id":"desktop-b","move_group":true,` +
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

	var preview resultSchema
	if err := json.Unmarshal(previewRaw, &preview); err != nil {
		t.Fatalf("window manager preview output schema unmarshal error = %v", err)
	}
	assertSchemaFields(t, "window manager preview output", preview, []string{
		"changed", "changes", "client", "command_id", "diagnostics", "revision", "snapshot", "workspace_id",
	}, []string{"changed", "changes", "command_id", "diagnostics", "revision", "snapshot", "workspace_id"})
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
			!slices.Contains(sessions, toolspkg.ToolIDSessionDescribe) ||
			!slices.Contains(sessions, toolspkg.ToolIDSessionHealth) ||
			slices.Contains(sessions, toolspkg.ToolID("agh__session_stop")) {
			t.Fatalf("sessions toolset expansion = %#v, want read-only session tools", sessions)
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
			slices.Contains(workspace, toolspkg.ToolID("agh__workspace_remove")) {
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
			slices.Contains(memory, toolspkg.ToolID("agh__memory_read")) ||
			slices.Contains(memory, toolspkg.ToolID("agh__memory_history")) ||
			slices.Contains(memory, toolspkg.ToolID("agh__memory_write")) {
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
			slices.Contains(observe, toolspkg.ToolID("agh__observe_delete")) {
			t.Fatalf("observe toolset expansion = %#v, want read-only observe tools", observe)
		}

		bridges, err := catalog.Expand(toolspkg.ToolsetIDBridges, universe)
		if err != nil {
			t.Fatalf("Expand(bridges) error = %v", err)
		}
		if !slices.Contains(bridges, toolspkg.ToolIDBridgesList) ||
			!slices.Contains(bridges, toolspkg.ToolIDBridgesStatus) ||
			slices.Contains(bridges, toolspkg.ToolID("agh__bridges_update")) {
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
			slices.Contains(automation, toolspkg.ToolID("agh__automation_webhook_secret_set")) {
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
			slices.Contains(loops, toolspkg.ToolID("agh__loop_edit")) {
			t.Fatalf("loops toolset expansion = %#v, want bounded loop tools without edit", loops)
		}

		extensions, err := catalog.Expand(toolspkg.ToolsetIDExtensions, universe)
		if err != nil {
			t.Fatalf("Expand(extensions) error = %v", err)
		}
		if !slices.Contains(extensions, toolspkg.ToolIDExtensionsInstall) ||
			!slices.Contains(extensions, toolspkg.ToolIDExtensionsRemove) ||
			slices.Contains(extensions, toolspkg.ToolID("agh__extensions_trust_root_set")) {
			t.Fatalf("extensions toolset expansion = %#v, want bounded extension lifecycle tools", extensions)
		}

		marketplace, err := catalog.Expand(toolspkg.ToolsetIDMarketplace, universe)
		if err != nil {
			t.Fatalf("Expand(marketplace) error = %v", err)
		}
		if !slices.Equal(marketplace, []toolspkg.ToolID{toolspkg.ToolIDMarketplaceSearch}) {
			t.Fatalf("marketplace toolset expansion = %#v, want marketplace search", marketplace)
		}

		bundles, err := catalog.Expand(toolspkg.ToolsetIDBundles, universe)
		if err != nil {
			t.Fatalf("Expand(bundles) error = %v", err)
		}
		if !slices.Contains(bundles, toolspkg.ToolIDBundlesActivate) ||
			!slices.Contains(bundles, toolspkg.ToolIDBundlesStatus) {
			t.Fatalf("bundles toolset expansion = %#v, want bundle lifecycle tools", bundles)
		}

		resourceTools, err := catalog.Expand(toolspkg.ToolsetIDResources, universe)
		if err != nil {
			t.Fatalf("Expand(resources) error = %v", err)
		}
		if !slices.Contains(resourceTools, toolspkg.ToolIDResourcesList) ||
			!slices.Contains(resourceTools, toolspkg.ToolIDResourcesSnapshot) ||
			slices.Contains(resourceTools, toolspkg.ToolID("agh__resource_list")) {
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
		toolspkg.ToolIDLayoutGet,
		toolspkg.ToolIDLayoutPreview,
		toolspkg.ToolIDLayoutRedo,
		toolspkg.ToolIDLayoutResize,
		toolspkg.ToolIDLayoutUndo,
		toolspkg.ToolIDLayoutValidate,
		toolspkg.ToolIDWindowClose,
		toolspkg.ToolIDWindowFloat,
		toolspkg.ToolIDWindowFocus,
		toolspkg.ToolIDWindowList,
		toolspkg.ToolIDWindowMove,
		toolspkg.ToolIDWindowNavigate,
		toolspkg.ToolIDWindowOpen,
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
