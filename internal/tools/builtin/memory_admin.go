package builtin

import (
	"strconv"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	memoryAdminMemoryKey    = "memory"
	memoryAdminSessionIDKey = "session_id"
)

type memoryAdminDescriptorSpec struct {
	id          toolspkg.ToolID
	nativeName  string
	title       string
	description string
	inputSchema string
	risk        toolspkg.RiskClass
	readOnly    bool
	destructive bool
	hints       []string
}

var memoryAdminTools = func() []toolspkg.Descriptor {
	specs := []memoryAdminDescriptorSpec{
		{
			toolspkg.ToolIDMemoryHealth,
			"memory_health",
			"Memory Health",
			"Read Memory v2 health and derived catalog state.",
			memoryAdminHealthInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory health", "memory status"},
		},
		{
			toolspkg.ToolIDMemoryScopeShow,
			"memory_scope_show",
			"Memory Scope Show",
			"Report effective Memory v2 scope resolution.",
			memoryAdminSelectorInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory scope", "memory roots"},
		},
		{
			toolspkg.ToolIDMemoryAdminHistory,
			"memory_admin_history",
			"Memory Admin History",
			"List Memory v2 operation history without exposing the removed legacy history tool.",
			memoryAdminHistoryInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory history", "memory operations"},
		},
		{
			toolspkg.ToolIDMemoryReindex,
			"memory_reindex",
			"Memory Reindex",
			"Rebuild Memory v2 derived indexes from durable Markdown memory files.",
			memoryAdminReindexInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory reindex", "memory catalog rebuild"},
		},
		{
			toolspkg.ToolIDMemoryPromote,
			"memory_promote",
			"Memory Promote",
			"Promote one Memory v2 entry across scope or tier boundaries through the controller.",
			memoryAdminPromoteInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory promote", "promote memory"},
		},
		{
			toolspkg.ToolIDMemoryReset,
			"memory_reset",
			"Memory Reset",
			"Reset derived Memory v2 indexes when explicitly confirmed.",
			memoryAdminResetInputSchema,
			toolspkg.RiskDestructive,
			false,
			true,
			[]string{"memory reset", "reset derived memory"},
		},
		{
			toolspkg.ToolIDMemoryReload,
			"memory_reload",
			"Memory Reload",
			"Invalidate future Memory v2 snapshots after scope changes.",
			memoryAdminSelectorInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory reload", "memory snapshot reload"},
		},
		{
			toolspkg.ToolIDMemoryDecisionsList,
			"memory_decisions_list",
			"Memory Decisions List",
			"List Memory v2 controller decisions.",
			memoryAdminDecisionListInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory decisions", "memory controller decisions"},
		},
		{
			toolspkg.ToolIDMemoryDecisionsShow,
			"memory_decisions_show",
			"Memory Decisions Show",
			"Read one Memory v2 controller decision.",
			memoryAdminDecisionIDInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory decision", "memory decision detail"},
		},
		{
			toolspkg.ToolIDMemoryDecisionsRevert,
			"memory_decisions_revert",
			"Memory Decisions Revert",
			"Revert one applied Memory v2 controller decision.",
			memoryAdminDecisionRevertInputSchema,
			toolspkg.RiskDestructive,
			false,
			true,
			[]string{"memory decision revert", "revert memory decision"},
		},
		{
			toolspkg.ToolIDMemoryRecallTrace,
			"memory_recall_trace",
			"Memory Recall Trace",
			"Read one materialized Memory v2 recall trace.",
			memoryAdminRecallTraceInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory recall trace", "recall diagnostics"},
		},
		{
			toolspkg.ToolIDMemoryDreamStatus,
			"memory_dream_status",
			"Memory Dream Status",
			"Read live Memory v2 dreaming status.",
			emptyInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory dream status", "dream status"},
		},
		{
			toolspkg.ToolIDMemoryDreamList,
			"memory_dream_list",
			"Memory Dream List",
			"List Memory v2 dreaming run records.",
			memoryAdminDreamListInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory dream list", "dream runs"},
		},
		{
			toolspkg.ToolIDMemoryDreamShow,
			"memory_dream_show",
			"Memory Dream Show",
			"Read one Memory v2 dreaming run record.",
			memoryAdminDreamIDInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory dream show", "dream run detail"},
		},
		{
			toolspkg.ToolIDMemoryDreamTrigger,
			"memory_dream_trigger",
			"Memory Dream Trigger",
			"Trigger Memory v2 dream consolidation.",
			memoryAdminDreamTriggerInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory dream trigger", "trigger dream"},
		},
		{
			toolspkg.ToolIDMemoryDreamRetry,
			"memory_dream_retry",
			"Memory Dream Retry",
			"Retry Memory v2 dream consolidation.",
			memoryAdminDreamRetryInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory dream retry", "retry dream"},
		},
		{
			toolspkg.ToolIDMemoryDailyList,
			"memory_daily_list",
			"Memory Daily List",
			"List Memory v2 daily operation logs.",
			memoryAdminDailyListInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory daily logs", "memory daily list"},
		},
		{
			toolspkg.ToolIDMemoryExtractorStatus,
			"memory_extractor_status",
			"Memory Extractor Status",
			"Read Memory v2 extractor queue status.",
			emptyInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory extractor status", "extractor queue"},
		},
		{
			toolspkg.ToolIDMemoryExtractorFailures,
			"memory_extractor_failures",
			"Memory Extractor Failures",
			"List Memory v2 extractor failures.",
			emptyInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory extractor failures", "memory dlq"},
		},
		{
			toolspkg.ToolIDMemoryExtractorRetry,
			"memory_extractor_retry",
			"Memory Extractor Retry",
			"Retry Memory v2 extractor failure records.",
			memoryAdminExtractorRetryInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory extractor retry", "retry extractor failures"},
		},
		{
			toolspkg.ToolIDMemoryExtractorDrain,
			"memory_extractor_drain",
			"Memory Extractor Drain",
			"Drain the Memory v2 extractor queue.",
			emptyInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory extractor drain", "drain extractor"},
		},
		{
			toolspkg.ToolIDMemoryProviderList,
			"memory_provider_list",
			"Memory Provider List",
			"List Memory v2 providers.",
			memoryAdminWorkspaceInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory providers", "memory provider list"},
		},
		{
			toolspkg.ToolIDMemoryProviderGet,
			"memory_provider_get",
			"Memory Provider Get",
			"Read one Memory v2 provider.",
			memoryAdminProviderNameInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory provider", "memory provider get"},
		},
		{
			toolspkg.ToolIDMemoryProviderSelect,
			"memory_provider_select",
			"Memory Provider Select",
			"Select the active Memory v2 provider.",
			memoryAdminProviderNameInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory provider select", "select memory provider"},
		},
		{
			toolspkg.ToolIDMemoryProviderEnable,
			"memory_provider_enable",
			"Memory Provider Enable",
			"Enable one Memory v2 provider.",
			memoryAdminProviderLifecycleInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory provider enable", "enable memory provider"},
		},
		{
			toolspkg.ToolIDMemoryProviderDisable,
			"memory_provider_disable",
			"Memory Provider Disable",
			"Disable one Memory v2 provider.",
			memoryAdminProviderLifecycleInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory provider disable", "disable memory provider"},
		},
		{
			toolspkg.ToolIDMemorySessionLedger,
			"memory_session_ledger",
			"Memory Session Ledger",
			"Read one materialized Memory v2 session ledger.",
			memoryAdminSessionIDInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{"memory session ledger", "session ledger"},
		},
		{
			toolspkg.ToolIDMemorySessionReplay,
			"memory_session_replay",
			"Memory Session Replay",
			"Replay one materialized Memory v2 session ledger.",
			memoryAdminSessionReplayInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory session replay", "session replay"},
		},
		{
			toolspkg.ToolIDMemorySessionsPrune,
			"memory_sessions_prune",
			"Memory Sessions Prune",
			"Prune Memory v2 session ledgers.",
			memoryAdminSessionsPruneInputSchema,
			toolspkg.RiskDestructive,
			false,
			true,
			[]string{"memory sessions prune", "prune session ledgers"},
		},
		{
			toolspkg.ToolIDMemorySessionsRepair,
			"memory_sessions_repair",
			"Memory Sessions Repair",
			"Repair Memory v2 session ledgers.",
			emptyInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{"memory sessions repair", "repair session ledgers"},
		},
	}
	descriptors := make([]toolspkg.Descriptor, 0, len(specs))
	for _, spec := range specs {
		descriptors = append(descriptors, memoryAdminDescriptor(spec))
	}
	return descriptors
}()

