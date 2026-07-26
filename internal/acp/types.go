// Package acp provides the AGH ACP client implementation.
package acp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/sandbox"
)

const (
	// EventTypeUserMessage is emitted when the agent echoes a user message chunk.
	EventTypeUserMessage = "user_message"
	// EventTypeSyntheticReentry is emitted for daemon-originated synthetic prompt input.
	EventTypeSyntheticReentry = "synthetic_reentry"
	// EventTypeAgentMessage is emitted for assistant message chunks.
	EventTypeAgentMessage = "agent_message"
	// EventTypeThought is emitted for assistant thought chunks.
	EventTypeThought = "thought"
	// EventTypeToolCall is emitted when a tool call starts or is updated in-flight.
	EventTypeToolCall = "tool_call"
	// EventTypeToolResult is emitted when a tool call finishes.
	EventTypeToolResult = "tool_result"
	// EventTypePlan is emitted for plan updates.
	EventTypePlan = "plan"
	// EventTypePermission is emitted when the daemon applies a permission decision.
	EventTypePermission = "permission"
	// EventTypeClarify is emitted for daemon-owned clarification lifecycle transitions.
	EventTypeClarify = "clarify"
	// EventTypeUsage is emitted when unstable usage metadata is reported.
	EventTypeUsage = "usage"
	// EventTypeSystem is emitted for system-level ACP updates.
	EventTypeSystem = "system"
	// EventTypeRuntimeProgress is emitted by AGH while a prompt remains active.
	EventTypeRuntimeProgress = "runtime_progress"
	// EventTypeRuntimeWarning is emitted by AGH when active prompt activity is stale.
	EventTypeRuntimeWarning = "runtime_warning"
	// EventTypeDone is emitted when a prompt turn finishes.
	EventTypeDone = "done"
	// EventTypeError is emitted when prompt processing fails.
	EventTypeError = "error"
	// SystemEventTitleAvailableCommandsUpdate replaces the provider-advertised command set.
	SystemEventTitleAvailableCommandsUpdate = "available_commands_update"
)

// StartOpts defines how to launch and initialize an ACP agent process.
type StartOpts struct {
	AgentName            string
	Command              string
	Cwd                  string
	AdditionalDirs       []string
	Env                  []string
	MCPServers           []aghconfig.MCPServer
	Permissions          aghconfig.PermissionMode
	SystemPrompt         string
	SystemPromptDelivery SystemPromptDeliveryMode
	PreferredModel       string
	ReasoningEffort      string
	ResumeSessionID      string
	Launcher             sandbox.Launcher
	ToolHost             sandbox.ToolHost
	ToolGateway          ToolExecutionGateway
	ProviderName         string
	ProviderConfig       *aghconfig.ProviderConfig
	ProviderAuthEnv      *authproviders.ProbeEnv
}

// SystemPromptDeliveryMode records how AGH delivered startup system guidance to
// the provider harness.
type SystemPromptDeliveryMode string

const (
	// SystemPromptDeliveryFirstTurnPrefix means AGH embedded startup system
	// guidance in the first ACP prompt because generic ACP has no system role.
	SystemPromptDeliveryFirstTurnPrefix SystemPromptDeliveryMode = "first_turn_prefix"
	// SystemPromptDeliveryNative means the provider received startup system
	// guidance through a provider-native system-prompt channel.
	SystemPromptDeliveryNative SystemPromptDeliveryMode = "native"
)

// Validate ensures the start options are minimally usable.
func (o StartOpts) Validate() error {
	switch {
	case strings.TrimSpace(o.AgentName) == "":
		return errors.New("acp: agent name is required")
	case strings.TrimSpace(o.Command) == "":
		return errors.New("acp: command is required")
	case strings.TrimSpace(o.Cwd) == "":
		return errors.New("acp: cwd is required")
	}
	for i, dir := range o.AdditionalDirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		if !filepath.IsAbs(trimmed) {
			return fmt.Errorf("acp: additional_dirs[%d] must be absolute", i)
		}
	}

	mode := o.Permissions
	if mode == "" {
		mode = aghconfig.PermissionModeApproveReads
	}
	if err := mode.Validate("start.permissions"); err != nil {
		return err
	}

	for i, server := range o.MCPServers {
		if err := server.Validate(fmt.Sprintf("start.mcp_servers[%d]", i)); err != nil {
			return err
		}
	}
	switch o.SystemPromptDelivery {
	case "", SystemPromptDeliveryFirstTurnPrefix, SystemPromptDeliveryNative:
		return nil
	default:
		return fmt.Errorf("acp: invalid system prompt delivery %q", o.SystemPromptDelivery)
	}
}

// PromptRequest describes one prompt turn sent to an active ACP session.
type PromptRequest struct {
	TurnID                    string
	Message                   string
	Meta                      PromptMeta
	ActivityReporter          PromptActivityReporter
	ActivityHeartbeatInterval time.Duration
}

// Validate ensures the prompt request can be sent to the agent.
func (r PromptRequest) Validate() error {
	if strings.TrimSpace(r.TurnID) == "" {
		return errors.New("acp: prompt turn id is required")
	}
	if strings.TrimSpace(r.Message) == "" {
		return errors.New("acp: prompt message is required")
	}
	if r.ActivityHeartbeatInterval < 0 {
		return errors.New("acp: prompt activity heartbeat interval must be zero or positive")
	}
	if err := r.Meta.Validate(); err != nil {
		return err
	}
	return nil
}

