package providers

import (
	pathpkg "path"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
)

// ProviderLoginPresence is the safe tri-state availability of a configured login executable.
type ProviderLoginPresence string

const (
	ProviderLoginPresencePresent ProviderLoginPresence = "present"
	ProviderLoginPresenceMissing ProviderLoginPresence = "missing"
	ProviderLoginPresenceUnknown ProviderLoginPresence = "unknown"
)

// ProviderLoginDescriptor is the only public-safe projection of auth_login_command.
type ProviderLoginDescriptor struct {
	Configured        bool                  `json:"configured"`
	Source            string                `json:"source,omitempty"`
	Executable        string                `json:"executable,omitempty"`
	Presence          ProviderLoginPresence `json:"presence"`
	RecommendedAction ProviderFailureAction `json:"recommended_action,omitempty"`
}

// DescribeProviderLogin derives safe login metadata without retaining the command line.
func DescribeProviderLogin(
	provider compozyconfig.ProviderConfig,
	nativeCLI *providerauth.NativeCLIStatus,
	classification Classification,
) ProviderLoginDescriptor {
	command := strings.TrimSpace(provider.AuthLoginCmd)
	descriptor := ProviderLoginDescriptor{
		Configured:        command != "",
		Presence:          ProviderLoginPresenceUnknown,
		RecommendedAction: actionForClassification(classification),
	}
	if command == "" {
		return descriptor
	}
	descriptor.Source = providerauth.NativeCLISourceAuthLogin
	parsed, err := providerauth.ParseCommand(command)
	if err != nil {
		if descriptor.RecommendedAction == ProviderFailureActionNone &&
			classification.State != ProviderAuthStateAuthenticated &&
			classification.State != ProviderAuthStateNone {
			descriptor.RecommendedAction = ProviderFailureActionInspect
		}
		return descriptor
	}
	descriptor.Executable = providerLoginExecutableBase(parsed.Executable)
	if nativeCLI == nil || providerLoginExecutableBase(nativeCLI.Command) != descriptor.Executable {
		return descriptor
	}
	if strings.TrimSpace(nativeCLI.Error) != "" {
		return descriptor
	}
	if nativeCLI.Present {
		descriptor.Presence = ProviderLoginPresencePresent
	} else {
		descriptor.Presence = ProviderLoginPresenceMissing
	}
	return descriptor
}

func providerLoginExecutableBase(executable string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(executable), `\`, "/")
	if normalized == "" {
		return ""
	}
	base := pathpkg.Base(normalized)
	if base == "." || base == "/" {
		return ""
	}
	return base
}
