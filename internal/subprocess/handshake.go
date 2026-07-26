package subprocess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	bridges "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

// CloneInitializeRequest returns a deep copy safe to expose to callers.
func CloneInitializeRequest(src InitializeRequest) InitializeRequest {
	cloned := src
	cloned.SupportedProtocolVersion = append([]string(nil), src.SupportedProtocolVersion...)
	cloned.Capabilities = cloneInitializeCapabilities(src.Capabilities)
	cloned.Methods.DaemonRequests = append([]string(nil), src.Methods.DaemonRequests...)
	cloned.Methods.ExtensionServices = append([]string(nil), src.Methods.ExtensionServices...)
	cloned.Runtime.Bridge = CloneInitializeBridgeRuntime(src.Runtime.Bridge)
	return cloned
}

func cloneInitializeCapabilities(src InitializeCapabilities) InitializeCapabilities {
	cloned := src
	cloned.Provides = append([]string(nil), src.Provides...)
	cloned.GrantedActions = append([]extensionprotocol.HostAPIMethod(nil), src.GrantedActions...)
	cloned.GrantedSecurity = append([]string(nil), src.GrantedSecurity...)
	cloned.GrantedResourceKinds = append([]string(nil), src.GrantedResourceKinds...)
	cloned.GrantedResourceScopes = append([]string(nil), src.GrantedResourceScopes...)
	return cloned
}

// InitializeRequest is the AGH -> extension session contract request.
type InitializeRequest struct {
	ProtocolVersion          string                 `json:"protocol_version"`
	SupportedProtocolVersion []string               `json:"supported_protocol_versions"`
	AGHVersion               string                 `json:"agh_version"`
	SessionNonce             string                 `json:"session_nonce"`
	Extension                InitializeExtension    `json:"extension"`
	Capabilities             InitializeCapabilities `json:"capabilities"`
	Methods                  InitializeMethods      `json:"methods"`
	Runtime                  InitializeRuntime      `json:"runtime"`
}

// InitializeExtension identifies the launched extension.
type InitializeExtension struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	SourceTier string `json:"source_tier"`
}

// InitializeCapabilities carries runtime-granted capabilities.
type InitializeCapabilities struct {
	Provides              []string                          `json:"provides"`
	GrantedActions        []extensionprotocol.HostAPIMethod `json:"granted_actions"`
	GrantedSecurity       []string                          `json:"granted_security"`
	GrantedResourceKinds  []string                          `json:"granted_resource_kinds"`
	GrantedResourceScopes []string                          `json:"granted_resource_scopes"`
}

// InitializeMethods lists callable method families for the session.
type InitializeMethods struct {
	DaemonRequests    []string `json:"daemon_requests"`
	ExtensionServices []string `json:"extension_services"`
}

// InitializeRuntime carries runtime intervals and deadlines negotiated during initialize.
type InitializeRuntime struct {
	HealthCheckIntervalMS int64                    `json:"health_check_interval_ms"`
	HealthCheckTimeoutMS  int64                    `json:"health_check_timeout_ms"`
	ShutdownTimeoutMS     int64                    `json:"shutdown_timeout_ms"`
	DefaultHookTimeoutMS  int64                    `json:"default_hook_timeout_ms"`
	Bridge                *InitializeBridgeRuntime `json:"bridge,omitempty"`
}

const (
	// InitializeBridgeRuntimeVersion2 is the purpose-scoped bridge runtime
	// handshake version negotiated by bridge-capable extensions.
	InitializeBridgeRuntimeVersion2 = "2"
)

// BridgeRuntimePurpose separates supervised service runtimes from isolated control calls.
type BridgeRuntimePurpose string

const (
	BridgeRuntimePurposeService BridgeRuntimePurpose = "service"
	BridgeRuntimePurposeControl BridgeRuntimePurpose = "control"
)

// BridgeRuntimePurposeValues returns the closed launch-purpose family in stable order.
func BridgeRuntimePurposeValues() []string {
	return []string{string(BridgeRuntimePurposeService), string(BridgeRuntimePurposeControl)}
}

// InitializeBridgeRuntime carries the provider-scoped bridge launch material
// granted to one bridge-capable extension session.
type InitializeBridgeRuntime struct {
	RuntimeVersion   string                            `json:"runtime_version"`
	Purpose          BridgeRuntimePurpose              `json:"purpose"`
	Provider         string                            `json:"provider"`
	Platform         string                            `json:"platform"`
	AllowedMethods   []string                          `json:"allowed_methods,omitempty"`
	ManagedInstances []InitializeBridgeManagedInstance `json:"managed_instances,omitempty"`
}

