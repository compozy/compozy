package subprocess

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

// Validate checks that a v2 purpose-scoped bridge launch payload is internally consistent.
func (r InitializeBridgeRuntime) Validate() error {
	normalized := r.normalize()
	if r.Purpose != normalized.Purpose {
		return fmt.Errorf(
			"subprocess: initialize bridge runtime purpose %q must be canonical",
			r.Purpose,
		)
	}
	if normalized.RuntimeVersion != InitializeBridgeRuntimeVersion2 {
		return fmt.Errorf(
			"subprocess: initialize bridge runtime version %q is unsupported; want %q",
			normalized.RuntimeVersion,
			InitializeBridgeRuntimeVersion2,
		)
	}
	if normalized.Provider == "" {
		return errors.New("subprocess: initialize bridge runtime provider is required")
	}
	if normalized.Platform == "" {
		return errors.New("subprocess: initialize bridge runtime platform is required")
	}
	if err := validateBridgeRuntimePurpose(normalized); err != nil {
		return err
	}
	if len(normalized.ManagedInstances) == 0 {
		return errors.New("subprocess: initialize bridge runtime managed instances are required")
	}

	seen := make(map[string]struct{}, len(normalized.ManagedInstances))
	for _, managed := range normalized.ManagedInstances {
		if err := managed.Validate(); err != nil {
			return fmt.Errorf("subprocess: initialize bridge managed instance: %w", err)
		}
		if strings.TrimSpace(managed.Instance.Platform) != normalized.Platform {
			return fmt.Errorf(
				"subprocess: initialize bridge managed instance %q platform %q does not match runtime platform %q",
				managed.Instance.ID,
				managed.Instance.Platform,
				normalized.Platform,
			)
		}
		if strings.TrimSpace(managed.Instance.ExtensionName) != normalized.Provider {
			return fmt.Errorf(
				"subprocess: initialize bridge managed instance %q extension %q does not match runtime provider %q",
				managed.Instance.ID,
				managed.Instance.ExtensionName,
				normalized.Provider,
			)
		}
		instanceID := strings.TrimSpace(managed.Instance.ID)
		if _, ok := seen[instanceID]; ok {
			return fmt.Errorf("subprocess: initialize bridge managed instance %q is duplicated", instanceID)
		}
		seen[instanceID] = struct{}{}
	}
	return nil
}

func validateBridgeRuntimePurpose(runtime InitializeBridgeRuntime) error {
	switch runtime.Purpose {
	case BridgeRuntimePurposeService:
		if len(runtime.AllowedMethods) != 0 {
			return errors.New("subprocess: initialize bridge service runtime must not grant control methods")
		}
	case BridgeRuntimePurposeControl:
		if len(runtime.ManagedInstances) != 1 {
			return errors.New("subprocess: initialize bridge control runtime requires exactly one managed instance")
		}
		if len(runtime.AllowedMethods) != 1 {
			return errors.New("subprocess: initialize bridge control runtime requires exactly one allowed method")
		}
		if err := bridgepkg.ControlMethod(runtime.AllowedMethods[0]).Validate(); err != nil {
			return fmt.Errorf("subprocess: initialize bridge control runtime: %w", err)
		}
	default:
		return fmt.Errorf("subprocess: initialize bridge runtime purpose %q is unsupported", runtime.Purpose)
	}
	return nil
}

func validateBridgeInitializeRequest(request InitializeRequest) error {
	runtime := request.Runtime.Bridge
	if runtime == nil || runtime.Purpose != BridgeRuntimePurposeControl {
		return nil
	}
	capabilities := request.Capabilities
	if len(capabilities.Provides) != 0 || len(capabilities.GrantedActions) != 0 ||
		len(capabilities.GrantedSecurity) != 0 || len(capabilities.GrantedResourceKinds) != 0 ||
		len(capabilities.GrantedResourceScopes) != 0 {
		return errors.New("subprocess: initialize bridge control runtime requires zero Host API grants")
	}
	if !slices.Equal(request.Methods.DaemonRequests, []string{shutdownMethod}) {
		return errors.New("subprocess: initialize bridge control runtime may expose only shutdown")
	}
	if !slices.Equal(request.Methods.ExtensionServices, runtime.AllowedMethods) {
		return errors.New(
			"subprocess: initialize bridge control runtime extension services must match the allowed method",
		)
	}
	return nil
}

func validateControlInitializeResponse(request InitializeRequest, response InitializeResponse) error {
	runtime := request.Runtime.Bridge.normalize()
	if len(response.AcceptedCapabilities.Provides) != 0 || len(response.AcceptedCapabilities.Actions) != 0 ||
		len(response.AcceptedCapabilities.Security) != 0 {
		return errors.New("subprocess: initialize bridge control response accepted unexpected capabilities")
	}
	implemented := append([]string(nil), response.ImplementedMethods...)
	slices.Sort(implemented)
	want := []string{runtime.AllowedMethods[0], shutdownMethod}
	slices.Sort(want)
	if !slices.Equal(implemented, want) {
		return fmt.Errorf(
			"subprocess: initialize bridge control response methods %#v, want exact methods %#v",
			implemented,
			want,
		)
	}
	if response.Supports.HealthCheck {
		return errors.New("subprocess: initialize bridge control response must not enable health checks")
	}
	if len(response.SupportedHookEvents) != 0 || len(response.WatchSourceKinds) != 0 {
		return errors.New("subprocess: initialize bridge control response must not expose hooks or watch sources")
	}
	return nil
}

func normalizeUniqueHandshakeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized
}
