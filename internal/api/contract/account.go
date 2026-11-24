package contract

import (
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	AccountPath             = "/api/account"
	AccountLoginPath        = AccountPath + "/login"
	AccountLogoutPath       = AccountPath + "/logout"
	AccountSignInEventsPath = AccountPath + "/sign-in/events"
	AccountCheckoutPath     = AccountPath + "/checkout"
	SignInProgressEvent     = "sign_in.progress"
	AccountStateEvent       = "account.state"
)

type AccountStatusResponse struct {
	remoteproto.AccountPlan
	Gate               string                `json:"gate"`
	Grace              *AccountGrace         `json:"grace"`
	State              account.State         `json:"state"`
	Source             *string               `json:"source"`
	SignedOutReason    *string               `json:"signed_out_reason"`
	PreAccountInstall  bool                  `json:"pre_account_install"`
	InstallationLinked bool                  `json:"installation_linked"`
	TokenName          *string               `json:"token_name"`
	SignedIn           bool                  `json:"signed_in"`
	Email              string                `json:"email,omitempty"`
	Name               string                `json:"name,omitempty"`
	Provider           string                `json:"provider,omitempty"`
	Device             *AccountDeviceSummary `json:"device"`
	MarketingOptIn     bool                  `json:"marketing_opt_in"`
	RecentSignIns      []AccountSignInNotice `json:"recent_sign_ins"`
}

type AccountGrace struct {
	Source  string    `json:"source"`
	Until   time.Time `json:"until"`
	Message string    `json:"message"`
}

type AccountSignInNotice struct {
	DeviceID string    `json:"device_id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Location string    `json:"location"`
	At       time.Time `json:"at"`
}

type AccountDeviceSummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

type AccountCLIStatus struct {
	State                account.State         `json:"state,omitempty"`
	Email                string                `json:"email,omitempty"`
	Plan                 string                `json:"plan,omitempty"`
	RemoteComputersUsed  int64                 `json:"remote_computers_used"`
	RemoteComputersLimit *int64                `json:"remote_computers_limit"`
	CloudRoutesUsed      int64                 `json:"cloud_routes_used"`
	CloudRoutesLimit     int64                 `json:"cloud_routes_limit"`
	Device               *AccountDeviceSummary `json:"device"`
}

func (status AccountStatusResponse) CLIStatus() AccountCLIStatus {
	result := AccountCLIStatus{Email: status.Email, Plan: status.Plan, Device: status.Device,
		RemoteComputersUsed: status.RemoteComputersUsed, RemoteComputersLimit: status.RemoteComputersLimit,
		CloudRoutesUsed: status.CloudRoutesUsed, CloudRoutesLimit: status.CloudRoutesLimit}
	if !status.SignedIn {
		result.State = status.State
	}
	return result
}

type AccountLoginRequest struct {
	Provider       *string `json:"provider"`
	MarketingOptIn bool    `json:"marketing_opt_in"`
}

type AccountLoginResponse struct {
	AttemptID               string    `json:"attempt_id"`
	VerificationURI         string    `json:"verification_uri"`
	VerificationURIComplete string    `json:"verification_uri_complete"`
	UserCode                string    `json:"user_code"`
	ExpiresAt               time.Time `json:"expires_at"`
}

type AccountLogoutRequest struct {
	Unlink bool `json:"unlink"`
}
type AccountLogoutResponse struct {
	State account.State `json:"state"`
}

type AccountStatePayload struct {
	State           string  `json:"state"`
	SignedOutReason *string `json:"signed_out_reason"`
}

type AccountSignInProgress struct {
	AttemptID string `json:"attempt_id"`
	State     string `json:"state"`
	Code      string `json:"code,omitempty"`
}

type AccountDevicesResponse struct {
	Devices []remoteproto.AccountDevice `json:"devices"`
}
type AccountApprovalsResponse struct {
	Approvals []remoteproto.AccountApproval `json:"approvals"`
}
type AccountApprovalDecisionRequest struct {
	Approve *bool `json:"approve"`
}
type AccountCheckoutRequest struct {
	Interval string `json:"interval"`
}
type AccountBillingResponse struct {
	URL string `json:"url"`
}
