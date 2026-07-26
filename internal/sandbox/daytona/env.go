package daytona

import (
	"fmt"
	"sort"
	"strings"
)

const (
	envDaytonaAPIKeyValue = "DAYTONA_API_KEY"
)

var blockedRemoteEnv = map[string]struct{}{
	envDaytonaAPIKeyValue: {},
	"DAYTONA_JWT_TOKEN":   {},
}

func remoteEnvMap(agentEnv []string, profileEnv map[string]string) map[string]string {
	env := make(map[string]string)
	for _, entry := range agentEnv {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || isBlockedRemoteEnv(key) {
			continue
		}
		if strings.HasPrefix(key, "AGH_") {
			env[key] = value
		}
	}
	for key, value := range profileEnv {
		key = strings.TrimSpace(key)
		if key == "" || isBlockedRemoteEnv(key) {
			continue
		}
		env[key] = value
	}
	return env
}

func remoteEnvList(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, fmt.Sprintf("%s=%s", key, env[key]))
	}
	return values
}

func isBlockedRemoteEnv(key string) bool {
	_, blocked := blockedRemoteEnv[key]
	return blocked
}
