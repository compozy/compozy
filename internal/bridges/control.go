package bridges

import (
	"context"
	"errors"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

var (
	// ErrBridgeControlTransportUnavailable reports that no extension runtime can execute a provider control call.
	ErrBridgeControlTransportUnavailable = errors.New("bridges: control transport unavailable")
)

// ControlMethod identifies one typed provider control operation.
type ControlMethod string

const (
	// ControlMethodCheck asks the adapter to run read-only provider checks.
	ControlMethodCheck ControlMethod = ControlMethod(bridgecontract.ControlMethodCheck)
	// ControlMethodWebhookRegister asks the adapter to register its configured webhook.
	ControlMethodWebhookRegister ControlMethod = ControlMethod(bridgecontract.ControlMethodWebhookRegister)
)

// Validate enforces the closed provider control method family.
func (m ControlMethod) Validate() error {
	return bridgecontract.ControlMethod(m).Validate()
}

// BridgeCheckStatus is the closed result state for one provider check.
type BridgeCheckStatus string

const (
	BridgeCheckStatusPass    BridgeCheckStatus = BridgeCheckStatus(bridgecontract.BridgeCheckStatusPass)
	BridgeCheckStatusWarn    BridgeCheckStatus = BridgeCheckStatus(bridgecontract.BridgeCheckStatusWarn)
	BridgeCheckStatusFail    BridgeCheckStatus = BridgeCheckStatus(bridgecontract.BridgeCheckStatusFail)
	BridgeCheckStatusSkipped BridgeCheckStatus = BridgeCheckStatus(bridgecontract.BridgeCheckStatusSkipped)
)

// Validate enforces the closed public check status family.
func (s BridgeCheckStatus) Validate() error {
	return bridgecontract.BridgeCheckStatus(s).Validate()
}

// BridgeCheckRecord is one stable, redacted provider check result.
// Provider response bodies and raw errors are deliberately absent from this contract.
type BridgeCheckRecord struct {
	Check       string            `json:"check"`
	Status      BridgeCheckStatus `json:"status"`
	Remediation string            `json:"remediation"`
}

// Validate enforces a named, closed, actionable provider check result.
func (r BridgeCheckRecord) Validate() error {
	return bridgeCheckRecordToContract(r).Validate()
}

// BridgeCheckRequest selects the daemon-owned bridge instance to inspect.
type BridgeCheckRequest struct {
	BridgeInstanceID string `json:"bridge_instance_id"`
}

// Validate requires an explicit bridge instance.
func (r BridgeCheckRequest) Validate() error {
	return (bridgecontract.BridgeCheckRequest{
		BridgeInstanceID: r.BridgeInstanceID,
	}).Validate()
}

// BridgeCheckResponse returns every explicit provider check record.
type BridgeCheckResponse struct {
	Checks []BridgeCheckRecord `json:"checks"`
}

// Validate rejects empty or malformed provider responses.
func (r BridgeCheckResponse) Validate() error {
	checks := make([]bridgecontract.BridgeCheckRecord, len(r.Checks))
	for index, check := range r.Checks {
		checks[index] = bridgeCheckRecordToContract(check)
	}
	return (bridgecontract.BridgeCheckResponse{Checks: checks}).Validate()
}

// BridgeWebhookRegistrationRequest selects the instance whose configured webhook is registered.
type BridgeWebhookRegistrationRequest struct {
	BridgeInstanceID string `json:"bridge_instance_id"`
}

// Validate requires an explicit bridge instance.
func (r BridgeWebhookRegistrationRequest) Validate() error {
	return (bridgecontract.BridgeWebhookRegistrationRequest{
		BridgeInstanceID: r.BridgeInstanceID,
	}).Validate()
}

// BridgeWebhookRegistrationResponse is the stable result of provider webhook registration.
type BridgeWebhookRegistrationResponse struct {
	Status      BridgeCheckStatus `json:"status"`
	Remediation string            `json:"remediation"`
}

// Validate enforces the same closed, actionable status contract as provider checks.
func (r BridgeWebhookRegistrationResponse) Validate() error {
	return (bridgecontract.BridgeWebhookRegistrationResponse{
		Status:      bridgecontract.BridgeCheckStatus(r.Status),
		Remediation: r.Remediation,
	}).Validate()
}

func bridgeCheckRecordToContract(record BridgeCheckRecord) bridgecontract.BridgeCheckRecord {
	return bridgecontract.BridgeCheckRecord{
		Check:       record.Check,
		Status:      bridgecontract.BridgeCheckStatus(record.Status),
		Remediation: record.Remediation,
	}
}

// BridgeControlTransport executes provider-owned control calls through isolated extension runtimes.
type BridgeControlTransport interface {
	CheckBridge(
		ctx context.Context,
		extensionName string,
		req BridgeCheckRequest,
	) (BridgeCheckResponse, error)
	RegisterBridgeWebhook(
		ctx context.Context,
		extensionName string,
		req BridgeWebhookRegistrationRequest,
	) (BridgeWebhookRegistrationResponse, error)
}
