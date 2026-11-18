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

// OptionalHeaderSecretInputs selects declared template inputs, ignoring literal and override headers.
func (s MCPServer) OptionalHeaderSecretInputs() map[string]struct{} {
	if len(s.InputValues) == 0 {
		return nil
	}
	names := mcppolicy.OptionalHeaderSecretNames(s.Headers)
	maps.DeleteFunc(names, func(name string, _ struct{}) bool {
		_, declared := s.InputValues[name]
		return !declared
	})
	return names
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
	render := func(value string, missingHeaderSecret *bool) string {
		return mcppolicy.ReplaceInputs(value, func(name, fallback string, optional bool) string {
			if value, ok := values[name]; ok {
				if optional && value == "" && s.InputValues[name].SecretRef == "" {
					if mcppolicy.SecretInputName(name) {
						if fallback != "" {
							secrets = append(secrets, fallback)
						} else if s.AgentPlugin && missingHeaderSecret != nil {
							*missingHeaderSecret = true
						}
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
	s.URL = render(s.URL, nil)
	s.Args = slices.Clone(s.Args)
	for i := range s.Args {
		s.Args[i] = render(s.Args[i], nil)
	}
	s.Env = maps.Clone(s.Env)
	for key, value := range s.Env {
		s.Env[key] = render(value, nil)
	}
	s.Headers = maps.Clone(s.Headers)
	for key, value := range s.Headers {
		missingSecret := false
		value = render(value, &missingSecret)
		if missingSecret {
			delete(s.Headers, key)
		} else {
			s.Headers[key] = value
		}
	}
	s.Auth.IssuerURL = render(s.Auth.IssuerURL, nil)
	return s, secrets, nil
}
