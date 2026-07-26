package session

import (
	"encoding/json"

	"fmt"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/fileutil"
)

func piCredentialEnv(
	slots []aghconfig.ProviderCredentialSlot,
	injectedTargetEnvs map[string]struct{},
) string {
	for _, slot := range slots {
		targetEnv := strings.TrimSpace(slot.TargetEnv)
		if strings.TrimSpace(slot.Kind) == providerRuntimeAPIKeyKey &&
			injectedProviderTargetEnv(targetEnv, injectedTargetEnvs) {
			return targetEnv
		}
	}
	for _, slot := range slots {
		targetEnv := strings.TrimSpace(slot.TargetEnv)
		if injectedProviderTargetEnv(targetEnv, injectedTargetEnvs) {
			return targetEnv
		}
	}
	return ""
}

func injectedProviderTargetEnv(targetEnv string, injectedTargetEnvs map[string]struct{}) bool {
	targetEnv = strings.TrimSpace(targetEnv)
	if targetEnv == "" || len(injectedTargetEnvs) == 0 {
		return false
	}
	_, ok := injectedTargetEnvs[targetEnv]
	return ok
}

func runProviderSecretRedactions(cleanups []func()) {
	for _, cleanup := range cleanups {
		if cleanup != nil {
			cleanup()
		}
	}
}

func writeProviderJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("session: marshal provider runtime file %q: %w", path, err)
	}
	payload = append(payload, '\n')
	if err := fileutil.AtomicWriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("session: write provider runtime file %q: %w", path, err)
	}
	return nil
}
