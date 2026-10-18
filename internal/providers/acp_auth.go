package providers

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// HasNativeLoginCommand reports whether a native login command owns the login
// operation for one provider.
func HasNativeLoginCommand(provider compozyconfig.ProviderConfig) bool {
	return strings.TrimSpace(provider.AuthLoginCmd) != ""
}

// HasNativeStatusCommand reports whether a native status command owns the
// probe operation for one provider.
func HasNativeStatusCommand(provider compozyconfig.ProviderConfig) bool {
	return strings.TrimSpace(provider.AuthStatusCmd) != ""
}

// HasACPMethod reports whether a protocol-owned auth method is configured for
// one provider. Native commands keep per-operation precedence; this only
// selects the fallback path when the respective native command is absent.
func HasACPMethod(provider compozyconfig.ProviderConfig) bool {
	return strings.TrimSpace(provider.AuthACPMethod) != ""
}

// UsesACPLogin reports whether the login operation runs through the
// configured protocol method instead of a native login command.
func UsesACPLogin(provider compozyconfig.ProviderConfig) bool {
	return !HasNativeLoginCommand(provider) && HasACPMethod(provider)
}

// UsesACPProbe reports whether the probe operation runs through the
// configured protocol method instead of a native status command.
func UsesACPProbe(provider compozyconfig.ProviderConfig) bool {
	return !HasNativeStatusCommand(provider) && HasACPMethod(provider)
}

// ClassifyErrorText maps free-form provider error text onto the canonical
// auth taxonomy. It owns the shared rate-limit/denial/transient/unauthorized
// needles for callers that receive errors instead of command output, such as
// disposable ACP probes. Unknown text stays unclassified.
func ClassifyErrorText(text string) (Classification, bool) {
	return classifyProbeOutput(strings.ToLower(text))
}
