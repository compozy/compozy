package remoteproto

import "time"

type AccountDevice struct {
	ID         DeviceID  `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Admin      bool      `json:"admin"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ThisDevice bool      `json:"this_device"`
}

type AccountDevicePatch struct {
	Name   *string                `json:"name,omitempty"`
	Admin  *bool                  `json:"admin,omitempty"`
	Scopes map[ComputerID][]Scope `json:"scopes,omitempty"`
}

type AccountPlan struct {
	Plan                 string               `json:"plan"`
	RemoteComputersUsed  int64                `json:"remote_computers_used"`
	RemoteComputersLimit *int64               `json:"remote_computers_limit"`
	CloudRoutesUsed      int64                `json:"cloud_routes_used"`
	CloudRoutesLimit     int64                `json:"cloud_routes_limit"`
	PaymentIssue         bool                 `json:"payment_issue"`
	Subscription         *AccountSubscription `json:"subscription"`
}

type AccountSubscription struct {
	Source              string     `json:"source"`
	Interval            *string    `json:"interval"`
	CurrentPeriodEnd    *time.Time `json:"current_period_end"`
	CancelAtPeriodEnd   bool       `json:"cancel_at_period_end"`
	RefundEligibleUntil *time.Time `json:"refund_eligible_until"`
}

type CheckoutRequest struct {
	Interval string `json:"interval"`
}

type BillingURL struct {
	URL string `json:"url"`
}

type AccountSummary struct {
	AccountPlan
	Email          string        `json:"email"`
	Name           string        `json:"name"`
	Provider       string        `json:"provider"`
	Device         AccountDevice `json:"device"`
	MarketingOptIn bool          `json:"marketing_opt_in"`
}

type AccountApproval struct {
	ID             string     `json:"id"`
	ComputerID     ComputerID `json:"computer_id"`
	DeviceName     string     `json:"device_name"`
	KeyFingerprint string     `json:"key_fingerprint"`
	CreatedAt      time.Time  `json:"created_at"`
	State          string     `json:"state"`
	AppliedResult  *string    `json:"applied_result"`
	CanDecide      bool       `json:"can_decide"`
}

type AccountApprovalDetails struct {
	AccountApproval
	DeviceID DeviceID `json:"device_id"`
}

type ApprovalDelivery struct {
	ApprovalID string `json:"approval_id"`
	State      string `json:"state"`
}

type CloudAccountEvent struct {
	ComputerID    ComputerID `json:"computer_id,omitempty"`
	DeviceID      DeviceID   `json:"device_id,omitempty"`
	SecurityEpoch uint64     `json:"security_epoch,omitzero"`
	ApprovalID    string     `json:"approval_id,omitempty"`
	Result        string     `json:"result,omitempty"`
}