func memoryAdminDescriptors() []toolspkg.Descriptor {
	return memoryAdminTools
}

func memoryAdminDescriptor(spec memoryAdminDescriptorSpec) toolspkg.Descriptor {
	return nativeDescriptor(
		spec.id,
		spec.nativeName,
		spec.title,
		spec.description,
		spec.inputSchema,
		spec.risk,
		spec.readOnly,
		spec.destructive,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMemoryAdmin},
		[]string{memoryAdminMemoryKey, "admin"},
		spec.hints,
	)
}

func memoryAdminSchema(required []string, properties string) string {
	var builder strings.Builder
	builder.WriteString(`{"type":"object",`)
	if len(required) > 0 {
		builder.WriteString(`"required":[`)
		for idx, field := range required {
			if idx > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(strconv.Quote(field))
		}
		builder.WriteString("],")
	}
	builder.WriteString(`"properties":{`)
	builder.WriteString(strings.TrimSpace(properties))
	builder.WriteString(`},"additionalProperties":false}`)
	return builder.String()
}

const memoryAdminSelectorProperties = `"scope":{"type":"string","enum":["","global","workspace","agent"]}` +
	`,"workspace_id":{"type":"string"}` +
	`,"workspace":{"type":"string"}` +
	`,"agent_name":{"type":"string"}` +
	`,"agent_tier":{"type":"string","enum":["","workspace","global"]}`

