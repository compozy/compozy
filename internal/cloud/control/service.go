package control

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/cloudflare"
	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	accountActive    = "active"
	approvalPending  = "pending"
	approvalApproved = "approved"
	approvalDenied   = "denied"
	auditComputerID  = "computer_id"
	auditDeviceID    = "device_id"
	auditSystem      = "system"
)

type DevicePrincipal struct {
	AccountID string
	DeviceID  remoteproto.DeviceID
	JKT       string
	SessionID string
}

type ServiceConfig struct {
	Signer        *remoteproto.Signer
	Issuer        string
	MonthlyPrice  string
	AnnualPrice   string
	Now           func() time.Time
	Logger        *slog.Logger
	HostedTunnels *cloudflare.Client
}

type Service struct {
	db     *store.Store
	config ServiceConfig
}

func NewService(db *store.Store, config ServiceConfig) *Service {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Service{db: db, config: config}
}

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Code
}

var (
	ErrInvalidInput  = &Error{http.StatusBadRequest, "invalid_request", "Invalid request."}
	ErrUnauthorized  = &Error{http.StatusUnauthorized, "unauthorized", "Authentication required."}
	ErrProofInvalid  = &Error{http.StatusUnauthorized, "computer_proof_invalid", "Invalid computer key proof."}
	errProofReplayed = &Error{
		http.StatusUnauthorized,
		"computer_proof_replayed",
		"Computer key proof was already used.",
	}
	errAccountSuspended = &Error{http.StatusForbidden, "account_suspended",
		"This account is suspended. Contact support@compozy.com."}
	errAccountDeleting = &Error{http.StatusConflict, "account_deleting",
		"Your account is being deleted. Try again in a few minutes."}
	errAdminRequired = &Error{
		http.StatusForbidden,
		"account_admin_required",
		"Only a device trusted for admin can do this.",
	}
	errDeviceRevoked = &Error{
		http.StatusForbidden,
		"device_revoked",
		"This device was removed from your account.",
	}
	errDeviceNotFound   = &Error{http.StatusNotFound, "device_not_found", "Device not found."}
	errComputerNotOwned = &Error{
		http.StatusForbidden,
		"computer_not_owned",
		"This computer belongs to another account.",
	}
	errComputerDisabled = &Error{
		http.StatusConflict,
		"remote_access_disabled",
		"Remote access is off on this computer.",
	}
	errPlanRequired      = &Error{http.StatusPaymentRequired, "plan_required", "This computer needs Pro."}
	errCloudPlanRequired = &Error{http.StatusPaymentRequired, "plan_required", "Hosted route is part of Pro."}
	errLinkTokenUsed     = &Error{
		http.StatusConflict,
		"link_token_used",
		"This link token was used by another computer.",
	}
	errLinkTokenInvalid = &Error{
		http.StatusUnauthorized,
		"link_token_invalid",
		"This link token is invalid or expired.",
	}
	errReservationExpired  = &Error{http.StatusConflict, "reservation_expired", "Reserve a remote computer slot again."}
	errApprovalNotFound    = &Error{http.StatusNotFound, "approval_not_found", "Approval not found."}
	errApprovalNotPending  = &Error{http.StatusConflict, "approval_not_pending", "This approval is no longer pending."}
	errApproverNotApproved = &Error{http.StatusConflict, "approver_not_approved_on_computer",
		"Approve on this computer, or from a device it already trusts."}
	errApprovalSignature = &Error{http.StatusForbidden, "approval_signature_invalid", "Invalid approval signature."}
)

func (s *Service) accountTransaction(
	ctx context.Context, id string, run func(*store.Queries, store.ControlAccount) error,
) error {
	if id == "" {
		return ErrUnauthorized
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		if err := q.LockPlanAccount(ctx, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ErrUnauthorized
			}
			return err
		}
		account, err := q.ControlAccount(ctx, id)
		if err != nil {
			return err
		}
		return run(q, account)
	})
}

func activeAccount(account store.ControlAccount) error {
	switch account.State {
	case accountActive:
		return nil
	case "suspended":
		return errAccountSuspended
	case "deleting":
		return errAccountDeleting
	default:
		return ErrUnauthorized
	}
}

func checkDevice(
	ctx context.Context, q *store.Queries, account store.ControlAccount, principal DevicePrincipal,
) (store.ControlDevice, error) {
	if err := activeAccount(account); err != nil {
		return store.ControlDevice{}, err
	}
	device, err := q.ControlDevice(ctx, account.ID, principal.DeviceID)
	if errors.Is(err, store.ErrNotFound) {
		return store.ControlDevice{}, errDeviceRevoked
	}
	if err != nil {
		return store.ControlDevice{}, err
	}
	active, err := q.ControlPrincipalActive(ctx, account.ID, device.ID, principal.JKT, principal.SessionID)
	if err != nil {
		return store.ControlDevice{}, err
	}
	if !active {
		return store.ControlDevice{}, ErrUnauthorized
	}
	return device, nil
}

func (s *Service) deviceTransaction(
	ctx context.Context, principal DevicePrincipal,
	run func(*store.Queries, store.ControlAccount, store.ControlDevice) error,
) error {
	return s.accountTransaction(ctx, principal.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		device, err := checkDevice(ctx, q, account, principal)
		if err != nil {
			return err
		}
		return run(q, account, device)
	})
}

func (s *Service) CheckDevice(ctx context.Context, principal DevicePrincipal) error {
	account, err := s.db.ControlAccount(ctx, principal.AccountID)
	if err != nil {
		return err
	}
	_, err = checkDevice(ctx, s.db.Queries, account, principal)
	return err
}

func ownedComputer(
	ctx context.Context, q *store.Queries, account string, id remoteproto.ComputerID,
) (store.ControlComputer, error) {
	computer, err := q.ControlComputer(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.ControlComputer{}, errComputerNotOwned
	}
	if err != nil {
		return store.ControlComputer{}, err
	}
	if computer.AccountID != account || !computer.UnlinkedAt.IsZero() {
		return store.ControlComputer{}, errComputerNotOwned
	}
	return computer, nil
}

func (s *Service) reconcilePlan(
	ctx context.Context, q *store.Queries, account string, now time.Time,
) (plan.Plan, error) {
	subscriptions, err := q.Subscriptions(ctx, account)
	if err != nil {
		return plan.Free, err
	}
	tier := plan.Derive(subscriptions, s.config.MonthlyPrice, s.config.AnnualPrice, now)
	return tier, q.ApplyAccountPlan(ctx, account, tier, now, false)
}

func (s *Service) securityEvent(
	ctx context.Context, q *store.Queries, account string, actor remoteproto.DeviceID, kind string,
	details map[string]any,
) error {
	return q.AppendSecurityEvent(ctx, store.SecurityEvent{
		AccountID: account, Type: kind, Actor: auditActor(actor), At: s.config.Now(), Details: details,
	})
}

func auditActor(device remoteproto.DeviceID) string {
	if device == "" {
		return auditSystem
	}
	return "account:" + string(device)
}
