package cli

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/config"
)

func parseSessionEnvConfigValue(kind configSetValueKind, raw string) (any, error) {
	value := any(raw)
	policyKind := config.ConfigValueSessionEnvBinding
	if kind == configSetSessionEnv || strings.HasPrefix(strings.TrimSpace(raw), "{") {
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, errors.New("cli: session_env requires a valid JSON object of reference declarations")
		}
	}
	if kind == configSetSessionEnv {
		policyKind = config.ConfigValueSessionEnv
	}
	return config.NormalizeToolConfigValue(policyKind, value)
}
