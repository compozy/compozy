package daemon

import (
	"errors"

	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

type configReadInput struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type configGetInput struct {
	Path          string `json:"path"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type configSetInput struct {
	Path          string `json:"path"`
	Value         any    `json:"value"`
	Scope         string `json:"scope,omitempty"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type configUnsetInput struct {
	Path          string `json:"path"`
	Scope         string `json:"scope,omitempty"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type configPathInput struct {
	Scope         string `json:"scope,omitempty"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type hooksListInput struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	Agent         string `json:"agent,omitempty"`
	Event         string `json:"event,omitempty"`
	Source        string `json:"source,omitempty"`
	Mode          string `json:"mode,omitempty"`
}

type hooksInfoInput struct {
	hooksListInput
	Name string `json:"name"`
}

type hooksEventsInput struct {
	Family   string `json:"family,omitempty"`
	SyncOnly bool   `json:"sync_only,omitempty"`
}

type hooksRunsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Event       string `json:"event,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
	Since       string `json:"since,omitempty"`
	Last        int    `json:"last,omitempty"`
}

func (i hooksRunsInput) query() (store.HookRunQuery, error) {
	query := store.HookRunQuery{
		SessionID: strings.TrimSpace(i.SessionID),
		Event:     strings.TrimSpace(i.Event),
		Limit:     i.Last,
	}
	if strings.TrimSpace(i.Since) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(i.Since))
		if err != nil {
			return store.HookRunQuery{}, err
		}
		query.Since = parsed
	}
	if outcome := strings.TrimSpace(i.Outcome); outcome != "" {
		query.Outcome = hookspkg.HookRunOutcome(outcome)
		if err := query.Outcome.Validate(); err != nil {
			return store.HookRunQuery{}, err
		}
	}
	if query.Event != "" {
		if err := hookspkg.HookEvent(query.Event).Validate(); err != nil {
			return store.HookRunQuery{}, err
		}
	}
	if err := query.Validate(); err != nil {
		return store.HookRunQuery{}, err
	}
	return query, nil
}

type hookMutationInput struct {
	Name          string                `json:"name"`
	Scope         string                `json:"scope,omitempty"`
	WorkspaceRoot string                `json:"workspace_root,omitempty"`
	Event         *string               `json:"event,omitempty"`
	Mode          *string               `json:"mode,omitempty"`
	Required      *bool                 `json:"required,omitempty"`
	Priority      *int                  `json:"priority,omitempty"`
	Timeout       *string               `json:"timeout,omitempty"`
	Matcher       *hookspkg.HookMatcher `json:"matcher,omitempty"`
	Command       *string               `json:"command,omitempty"`
	Args          *[]string             `json:"args,omitempty"`
	Env           *map[string]string    `json:"env,omitempty"`
	SecretEnv     *map[string]string    `json:"secret_env,omitempty"`
	Enabled       *bool                 `json:"enabled,omitempty"`
	Source        *string               `json:"source,omitempty"`
}

func (i hookMutationInput) newDecl() (hookspkg.HookDecl, error) {
	decl := hookspkg.HookDecl{
		Name:   strings.TrimSpace(i.Name),
		Source: hookspkg.HookSourceConfig,
	}
	return i.apply(decl)
}

func (i hookMutationInput) apply(decl hookspkg.HookDecl) (hookspkg.HookDecl, error) {
	decl.Name = strings.TrimSpace(firstNonEmpty(i.Name, decl.Name))
	decl.Source = hookspkg.HookSourceConfig
	if i.Event != nil {
		decl.Event = hookspkg.HookEvent(strings.TrimSpace(*i.Event))
	}
	if i.Mode != nil {
		decl.Mode = hookspkg.HookMode(strings.TrimSpace(*i.Mode))
	}
	if i.Required != nil {
		decl.Required = *i.Required
	}
	if i.Priority != nil {
		priority, err := hookspkg.PriorityFromInt(*i.Priority)
		if err != nil {
			return hookspkg.HookDecl{}, err
		}
		decl.Priority = priority
		decl.PrioritySet = true
	}
	if i.Timeout != nil {
		timeout, err := time.ParseDuration(strings.TrimSpace(*i.Timeout))
		if err != nil {
			return hookspkg.HookDecl{}, err
		}
		decl.Timeout = timeout
	}
	if i.Matcher != nil {
		matcher := cloneHookMatcher(*i.Matcher)
		if hookMatcherHasUnsupportedConfigFields(matcher) {
			return hookspkg.HookDecl{}, errors.New("hook matcher contains fields not supported by config overlays")
		}
		decl.Matcher = matcher
	}
	if i.Command != nil {
		decl.Command = strings.TrimSpace(*i.Command)
	}
	if i.Args != nil {
		decl.Args = append([]string(nil), (*i.Args)...)
	}
	if i.Env != nil {
		decl.Env = cloneStringMap(*i.Env)
	}
	if i.SecretEnv != nil {
		decl.SecretEnv = cloneStringMap(*i.SecretEnv)
	}
	if i.Enabled != nil {
		decl.Enabled = new(*i.Enabled)
	}
	return decl, nil
}

func hookMatcherHasUnsupportedConfigFields(matcher hookspkg.HookMatcher) bool {
	return strings.TrimSpace(matcher.SandboxID) != "" ||
		strings.TrimSpace(matcher.SandboxBackend) != "" ||
		strings.TrimSpace(matcher.SandboxProfile) != "" ||
		strings.TrimSpace(matcher.SyncDirection) != "" ||
		matcher.Autonomy != nil
}

type hookNameMutationInput struct {
	Name          string `json:"name"`
	Scope         string `json:"scope,omitempty"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}