// PromptActivityReporter receives runtime activity observed while a prompt is in flight.
type PromptActivityReporter func(PromptActivityReport)

// PromptActivityReport is a low-frequency runtime heartbeat emitted by the driver.
type PromptActivityReport struct {
	Timestamp time.Time
	Kind      string
	Detail    string
}

const (
	// PromptActionSteered identifies a user message injected from the busy-input steer queue.
	PromptActionSteered = "prompt_steered"
)

// SteerInput is one staged operator message consumed at a tool-result boundary.
type SteerInput struct {
	Text            string
	QueueEntryID    string
	QueueGeneration int64
}

// SteerSource is the ACP-side consumption boundary for staged steer input.
type SteerSource interface {
	ConsumeSteer(ctx context.Context, sessionID string) (SteerInput, bool, error)
}

// Caps captures the usable capabilities exposed by an ACP agent.
type Caps struct {
	SupportsLoadSession bool
	SupportedModes      []string
	ConfigOptions       []SessionConfigOption
}

// CloneCaps returns a deep copy of ACP caps.
func CloneCaps(caps Caps) Caps {
	return Caps{
		SupportsLoadSession: caps.SupportsLoadSession,
		SupportedModes:      append([]string(nil), caps.SupportedModes...),
		ConfigOptions:       CloneSessionConfigOptions(caps.ConfigOptions),
	}
}

// TokenUsage captures per-turn usage reported by the agent.
type TokenUsage struct {
	TurnID           string
	InputTokens      *int64
	OutputTokens     *int64
	TotalTokens      *int64
	ThoughtTokens    *int64
	CacheReadTokens  *int64
	CacheWriteTokens *int64
	ContextUsed      *int64
	ContextSize      *int64
	CostAmount       *float64
	CostCurrency     *string
	Timestamp        time.Time
}

// RuntimeActivity is the structured activity payload attached to AGH runtime
// progress and warning events.
type RuntimeActivity struct {
	TurnID             string     `json:"turn_id,omitempty"`
	TurnSource         string     `json:"turn_source,omitempty"`
	TurnStartedAt      *time.Time `json:"turn_started_at,omitempty"`
	DeadlineAt         *time.Time `json:"deadline_at,omitempty"`
	LastActivityAt     *time.Time `json:"last_activity_at,omitempty"`
	LastActivityKind   string     `json:"last_activity_kind,omitempty"`
	LastActivityDetail string     `json:"last_activity_detail,omitempty"`
	CurrentTool        string     `json:"current_tool,omitempty"`
	ToolCallID         string     `json:"tool_call_id,omitempty"`
	LastProgressAt     *time.Time `json:"last_progress_at,omitempty"`
	IterationCurrent   int        `json:"iteration_current,omitempty"`
	IterationMax       int        `json:"iteration_max,omitempty"`
	IdleSeconds        int64      `json:"idle_seconds,omitempty"`
	ElapsedSeconds     int64      `json:"elapsed_seconds,omitempty"`
	ElapsedMS          int64      `json:"elapsed_ms,omitempty"`
}

// Merge overlays non-nil usage fields from other into the receiver.
func (u TokenUsage) Merge(other TokenUsage) TokenUsage {
	merged := u
	if turnID := strings.TrimSpace(other.TurnID); turnID != "" {
		merged.TurnID = turnID
	}
	merged.InputTokens = chooseInt64(other.InputTokens, merged.InputTokens)
	merged.OutputTokens = chooseInt64(other.OutputTokens, merged.OutputTokens)
	merged.TotalTokens = chooseInt64(other.TotalTokens, merged.TotalTokens)
	merged.ThoughtTokens = chooseInt64(other.ThoughtTokens, merged.ThoughtTokens)
	merged.CacheReadTokens = chooseInt64(other.CacheReadTokens, merged.CacheReadTokens)
	merged.CacheWriteTokens = chooseInt64(other.CacheWriteTokens, merged.CacheWriteTokens)
	merged.ContextUsed = chooseInt64(other.ContextUsed, merged.ContextUsed)
	merged.ContextSize = chooseInt64(other.ContextSize, merged.ContextSize)
	merged.CostAmount = chooseFloat64(other.CostAmount, merged.CostAmount)
	merged.CostCurrency = chooseString(other.CostCurrency, merged.CostCurrency)
	if !other.Timestamp.IsZero() {
		merged.Timestamp = other.Timestamp
	}
	return merged
}

// IsZero reports whether the usage payload carries any reported values.
func (u TokenUsage) IsZero() bool {
	return u.InputTokens == nil &&
		u.OutputTokens == nil &&
		u.TotalTokens == nil &&
		u.ThoughtTokens == nil &&
		u.CacheReadTokens == nil &&
		u.CacheWriteTokens == nil &&
		u.ContextUsed == nil &&
		u.ContextSize == nil &&
		u.CostAmount == nil &&
		u.CostCurrency == nil
}

func chooseInt64(primary, fallback *int64) *int64 {
	if primary != nil {
		return primary
	}
	return fallback
}

func chooseFloat64(primary, fallback *float64) *float64 {
	if primary != nil {
		return primary
	}
	return fallback
}

func chooseString(primary, fallback *string) *string {
	if primary != nil {
		return primary
	}
	return fallback
}
