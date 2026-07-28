package builtin

const taskReadInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskCreateInputSchema = `{
	"type":"object",
	"required":["scope","title"],
	"properties":{
		"id":{"type":"string"},
		"identifier":{"type":"string"},
		"scope":{"type":"string"},
		"workspace_id":{"type":"string"},
		"network_participation":` + networkParticipationRequestSchema + `,
		"title":{"type":"string"},
		"description":{"type":"string"},
		"priority":{"type":"string"},
		"max_attempts":{"type":"integer"},
		"draft":{"type":"boolean"},
		"approval_policy":{"type":"string"},
		"owner":` + ownerSchema + `,
		"wake_creator":{"type":"boolean"},
		"metadata":{}
	},
	"additionalProperties":false
}`

const taskChildCreateInputSchema = `{
	"type":"object",
	"required":["parent_task_id","scope","title"],
	"properties":{
		"parent_task_id":{"type":"string"},
		"id":{"type":"string"},
		"identifier":{"type":"string"},
		"scope":{"type":"string"},
		"workspace_id":{"type":"string"},
		"network_participation":` + networkParticipationRequestSchema + `,
		"title":{"type":"string"},
		"description":{"type":"string"},
		"priority":{"type":"string"},
		"max_attempts":{"type":"integer"},
		"draft":{"type":"boolean"},
		"approval_policy":{"type":"string"},
		"owner":` + ownerSchema + `,
		"wake_creator":{"type":"boolean"},
		"metadata":{}
	},
	"additionalProperties":false
}`

const taskUpdateInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"},
		"title":{"type":"string"},
		"description":{"type":"string"},
		"priority":{"type":"string"},
		"max_attempts":{"type":"integer"},
		"approval_policy":{"type":"string"},
		"metadata":{},
		"network_participation":` + networkParticipationRequestSchema + `,
		"owner":` + ownerSchema + `,
		"clear_owner":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const taskCancelInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"},
		"reason":{"type":"string"},
		"metadata":{}
	},
	"additionalProperties":false
}`

const taskBlockInputSchema = `{
	"type":"object",
	"required":["task_id","kind","reason"],
	"properties":{
		"task_id":{"type":"string"},
		"kind":{"type":"string","enum":["needs_input","capability","transient"]},
		"reason":{"type":"string"},
		"details":{},
		"expires_at":{"type":"string","format":"date-time"},
		"run_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskUnblockInputSchema = `{
	"type":"object",
	"required":["task_id","block_id"],
	"properties":{
		"task_id":{"type":"string"},
		"block_id":{"type":"string"},
		"run_id":{"type":"string"},
		"note":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskBlocksInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"},
		"include_cleared":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const taskRecoverInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"},
		"note":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskRunListInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"},
		"status":{"type":"string"},
		"session_id":{"type":"string"},
		"participation_channel":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const taskRunReviewRequestInputSchema = `{
	"type":"object",
	"required":["task_id","run_id"],
	"properties":{
		"task_id":{"type":"string"},
		"run_id":{"type":"string"},
		"policy":{"type":"string","enum":["","on_success","on_failure","always"]},
		"review_round":{"type":"integer"},
		"attempt":{"type":"integer"},
		"parent_review_id":{"type":"string"},
		"reason":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskRunReviewListInputSchema = `{
	"type":"object",
	"properties":{
		"task_id":{"type":"string"},
		"run_id":{"type":"string"},
		"status":{"type":"string","enum":["","requested","routed","in_review","recorded","circuit_opened","canceled"]},
		"reviewer_session_id":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const taskRunReviewShowInputSchema = `{
	"type":"object",
	"required":["review_id"],
	"properties":{
		"review_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskExecutionProfileGetInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskExecutionProfileDeleteInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskExecutionProfileSetInputSchema = `{
	"type":"object",
	"required":["task_id","profile"],
	"properties":{
		"task_id":{"type":"string"},
		"profile":` + taskExecutionProfileSchema + `
	},
	"additionalProperties":false
}`

const taskNotificationSubscribeInputSchema = `{
	"type":"object",
	"required":["task_id","bridge_instance_id"],
	"properties":{
		"task_id":{"type":"string"},
		"subscription_id":{"type":"string"},
		"bridge_instance_id":{"type":"string"},
		"scope":{"type":"string","enum":["","global","workspace"]},
		"workspace_id":{"type":"string"},
		"peer_id":{"type":"string"},
		"thread_id":{"type":"string"},
		"group_id":{"type":"string"},
		"delivery_mode":{"type":"string","enum":["","direct-send","reply"]}
	},
	"additionalProperties":false
}`

const taskNotificationListInputSchema = `{
	"type":"object",
	"required":["task_id"],
	"properties":{
		"task_id":{"type":"string"},
		"bridge_instance_id":{"type":"string"},
		"scope":{"type":"string","enum":["","global","workspace"]},
		"workspace_id":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const taskNotificationShowInputSchema = `{
	"type":"object",
	"required":["task_id","subscription_id"],
	"properties":{
		"task_id":{"type":"string"},
		"subscription_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskNotificationDeleteInputSchema = `{
	"type":"object",
	"required":["task_id","subscription_id"],
	"properties":{
		"task_id":{"type":"string"},
		"subscription_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const taskExecutionProfileSchema = `{
	"type":"object",
	"properties":{
		"task_id":{"type":"string"},
		"coordinator":` + coordinatorProfileSchema + `,
		"worker":` + workerProfileSchema + `,
		"review":` + reviewProfileSchema + `,
		"participants":` + participantPolicySchema + `,
		"sandbox":` + sandboxPolicySchema + `,
		"runtime":` + runtimePolicySchema + `,
		"network_participation":` + networkParticipationRequestSchema + `
	},
	"additionalProperties":false
}`

const coordinatorProfileSchema = `{
	"type":"object",
	"properties":{
		"mode":{"type":"string","enum":["","inherit","guided"]},
		"agent_name":{"type":"string"},
		"provider":{"type":"string"},
		"model":{"type":"string"},
		"guidance":{"type":"string"}
	},
	"additionalProperties":false
}`

const workerProfileSchema = `{
	"type":"object",
	"properties":{
		"mode":{"type":"string","enum":["","inherit","select"]},
		"agent_name":{"type":"string"},
		"provider":{"type":"string"},
		"model":{"type":"string"},
		"allowed_agent_names":{"type":"array","items":{"type":"string"}},
		"preferred_agent_names":{"type":"array","items":{"type":"string"}},
		"required_capabilities":{"type":"array","items":{"type":"string"}},
		"preferred_capabilities":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`

const reviewProfileSchema = `{
	"type":"object",
	"properties":{
		"agent_name":{"type":"string"},
		"provider":{"type":"string"},
		"model":{"type":"string"},
		"allowed_agent_names":{"type":"array","items":{"type":"string"}},
		"preferred_agent_names":{"type":"array","items":{"type":"string"}},
		"allowed_channel_ids":{"type":"array","items":{"type":"string"}},
		"preferred_channel_ids":{"type":"array","items":{"type":"string"}},
		"allowed_peer_ids":{"type":"array","items":{"type":"string"}},
		"preferred_peer_ids":{"type":"array","items":{"type":"string"}},
		"required_capabilities":{"type":"array","items":{"type":"string"}},
		"preferred_capabilities":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`

const participantPolicySchema = `{
	"type":"object",
	"properties":{
		"allowed_channel_ids":{"type":"array","items":{"type":"string"}},
		"preferred_channel_ids":{"type":"array","items":{"type":"string"}},
		"allowed_peer_ids":{"type":"array","items":{"type":"string"}},
		"preferred_peer_ids":{"type":"array","items":{"type":"string"}},
		"allowed_agent_names":{"type":"array","items":{"type":"string"}},
		"preferred_agent_names":{"type":"array","items":{"type":"string"}},
		"required_capabilities":{"type":"array","items":{"type":"string"}},
		"preferred_capabilities":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`

const sandboxPolicySchema = `{
	"type":"object",
	"properties":{
		"mode":{"type":"string","enum":["","inherit","none","ref"]},
		"sandbox_ref":{"type":"string"}
	},
	"additionalProperties":false
}`

const runtimePolicySchema = `{
	"type":"object",
	"required":["mode"],
	"properties":{
		"mode":{"type":"string","enum":["default","evidence"],"default":"default"}
	},
	"additionalProperties":false
}`

const ownerSchema = `{
	"type":"object",
	"required":["kind","ref"],
	"properties":{
		"kind":{"type":"string"},
		"ref":{"type":"string"}
	},
	"additionalProperties":false
}`
