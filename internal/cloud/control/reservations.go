// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

const (
	reservationTTL  = 15 * time.Minute
	linkTokenTTL    = 10 * time.Minute
	LinkTokenPrefix = "cpl_"
)

var ErrPlanLimit = &Error{http.StatusPaymentRequired, "plan_limit_reached",
	"Free includes 1 remote computer. Upgrade to Pro for unlimited computers."}

var errReservationLinked = &Error{http.StatusConflict, "computer_setup_linked",
	"This computer is already linked. Remove it instead."}

type LinkToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) Reserve(
	ctx context.Context,
	principal DevicePrincipal,
	previous string,
) (store.SlotReservation, error) {
	var result store.SlotReservation
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			now := s.config.Now()
			tier, err := s.reconcilePlan(ctx, q, account.ID, now)
			if err != nil {
				return err
			}
			if previous != "" {
				// A committed link has already consumed this setup's slot.
				id, err := q.LinkedControlReservation(ctx, previous, account.ID, device.ID)
				if !errors.Is(err, store.ErrNotFound) {
					result = store.SlotReservation{ID: previous, ComputerID: id}
					return err
				}
				reservation, err := q.ActiveControlReservation(ctx, previous, account.ID, device.ID, now)
				if !errors.Is(err, store.ErrNotFound) {
					result = reservation
					return err
				}
			}
			if err := admitSlot(ctx, q, account.ID, tier, now); err != nil {
				return err
			}
			result, err = q.CreateControlReservation(ctx, account.ID, device.ID, store.SlotReservation{
				ID: "rsv_" + rand.Text(), ExpiresAt: now.Add(reservationTTL).UTC().Truncate(time.Microsecond),
			})
			return err
		},
	)
	if errors.Is(err, ErrPlanLimit) {
		s.config.Metrics.Paywall(ErrPlanLimit.Code)
	}
	return result, err
}

func (s *Service) ReleaseReservation(ctx context.Context, principal DevicePrincipal, id string) error {
	return s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			if _, err := q.LinkedControlReservation(ctx, id, account.ID, device.ID); err == nil {
				return errReservationLinked
			} else if !errors.Is(err, store.ErrNotFound) {
				return err
			}
			return q.ReleaseControlReservation(ctx, id, account.ID, device.ID, s.config.Now())
		},
	)
}

func admitSlot(ctx context.Context, q *store.Queries, account string, tier plan.Plan, now time.Time) error {
	usage, err := q.PlanUsage(ctx, account, now)
	if err != nil {
		return err
	}
	if errors.Is(plan.CanReserve(tier, usage), plan.ErrLimitReached) {
		return ErrPlanLimit
	}
	return nil
}

func (s *Service) CreateLinkToken(
	ctx context.Context,
	principal DevicePrincipal,
	reservation string,
) (LinkToken, error) {
	var result LinkToken
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			now := s.config.Now()
			if reservation != "" {
				_, err := q.ActiveControlReservation(ctx, reservation, account.ID, device.ID, now)
				if errors.Is(err, store.ErrNotFound) {
					return errReservationExpired
				}
				if err != nil {
					return err
				}
			}
			result = LinkToken{Token: LinkTokenPrefix + rand.Text(), ExpiresAt: now.Add(linkTokenTTL).UTC()}
			hash := sha256.Sum256([]byte(result.Token))
			return q.CreateControlLinkToken(ctx, hash[:], store.ControlLinkToken{
				AccountID: account.ID, DeviceID: device.ID, ReservationID: reservation, ExpiresAt: result.ExpiresAt,
			})
		},
	)
	return result, err
}
