package runtimemcp

import (
	"cmp"
	"net/url"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/mcppolicy"
)

func copyBlocked(s Server, d object) string {
	ownedScopes := []string{scopePlugin, "connector", "builtin", scopeManaged, "enterprise", "account", "dynamic"}
	if slices.Contains(ownedScopes, s.Scope) || strings.HasPrefix(s.Name, "plugin:") {
		return s.Name + " is provided by a runtime-owned source and can't be copied"
	}
	if s.Transport != transportStdio && s.Transport != transportHTTP {
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
		fieldType,
		fieldCommand,
		fieldArgs,
		fieldEnv,
		fieldEnvironment,
		fieldURL,
		fieldHTTPURL,
		fieldHeaders,
		fieldHTTPHeaders,
		fieldHeader,
		authOAuth,
		fieldAuth,
		"enabled",
		"disabled",
		"description",
		"name",
		"tools",
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
			toolFiltersCopyReason,
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
		return toolFiltersCopyReason
	}
	if str(d["exposure"]) == "hidden" {
		return toolFiltersCopyReason
	}
	return ""
}

func blockedRemoteDefinition(d object) string {
	if endpoint := cmp.Or(str(d[fieldURL]), str(d[fieldHTTPURL])); endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != transportHTTP) || parsed.Host == "" {
			return "has a URL CompozyOS can't represent"
		}
		if parsed.RawQuery != "" || parsed.ForceQuery || parsed.User != nil || parsed.Fragment != "" {
			return "uses a URL query or credentials, which CompozyOS servers can't keep"
		}
		if mcppolicy.ValidateRemoteURL(endpoint) != nil {
			return "has a URL CompozyOS can't represent"
		}
	}
	customAuth := has(d, fieldAuth) && str(d[fieldAuth]) != authOAuth
	customOAuth := has(d, authOAuth) && (obj(d[authOAuth]) == nil || len(obj(d[authOAuth])) > 0)
	if customOAuth || customAuth || has(d, "authProviderType") || has(d, "authProvider") {
		return "uses a custom OAuth client, which CompozyOS servers can't keep"
	}
	return ""
}

func usesEnvironmentIndirection(d object) bool {
	for _, key := range []string{fieldEnv, fieldEnvironment, fieldHeaders, fieldHTTPHeaders, fieldHeader} {
		for _, value := range obj(d[key]) {
			text, ok := value.(string)
			if !ok || environmentReference(text) {
				return true
			}
		}
	}
	for _, key := range []string{fieldCommand, fieldArgs, fieldURL, fieldHTTPURL} {
		if environmentReference(str(d[key])) {
			return true
		}
		if slices.ContainsFunc(names(d[key]), environmentReference) {
			return true
		}
	}
	return false
}

func environmentReference(text string) bool {
	return strings.Contains(text, "${") || strings.Contains(text, "{env:") || strings.HasPrefix(text, "$")
}
