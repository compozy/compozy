package extensiontest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"

	"github.com/compozy/agh/internal/subprocess"
)

const (
	bridgeAdapterHarnessBrgTelegramReferenceValue = "brg-telegram-reference"
	bridgeAdapterHarnessCoderKey                  = "coder"
	bridgeAdapterHarnessHelloKey                  = "hello"
)

const (
	EnvHandshakePath = "AGH_BRIDGE_ADAPTER_HANDSHAKE_PATH"
	EnvOwnershipPath = "AGH_BRIDGE_ADAPTER_OWNERSHIP_PATH"
	EnvStatePath     = "AGH_BRIDGE_ADAPTER_STATE_PATH"
	EnvDeliveryPath  = "AGH_BRIDGE_ADAPTER_DELIVERY_PATH"
	EnvIngestPath    = "AGH_BRIDGE_ADAPTER_INGEST_PATH"
	EnvUpdatesPath   = "AGH_BRIDGE_ADAPTER_UPDATES_PATH"
	EnvStartsPath    = "AGH_BRIDGE_ADAPTER_STARTS_PATH"
	EnvShutdownPath  = "AGH_BRIDGE_ADAPTER_SHUTDOWN_PATH"
	EnvCrashOncePath = "AGH_BRIDGE_ADAPTER_CRASH_ONCE_PATH"
)

// MarkerPaths contains the standardized marker and control files used by the
// reference adapter and the conformance harness.
type MarkerPaths struct {
	Handshake string
	Ownership string
	State     string
	Delivery  string
	Ingest    string
	Updates   string
	Starts    string
	Shutdown  string
	CrashOnce string
}

// Env returns the environment expected by the reference adapter marker contract.
func (m MarkerPaths) Env() map[string]string {
	return map[string]string{
		EnvHandshakePath: m.Handshake,
		EnvOwnershipPath: m.Ownership,
		EnvStatePath:     m.State,
		EnvDeliveryPath:  m.Delivery,
		EnvIngestPath:    m.Ingest,
		EnvUpdatesPath:   m.Updates,
		EnvStartsPath:    m.Starts,
		EnvShutdownPath:  m.Shutdown,
		EnvCrashOncePath: m.CrashOnce,
	}
}

// NewMarkerPaths returns a temp-root-relative marker layout used by the
// subprocess-backed adapter tests.
func NewMarkerPaths(root string) MarkerPaths {
	return MarkerPaths{
		Handshake: filepath.Join(root, "adapter-handshake.json"),
		Ownership: filepath.Join(root, "adapter-ownership.json"),
		State:     filepath.Join(root, "adapter-states.jsonl"),
		Delivery:  filepath.Join(root, "adapter-deliveries.jsonl"),
		Ingest:    filepath.Join(root, "adapter-ingest.jsonl"),
		Updates:   filepath.Join(root, "adapter-updates.jsonl"),
		Starts:    filepath.Join(root, "adapter-starts.log"),
		Shutdown:  filepath.Join(root, "adapter-shutdown.log"),
		CrashOnce: filepath.Join(root, "adapter-crash-once.json"),
	}
}

// NewTempMarkerPaths keeps side-effect files under os.TempDir(), which is the
// boundary enforced by subprocess-backed reference adapters.
func NewTempMarkerPaths(t testing.TB) MarkerPaths {
	t.Helper()

	root, err := os.MkdirTemp("", "agh-e2e-bridge-markers-")
	if err != nil {
		t.Fatalf("os.MkdirTemp(bridge markers) error = %v", err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("os.RemoveAll(%q) error = %v", root, err)
		}
	})
	return NewMarkerPaths(root)
}

// HandshakeRecord captures the adapter initialize marker.
type HandshakeRecord struct {
	Request  subprocess.InitializeRequest  `json:"request"`
	Response subprocess.InitializeResponse `json:"response"`
}

// OwnershipRecord captures the provider-owned instance list/get evidence
// collected by the reference adapter during boot.
type OwnershipRecord struct {
	Listed  []bridgepkg.BridgeInstance `json:"listed,omitempty"`
	Fetched []bridgepkg.BridgeInstance `json:"fetched,omitempty"`
	Error   string                     `json:"error,omitempty"`
}

