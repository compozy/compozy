// Package cmdpalette owns the daemon-canonical command catalog and invocation contract.
package cmdpalette

import (
	"context"
	"encoding/json"
	"time"
)

type WorkspaceID string
type ClientID string
type CommandID string

type SourceKind string

const (
	SourceKindCore      SourceKind = "core"
	SourceKindExtension SourceKind = "extension"
)

type Source struct {
	Kind      SourceKind `json:"kind"`
	Extension string     `json:"extension,omitempty"`
}

func (s Source) ID() string {
	if s.Kind == SourceKindExtension {
		return "ext." + s.Extension
	}
	return string(s.Kind)
}

type ActionKind string

const (
	ActionKindClientOp ActionKind = "client_op"
	ActionKindTool     ActionKind = "tool"
	ActionKindView     ActionKind = "view"
	ActionKindNavigate ActionKind = "navigate"
	ActionKindURL      ActionKind = "url"
)

type Action struct {
	Kind ActionKind     `json:"kind"`
	Op   string         `json:"op,omitempty"`
	Tool string         `json:"tool,omitempty"`
	View string         `json:"view,omitempty"`
	App  string         `json:"app,omitempty"`
	URL  string         `json:"url,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

type ArgumentType string

const (
	ArgumentTypeText     ArgumentType = "text"
	ArgumentTypePassword ArgumentType = "password"
	ArgumentTypeDropdown ArgumentType = "dropdown"
	ArgumentTypeCheckbox ArgumentType = "checkbox"
)

type Argument struct {
	Name        string       `json:"name"`
	Type        ArgumentType `json:"type"`
	Placeholder string       `json:"placeholder,omitempty"`
	Required    bool         `json:"required"`
	Options     []string     `json:"options,omitempty"`
}

type Confirmation struct {
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
	Confirm string `json:"confirm"`
}

type ExecutionPolicy struct {
	SingleFlight bool `json:"single_flight"`
	RetrySafe    bool `json:"retry_safe"`
}

type ContextKey string

const (
	ContextWindowFocused      ContextKey = "window.focused"
	ContextWindowFloating     ContextKey = "window.floating"
	ContextWindowStacked      ContextKey = "window.stacked"
	ContextDesktopWindowCount ContextKey = "desktop.windowCount"
	ContextScopeGlobal        ContextKey = "scope.global"
	ContextShellDesktop       ContextKey = "shell.desktop"
	ContextSessionState       ContextKey = "session.focused.state"
	ContextWorkspaceTrusted   ContextKey = "workspace.trusted"
)

type PredicateOperator string

const (
	PredicateEquals             PredicateOperator = "equals"
	PredicateNotEquals          PredicateOperator = "not_equals"
	PredicateGreaterThanOrEqual PredicateOperator = "greater_than_or_equal"
)

type Predicate struct {
	Key      ContextKey        `json:"key"`
	Operator PredicateOperator `json:"operator,omitempty"`
	Value    any               `json:"value"`
	Reason   string            `json:"reason,omitempty"`
}

type Descriptor struct {
	ID                 CommandID       `json:"id"`
	Title              string          `json:"title"`
	Section            string          `json:"section"`
	Icon               string          `json:"icon"`
	Keywords           []string        `json:"keywords,omitempty"`
	Source             Source          `json:"source"`
	Action             Action          `json:"action"`
	Arguments          []Argument      `json:"arguments"`
	Destructive        bool            `json:"destructive"`
	Confirmation       *Confirmation   `json:"confirmation,omitempty"`
	When               []Predicate     `json:"when,omitempty"`
	AvailabilityExempt bool            `json:"availability_exempt"`
	Policy             ExecutionPolicy `json:"execution"`
}

type SourceHealth string

const (
	SourceHealthy   SourceHealth = "healthy"
	SourceDegraded  SourceHealth = "degraded"
	SourceUnhealthy SourceHealth = "unhealthy"
	SourceDisabled  SourceHealth = "disabled"
)

type SourceStatus struct {
	Source string       `json:"source"`
	Status SourceHealth `json:"status"`
	Reason string       `json:"reason,omitempty"`
}

type ResolvedCommand struct {
	Descriptor
	Available         bool     `json:"available"`
	UnavailableReason string   `json:"reason,omitempty"`
	Bindings          []string `json:"bindings"`
	Alias             *string  `json:"alias"`
}

type Catalog struct {
	Commands        []ResolvedCommand `json:"commands"`
	Sources         []SourceStatus    `json:"sources"`
	Revision        string            `json:"catalog_revision"`
	ContextRevision string            `json:"context_revision,omitempty"`
}

type Client struct {
	ID              ClientID    `json:"client_id"`
	Kind            string      `json:"kind"`
	WorkspaceID     WorkspaceID `json:"workspace"`
	AttachedAt      time.Time   `json:"attached_at"`
	ContextRevision string      `json:"context_revision"`
}

type ContextSnapshot struct {
	Revision string
	Values   map[ContextKey]any
}

type CallerKind string

const (
	CallerControlPlane   CallerKind = "control_plane"
	CallerAttachedClient CallerKind = "attached_client"
)

type InvokeRequest struct {
	WorkspaceID WorkspaceID    `json:"workspace_id"`
	CommandID   CommandID      `json:"command_id"`
	Args        map[string]any `json:"args"`
	ClientID    ClientID       `json:"client,omitempty"`
	ClientToken string         `json:"-"`
	Caller      CallerKind     `json:"-"`
}

type InvokeStatus string

const (
	InvokeStatusOK              InvokeStatus = "ok"
	InvokeStatusApprovalPending InvokeStatus = "approval_pending"
)

type InvokeResult struct {
	Status       InvokeStatus    `json:"status"`
	Result       json.RawMessage `json:"result,omitempty"`
	ApprovalID   string          `json:"approval_id,omitempty"`
	InvocationID string          `json:"invocation_id"`
}

type ExecutionRequest struct {
	WorkspaceID  WorkspaceID
	InvocationID string
	ClientID     ClientID
	Descriptor   Descriptor
	Args         map[string]any
}

type ExecutionResult struct {
	Result     json.RawMessage
	ApprovalID string
	Completion <-chan struct{}
}

type Usage struct {
	WorkspaceID WorkspaceID
	CommandID   CommandID
	Query       string
	UsedAt      time.Time
}

type Provider interface {
	ProvideCommands(context.Context, WorkspaceID) ([]Descriptor, error)
}

type StaticProvider interface {
	Provider
	StaticCommands() []Descriptor
}

type ProviderRegistration struct {
	Source   Source
	Provider Provider
}

type ActionExecutor interface {
	ExecuteAction(context.Context, ExecutionRequest) (ExecutionResult, error)
}

type ApprovalPreflight interface {
	ApprovalRequired(context.Context, ExecutionRequest) (bool, error)
}

type ClientDirectory interface {
	Clients(context.Context, WorkspaceID) ([]Client, error)
	Context(context.Context, WorkspaceID, ClientID) (ContextSnapshot, error)
	Authorize(context.Context, WorkspaceID, ClientID, string) error
}

type BindingsResolver interface {
	Bindings(context.Context, WorkspaceID) (map[CommandID][]string, map[CommandID]string, error)
}

type Registry interface {
	Catalog(context.Context, WorkspaceID, ClientID) (Catalog, error)
	Clients(context.Context, WorkspaceID) ([]Client, error)
	Invoke(context.Context, InvokeRequest) (InvokeResult, error)
	RecordUsage(context.Context, Usage) error
	Personalization(context.Context, WorkspaceID) (Snapshot, error)
	PersonalizationSummary(context.Context, WorkspaceID) (PersonalizationSummary, error)
	ResetPersonalization(context.Context, WorkspaceID) error
	Pin(context.Context, WorkspaceID, CommandID) error
	Unpin(context.Context, WorkspaceID, CommandID) error
}
