// Package cmdpalette owns the daemon-canonical command catalog and invocation contract.
package cmdpalette

import (
	"encoding/json"
	"time"
)

type WorkspaceID string
type ClientID string
type CommandID string

type SourceKind string

const (
	SourceKindCore        SourceKind = "core"
	SourceKindExtension   SourceKind = "extension"
	extensionSourcePrefix            = "ext."
)

type Source struct {
	Kind      SourceKind `json:"kind"`
	Extension string     `json:"extension,omitempty"`
}

func (s Source) ID() string {
	if s.Kind == SourceKindExtension {
		if s.Extension == "" {
			return string(SourceKindExtension)
		}
		return extensionSourcePrefix + s.Extension
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
	ActionKindCopy     ActionKind = "copy"
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
	ID                        CommandID       `json:"id"`
	Title                     string          `json:"title"`
	Section                   string          `json:"section"`
	Icon                      string          `json:"icon"`
	Keywords                  []string        `json:"keywords,omitempty"`
	Source                    Source          `json:"source"`
	Action                    Action          `json:"action"`
	Arguments                 []Argument      `json:"arguments"`
	Destructive               bool            `json:"destructive"`
	Confirmation              *Confirmation   `json:"confirmation,omitempty"`
	When                      []Predicate     `json:"when,omitempty"`
	AvailabilityExempt        bool            `json:"availability_exempt"`
	Policy                    ExecutionPolicy `json:"execution"`
	ProviderUnavailableReason string          `json:"-"`
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
	Available         bool            `json:"available"`
	UnavailableReason string          `json:"reason,omitempty"`
	Bindings          []string        `json:"bindings"`
	Alias             *string         `json:"alias"`
	GlobalShortcut    *GlobalShortcut `json:"global_shortcut,omitempty"`
}

// GlobalShortcut projects daemon-owned intent and one shell client's registration truth.
type GlobalShortcut struct {
	IntendedChord string `json:"intended_chord"`
	ActiveChord   string `json:"active_chord,omitempty"`
	Status        string `json:"status,omitempty"`
	Reason        string `json:"reason,omitempty"`
	SettingsURL   string `json:"settings_url,omitempty"`
}

type Catalog struct {
	Commands        []ResolvedCommand `json:"commands"`
	Sources         []SourceStatus    `json:"sources"`
	Revision        string            `json:"catalog_revision"`
	ContextRevision string            `json:"context_revision,omitempty"`
	ProfileLens     ProfileLens       `json:"profile_lens"`
}

// CatalogRequest identifies one complete command projection.
type CatalogRequest struct {
	ProfileLens ProfileLens
	WorkspaceID WorkspaceID
	ClientID    ClientID
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
	ProfileLens     ProfileLens    `json:"profile_lens"`
	WorkspaceID     WorkspaceID    `json:"workspace_id"`
	CommandID       CommandID      `json:"command_id"`
	Args            map[string]any `json:"args"`
	ClientID        ClientID       `json:"client,omitempty"`
	ClientToken     string         `json:"-"`
	Caller          CallerKind     `json:"-"`
	ManagementLocal bool           `json:"-"`
}

type InvokeStatus string

const (
	InvokeStatusOK              InvokeStatus = "ok"
	InvokeStatusApprovalPending InvokeStatus = "approval_pending"
)

type InvokeResult struct {
	ProfileLens  ProfileLens     `json:"profile_lens"`
	Status       InvokeStatus    `json:"status"`
	Result       json.RawMessage `json:"result,omitempty"`
	ApprovalID   string          `json:"approval_id,omitempty"`
	InvocationID string          `json:"invocation_id"`
}

type ExecutionRequest struct {
	ProfileLens  ProfileLens
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
	ProfileLens ProfileLens
	WorkspaceID WorkspaceID
	CommandID   CommandID
	Query       string
	UsedAt      time.Time
}

// Contribution is one atomic extension catalog snapshot.
type Contribution struct {
	Commands []Descriptor
	Sources  []SourceStatus
	Defaults []ExtensionDefaultShortcut
}

// ExtensionDefaultShortcut is one bind-if-free extension shortcut claim.
type ExtensionDefaultShortcut struct {
	CommandID CommandID
	Chord     string
	Source    string
	Active    bool
}
