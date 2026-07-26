package acpmock

import "encoding/json"

const FixtureVersion = 2

type StepKind string

const (
	StepKindAssistant     StepKind = "assistant"
	StepKindThought       StepKind = "thought"
	StepKindToolCall      StepKind = "tool_call"
	StepKindPermission    StepKind = "permission"
	StepKindSandbox       StepKind = "sandbox_exec"
	StepKindBridgeContent StepKind = "bridge_response"
	StepKindDriverControl StepKind = "driver_control"
)

// Fixture describes one deterministic multi-agent ACP mock scenario.
type Fixture struct {
	Version int            `json:"version"`
	Agents  []AgentFixture `json:"agents"`
}

// AgentFixture describes one named ACP mock agent inside a fixture file.
type AgentFixture struct {
	Name            string                       `json:"name"`
	Provider        string                       `json:"provider"`
	Model           string                       `json:"model,omitempty"`
	ReasoningEffort string                       `json:"reasoning_effort,omitempty"`
	Permissions     string                       `json:"permissions,omitempty"`
	Prompt          string                       `json:"prompt,omitempty"`
	LoadSession     *bool                        `json:"load_session,omitempty"`
	ConfigOptions   []SessionConfigOptionFixture `json:"config_options,omitempty"`
	Turns           []TurnFixture                `json:"turns"`
}

// SupportsLoadSession reports the fixture's advertised ACP session/load capability.
func (a AgentFixture) SupportsLoadSession() bool {
	return a.LoadSession == nil || *a.LoadSession
}

// SessionConfigOptionFixture describes one deterministic ACP session config select option.
type SessionConfigOptionFixture struct {
	ID      string                            `json:"id"`
	Name    string                            `json:"name"`
	Current string                            `json:"current"`
	Values  []SessionConfigOptionValueFixture `json:"values"`
}

// SessionConfigOptionValueFixture describes one selectable ACP config option value.
type SessionConfigOptionValueFixture struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// TurnFixture describes one deterministic prompt turn for an agent.
type TurnFixture struct {
	Name       string     `json:"name,omitempty"`
	Match      TurnMatch  `json:"match"`
	Steps      []Step     `json:"steps"`
	StopReason string     `json:"stop_reason,omitempty"`
	Usage      *TurnUsage `json:"usage,omitempty"`
}

// TurnUsage scripts deterministic aggregate token usage for one ACP prompt response.
type TurnUsage struct {
	InputTokens  int  `json:"input_tokens"`
	OutputTokens int  `json:"output_tokens"`
	TotalTokens  *int `json:"total_tokens,omitempty"`
}

// TurnMatch routes a prompt to a turn fixture using stable prompt fields.
type TurnMatch struct {
	TurnSource string            `json:"turn_source,omitempty"`
	UserText   string            `json:"user_text,omitempty"`
	Network    *TurnMatchNetwork `json:"network,omitempty"`
	Goal       *TurnMatchGoal    `json:"goal,omitempty"`
	Judge      *TurnMatchJudge   `json:"judge,omitempty"`
}

// TurnMatchGoal captures exact Goal prompt metadata fields.
type TurnMatchGoal struct {
	Kind          string `json:"kind,omitempty"`
	RunID         string `json:"run_id,omitempty"`
	NodeID        string `json:"node_id,omitempty"`
	Generation    int64  `json:"generation,omitempty"`
	ItemIndex     *int   `json:"item_index,omitempty"`
	Turn          *int   `json:"turn,omitempty"`
	PromptAttempt *int   `json:"prompt_attempt,omitempty"`
	PromptID      string `json:"prompt_id,omitempty"`
}

// TurnMatchJudge captures exact daemon-owned judge prompt metadata fields.
type TurnMatchJudge struct {
	Role          string `json:"role,omitempty"`
	Attempt       int    `json:"attempt,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	GateID        string `json:"gate_id,omitempty"`
	CriterionID   string `json:"criterion_id,omitempty"`
}

// TurnMatchNetwork captures exact AGH network envelope field matching.
type TurnMatchNetwork struct {
	MessageID   string `json:"message_id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Surface     string `json:"surface,omitempty"`
	ThreadID    string `json:"thread_id,omitempty"`
	DirectID    string `json:"direct_id,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	WorkID      string `json:"work_id,omitempty"`
	ReplyTo     string `json:"reply_to,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
	CausationID string `json:"causation_id,omitempty"`
	Trust       string `json:"trust,omitempty"`
}

// Step describes one deterministic ACP action emitted or executed by the driver.
type Step struct {
	Kind StepKind `json:"kind"`

	Text   string   `json:"text,omitempty"`
	Chunks []string `json:"chunks,omitempty"`

	ToolCallID  string          `json:"tool_call_id,omitempty"`
	Title       string          `json:"title,omitempty"`
	ToolKind    string          `json:"tool_kind,omitempty"`
	Path        string          `json:"path,omitempty"`
	Status      string          `json:"status,omitempty"`
	ContentText string          `json:"content_text,omitempty"`
	RawInput    json.RawMessage `json:"raw_input,omitempty"`
	RawOutput   json.RawMessage `json:"raw_output,omitempty"`

	ExpectDecision string `json:"expect_decision,omitempty"`
	EmitDecision   bool   `json:"emit_decision,omitempty"`
	EmitText       string `json:"emit_text,omitempty"`

	Command              string             `json:"command,omitempty"`
	Args                 []string           `json:"args,omitempty"`
	Cwd                  string             `json:"cwd,omitempty"`
	ExpectExitCode       *int               `json:"expect_exit_code,omitempty"`
	ExpectOutputContains string             `json:"expect_output_contains,omitempty"`
	ExpectErrorContains  string             `json:"expect_error_contains,omitempty"`
	EmitOutput           bool               `json:"emit_output,omitempty"`
	DriverControl        *DriverControlStep `json:"driver_control,omitempty"`
}

// DriverControlStep injects driver-level protocol or lifecycle faults.
type DriverControlStep struct {
	Action     DriverControlAction `json:"action"`
	RawJSONRPC string              `json:"raw_jsonrpc,omitempty"`
	Async      bool                `json:"async,omitempty"`
	DelayMS    int                 `json:"delay_ms,omitempty"`
}

// DriverControlAction identifies one supported driver fault injection action.
type DriverControlAction string

const (
	DriverControlDisconnect       DriverControlAction = "disconnect"
	DriverControlWriteRawJSONRPC  DriverControlAction = "write_raw_jsonrpc"
	DriverControlBlockUntilCancel DriverControlAction = "block_until_cancel"
	DriverControlDelay            DriverControlAction = "delay"
)
