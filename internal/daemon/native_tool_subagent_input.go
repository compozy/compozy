package daemon

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
)

type nativeSubagentIDInput struct {
	SubagentID string `json:"subagent_id"`
}
type nativeSubagentCancelInput struct {
	SubagentID string `json:"subagent_id"`
	Reason     string `json:"reason"`
}

type nativeSubagentTargetInput struct {
	Agent           string                             `json:"agent"`
	Provider        string                             `json:"provider"`
	Model           string                             `json:"model"`
	ReasoningEffort string                             `json:"reasoning_effort"`
	Speed           string                             `json:"speed"`
	ACPOptions      []contract.AgentACPOptionSelection `json:"acp_options"`
}

type nativeSubagentDelegateInput struct {
	Task           string                    `json:"task"`
	Title          string                    `json:"title"`
	Role           string                    `json:"role"`
	Target         nativeSubagentTargetInput `json:"target"`
	Mode           string                    `json:"mode"`
	TimeoutMS      *int64                    `json:"timeout_ms"`
	IdempotencyKey string                    `json:"idempotency_key"`
	PermissionMode string                    `json:"permission_mode"`
	Tools          []string                  `json:"tools"`
	Skills         []string                  `json:"skills"`
	MCPServers     []string                  `json:"mcp_servers"`
	WorkspacePaths []string                  `json:"workspace_paths"`
}

func (in nativeSubagentDelegateInput) request(caller session.SubagentCaller) (session.SubagentRequest, error) {
	if err := in.validate(); err != nil {
		return session.SubagentRequest{}, err
	}
	title := in.Title
	if title == "" {
		title, _, _ = strings.Cut(in.Task, "\n")
		title = string([]rune(title)[:min(512, utf8.RuneCountInString(title))])
	}
	timeout := int64(600000)
	if in.TimeoutMS != nil {
		timeout = max(1000, min(3600000, *in.TimeoutMS))
	}
	mode := config.PermissionMode(in.PermissionMode)
	if mode == "inherit" {
		mode = ""
	}
	var narrowing *session.SubagentPermissionNarrowing
	if in.Tools != nil || in.Skills != nil || in.MCPServers != nil || in.WorkspacePaths != nil {
		narrowing = &session.SubagentPermissionNarrowing{
			Tools:          in.Tools,
			Skills:         in.Skills,
			MCPServers:     in.MCPServers,
			WorkspacePaths: in.WorkspacePaths,
		}
	}
	return session.SubagentRequest{
		Caller: caller, Task: in.Task, Title: title, Role: cmp.Or(in.Role, "general"),
		Mode: cmp.Or(in.Mode, session.SubagentModeAsync), Timeout: time.Duration(timeout) * time.Millisecond,
		IdempotencyKey: cmp.Or(in.IdempotencyKey, caller.ToolCallID), PermissionMode: mode, Narrowing: narrowing,
		Target: session.SubagentTarget{
			Agent:           in.Target.Agent,
			Provider:        in.Target.Provider,
			Model:           in.Target.Model,
			ReasoningEffort: in.Target.ReasoningEffort,
			Speed:           in.Target.Speed,
			ACPOptions:      contract.ACPOptionSelectionsFromPayload(in.Target.ACPOptions),
		},
	}, nil
}

func (in nativeSubagentDelegateInput) validate() error {
	invalid := func(message string) error {
		return &session.SubagentError{Code: "invalid_request", Message: message, Err: session.ErrSubagentInvalidRequest}
	}
	if strings.TrimSpace(in.Task) == "" {
		return invalid("task is required.")
	}
	if utf8.RuneCountInString(in.Task) > 120000 {
		return invalid("task exceeds 120000 characters.")
	}
	if utf8.RuneCountInString(in.Title) > 512 {
		return invalid("title exceeds 512 characters.")
	}
	if utf8.RuneCountInString(in.IdempotencyKey) > 256 {
		return invalid("idempotency_key exceeds 256 characters.")
	}
	if !slices.Contains([]string{"", "general", "implementation", "research", "review", "design", "test"}, in.Role) {
		return invalid("role must be general, implementation, research, review, design, or test.")
	}
	if !slices.Contains([]string{"", "async", "wait"}, in.Mode) {
		return invalid("mode must be async or wait.")
	}
	if !slices.Contains([]string{"", "inherit", "deny-all", "approve-reads", "approve-all"}, in.PermissionMode) {
		return invalid("permission_mode must be inherit, deny-all, approve-reads, or approve-all.")
	}
	if !slices.Contains([]string{"", "normal", "fast"}, in.Target.Speed) {
		return invalid("target.speed must be normal or fast.")
	}
	return nil
}