const memoryAdminSelectorPayloadProperties = `"scope":{"type":"string","enum":["","global","workspace","agent"]}` +
	`,"workspace_id":{"type":"string"}` +
	`,"agent_name":{"type":"string"}` +
	`,"agent_tier":{"type":"string","enum":["","workspace","global"]}`

var (
	memoryAdminSelectorInputSchema   = memoryAdminSchema(nil, memoryAdminSelectorProperties)
	memoryAdminSelectorPayloadSchema = memoryAdminSchema(nil, memoryAdminSelectorPayloadProperties)
	memoryAdminHealthInputSchema     = memoryAdminSchema(
		nil,
		`"workspace_id":{"type":"string"},"workspace":{"type":"string"}`,
	)
	memoryAdminHistoryInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+
			`,"operation":{"type":"string"}`+
			`,"since":{"type":"string"}`+
			`,"limit":{"type":"integer"}`,
	)
	memoryAdminReindexInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+`,"include_system":{"type":"boolean"}`,
	)
	memoryAdminResetInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+`,"derived_only":{"type":"boolean"},"confirm":{"type":"boolean"}`,
	)
	memoryAdminRecallTraceInputSchema = memoryAdminSchema(
		[]string{memoryAdminSessionIDKey, "turn_seq"},
		`"session_id":{"type":"string"},"turn_seq":{"type":"integer"}`,
	)
	memoryAdminDreamListInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+`,"limit":{"type":"integer"}`,
	)
	memoryAdminDreamIDInputSchema      = memoryAdminSchema([]string{"dream_id"}, `"dream_id":{"type":"string"}`)
	memoryAdminDreamTriggerInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+`,"force":{"type":"boolean"}`,
	)
	memoryAdminDreamRetryInputSchema = memoryAdminSchema(
		nil,
		`"failure_id":{"type":"string"},"dream_id":{"type":"string"},"force":{"type":"boolean"}`,
	)
	memoryAdminDailyListInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+`,"date":{"type":"string"},"limit":{"type":"integer"}`,
	)
	memoryAdminExtractorRetryInputSchema = memoryAdminSchema(
		nil,
		`"failure_id":{"type":"string"},"session_id":{"type":"string"}`,
	)
	memoryAdminWorkspaceInputSchema    = memoryAdminSchema(nil, `"workspace_id":{"type":"string"}`)
	memoryAdminProviderNameInputSchema = memoryAdminSchema(
		[]string{"name"},
		`"name":{"type":"string"},"workspace_id":{"type":"string"}`,
	)
	memoryAdminProviderLifecycleInputSchema = memoryAdminSchema(
		[]string{"name"},
		`"name":{"type":"string"},"workspace_id":{"type":"string"},"reason":{"type":"string"}`,
	)
	memoryAdminSessionIDInputSchema = memoryAdminSchema(
		[]string{descriptorWorkspaceIDKey, memoryAdminSessionIDKey},
		`"workspace_id":{"type":"string"},"session_id":{"type":"string"}`,
	)
	memoryAdminSessionReplayInputSchema = memoryAdminSchema(
		[]string{descriptorWorkspaceIDKey, memoryAdminSessionIDKey},
		`"workspace_id":{"type":"string"},"session_id":{"type":"string"},"include_tool_events":{"type":"boolean"},"include_memory":{"type":"boolean"}`,
	)
	memoryAdminSessionsPruneInputSchema = memoryAdminSchema(
		[]string{"older_than_hours"},
		`"older_than_hours":{"type":"integer"},"dry_run":{"type":"boolean"}`,
	)
	memoryAdminPromoteInputSchema = memoryAdminSchema(
		[]string{"filename", "from", "to"},
		`"filename":{"type":"string"}`+
			`,"from":`+memoryAdminSelectorPayloadSchema+
			`,"to":`+memoryAdminSelectorPayloadSchema+
			`,"idempotency_key":{"type":"string"}`+
			`,"dry_run":{"type":"boolean"}`,
	)
)
