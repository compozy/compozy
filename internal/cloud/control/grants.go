package control

import (
	"context"
	"crypto/rand"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

type Grant struct {
	Grant     string    `json:"grant"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) IssueGrant(
	ctx context.Context, principal DevicePrincipal, id remoteproto.ComputerID, route remoteproto.RouteKind,
) (Grant, error) {
	var result Grant
	if !route.Grantable() {
		return result, ErrInvalidInput
	}
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			now := s.config.Now()
			tier, err := s.reconcilePlan(ctx, q, account.ID, now)
			if err != nil {
				return err
			}
			computer, err := ownedComputer(ctx, q, account.ID, id)
			if err != nil {
				return err
			}
			if err := admitGrant(computer, tier, route); err != nil {
				return err
			}
			result, err = s.issueGrant(ctx, q, account, device, principal.JKT, computer.ID, route, now)
			return err
		},
	)
	return result, err
}

func admitGrant(computer store.ControlComputer, tier plan.Plan, route remoteproto.RouteKind) error {
	if computer.PlanLimited {
		return &Error{errPlanRequired.Status, errPlanRequired.Code, fmt.Sprintf("%s needs Pro.", computer.Name)}
	}
	if !slices.Contains(plan.GrantRoutes(tier), route) {
		return errCloudPlanRequired
	}
	if !computer.RemoteEnabled {
		return errComputerDisabled
	}
	return nil
}

func (s *Service) issueGrant(
	ctx context.Context, q *store.Queries, account store.ControlAccount, device store.ControlDevice, jkt string,
	computer remoteproto.ComputerID, route remoteproto.RouteKind, now time.Time,
) (Grant, error) {
	scopes, err := q.EffectiveControlScopes(ctx, device, computer)
	if err != nil {
		return Grant{}, err
	}
	claims := remoteproto.GrantClaims{
		Issuer: s.config.Issuer, Audience: computer, Subject: device.ID, AccountID: account.ID,
		Cnf: remoteproto.Confirmation{JKT: jkt}, Route: route, Scopes: scopes, JTI: rand.Text(),
		IssuedAt: now.Unix(), ExpiresAt: now.Add(remoteproto.GrantTTL).Unix(), SecurityEpoch: account.SecurityEpoch,
	}
	token, err := s.config.Signer.SignGrant(claims)
	if err != nil {
		return Grant{}, err
	}
	if err := q.RecordControlGrant(ctx, claims); err != nil {
		return Grant{}, err
	}
	return Grant{Grant: token, ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC()}, nil
}
