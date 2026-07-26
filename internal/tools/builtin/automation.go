package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	automationJobsKey = "jobs"
)

const (
	automationAutomationKey  = "automation"
	automationHistoryKey     = "history"
	automationMutationKey    = "mutation"
	automationRunsKey        = "runs"
	automationSuggestionsKey = "suggestions"
	automationTriggersKey    = "triggers"
)

var automationTools = []toolspkg.Descriptor{
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsList,
		"automation_jobs_list",
		"Automation Jobs List",
		"List automation jobs through the live automation manager.",
		automationJobsListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationJobsKey, descriptorKeywordCatalog},
		[]string{"automation jobs", "scheduled jobs"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsGet,
		"automation_jobs_get",
		"Automation Jobs Get",
		"Read one automation job through the live automation manager.",
		automationJobIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationJobsKey},
		[]string{"automation job details", "scheduled job"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsCreate,
		"automation_jobs_create",
		"Automation Jobs Create",
		"Create one dynamic automation job through the validated automation manager writer.",
		automationJobCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationJobsKey, automationMutationKey},
		[]string{"create automation job", "add scheduled job"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsUpdate,
		"automation_jobs_update",
		"Automation Jobs Update",
		"Update one automation job through the validated automation manager writer.",
		automationJobUpdateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationJobsKey, automationMutationKey},
		[]string{"update automation job", "edit scheduled job"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsDelete,
		"automation_jobs_delete",
		"Automation Jobs Delete",
		"Delete one dynamic automation job through the automation manager.",
		automationJobIDInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{automationAutomationKey, automationJobsKey, automationMutationKey},
		[]string{"delete automation job", "remove scheduled job"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsEnable,
		"automation_jobs_enable",
		"Automation Jobs Enable",
		"Enable one automation job through the automation manager.",
		automationJobIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationJobsKey, automationMutationKey},
		[]string{"enable automation job", "activate scheduled job"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsDisable,
		"automation_jobs_disable",
		"Automation Jobs Disable",
		"Disable one automation job through the automation manager.",
		automationJobIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationJobsKey, automationMutationKey},
		[]string{"disable automation job", "pause scheduled job"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsTrigger,
		"automation_jobs_trigger",
		"Automation Jobs Trigger",
		"Trigger one immediate manual automation job run through the automation manager.",
		automationJobIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationJobsKey, automationRunsKey, automationMutationKey},
		[]string{"trigger automation job", "manual automation run"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationJobsHistory,
		"automation_jobs_history",
		"Automation Jobs History",
		"List persisted run history for one automation job.",
		automationJobHistoryInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationJobsKey, automationRunsKey, automationHistoryKey},
		[]string{"automation job history", "job runs"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersList,
		"automation_triggers_list",
		"Automation Triggers List",
		"List automation triggers through the live automation manager.",
		automationTriggersListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationTriggersKey, "catalog"},
		[]string{"automation triggers", "event triggers"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersGet,
		"automation_triggers_get",
		"Automation Triggers Get",
		"Read one automation trigger through the live automation manager.",
		automationTriggerIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationTriggersKey},
		[]string{"automation trigger details", "event trigger"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersCreate,
		"automation_triggers_create",
		"Automation Triggers Create",
		"Create one dynamic automation trigger through the validated automation manager writer.",
		automationTriggerCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationTriggersKey, automationMutationKey},
		[]string{"create automation trigger", "add event trigger"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersUpdate,
		"automation_triggers_update",
		"Automation Triggers Update",
		"Update one automation trigger through the validated automation manager writer.",
		automationTriggerUpdateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationTriggersKey, automationMutationKey},
		[]string{"update automation trigger", "edit event trigger"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersDelete,
		"automation_triggers_delete",
		"Automation Triggers Delete",
		"Delete one dynamic automation trigger through the automation manager.",
		automationTriggerIDInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{automationAutomationKey, automationTriggersKey, automationMutationKey},
		[]string{"delete automation trigger", "remove event trigger"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersEnable,
		"automation_triggers_enable",
		"Automation Triggers Enable",
		"Enable one automation trigger through the automation manager.",
		automationTriggerIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationTriggersKey, automationMutationKey},
		[]string{"enable automation trigger", "activate event trigger"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersDisable,
		"automation_triggers_disable",
		"Automation Triggers Disable",
		"Disable one automation trigger through the automation manager.",
		automationTriggerIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{automationAutomationKey, automationTriggersKey, automationMutationKey},
		[]string{"disable automation trigger", "pause event trigger"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationTriggersHistory,
		"automation_triggers_history",
		"Automation Triggers History",
		"List persisted run history for one automation trigger.",
		automationTriggerHistoryInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationTriggersKey, automationRunsKey, automationHistoryKey},
		[]string{"automation trigger history", "trigger runs"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationRunsList,
		"automation_runs_list",
		"Automation Runs List",
		"List persisted automation run records.",
		automationRunsListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationRunsKey, automationHistoryKey},
		[]string{"automation runs", "run history"},
	),
	nativeAutomationDescriptor(
		toolspkg.ToolIDAutomationRunsGet,
		"automation_runs_get",
		"Automation Runs Get",
		"Read one persisted automation run record.",
		automationRunIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{automationAutomationKey, automationRunsKey},
		[]string{"automation run details", "run record"},
	),
}

func automationDescriptors() []toolspkg.Descriptor {
	return append(automationTools, automationSuggestionDescriptors()...)
}

func nativeAutomationDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	return nativeDescriptor(
		id,
		nativeName,
		title,
		description,
		inputSchema,
		risk,
		readOnly,
		destructive,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutomation},
		tags,
		searchHints,
	)
}

