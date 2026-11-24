// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	UpgradePath       = "/billing/upgrade"
	SuccessPath       = UpgradePath + "/success"
	CheckoutCancelled = "cancelled" //nolint:misspell // Public purchase return URL.
)

type Config struct {
	Metrics                   *observe.Metrics
	Stripe                    *Stripe
	Issuer, HostedDomain      string
	MonthlyPrice, AnnualPrice string
	Now                       func() time.Time
}

type Service struct {
	db  *store.Store
	cfg Config
}

func New(db *store.Store, cfg Config) *Service {
	return &Service{db: db, cfg: cfg}
}

type Failure struct {
	Status        int
	Code, Message string
}

func (f *Failure) Error() string { return f.Code }

var (
	ErrInvalidRequest = &Failure{http.StatusBadRequest, "invalid_request", "Choose a monthly or annual checkout."}
	ErrAlreadyPro     = &Failure{http.StatusConflict, "already_pro", "You're already on Pro."}
	ErrUnavailable    = &Failure{http.StatusServiceUnavailable, "cloud_unavailable",
		"Compozy Cloud is temporarily unavailable."}
	ErrUnauthorized     = &Failure{http.StatusUnauthorized, "unauthorized", "Sign in to manage billing."}
	ErrPaymentPending   = &Failure{http.StatusAccepted, "checkout_pending", "We're confirming your payment."}
	errComputerNotOwned = &Failure{
		http.StatusForbidden,
		"computer_not_owned",
		"This computer belongs to another account.",
	}
	errNoSubscription = &Failure{
		http.StatusConflict,
		"billing_subscription_missing",
		"There is no Stripe subscription to manage.",
	}
	errSuspended = &Failure{http.StatusForbidden, "account_suspended",
		"This account is suspended. Contact support@compozy.com."}
	errDeleting = &Failure{http.StatusConflict, "account_deleting",
		"Your account is being deleted. Try again in a few minutes."}
)

const (
	stripeSource         = "stripe"
	stripeStatusPaid     = "paid"
	stripeStatusCanceled = "canceled"
	stripeLimitParam     = "limit"
)

type Target struct {
	Interval string
	Hostname string
}

func (t Target) URL() string {
	q := url.Values{"interval": {t.Interval}}
	if t.Hostname != "" {
		q.Set("hostname", t.Hostname)
	}
	return UpgradePath + "?" + q.Encode()
}

func (s *Service) Target(form url.Values) (Target, error) {
	target := Target{Interval: form.Get("interval"), Hostname: form.Get("hostname")}
	if target.Interval == "" {
		target.Interval = "year"
	}
	if !s.validTarget(target) {
		return Target{}, ErrInvalidRequest
	}
	return target, nil
}

func (s *Service) validTarget(target Target) bool {
	if target.Interval != "month" && target.Interval != "year" {
		return false
	}
	if target.Hostname == "" {
		return true
	}
	label, ok := strings.CutSuffix(target.Hostname, "."+s.cfg.HostedDomain)
	return ok && s.cfg.HostedDomain != "" && label != "" && len(label) <= 63 &&
		label[0] != '-' && label[len(label)-1] != '-' &&
		!strings.ContainsFunc(label, func(r rune) bool {
			return r != '-' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
		})
}

// Authorize rechecks the credential while the account row is locked.
type Authorize func(context.Context, *store.Queries) error

type Overview struct {
	Email, ComputerName string
	HostedURL           string
	Billing             plan.Billing
	HostedUsed          int64
	HostedLimit         int32
}

func (s *Service) Overview(
	ctx context.Context,
	accountID string,
	target Target,
	authorize Authorize,
) (Overview, error) {
	var result Overview
	err := s.accountTransaction(ctx, accountID, authorize, func(q *store.Queries, account store.AuthAccount) error {
		result.Email = account.Email
		computer, err := s.ownedHostedComputer(ctx, q, accountID, target.Hostname)
		if err != nil {
			return err
		}
		if computer.ID != "" {
			result.ComputerName, result.HostedURL = computer.Name, "https://"+target.Hostname
		}
		subscriptions, err := q.Subscriptions(ctx, accountID)
		if err != nil {
			return err
		}
		result.Billing = s.status(subscriptions, s.cfg.Now())
		if result.Billing.Plan == plan.Pro {
			result.HostedLimit = account.CloudRoutesMax
			result.HostedUsed, err = q.AccountHostedRouteUsage(ctx, accountID)
		}
		return err
	})
	return result, err
}

