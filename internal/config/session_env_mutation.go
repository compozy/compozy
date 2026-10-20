package config

import "errors"

func sessionEnvMutationKind(path []string) (ValueKind, bool) {
	if len(path) < 3 || path[0] != providersConfigKey || path[2] != "session_env" {
		return 0, false
	}
	switch len(path) {
	case 3:
		return ConfigValueSessionEnv, true
	case 4:
		if err := (SessionEnv{path[3]: {Ref: "env:VALID"}}).Validate("session_env"); err == nil {
			return ConfigValueSessionEnvBinding, true
		}
	}
	return 0, false
}

func normalizeSessionEnvMutation(kind ValueKind, value any) (any, error) {
	if kind == ConfigValueSessionEnvBinding {
		var binding SessionEnvBinding
		if err := binding.decode(value); err != nil {
			return nil, err
		}
		return map[string]any{sessionEnvRefField: binding.Ref, sessionEnvRequiredField: binding.Required}, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("session_env: expected an object of reference declarations")
	}
	declarations := make(SessionEnv, len(table))
	result := make(map[string]any, len(table))
	for name, raw := range table {
		var binding SessionEnvBinding
		if err := binding.decode(raw); err != nil {
			return nil, err
		}
		declarations[name] = binding
		result[name] = map[string]any{sessionEnvRefField: binding.Ref, sessionEnvRequiredField: binding.Required}
	}
	if err := declarations.Validate("session_env"); err != nil {
		return nil, err
	}
	return result, nil
}
