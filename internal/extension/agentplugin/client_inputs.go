package agentplugin

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/compozy/compozy/internal/mcppolicy"
)

const ClientPlaceholderInvalid = "client_placeholder_invalid"

func deriveClientInputs(pkg *Package, server ServerSpec) {
	scan := func(value string) {
		mcppolicy.ReplaceInputs(value, func(name, fallback string, optional bool) string {
			if name == pluginRootVariable || name == pluginDataVariable {
				return ""
			}
			if !mcppolicy.ValidInputName(name) {
				pkg.Diagnostics = append(
					pkg.Diagnostics,
					Diagnostic{
						Code:    ClientPlaceholderInvalid,
						Scope:   "mcp:" + server.Name,
						Message: "placeholder name must match [A-Z_][A-Z0-9_]*",
					},
				)
				return ""
			}
			index := slices.IndexFunc(pkg.Inputs, func(input DerivedInput) bool { return input.ID == name })
			if index < 0 {
				pkg.Inputs = append(
					pkg.Inputs,
					DerivedInput{
						ID:       name,
						Secret:   mcppolicy.SecretInputName(name),
						Required: !optional,
						Default:  fallback,
					},
				)
				index = len(pkg.Inputs) - 1
			}
			input := &pkg.Inputs[index]
			input.Required = input.Required || !optional
			if input.Default != fallback {
				input.Default = ""
			}
			if !slices.Contains(input.Servers, server.Name) {
				input.Servers = append(input.Servers, server.Name)
			}
			return ""
		})
	}
	scan(server.URL)
	for _, values := range []map[string]string{server.Headers, server.Env} {
		for _, key := range slices.Sorted(maps.Keys(values)) {
			scan(values[key])
		}
	}
	for _, value := range server.Args {
		scan(value)
	}
	if server.Auth != nil {
		scan(server.Auth.ClientID)
		scan(server.Auth.ClientSecret)
	}
	slices.SortFunc(pkg.Inputs, func(a, b DerivedInput) int {
		return cmp.Compare(a.ID, b.ID)
	})
}

func decodeClientAuth(name string, raw json.RawMessage, transport string) (*OAuthConfig, []Diagnostic) {
	if len(raw) == 0 {
		return nil, nil
	}
	var fields map[string]json.RawMessage
	var diagnostics []Diagnostic
	ignored := func(field string) {
		diagnostics = append(
			diagnostics,
			Diagnostic{
				Code:    ClientFieldIgnored,
				Scope:   "mcp:" + name,
				Message: fmt.Sprintf("%q is not supported and was ignored", field),
			},
		)
	}
	if transport != transportStreamableHTTP || json.Unmarshal(raw, &fields) != nil || fields == nil {
		ignored("auth")
		return nil, diagnostics
	}
	auth := &OAuthConfig{}
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		switch key {
		case "CLIENT_ID":
			value, ok := stringField(fields, key, true)
			if ok && value != "" && (!containsPlaceholder(value) || validClientInput(value, false)) {
				auth.ClientID = value
			} else {
				ignored("auth." + key)
			}
		case "CLIENT_SECRET":
			value, ok := stringField(fields, key, true)
			if ok && validClientInput(value, true) {
				auth.ClientSecret = value
			} else {
				ignored("auth." + key)
			}
		case "scopes":
			scopes, ok := stringSliceField(fields, key)
			if ok {
				auth.Scopes = scopes
			} else {
				ignored("auth." + key)
			}
		default:
			ignored("auth." + key)
		}
	}
	if auth.ClientID == "" && auth.ClientSecret != "" {
		auth.ClientSecret = ""
		ignored("auth.CLIENT_SECRET")
	}
	return auth, diagnostics
}

func containsPlaceholder(value string) bool {
	found := false
	mcppolicy.ReplaceInputs(value, func(_, _ string, _ bool) string { found = true; return "" })
	return found
}

func validClientInput(value string, secret bool) bool {
	valid := false
	mcppolicy.ReplaceInputs(value, func(name, _ string, optional bool) string {
		valid = value == "${"+name+"}" && mcppolicy.ValidInputName(name) && mcppolicy.SecretInputName(name) == secret &&
			!optional
		return ""
	})
	return valid
}