// InitializeBridgeManagedInstance is one daemon-owned bridge instance snapshot
// granted to the provider runtime together with its resolved secret bindings.
type InitializeBridgeManagedInstance struct {
	Instance     bridges.BridgeInstance        `json:"instance"`
	BoundSecrets []InitializeBridgeBoundSecret `json:"bound_secrets,omitempty"`
}

// InitializeBridgeBoundSecret is one launch-time bridge secret resolved by AGH.
type InitializeBridgeBoundSecret struct {
	BindingName string `json:"binding_name"`
	Kind        string `json:"kind"`
	Value       string `json:"value"`
}

// InitializeResponse is the extension -> AGH initialize acknowledgment.
type InitializeResponse struct {
	ProtocolVersion      string                  `json:"protocol_version"`
	ExtensionInfo        InitializeExtensionInfo `json:"extension_info"`
	AcceptedCapabilities AcceptedCapabilities    `json:"accepted_capabilities"`
	ImplementedMethods   []string                `json:"implemented_methods"`
	SupportedHookEvents  []string                `json:"supported_hook_events"`
	WatchSourceKinds     []string                `json:"watch_source_kinds,omitempty"`
	Supports             InitializeSupports      `json:"supports"`
}

// InitializeExtensionInfo identifies the running extension implementation.
type InitializeExtensionInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	SDKName    string `json:"sdk_name,omitempty"`
	SDKVersion string `json:"sdk_version,omitempty"`
}

// AcceptedCapabilities is the subset the extension accepted for this session.
type AcceptedCapabilities struct {
	Provides []string                          `json:"provides"`
	Actions  []extensionprotocol.HostAPIMethod `json:"actions"`
	Security []string                          `json:"security"`
}

// InitializeSupports advertises optional protocol features.
type InitializeSupports struct {
	HealthCheck bool `json:"health_check"`
}

// ShutdownRequest is the cooperative drain request sent before signal escalation.
type ShutdownRequest struct {
	Reason     string `json:"reason"`
	DeadlineMS int64  `json:"deadline_ms"`
}

// ShutdownResponse acknowledges a cooperative drain request.
type ShutdownResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// Initialize performs the session initialize handshake and transitions the process to ready on success.
func (p *Process) Initialize(ctx context.Context, request InitializeRequest) (InitializeResponse, error) {
	if p == nil {
		return InitializeResponse{}, errors.New("subprocess: process is required")
	}
	if p.currentState() != processStateStarting {
		return InitializeResponse{}, errors.New("subprocess: initialize may only be called once")
	}

	if err := request.Validate(); err != nil {
		return InitializeResponse{}, err
	}

	var response InitializeResponse
	if err := p.Call(ctx, initializeMethod, request, &response); err != nil {
		return InitializeResponse{}, err
	}
	if err := validateInitializeResponse(request, response); err != nil {
		return InitializeResponse{}, err
	}

	p.markReady()
	p.maybeStartHealthMonitor(request.Runtime, response.Supports)

	return response, nil
}

// Validate checks that the initialize request carries the mandatory contract fields.
func (r InitializeRequest) Validate() error {
	if strings.TrimSpace(r.ProtocolVersion) == "" {
		return errors.New("subprocess: initialize protocol_version is required")
	}
	if strings.TrimSpace(r.SessionNonce) == "" {
		return errors.New("subprocess: initialize session_nonce is required")
	}
	if len(r.SupportedProtocolVersion) == 0 {
		return errors.New("subprocess: initialize supported_protocol_versions is required")
	}
	if strings.TrimSpace(r.Extension.Name) == "" {
		return errors.New("subprocess: initialize extension.name is required")
	}
	if strings.TrimSpace(r.Extension.Version) == "" {
		return errors.New("subprocess: initialize extension.version is required")
	}
	if r.Runtime.HealthCheckIntervalMS <= 0 {
		return errors.New("subprocess: initialize health_check_interval_ms must be > 0")
	}
	if r.Runtime.HealthCheckTimeoutMS <= 0 {
		return errors.New("subprocess: initialize health_check_timeout_ms must be > 0")
	}
	if r.Runtime.ShutdownTimeoutMS <= 0 {
		return errors.New("subprocess: initialize shutdown_timeout_ms must be > 0")
	}
	if r.Runtime.DefaultHookTimeoutMS <= 0 {
		return errors.New("subprocess: initialize default_hook_timeout_ms must be > 0")
	}
	if r.Runtime.Bridge != nil {
		if err := r.Runtime.Bridge.Validate(); err != nil {
			return err
		}
		if err := validateBridgeInitializeRequest(r); err != nil {
			return err
		}
	}
	return nil
}

