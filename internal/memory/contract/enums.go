package contract

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Scope identifies the owner layer a memory entry belongs to.
type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeWorkspace Scope = "workspace"
	ScopeAgent     Scope = "agent"
)

// Normalize returns the normalized representation of the scope.
func (s Scope) Normalize() Scope {
	return Scope(normalizeEnum(string(s)))
}

// Validate reports whether the scope belongs to the closed taxonomy.
func (s Scope) Validate() error {
	switch s.Normalize() {
	case ScopeGlobal, ScopeWorkspace, ScopeAgent:
		return nil
	case "":
		return fmt.Errorf("scope is required")
	default:
		return fmt.Errorf("unsupported scope %q", s)
	}
}

// AgentTier identifies which agent tier owns an agent-scoped memory entry.
type AgentTier string

const (
	AgentTierWorkspace AgentTier = "workspace"
	AgentTierGlobal    AgentTier = "global"
)

// Normalize returns the normalized representation of the agent tier.
func (t AgentTier) Normalize() AgentTier {
	return AgentTier(normalizeEnum(string(t)))
}

// Validate reports whether the agent tier belongs to the closed taxonomy.
func (t AgentTier) Validate() error {
	switch t.Normalize() {
	case AgentTierWorkspace, AgentTierGlobal:
		return nil
	case "":
		return fmt.Errorf("agent tier is required")
	default:
		return fmt.Errorf("unsupported agent tier %q", t)
	}
}

// Origin identifies the surface that submitted a memory candidate.
type Origin string

const (
	OriginCLI       Origin = "cli"
	OriginHTTP      Origin = "http"
	OriginUDS       Origin = "uds"
	OriginTool      Origin = "tool"
	OriginExtractor Origin = "extractor"
	OriginDreaming  Origin = "dreaming"
	OriginFile      Origin = "file"
	OriginProvider  Origin = "provider"
)

// Normalize returns the normalized representation of the origin.
func (o Origin) Normalize() Origin {
	return Origin(normalizeEnum(string(o)))
}

// Validate reports whether the origin belongs to the closed taxonomy.
func (o Origin) Validate() error {
	switch o.Normalize() {
	case OriginCLI, OriginHTTP, OriginUDS, OriginTool, OriginExtractor, OriginDreaming, OriginFile, OriginProvider:
		return nil
	case "":
		return fmt.Errorf("origin is required")
	default:
		return fmt.Errorf("unsupported origin %q", o)
	}
}

// Type identifies the closed persistent-memory taxonomy.
type Type string

const (
	TypeUser      Type = "user"
	TypeFeedback  Type = "feedback"
	TypeProject   Type = "project"
	TypeReference Type = "reference"
)

// Normalize returns the normalized representation of the memory type.
func (t Type) Normalize() Type {
	return Type(normalizeEnum(string(t)))
}

// Validate reports whether the memory type belongs to the closed taxonomy.
func (t Type) Validate() error {
	switch t.Normalize() {
	case TypeUser, TypeFeedback, TypeProject, TypeReference:
		return nil
	case "":
		return fmt.Errorf("memory type is required")
	default:
		return fmt.Errorf("unsupported memory type %q", t)
	}
}

// DefaultScopeForType resolves the default persistence scope for a memory type.
func DefaultScopeForType(t Type) (Scope, error) {
	switch t.Normalize() {
	case TypeUser, TypeFeedback:
		return ScopeGlobal, nil
	case TypeProject, TypeReference:
		return ScopeWorkspace, nil
	case "":
		return "", fmt.Errorf("memory type is required")
	default:
		return "", fmt.Errorf("unsupported memory type %q", t)
	}
}

// Operation identifies a durable memory operation surfaced in operator history.
type Operation string

const (
	OperationWrite   Operation = "memory.write"
	OperationDelete  Operation = "memory.delete"
	OperationSearch  Operation = "memory.search"
	OperationReindex Operation = "memory.reindex"
)

// Normalize returns the normalized operation string.
func (o Operation) Normalize() Operation {
	return Operation(normalizeEnum(string(o)))
}

// Op identifies a write-controller decision.
type Op uint8

const (
	OpNoop Op = iota
	OpAdd
	OpUpdate
	OpDelete
	OpReject
)

var opNames = map[Op]string{
	OpNoop:   "noop",
	OpAdd:    "add",
	OpUpdate: "update",
	OpDelete: "delete",
	OpReject: "reject",
}

// String returns the canonical JSON/DB value for the operation.
func (o Op) String() string {
	if name, ok := opNames[o]; ok {
		return name
	}
	return ""
}

// Normalize returns the canonical operation value.
func (o Op) Normalize() Op {
	name := normalizeEnum(o.String())
	for value, candidate := range opNames {
		if candidate == name {
			return value
		}
	}
	return o
}

// Validate reports whether the operation belongs to the closed taxonomy.
func (o Op) Validate() error {
	if _, ok := opNames[o]; ok {
		return nil
	}
	return fmt.Errorf("unsupported operation %d", o)
}

// MarshalJSON serializes Op as its canonical string value.
func (o Op) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(o.String())
}

// UnmarshalJSON decodes Op from its canonical string value.
func (o *Op) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode operation: %w", err)
	}
	value, err := parseOp(raw)
	if err != nil {
		return err
	}
	*o = value
	return nil
}

func parseOp(raw string) (Op, error) {
	normalized := normalizeEnum(raw)
	for op, name := range opNames {
		if name == normalized {
			return op, nil
		}
	}
	if normalized == "" {
		return OpNoop, fmt.Errorf("operation is required")
	}
	return OpNoop, fmt.Errorf("unsupported operation %q", raw)
}

// DecisionSource identifies whether a decision came from rules or an LLM tiebreaker.
type DecisionSource string

const (
	SourceRule DecisionSource = "rule"
	SourceLLM  DecisionSource = "llm"
)

// Normalize returns the normalized representation of the decision source.
func (s DecisionSource) Normalize() DecisionSource {
	return DecisionSource(normalizeEnum(string(s)))
}

// Validate reports whether the decision source belongs to the closed taxonomy.
func (s DecisionSource) Validate() error {
	switch s.Normalize() {
	case SourceRule, SourceLLM:
		return nil
	case "":
		return fmt.Errorf("decision source is required")
	default:
		return fmt.Errorf("unsupported decision source %q", s)
	}
}

// Trigger identifies why an extractor run was requested.
type Trigger string

const (
	TriggerPostMessage     Trigger = "post_message"
	TriggerCompactionFlush Trigger = "compaction_flush"
)

// Normalize returns the normalized representation of the extractor trigger.
func (t Trigger) Normalize() Trigger {
	return Trigger(normalizeEnum(string(t)))
}

// Validate reports whether the trigger belongs to the closed taxonomy.
func (t Trigger) Validate() error {
	switch t.Normalize() {
	case TriggerPostMessage, TriggerCompactionFlush:
		return nil
	case "":
		return fmt.Errorf("trigger is required")
	default:
		return fmt.Errorf("unsupported trigger %q", t)
	}
}

func normalizeEnum(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
