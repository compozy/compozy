// Package sshsetup owns the controller's SSH onboarding and upgrade workflow.
package sshsetup

import (
	"context"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

const (
	StepAdmit   = "admit"
	StepVerify  = "verify"
	StepConnect = "connect"
	StepInstall = "install"
	StepService = "service"
	StepLink    = "link"
	StepForward = "forward"
	StepDone    = "done"

	Running             = "running"
	NeedsInput          = "needs_input"
	NeedsOperatorSecret = "needs_operator_secret"
	Failed              = "failed"
	Canceled            = "cancelled" //nolint:misspell // Public setup state.

	TrustHost = "trust_host"
	Secret    = "secret"

	KindAdd    = "add"
	KindUpdate = "update"
)

type Request struct {
	Host, User, Name, TrustFingerprint string
	Port                               int
}

type Answer struct {
	Kind   string
	Accept bool
	Value  string
}

type Prompt struct {
	Kind        string
	Fingerprint string
	Host        string
}

type Status struct {
	ID         string
	Step       string
	State      string
	Prompt     *Prompt
	Error      *Failure
	ComputerID remoteproto.ComputerID
	Answered   bool
}

type Failure struct {
	HTTPStatus int
	Code       string
	Message    string
	Step       string
	Details    map[string]string
}

func (f *Failure) Error() string { return f.Message }

var (
	ErrNotFound   = failure(http.StatusNotFound, "computer_setup_not_found", "Computer setup not found.")
	ErrInProgress = failure(http.StatusConflict, "computer_setup_in_progress",
		"This computer already has a setup in progress.")
	ErrLinked = failure(
		http.StatusConflict,
		"computer_setup_linked",
		"This computer is already linked. Remove it instead.",
	)
	ErrSecretOperatorOnly = failure(http.StatusForbidden, "secret_answer_operator_only",
		"An operator must answer this SSH prompt.")
	ErrNoPrompt = failure(
		http.StatusConflict,
		"computer_setup_no_prompt",
		"This setup is not waiting for that answer.",
	)
	ErrUpdateManual = failure(http.StatusConflict, "computer_update_manual", "Run compozy update on this computer.")

	errProvenance = failure(http.StatusBadGateway, "ssh_install_provenance_failed",
		"Couldn't verify this Compozy release. Nothing was installed.")
	errChecksum = failure(http.StatusBadGateway, "ssh_install_checksum_mismatch",
		"Download didn't match the published checksum.")
	errPlatform = failure(http.StatusUnprocessableEntity, "ssh_platform_unsupported",
		"This computer's system isn't supported over SSH. Link it with compozy remote link instead.")
	errInterrupted = failure(http.StatusServiceUnavailable, "computer_setup_interrupted",
		"SSH setup was interrupted. Retry to resume.")
	errDeclined = failure(http.StatusConflict, "ssh_host_key_declined", "The host key was declined.")
)

func failure(status int, code, message string) *Failure {
	return &Failure{HTTPStatus: status, Code: code, Message: message}
}

type Accounts interface {
	IdentityLifetime() (context.Context, account.Installation)
	UseDevice(context.Context, func(context.Context, cloudclient.Credentials) error) error
	GetHostSecret(string) (string, bool, error)
	PutHostSecret(string, string) error
	DeleteHostSecret(string) error
}

type Cloud interface {
	ReserveComputer(context.Context, cloudclient.Credentials, string) (cloudclient.ComputerReservation, error)
	ReleaseComputerReservation(context.Context, cloudclient.Credentials, string) error
	NewComputerLinkToken(context.Context, cloudclient.Credentials, string) (cloudclient.ComputerLinkToken, error)
	Computers(context.Context, cloudclient.Credentials) ([]remoteproto.AccountComputer, error)
}

type Registry interface {
	HasSSHRoute(ctx context.Context, owner, address string) (bool, error)
	SSHUpdateTarget(ctx context.Context, owner string, id remoteproto.ComputerID) (store.ComputerRoute, error)
	RegisterSSHComputer(
		ctx context.Context,
		owner, name string,
		id remoteproto.ComputerID,
		route store.ComputerRoute,
	) error
	WaitForSSHComputer(ctx context.Context, owner string, id remoteproto.ComputerID) error
	PublishSSHSetup(owner string, generation int64, status Status)
}

const (
	connectBudget = 10 * time.Second
	commandBudget = time.Minute
	installBudget = 600 * time.Second
	MaxStatusWait = 30 * time.Second
)
