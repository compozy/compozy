package dsl

// GateCriterion is one typed check in a gate or verification bundle.
type GateCriterion struct {
	ID     string         `json:"id"               yaml:"id"`
	Type   CriterionType  `json:"type"             yaml:"type"`
	Check  string         `json:"check,omitempty"  yaml:"check,omitempty"`
	Expect string         `json:"expect,omitempty" yaml:"expect,omitempty"`
	Agent  string         `json:"agent,omitempty"  yaml:"agent,omitempty"`
	Model  string         `json:"model,omitempty"  yaml:"model,omitempty"`
	Rubric string         `json:"rubric,omitempty" yaml:"rubric,omitempty"`
	Prompt string         `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	Tool   string         `json:"tool,omitempty"   yaml:"tool,omitempty"`
	Inputs map[string]any `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Extra  map[string]any `json:"-"                yaml:",inline"`
}

// CriterionType is the closed gate criterion vocabulary.
type CriterionType string

const (
	// CriterionCommand runs a project command.
	CriterionCommand CriterionType = "command"
	// CriterionAgentJudge asks an agent to judge against a rubric.
	CriterionAgentJudge CriterionType = "agent-judge"
	// CriterionHuman waits for a human decision.
	CriterionHuman CriterionType = "human"
	// CriterionExtension calls an extension tool.
	CriterionExtension CriterionType = "extension"
)

// VerdictPolicy controls how a gate reaches a verdict.
type VerdictPolicy string

const (
	// VerdictPolicyReviseUntilClean requires an agent/human verdict source.
	VerdictPolicyReviseUntilClean VerdictPolicy = "revise_until_clean"
	// VerdictPolicyFixedPasses is command-only compatible.
	VerdictPolicyFixedPasses VerdictPolicy = "fixed_passes"
)

// StartKind is the closed start-surface allowlist vocabulary.
type StartKind string

const (
	// StartManual allows operator web/manual starts.
	StartManual StartKind = "manual"
	// StartCLI allows CLI starts.
	StartCLI StartKind = "cli"
	// StartHTTP allows HTTP starts.
	StartHTTP StartKind = "http"
	// StartUDS allows UDS starts.
	StartUDS StartKind = "uds"
	// StartTrigger allows automation trigger starts.
	StartTrigger StartKind = "trigger"
	// StartSchedule allows scheduled starts.
	StartSchedule StartKind = "schedule"
	// StartWebhook allows webhook starts.
	StartWebhook StartKind = "webhook"
	// StartNetwork allows network starts.
	StartNetwork StartKind = "network"
	// StartExtension allows extension starts.
	StartExtension StartKind = "extension"
	// StartNativeTool allows native-tool starts.
	StartNativeTool StartKind = "native_tool"
)

// StartBinding declares one permitted start surface.
type StartBinding struct {
	Kind         StartKind         `json:"kind"                    yaml:"kind"`
	Inputs       map[string]any    `json:"inputs,omitempty"        yaml:"inputs,omitempty"`
	InputMapping map[string]string `json:"input_mapping,omitempty" yaml:"input_mapping,omitempty"`
	Extra        map[string]any    `json:"-"                       yaml:",inline"`
}
