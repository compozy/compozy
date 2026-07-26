package settings

import (
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func hookDeclarationMap(declaration hookspkg.HookDecl) map[string]any {
	values := map[string]any{
		"event": string(declaration.Event),
	}
	if declaration.Mode != "" {
		values["mode"] = string(declaration.Mode)
	}
	if declaration.Required {
		values["required"] = declaration.Required
	}
	if declaration.Enabled != nil {
		values["enabled"] = *declaration.Enabled
	}
	if declaration.PrioritySet {
		values["priority"] = declaration.Priority
	}
	if declaration.Timeout > 0 {
		values["timeout"] = declaration.Timeout.String()
	}
	if matcher := hookMatcherMap(declaration); len(matcher) > 0 {
		values["matcher"] = matcher
	}
	if executor := hookExecutorMap(declaration); len(executor) > 0 {
		values["executor"] = executor
	} else {
		if strings.TrimSpace(declaration.Command) != "" {
			values["command"] = declaration.Command
		}
		if len(declaration.Args) > 0 {
			values["args"] = append([]string(nil), declaration.Args...)
		}
		if len(declaration.Env) > 0 {
			values[settingsCredentialSourceEnv] = cloneStringMap(declaration.Env)
		}
		if len(declaration.SecretEnv) > 0 {
			values["secret_env"] = cloneStringMap(declaration.SecretEnv)
		}
	}
	return values
}
