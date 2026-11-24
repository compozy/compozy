// Package account owns the daemon installation's Compozy account session.
package account

import (
	"context"
	"crypto"
	"io"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

type State string

const (
	SignedOut      State = "signed_out"
	SigningIn      State = "signing_in"
	SignedIn       State = "signed_in"
	ReauthRequired State = "reauth_required"
)

type Installation struct {
	InstallationID                     string
	PreAccountInstall                  bool
	State                              State
	Source, AccountID, SignedOutReason string
	AuthGeneration                     int64
	LastValidatedAt                    time.Time
	SignInNoticesDismissedAt           time.Time
	Profile                            remoteproto.AccountSummary
	GatePolicyJWS                      string
	GatePolicyKeys                     string
	GatePolicyFetchedAt                time.Time
	OfflineGraceStartedAt              time.Time
	TokenName                          string

	InstallationLinked bool
	Gate               string
	Grace              *Grace
}

type Grace struct {
	Source  string
	Until   time.Time
	Message string
}

type Store interface {
	ReadInstallationAccount(context.Context) (Installation, error)
	SaveInstallationAccount(context.Context, int64, Installation) error
}

type Cloud interface {
	StartDeviceFlow(context.Context, remoteproto.DeviceFlowRequest) (remoteproto.DeviceFlowResponse, error)
	CancelDeviceFlow(context.Context, string) error
	PollDeviceFlow(context.Context, string) (remoteproto.DeviceFlowTokenResponse, error)
	Enroll(
		context.Context,
		string,
		crypto.Signer,
		remoteproto.DeviceEnrollRequest,
	) (remoteproto.DeviceEnrollResponse, error)
	Refresh(context.Context, string, crypto.Signer) (remoteproto.DeviceSessionTokens, error)
	Logout(context.Context, string, crypto.Signer) error
	Account(context.Context, cloudclient.Credentials) (remoteproto.AccountSummary, error)
	PatchAccountEmail(
		context.Context,
		cloudclient.Credentials,
		remoteproto.AccountEmailPatch,
	) (remoteproto.AccountSummary, error)
	DeleteAccount(
		context.Context,
		cloudclient.Credentials,
		remoteproto.AccountDeleteRequest,
	) (remoteproto.AccountDeleteResponse, error)
	Devices(context.Context, cloudclient.Credentials) ([]remoteproto.AccountDevice, error)
	PatchDevice(
		context.Context,
		cloudclient.Credentials,
		string,
		remoteproto.AccountDevicePatch,
	) (remoteproto.AccountDevice, error)
	RevokeDevice(context.Context, cloudclient.Credentials, string) error
	Approvals(context.Context, cloudclient.Credentials) ([]remoteproto.AccountApproval, error)
	Approval(context.Context, cloudclient.Credentials, string) (remoteproto.AccountApprovalDetails, error)
	DecideApproval(context.Context, cloudclient.Credentials, string, string) (remoteproto.ApprovalDelivery, error)
	Checkout(context.Context, cloudclient.Credentials, string) (string, error)
	BillingPortal(context.Context, cloudclient.Credentials) (string, error)
	OpenAccountEvents(context.Context, cloudclient.Credentials) (io.ReadCloser, error)
}

var _ Cloud = (*cloudclient.Client)(nil)

type Login struct {
	AttemptID               string
	VerificationURI         string
	VerificationURIComplete string
	UserCode                string
	ExpiresAt               time.Time
}

type SignInEvent struct {
	AttemptID       string
	State           string
	Code            string
	NewAccount      bool
	SignedOutReason string
}

type Change struct {
	Kind          string                 `json:"kind,omitempty"`
	Plan          string                 `json:"plan,omitempty"`
	PaymentIssue  *bool                  `json:"payment_issue,omitempty"`
	EvictRemote   bool                   `json:"evict_remote,omitzero"`
	Approval      *ApprovalResult        `json:"approval,omitempty"`
	ApprovalID    string                 `json:"approval_id,omitempty"`
	ComputerID    remoteproto.ComputerID `json:"computer_id,omitempty"`
	DeviceID      remoteproto.DeviceID   `json:"device_id,omitempty"`
	SecurityEpoch uint64                 `json:"security_epoch,omitzero"`
}

type ApprovalResult struct {
	ID     string `json:"id"`
	Result string `json:"result"`
}

var (
	ErrSignInRequired = &cloudclient.Failure{Status: http.StatusUnauthorized, Code: "sign_in_required",
		Message: "Sign in to use CompozyOS: compozy account login"}
	ErrTokenLocalOnly = &cloudclient.Failure{Status: http.StatusForbidden, Code: "account_token_local_only",
		Message: "Account tokens unlock local use only. Sign in with Google or GitHub for this."}
	ErrLogoutRequiresUnlink = &cloudclient.Failure{Status: http.StatusConflict, Code: "logout_requires_unlink",
		Message: "Signing out also unlinks this computer from remote access."}
	errExpired = &cloudclient.Failure{Status: http.StatusBadRequest, Code: "account_login_expired",
		Message: "Sign-in timed out. Try again."}
	errCancelled = &cloudclient.Failure{Status: http.StatusConflict, Code: "access_denied",
		Message: "Sign-in was cancelled."} //nolint:misspell // Account sign-in contract.
	errKeyring = &cloudclient.Failure{Status: http.StatusServiceUnavailable, Code: "account_credentials_unavailable",
		Message: "Can't read or save account credentials. Check the system keyring and try again."}
)