const automationJobsListInputSchema = `{
	"type":"object",
	"properties":{
		"scope":{"type":"string"},
		"workspace_id":{"type":"string"},
		"source":{"type":"string","enum":["config","package","dynamic"]},
		"enabled":{"type":"boolean"},
		"loop":{"type":"string"},
		"q":{"type":"string"},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":0,"maximum":200}
	},
	"additionalProperties":false
}`

const automationJobIDInputSchema = `{
	"type":"object",
	"required":["job_id"],
	"properties":{
		"job_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const automationJobHistoryInputSchema = `{
	"type":"object",
	"required":["job_id"],
	"properties":` + automationJobHistoryProperties + `,
	"additionalProperties":false
}`

const automationJobCreateInputSchema = `{
	"type":"object",
	"required":["scope","name","agent_name","prompt","schedule"],
	"properties":` + automationJobProperties + `,
	"additionalProperties":false
}`

const automationJobUpdateInputSchema = `{
	"type":"object",
	"required":["job_id"],
	"properties":` + automationJobPatchProperties + `,
	"additionalProperties":false
}`

const automationTriggersListInputSchema = `{
	"type":"object",
	"properties":{
		"scope":{"type":"string"},
		"workspace_id":{"type":"string"},
		"event":{"type":"string"},
		"source":{"type":"string","enum":["config","package","dynamic"]},
		"enabled":{"type":"boolean"},
		"loop":{"type":"string"},
		"q":{"type":"string"},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":0,"maximum":200}
	},
	"additionalProperties":false
}`

const automationTriggerIDInputSchema = `{
	"type":"object",
	"required":["trigger_id"],
	"properties":{
		"trigger_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const automationTriggerHistoryInputSchema = `{
	"type":"object",
	"required":["trigger_id"],
	"properties":` + automationTriggerHistoryProperties + `,
	"additionalProperties":false
}`

const automationTriggerCreateInputSchema = `{
	"type":"object",
	"required":["scope","name","agent_name","prompt","event"],
	"properties":` + automationTriggerProperties + `,
	"additionalProperties":false
}`

const automationTriggerUpdateInputSchema = `{
	"type":"object",
	"required":["trigger_id"],
	"properties":` + automationTriggerPatchProperties + `,
	"additionalProperties":false
}`

const automationRunsListInputSchema = `{
	"type":"object",
	"properties":` + automationRunQueryProperties + `,
	"additionalProperties":false
}`

const automationRunIDInputSchema = `{
	"type":"object",
	"required":["run_id"],
	"properties":{
		"run_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const automationJobProperties = `{
	"scope":{"type":"string"},
	"name":{"type":"string"},
	"agent_name":{"type":"string"},
	"workspace_id":{"type":"string"},
	"prompt":{"type":"string"},
	"schedule":` + automationScheduleInputSchema + `,
	"task":{"type":"object"},
	"enabled":{"type":"boolean"},
	"retry":{"type":"object"},
	"fire_limit":{"type":"object"}
}`

const automationJobPatchProperties = `{
	"job_id":{"type":"string"},
	"name":{"type":"string"},
	"prompt":{"type":"string"},
	"schedule":` + automationScheduleInputSchema + `,
	"task":{"type":"object"},
	"enabled":{"type":"boolean"},
	"retry":{"type":"object"},
	"fire_limit":{"type":"object"}
}`

const automationTriggerProperties = `{
	"scope":{"type":"string"},
	"name":{"type":"string"},
	"agent_name":{"type":"string"},
	"workspace_id":{"type":"string"},
	"prompt":{"type":"string"},
	"event":{"type":"string"},
	"filter":{"type":"object"},
	"enabled":{"type":"boolean"},
	"retry":{"type":"object"},
	"fire_limit":{"type":"object"},
	"webhook_id":{"type":"string"},
	"endpoint_slug":{"type":"string"},
	"webhook_secret_value":{"type":"string"}
}`

const automationTriggerPatchProperties = `{
	"trigger_id":{"type":"string"},
	"name":{"type":"string"},
	"prompt":{"type":"string"},
	"event":{"type":"string"},
	"filter":{"type":"object"},
	"enabled":{"type":"boolean"},
	"retry":{"type":"object"},
	"fire_limit":{"type":"object"},
	"webhook_id":{"type":"string"},
	"endpoint_slug":{"type":"string"},
	"webhook_secret_value":{"type":"string"}
}`

const automationJobHistoryProperties = `{
	"job_id":{"type":"string"},
	"status":{"type":"string"},
	"since":{"type":"string"},
	"until":{"type":"string"},
	"limit":{"type":"integer"}
}`

const automationTriggerHistoryProperties = `{
	"trigger_id":{"type":"string"},
	"status":{"type":"string"},
	"since":{"type":"string"},
	"until":{"type":"string"},
	"limit":{"type":"integer"}
}`

const automationRunQueryProperties = `{
	"job_id":{"type":"string"},
	"trigger_id":{"type":"string"},
	"status":{"type":"string"},
	"since":{"type":"string"},
	"until":{"type":"string"},
	"limit":{"type":"integer"}
}`