// DeliveryRecord captures one `bridges/deliver` request plus the adapter ack
// when the subprocess remained alive long enough to return it.
type DeliveryRecord struct {
	PID     int                       `json:"pid"`
	Request bridgepkg.DeliveryRequest `json:"request"`
	Ack     *bridgepkg.DeliveryAck    `json:"ack,omitempty"`
	Error   string                    `json:"error,omitempty"`
}

// StateRecord captures one adapter-driven `bridges/instances/report_state`
// result marker.
type StateRecord struct {
	BridgeInstanceID string                   `json:"bridge_instance_id,omitempty"`
	Status           bridgepkg.BridgeStatus   `json:"status"`
	Instance         bridgepkg.BridgeInstance `json:"instance"`
	Error            string                   `json:"error,omitempty"`
}

// IngestRecord captures one fake inbound update mapped into a normalized ingest.
type IngestRecord struct {
	Envelope bridgepkg.InboundMessageEnvelope           `json:"envelope"`
	Result   bridgecontract.BridgesMessagesIngestResult `json:"result"`
	Error    string                                     `json:"error,omitempty"`
}

// ConformanceReport is the collected adapter evidence used by the validator.
type ConformanceReport struct {
	Handshake  *HandshakeRecord
	Ownership  *OwnershipRecord
	States     []StateRecord
	Deliveries []DeliveryRecord
	Ingests    []IngestRecord
}

// ManagedInstanceExpectation describes one provider-owned bridge instance that
// must appear in the negotiated runtime and conformance evidence.
type ManagedInstanceExpectation struct {
	InstanceID          string
	ExtensionName       string
	BoundSecretNames    []string
	ExpectedFinalStatus bridgepkg.BridgeStatus
}

// ConformanceExpectation configures the reusable adapter validator.
type ConformanceExpectation struct {
	Provider                  string
	Platform                  string
	ManagedInstances          []ManagedInstanceExpectation
	RequireOwnedInstanceList  bool
	RequireOwnedInstanceFetch bool
	RequireStateReport        bool
	RequireDelivery           bool
	RequireResume             bool
}

// ConformanceIssue reports one adapter contract failure.
type ConformanceIssue struct {
	Code    string
	Message string
}

// ConformanceError aggregates reusable harness failures.
type ConformanceError struct {
	Issues []ConformanceIssue
}

func (e *ConformanceError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ""
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
	}
	return strings.Join(parts, "; ")
}

// ValidateConformance checks the adapter evidence against the reusable bridge
// adapter contract enforced by the harness.
func ValidateConformance(report ConformanceReport, expect ConformanceExpectation) error {
	expectedByID := make(map[string]ManagedInstanceExpectation, len(expect.ManagedInstances))
	for _, managed := range expect.ManagedInstances {
		expectedByID[strings.TrimSpace(managed.InstanceID)] = managed
	}

	issues := make([]ConformanceIssue, 0)
	issues = append(issues, validateHandshakeConformance(report.Handshake, expect, expectedByID)...)
	issues = append(issues, validateOwnershipConformance(report.Ownership, expect, expectedByID)...)
	issues = append(issues, validateStateConformance(report.States, expect, expectedByID)...)
	issues = append(issues, validateDeliveryConformance(report.Deliveries, expect, expectedByID)...)
	issues = append(issues, validateIngestConformance(report.Ingests, expectedByID)...)
	if len(issues) > 0 {
		return &ConformanceError{Issues: issues}
	}
	return nil
}

func validateHandshakeConformance(
	handshake *HandshakeRecord,
	expect ConformanceExpectation,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	if handshake == nil {
		return []ConformanceIssue{{
			Code:    "missing_handshake",
			Message: "adapter did not write an initialize marker",
		}}
	}

	issues := make([]ConformanceIssue, 0)
	request := handshake.Request
	if !slices.Contains(request.Methods.ExtensionServices, "bridges/deliver") {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_delivery_negotiation",
			Message: "initialize did not negotiate bridges/deliver",
		})
	}
	if request.Runtime.Bridge == nil {
		return append(issues, ConformanceIssue{
			Code:    "missing_bridge_runtime",
			Message: "initialize runtime.bridge was nil",
		})
	}

	runtime := request.Runtime.Bridge
	issues = append(issues, validateHandshakeRuntime(runtime, expect, expectedByID)...)
	issues = append(issues, validateGrantedActions(request.Capabilities.GrantedActions)...)
	return issues
}

