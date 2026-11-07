package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/compozy/compozy/internal/mcppolicy"
)

// MCPInputValue retains user data separately from package templates and secret plaintext.
type MCPInputValue struct {
	Value     string `json:"value,omitempty"`
	SecretRef string `json:"secret_ref,omitempty"`
}

// RenderInputs returns a private launch copy; the published spec retains only vault references.
func (s MCPServer) RenderInputs(resolve func(string) (string, error)) (MCPServer, []string, error) {
	if len(s.InputValues) == 0 {
		return s, nil, nil
	}
	values := make(map[string]string, len(s.InputValues))
	var secrets []string
	for _, name := range slices.Sorted(maps.Keys(s.InputValues)) {
		input := s.InputValues[name]
		value := input.Value
		if input.SecretRef != "" {
			var err error
			value, err = resolve(input.SecretRef)
			if err != nil {
				return MCPServer{}, nil, fmt.Errorf("mcp: resolve input %q", name)
			}
			secrets = append(secrets, value)
		}
		values[name] = value
	}
	render := func(value string) string {
		return mcppolicy.ReplaceInputs(value, func(name, fallback string, optional bool) string {
			if value, ok := values[name]; ok {
				if optional && value == "" && s.InputValues[name].SecretRef == "" {
					if mcppolicy.SecretInputName(name) {
						secrets = append(secrets, fallback)
					}
					return fallback
				}
				return value
			}
			if optional {
				return fallback
			}
			return "${" + name + "}"
		})
	}
	s.URL = render(s.URL)
	s.Args = slices.Clone(s.Args)
	for i := range s.Args {
		s.Args[i] = render(s.Args[i])
	}
	s.Env = maps.Clone(s.Env)
	for key, value := range s.Env {
		s.Env[key] = render(value)
	}
	s.Headers = maps.Clone(s.Headers)
	for key, value := range s.Headers {
		s.Headers[key] = render(value)
	}
	s.Auth.IssuerURL = render(s.Auth.IssuerURL)
	return s, secrets, nil
}