func validateInitializeResponse(request InitializeRequest, response InitializeResponse) error {
	if !slices.Contains(request.SupportedProtocolVersion, response.ProtocolVersion) {
		return fmt.Errorf("subprocess: initialize selected unsupported protocol version %q", response.ProtocolVersion)
	}
	if request.Runtime.Bridge != nil && request.Runtime.Bridge.Purpose == BridgeRuntimePurposeControl {
		return validateControlInitializeResponse(request, response)
	}
	if err := validateSubset(
		"accepted actions",
		response.AcceptedCapabilities.Actions,
		request.Capabilities.GrantedActions,
	); err != nil {
		return err
	}
	if err := validateSubset(
		"accepted security",
		response.AcceptedCapabilities.Security,
		request.Capabilities.GrantedSecurity,
	); err != nil {
		return err
	}
	if err := validateSubset(
		"accepted provides",
		response.AcceptedCapabilities.Provides,
		request.Capabilities.Provides,
	); err != nil {
		return err
	}

	if !slices.Contains(response.ImplementedMethods, shutdownMethod) {
		return errors.New("subprocess: initialize response missing required shutdown method")
	}
	if !response.Supports.HealthCheck || !slices.Contains(response.ImplementedMethods, "health_check") {
		return errors.New("subprocess: initialize response missing required health_check support")
	}
	for _, method := range extensionprotocol.CapabilityServiceMethods(response.AcceptedCapabilities.Provides) {
		if !slices.Contains(response.ImplementedMethods, method) {
			return fmt.Errorf("subprocess: initialize response missing required capability service method %q", method)
		}
	}

	return nil
}

// Validate checks that the managed instance payload is complete and internally consistent.
func (m InitializeBridgeManagedInstance) Validate() error {
	instance := m.Instance
	if err := instance.Validate(); err != nil {
		return fmt.Errorf("subprocess: initialize bridge instance: %w", err)
	}

	seen := make(map[string]struct{}, len(m.BoundSecrets))
	for _, secret := range m.BoundSecrets {
		normalized := secret.normalize()
		if err := normalized.Validate(); err != nil {
			return fmt.Errorf("subprocess: initialize bridge bound secret: %w", err)
		}
		if _, ok := seen[normalized.BindingName]; ok {
			return fmt.Errorf("subprocess: initialize bridge bound secret %q is duplicated", normalized.BindingName)
		}
		seen[normalized.BindingName] = struct{}{}
	}

	return nil
}

// Validate checks that the bound secret payload is complete.
func (s InitializeBridgeBoundSecret) Validate() error {
	normalized := s.normalize()
	if strings.TrimSpace(normalized.BindingName) == "" {
		return errors.New("subprocess: initialize bridge bound secret binding_name is required")
	}
	if strings.TrimSpace(normalized.Kind) == "" {
		return errors.New("subprocess: initialize bridge bound secret kind is required")
	}
	if strings.TrimSpace(normalized.Value) == "" {
		return errors.New("subprocess: initialize bridge bound secret value is required")
	}
	return nil
}

func (r InitializeBridgeRuntime) normalize() InitializeBridgeRuntime {
	normalized := r
	normalized.RuntimeVersion = strings.TrimSpace(normalized.RuntimeVersion)
	normalized.Purpose = BridgeRuntimePurpose(strings.TrimSpace(string(normalized.Purpose)))
	normalized.Provider = strings.TrimSpace(normalized.Provider)
	normalized.Platform = strings.TrimSpace(normalized.Platform)
	normalized.AllowedMethods = normalizeUniqueHandshakeStrings(normalized.AllowedMethods)
	if len(normalized.ManagedInstances) == 0 {
		normalized.ManagedInstances = nil
		return normalized
	}

	managedInstances := make([]InitializeBridgeManagedInstance, 0, len(normalized.ManagedInstances))
	for _, managed := range normalized.ManagedInstances {
		managedInstances = append(managedInstances, managed.normalize())
	}
	slices.SortFunc(
		managedInstances,
		func(left InitializeBridgeManagedInstance, right InitializeBridgeManagedInstance) int {
			return strings.Compare(strings.TrimSpace(left.Instance.ID), strings.TrimSpace(right.Instance.ID))
		},
	)
	normalized.ManagedInstances = managedInstances
	return normalized
}