func validateHandshakeRuntime(
	runtime *subprocess.InitializeBridgeRuntime,
	expect ConformanceExpectation,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if got, want := strings.TrimSpace(runtime.RuntimeVersion), subprocess.InitializeBridgeRuntimeVersion2; got != want {
		issues = append(issues, ConformanceIssue{
			Code:    "wrong_runtime_version",
			Message: fmt.Sprintf("initialize runtime bridge version = %q, want %q", got, want),
		})
	}
	if expect.Provider != "" && strings.TrimSpace(runtime.Provider) != strings.TrimSpace(expect.Provider) {
		issues = append(issues, ConformanceIssue{
			Code: "wrong_provider",
			Message: fmt.Sprintf(
				"initialize runtime bridge provider = %q, want %q",
				runtime.Provider,
				expect.Provider,
			),
		})
	}
	if expect.Platform != "" && strings.TrimSpace(runtime.Platform) != strings.TrimSpace(expect.Platform) {
		issues = append(issues, ConformanceIssue{
			Code: "wrong_platform",
			Message: fmt.Sprintf(
				"initialize runtime bridge platform = %q, want %q",
				runtime.Platform,
				expect.Platform,
			),
		})
	}
	issues = append(issues, validateExpectedManagedInstances(runtime, expect.ManagedInstances)...)
	issues = append(issues, validateUnexpectedManagedInstances(runtime, expectedByID)...)
	return issues
}

func validateExpectedManagedInstances(
	runtime *subprocess.InitializeBridgeRuntime,
	expected []ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	for _, managedExpect := range expected {
		managed, ok := runtime.ManagedInstance(managedExpect.InstanceID)
		if !ok {
			issues = append(issues, ConformanceIssue{
				Code: "missing_managed_instance",
				Message: fmt.Sprintf(
					"initialize runtime bridge did not include managed instance %q",
					managedExpect.InstanceID,
				),
			})
			continue
		}
		if managedExpect.ExtensionName != "" &&
			strings.TrimSpace(managed.Instance.ExtensionName) != strings.TrimSpace(managedExpect.ExtensionName) {
			issues = append(issues, ConformanceIssue{
				Code: "wrong_extension",
				Message: fmt.Sprintf(
					"initialize runtime bridge instance %q extension = %q, want %q",
					managedExpect.InstanceID,
					managed.Instance.ExtensionName,
					managedExpect.ExtensionName,
				),
			})
		}
		issues = append(issues, validateManagedBoundSecrets(managed, managedExpect)...)
	}
	return issues
}

func validateManagedBoundSecrets(
	managed *subprocess.InitializeBridgeManagedInstance,
	expect ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if managed == nil {
		return append(issues, ConformanceIssue{
			Code:    "missing_managed_instance",
			Message: fmt.Sprintf("initialize runtime omitted managed instance %q", expect.InstanceID),
		})
	}
	bound := make(map[string]struct{}, len(managed.BoundSecrets))
	for _, secret := range managed.BoundSecrets {
		bound[strings.TrimSpace(secret.BindingName)] = struct{}{}
	}
	for _, bindingName := range expect.BoundSecretNames {
		if _, ok := bound[strings.TrimSpace(bindingName)]; !ok {
			issues = append(issues, ConformanceIssue{
				Code: "missing_bound_secret",
				Message: fmt.Sprintf(
					"initialize runtime did not include bound secret %q for managed instance %q",
					bindingName,
					expect.InstanceID,
				),
			})
		}
	}
	return issues
}

func validateUnexpectedManagedInstances(
	runtime *subprocess.InitializeBridgeRuntime,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	if len(expectedByID) == 0 {
		return nil
	}
	issues := make([]ConformanceIssue, 0)
	for _, managed := range runtime.ManagedInstances {
		if _, ok := expectedByID[strings.TrimSpace(managed.Instance.ID)]; ok {
			continue
		}
		issues = append(issues, ConformanceIssue{
			Code: "unexpected_managed_instance",
			Message: fmt.Sprintf(
				"initialize runtime included unexpected managed instance %q",
				managed.Instance.ID,
			),
		})
	}
	return issues
}

func validateGrantedActions(actions []extensionprotocol.HostAPIMethod) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	for _, method := range actions {
		text := strings.ToLower(strings.TrimSpace(string(method)))
		if strings.Contains(text, "vault/") || strings.Contains(text, "secret/") {
			issues = append(issues, ConformanceIssue{
				Code:    "leaked_secret_surface",
				Message: fmt.Sprintf("initialize granted unexpected secret surface %q", method),
			})
		}
	}
	return issues
}
