package daytona

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/compozy/agh/internal/sandbox"
)

const (
	providerStateVersion = 1
	defaultAPIURL        = "https://app.daytona.io/api"
	defaultSSHHost       = "ssh.app.daytona.io"
	defaultRuntimeRoot   = "/home/daytona/workspace"
)

type providerState struct {
	Version               int                     `json:"version"`
	SandboxID             string                  `json:"sandbox_id"`
	SandboxName           string                  `json:"sandbox_name,omitempty"`
	APIURL                string                  `json:"api_url,omitempty"`
	SSHHost               string                  `json:"ssh_host,omitempty"`
	LocalRootDir          string                  `json:"local_root_dir"`
	LocalAdditionalDirs   []string                `json:"local_additional_dirs,omitempty"`
	RuntimeRootDir        string                  `json:"runtime_root_dir"`
	RuntimeAdditionalDirs []string                `json:"runtime_additional_dirs,omitempty"`
	Persistence           sandbox.PersistenceMode `json:"persistence"`
	StartupSource         sandbox.DaytonaStartupSource
	StartupRef            string     `json:"startup_ref,omitempty"`
	SSHAccessExpiresAt    *time.Time `json:"ssh_access_expires_at,omitempty"`
	PreparedAt            time.Time  `json:"prepared_at"`
}

func decodeProviderState(raw json.RawMessage) (providerState, error) {
	if len(raw) == 0 {
		return providerState{}, nil
	}
	var state providerState
	if err := json.Unmarshal(raw, &state); err != nil {
		return providerState{}, fmt.Errorf("sandbox/daytona: decode provider state: %w", err)
	}
	return state, nil
}

func encodeProviderState(state providerState) (json.RawMessage, error) {
	state.Version = providerStateVersion
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: encode provider state: %w", err)
	}
	return json.RawMessage(raw), nil
}

func normalizeAPIURL(apiURL string) string {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return defaultAPIURL
	}
	return strings.TrimRight(apiURL, "/")
}

func normalizeSSHHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return defaultSSHHost
	}
	return host
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	maps.Copy(cloned, values)
	return cloned
}