func (m InitializeBridgeManagedInstance) normalize() InitializeBridgeManagedInstance {
	normalized := m
	if len(normalized.BoundSecrets) == 0 {
		normalized.BoundSecrets = nil
		return normalized
	}

	boundSecrets := make([]InitializeBridgeBoundSecret, 0, len(normalized.BoundSecrets))
	for _, secret := range normalized.BoundSecrets {
		boundSecrets = append(boundSecrets, secret.normalize())
	}
	slices.SortFunc(boundSecrets, func(left InitializeBridgeBoundSecret, right InitializeBridgeBoundSecret) int {
		return strings.Compare(left.BindingName, right.BindingName)
	})
	normalized.BoundSecrets = boundSecrets
	return normalized
}

func (s InitializeBridgeBoundSecret) normalize() InitializeBridgeBoundSecret {
	normalized := s
	normalized.BindingName = strings.TrimSpace(normalized.BindingName)
	normalized.Kind = strings.TrimSpace(normalized.Kind)
	return normalized
}

// CloneInitializeBridgeRuntime returns a deep copy safe to retain in manager state.
func CloneInitializeBridgeRuntime(src *InitializeBridgeRuntime) *InitializeBridgeRuntime {
	if src == nil {
		return nil
	}

	cloned := src.normalize()
	if len(cloned.ManagedInstances) > 0 {
		managedInstances := make([]InitializeBridgeManagedInstance, 0, len(cloned.ManagedInstances))
		for _, managed := range cloned.ManagedInstances {
			managedInstances = append(managedInstances, cloneInitializeBridgeManagedInstance(managed))
		}
		cloned.ManagedInstances = managedInstances
	}
	return &cloned
}

func cloneInitializeBridgeManagedInstance(src InitializeBridgeManagedInstance) InitializeBridgeManagedInstance {
	cloned := src.normalize()
	cloned.Instance = cloneBridgeInstance(cloned.Instance)
	cloned.BoundSecrets = append([]InitializeBridgeBoundSecret(nil), cloned.BoundSecrets...)
	return cloned
}

func cloneBridgeInstance(instance bridges.BridgeInstance) bridges.BridgeInstance {
	cloned := instance
	if len(cloned.ProviderConfig) > 0 {
		cloned.ProviderConfig = append(json.RawMessage(nil), cloned.ProviderConfig...)
	}
	if len(cloned.DeliveryDefaults) > 0 {
		cloned.DeliveryDefaults = append(json.RawMessage(nil), cloned.DeliveryDefaults...)
	}
	if cloned.Degradation != nil {
		degradation := *cloned.Degradation
		cloned.Degradation = &degradation
	}
	return cloned
}

// SingleManagedInstance returns the only managed bridge instance snapshot in
// the provider runtime. It fails when the runtime owns zero or multiple
// instances and the caller did not select one explicitly.
func (r InitializeBridgeRuntime) SingleManagedInstance() (*InitializeBridgeManagedInstance, error) {
	normalized := r.normalize()
	switch len(normalized.ManagedInstances) {
	case 0:
		return nil, errors.New("subprocess: initialize bridge runtime managed instance is required")
	case 1:
		managed := cloneInitializeBridgeManagedInstance(normalized.ManagedInstances[0])
		return &managed, nil
	default:
		return nil, errors.New("subprocess: initialize bridge runtime requires explicit managed instance selection")
	}
}

// ManagedInstance returns one managed bridge instance snapshot by id.
func (r InitializeBridgeRuntime) ManagedInstance(id string) (*InitializeBridgeManagedInstance, bool) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, false
	}

	for _, managed := range r.normalize().ManagedInstances {
		if strings.TrimSpace(managed.Instance.ID) != trimmedID {
			continue
		}
		cloned := cloneInitializeBridgeManagedInstance(managed)
		return &cloned, true
	}
	return nil, false
}

// ManagedBridgeInstanceIDs returns the provider-owned bridge instance ids in a
// stable order suitable for telemetry fan-out and restart bookkeeping.
func (r InitializeBridgeRuntime) ManagedBridgeInstanceIDs() []string {
	managed := r.normalize().ManagedInstances
	if len(managed) == 0 {
		return nil
	}

	ids := make([]string, 0, len(managed))
	for _, item := range managed {
		ids = append(ids, strings.TrimSpace(item.Instance.ID))
	}
	return ids
}

func validateSubset[T ~string](label string, accepted []T, granted []T) error {
	for _, value := range accepted {
		if !slices.Contains(granted, value) {
			return fmt.Errorf("subprocess: %s %q is not granted", label, value)
		}
	}
	return nil
}
