package runtimemcp

import (
	"cmp"
	"github.com/compozy/compozy/internal/mcppolicy"
	"net/url"
	"slices"
	"strings"
)

func copyBlocked(s Server, d object) string {
	ownedScopes := []string{"plugin", "connector", "builtin", "managed", "enterprise", "account", "dynamic"}
	if slices.Contains(ownedScopes, s.Scope) || strings.HasPrefix(s.Name, "plugin:") {
		return s.Name + " is provided by a runtime-owned source and can't be copied"
	}
	if s.Transport != "stdio" && s.Transport != "http" {
		return "CompozyOS runs stdio and HTTP servers; " + s.Name + " uses " + strings.ToUpper(s.Transport)
	}
	if reason := blockedRuntimeFields(d); reason != "" {
		return s.Name + " " + reason
	}
	if reason := blockedRemoteDefinition(d); reason != "" {
		return s.Name + " " + reason
	}
	if !representableCopyShape(s.Transport, d) {
		return s.Name + " has a definition CompozyOS can't represent"
	}
	if usesEnvironmentIndirection(d) {
		return s.Name + " uses environment indirection, which CompozyOS servers can't keep"
	}
	allowed := []string{
		"type", "command", "args", "env", "environment", "url", "httpUrl", "headers", "http_headers",
		"header", "oauth", "auth", "enabled", "disabled", "description", "name", "tools",
	}
	for _, key := range keys(d) {
		if !slices.Contains(allowed, key) {
			return s.Name + " uses runtime-specific settings (" + key + "), which CompozyOS servers can't keep"
		}
	}
	return ""
}

func blockedRuntimeFields(d object) string {
	checks := []struct {
		keys   []string
		reason string
	}{
		{[]string{"cwd"}, "runs in its own working directory (cwd), which CompozyOS servers can't keep"},
		{[]string{"headersHelper", "http_headers_helper"}, "uses a headers helper, which CompozyOS servers can't keep"},
		{
			[]string{"env_vars", "bearer_token_env_var", "bearerTokenEnvVar", "env_http_headers"},
			"uses environment passthrough, which CompozyOS servers can't keep",
		},
		{
			[]string{"enabled_tools", "disabled_tools", "enabledTools", "disabledTools", "includeTools", "excludeTools",
				"toolExposure", "omit_tools_from", "filterMapping"},
			"uses tool filters, which CompozyOS servers can't keep",
		},
	}
	for _, check := range checks {
		for _, key := range check.keys {
			if has(d, key) {
				return check.reason
			}
		}
	}
	if tools, ok := d["tools"]; ok && str(tools) != "*" {
		return "uses tool filters, which CompozyOS servers can't keep"
	}
	if str(d["exposure"]) == "hidden" {
		return "uses tool filters, which CompozyOS servers can't keep"
	}
	return ""
}

func blockedRemoteDefinition(d object) string {
	if endpoint := cmp.Or(str(d["url"]), str(d["httpUrl"])); endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return "has a URL CompozyOS can't represent"
		}
		if parsed.RawQuery != "" || parsed.ForceQuery || parsed.User != nil || parsed.Fragment != "" {
			return "uses a URL query or credentials, which CompozyOS servers can't keep"
		}
		if mcppolicy.ValidateRemoteURL(endpoint) != nil {
			return "has a URL CompozyOS can't represent"
		}
	}
	customAuth := has(d, "auth") && str(d["auth"]) != "oauth"
	customOAuth := has(d, "oauth") && (obj(d["oauth"]) == nil || len(obj(d["oauth"])) > 0)
	if customOAuth || customAuth || has(d, "authProviderType") || has(d, "authProvider") {
		return "uses a custom OAuth client, which CompozyOS servers can't keep"
	}
	return ""
}

func usesEnvironmentIndirection(d object) bool {
	for _, key := range []string{"env", "environment", "headers", "http_headers", "header"} {
		for _, value := range obj(d[key]) {
			text, ok := value.(string)
			if !ok || environmentReference(text) {
				return true
			}
		}
	}
	for _, key := range []string{"command", "args", "url", "httpUrl"} {
		if environmentReference(str(d[key])) {
			return true
		}
		for _, value := range names(d[key]) {
			if environmentReference(value) {
				return true
			}
		}
	}
	return false
}

func environmentReference(text string) bool {
	return strings.Contains(text, "${") || strings.Contains(text, "{env:") || strings.HasPrefix(text, "$")
}