func (s *Service) Checkout(ctx context.Context, accountID string, target Target, authorize Authorize) (string, error) {
	if !s.validTarget(target) {
		return "", ErrInvalidRequest
	}
	if s.cfg.Stripe == nil {
		return "", ErrUnavailable
	}
	price := s.cfg.AnnualPrice
	if target.Interval == "month" {
		price = s.cfg.MonthlyPrice
	}
	if price == "" {
		return "", ErrUnavailable
	}
	var checkout stripeCheckout
	err := s.accountTransaction(ctx, accountID, authorize, func(q *store.Queries, account store.AuthAccount) error {
		if _, err := s.ownedHostedComputer(ctx, q, accountID, target.Hostname); err != nil {
			return err
		}
		subscriptions, err := q.Subscriptions(ctx, accountID)
		if err != nil {
			return err
		}
		if s.status(subscriptions, s.cfg.Now()).Plan == plan.Pro {
			return ErrAlreadyPro
		}
		checkout, err = s.cfg.Stripe.createCheckout(ctx, stripeCheckoutRequest{
			AccountID: accountID, Email: account.Email, CustomerID: billingCustomer(subscriptions), PriceID: price,
			SuccessURL: s.cfg.Issuer + SuccessPath + "?session_id={CHECKOUT_SESSION_ID}",
			CancelURL:  s.cfg.Issuer + target.URL() + "&checkout=" + CheckoutCancelled,
			Key:        "checkout_" + rand.Text(),
		})
		if err != nil {
			return err
		}
		return q.InsertBillingCheckout(ctx, store.BillingCheckout{
			ID: checkout.ID, AccountID: accountID, Interval: target.Interval, Hostname: target.Hostname,
		})
	})
	if err == nil {
		s.countCheckout(target.Interval, "created")
	}
	return checkout.URL, err
}

func (s *Service) Portal(ctx context.Context, accountID string, authorize Authorize) (string, error) {
	if s.cfg.Stripe == nil {
		return "", ErrUnavailable
	}
	var target string
	err := s.accountTransaction(ctx, accountID, authorize, func(q *store.Queries, _ store.AuthAccount) error {
		subscriptions, err := q.Subscriptions(ctx, accountID)
		if err != nil {
			return err
		}
		customer := billingCustomer(subscriptions)
		if customer == "" {
			return errNoSubscription
		}
		target, err = s.cfg.Stripe.portal(ctx, customer, s.cfg.Issuer+UpgradePath)
		return err
	})
	return target, err
}

func (s *Service) CompleteCheckout(
	ctx context.Context, accountID, id string, authorize Authorize,
) (Overview, error) {
	var result Overview
	if s.cfg.Stripe == nil {
		return result, ErrUnavailable
	}
	if !validStripeID(id, "cs_") {
		return result, ErrInvalidRequest
	}
	record, err := s.db.AccountBillingCheckout(ctx, accountID, id)
	if errors.Is(err, store.ErrNotFound) {
		return result, ErrInvalidRequest
	}
	if err != nil {
		return result, err
	}
	target := Target{Hostname: record.Hostname, Interval: record.Interval}
	result, err = s.Overview(ctx, accountID, target, authorize)
	if err != nil {
		return result, err
	}
	current, err := s.cfg.Stripe.checkout(ctx, id)
	if err != nil {
		return result, err
	}
	if current.ClientReference != accountID {
		return result, ErrInvalidRequest
	}
	if current.Status != "complete" ||
		(current.PaymentStatus != stripeStatusPaid && current.PaymentStatus != "no_payment_required") {
		return result, ErrPaymentPending
	}
	if err := s.refreshSubscription(ctx, current.Subscription, stripeEvent{}, accountID); err != nil {
		return result, err
	}
	result, err = s.Overview(ctx, accountID, target, authorize)
	if err == nil && result.Billing.Plan != plan.Pro {
		err = ErrPaymentPending
	}
	return result, err
}

func (s *Service) ownedHostedComputer(
	ctx context.Context, q *store.Queries, accountID, hostname string,
) (store.ControlComputer, error) {
	if hostname == "" {
		return store.ControlComputer{}, nil
	}
	id, owner, err := q.FindBrowserComputer(ctx, hostname)
	if errors.Is(err, store.ErrAuthNotFound) || (err == nil && owner != accountID) {
		return store.ControlComputer{}, errComputerNotOwned
	}
	if err != nil {
		return store.ControlComputer{}, err
	}
	return q.ControlComputer(ctx, remoteproto.ComputerID(id))
}

func (s *Service) accountTransaction(
	ctx context.Context, accountID string, authorize Authorize, run func(*store.Queries, store.AuthAccount) error,
) error {
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, accountID)
		if errors.Is(err, store.ErrAuthNotFound) {
			return ErrUnauthorized
		}
		if err != nil {
			return err
		}
		switch account.State {
		case "suspended":
			return errSuspended
		case "deleting":
			return errDeleting
		}
		if authorize != nil {
			if err := authorize(ctx, q); err != nil {
				return err
			}
		}
		return run(q, account)
	})
}

func (s *Service) status(subscriptions []plan.Subscription, now time.Time) plan.Billing {
	return plan.BillingStatus(subscriptions, s.cfg.MonthlyPrice, s.cfg.AnnualPrice, now)
}

func billingCustomer(subscriptions []plan.Subscription) string {
	for _, sub := range subscriptions {
		if sub.Source == stripeSource && sub.CustomerID != "" {
			return sub.CustomerID
		}
	}
	return ""
}
