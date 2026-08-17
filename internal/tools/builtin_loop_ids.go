package tools

const (
	// ToolIDLoopList lists Loop definitions available in a workspace.
	ToolIDLoopList ToolID = "compozy__loop_list"
	// ToolIDLoopInspect reads one Loop definition and authoring contract.
	ToolIDLoopInspect ToolID = "compozy__loop_inspect"
	// ToolIDLoopValidate lints and compiles one Loop definition without saving it.
	ToolIDLoopValidate ToolID = "compozy__loop_validate"
	// ToolIDLoopCreate creates or forks one Loop definition.
	ToolIDLoopCreate ToolID = "compozy__loop_create"
	// ToolIDLoopRun starts or dry-runs one Loop.
	ToolIDLoopRun ToolID = "compozy__loop_run"
	// ToolIDLoopStatus reads one Loop run status.
	ToolIDLoopStatus ToolID = "compozy__loop_status"
	// ToolIDLoopRuns lists workspace-scoped Loop runs.
	ToolIDLoopRuns ToolID = "compozy__loop_runs"
	// ToolIDLoopTurns lists one Loop run's total-order Goal turn audit.
	ToolIDLoopTurns ToolID = "compozy__loop_turns"
	// ToolIDLoopCancel requests cooperative cancellation for one Loop run.
	ToolIDLoopCancel ToolID = "compozy__loop_cancel"
	// ToolIDLoopKill immediately terminalizes one Loop run.
	ToolIDLoopKill ToolID = "compozy__loop_kill"
	// ToolIDLoopNodes lists workspace-scoped Loop node state.
	ToolIDLoopNodes ToolID = "compozy__loop_nodes"
	// ToolIDLoopNodePause parks one authored Loop node.
	ToolIDLoopNodePause ToolID = "compozy__loop_node_pause"
	// ToolIDLoopNodeResume releases one authored Loop node or manual wait.
	ToolIDLoopNodeResume ToolID = "compozy__loop_node_resume"
	// ToolIDLoopNodeCancel requests cooperative node cancellation.
	ToolIDLoopNodeCancel ToolID = "compozy__loop_node_cancel"
	// ToolIDLoopNodeKill immediately fences one Loop node.
	ToolIDLoopNodeKill ToolID = "compozy__loop_node_kill"
	// ToolIDLoopNodeRequeue clears one Loop node quarantine.
	ToolIDLoopNodeRequeue ToolID = "compozy__loop_node_requeue"
	// ToolIDLoopPause pauses one running Loop at a generation boundary.
	ToolIDLoopPause ToolID = "compozy__loop_pause"
	// ToolIDLoopResume resumes one paused Loop run.
	ToolIDLoopResume ToolID = "compozy__loop_resume"
	// ToolIDLoopConfigure writes per-loop runtime config overrides.
	ToolIDLoopConfigure ToolID = "compozy__loop_configure"
	// ToolIDLoopApprove applies one human-gate decision.
	ToolIDLoopApprove ToolID = "compozy__loop_approve"
	// ToolIDLoopRequests lists human requests.
	ToolIDLoopRequests ToolID = "compozy__loop_requests"
	// ToolIDLoopRequest reads one human request in full.
	ToolIDLoopRequest ToolID = "compozy__loop_request"
	// ToolIDLoopRespond answers one human request.
	ToolIDLoopRespond ToolID = "compozy__loop_respond"
	// ToolIDLoopNodeAmend appends one effective-output amendment.
	ToolIDLoopNodeAmend ToolID = "compozy__loop_node_amend"
	// ToolIDLoopDiff compares generations or same-loop runs.
	ToolIDLoopDiff ToolID = "compozy__loop_diff"
	// ToolIDLoopRerun reopens execution from one settled node.
	ToolIDLoopRerun ToolID = "compozy__loop_rerun"
	// ToolIDLoopFork creates a linked run from one historical generation.
	ToolIDLoopFork ToolID = "compozy__loop_fork"
	// ToolIDLoopDelete deletes one user-authored Loop definition.
	ToolIDLoopDelete ToolID = "compozy__loop_delete"
	// ToolsetIDLoops groups Loop authoring, execution, and run-management tools.
	ToolsetIDLoops ToolsetID = "compozy__loops"
)
