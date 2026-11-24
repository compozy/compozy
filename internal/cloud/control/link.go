package control

import (
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const LeaseRefresh = 5 * time.Minute

var (
	ErrLinkReplaced   = errors.New("computer control link was replaced")
	ErrSnapshotNeeded = errors.New("computer control link requires a new snapshot")
)

type Link struct {
	ComputerID remoteproto.ComputerID
	AccountID  string
	id         string
}

func (s *Service) OpenLink(ctx context.Context, identity store.ControlComputer) (Link, error) {
	link := Link{ComputerID: identity.ID, AccountID: identity.AccountID, id: rand.Text()}
	err := s.accountTransaction(ctx, link.AccountID, func(q *store.Queries, _ store.ControlAccount) error {
		current, err := q.ControlComputer(ctx, link.ComputerID)
		if err != nil || !current.UnlinkedAt.IsZero() {
			return err
		}
		if current.AccountID != link.AccountID {
			return errComputerNotOwned
		}
		if err := q.OpenComputerControlLink(ctx, current, link.id, s.config.Now()); err != nil {
			return err
		}
		if current.LinkID == "" {
			return notifyComputerChanged(ctx, q, current)
		}
		return q.NotifyControlEvent(ctx, store.ControlEvent{
			Type: remoteproto.ControlLinkReplaced, AccountID: link.AccountID, ComputerID: link.ComputerID,
			ResourceID: link.id,
		})
	})
	return link, err
}

func (s *Service) CloseLink(ctx context.Context, link Link) error {
	return s.accountTransaction(ctx, link.AccountID, func(q *store.Queries, _ store.ControlAccount) error {
		closed, err := q.CloseComputerControlLink(ctx, link.ComputerID, link.id)
		if err != nil || !closed {
			return err
		}
		return q.NotifyControlEvent(ctx, store.ControlEvent{
			Type: remoteproto.ControlComputersChanged, AccountID: link.AccountID, ComputerID: link.ComputerID,
		})
	})
}

func (s *Service) linkTransaction(
	ctx context.Context, link Link,
	run func(*store.Queries, store.ControlAccount, store.ControlComputer) error,
) error {
	return s.accountTransaction(ctx, link.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		computer, err := q.ControlComputer(ctx, link.ComputerID)
		if err != nil {
			return err
		}
		if computer.AccountID != account.ID {
			return errComputerNotOwned
		}
		if computer.UnlinkedAt.IsZero() && computer.LinkID != link.id {
			return ErrLinkReplaced
		}
		return run(q, account, computer)
	})
}

func (s *Service) LinkSnapshot(ctx context.Context, link Link) (remoteproto.SecuritySnapshot, error) {
	var snapshot remoteproto.SecuritySnapshot
	err := s.linkTransaction(
		ctx,
		link,
		func(q *store.Queries, account store.ControlAccount, computer store.ControlComputer) error {
			snapshot = remoteproto.SecuritySnapshot{
				AccountID: account.ID, SecurityEpoch: account.SecurityEpoch,
				Linked: computer.UnlinkedAt.IsZero(), Devices: []remoteproto.SnapshotDevice{},
			}
			if snapshot.Linked && account.State == accountActive {
				devices, err := q.ControlSnapshotDevices(ctx, account.ID, computer.ID)
				if err != nil {
					return err
				}
				snapshot.Devices = devices
			}
			return nil
		},
	)
	return snapshot, err
}

func (s *Service) SignedSnapshot(snapshot remoteproto.SecuritySnapshot) (string, error) {
	return s.config.Signer.SignSnapshot(snapshot)
}

func (s *Service) LinkLease(ctx context.Context, link Link, applied uint64) (string, error) {
	var token string
	err := s.linkTransaction(
		ctx,
		link,
		func(q *store.Queries, account store.ControlAccount, computer store.ControlComputer) error {
			if !computer.UnlinkedAt.IsZero() || applied != account.SecurityEpoch {
				return ErrSnapshotNeeded
			}
			if account.State == accountSuspended {
				return nil
			}
			now := s.config.Now()
			lease, err := s.issueLease(ctx, q, account, computer, link.id, now)
			if err != nil {
				return err
			}
			token, err = s.config.Signer.SignLease(lease)
			return err
		},
	)
	return token, err
}

func (s *Service) issueLease(
	ctx context.Context, q *store.Queries, account store.ControlAccount, computer store.ControlComputer,
	linkID string, now time.Time,
) (remoteproto.EntitlementLease, error) {
	tier := plan.Free
	if account.State == accountActive {
		var err error
		tier, err = s.reconcilePlan(ctx, q, account.ID, now)
		if err != nil {
			return remoteproto.EntitlementLease{}, err
		}
		computer, err = q.ControlComputer(ctx, computer.ID)
		if err != nil {
			return remoteproto.EntitlementLease{}, err
		}
	}
	sequence, err := q.AdvanceControlLease(ctx, computer, linkID, now)
	if err != nil {
		return remoteproto.EntitlementLease{}, err
	}
	routes := []remoteproto.RouteKind{}
	if account.State == accountActive && computer.RemoteEnabled && !computer.PlanLimited {
		routes = plan.GrantRoutes(tier)
	}
	return remoteproto.EntitlementLease{
		ComputerID: computer.ID, AccountID: account.ID, Plan: string(tier), PlanLimited: computer.PlanLimited,
		AllowedRoutes: routes, Sequence: sequence, ValidUntil: now.Add(remoteproto.LeaseTTL).Unix(),
		SecurityEpoch: account.SecurityEpoch,
	}, nil
}

func (s *Service) LinkPeers(ctx context.Context, link Link) ([]remoteproto.HostedPeer, error) {
	peers := []remoteproto.HostedPeer{}
	err := s.linkTransaction(
		ctx,
		link,
		func(q *store.Queries, account store.ControlAccount, computer store.ControlComputer) error {
			if account.State != accountActive || !computer.UnlinkedAt.IsZero() || !computer.RemoteEnabled {
				return nil
			}
			now := s.config.Now()
			subscriptions, err := q.Subscriptions(ctx, account.ID)
			if err != nil {
				return err
			}
			if plan.Derive(subscriptions, s.config.MonthlyPrice, s.config.AnnualPrice, now) == plan.Pro {
				peers, err = q.ListControlPeers(ctx, computer, now)
			}
			return err
		},
	)
	return peers, err
}

func (s *Service) LinkApprovals(ctx context.Context, link Link) ([]string, error) {
	var envelopes []string
	err := s.linkTransaction(
		ctx,
		link,
		func(q *store.Queries, account store.ControlAccount, computer store.ControlComputer) error {
			if account.State != accountActive || !computer.UnlinkedAt.IsZero() || !computer.RemoteEnabled {
				return nil
			}
			var err error
			envelopes, err = q.UndeliveredCloudApprovals(ctx, computer, s.config.Now().Add(-remoteproto.ApprovalTTL))
			return err
		},
	)
	return envelopes, err
}

func (s *Service) AcceptLinkReport(ctx context.Context, link Link, message remoteproto.ControlMessage) error {
	return s.linkTransaction(
		ctx,
		link,
		func(q *store.Queries, account store.ControlAccount, computer store.ControlComputer) error {
			if account.State != accountActive || !computer.UnlinkedAt.IsZero() || !computer.RemoteEnabled {
				return nil
			}
			return s.acceptReport(ctx, q, computer, message)
		},
	)
}

func (s *Service) acceptReport(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, message remoteproto.ControlMessage,
) error {
	now := s.config.Now()
	switch message.Type {
	case remoteproto.ControlSessionMinted:
		var mint remoteproto.SessionMinted
		if err := json.Unmarshal(message.Data, &mint); err != nil || mint.At <= 0 ||
			mint.At > now.Add(remoteproto.GrantLeeway).Unix() || mint.DeviceID == "" || mint.JKT == "" {
			return ErrInvalidInput
		}
		mint.At = min(now.Unix(), mint.At)
		return q.RecordControlMint(ctx, computer, mint)
	case remoteproto.ControlApprovalPending:
		var pending remoteproto.ApprovalPending
		if err := json.Unmarshal(message.Data, &pending); err != nil {
			return ErrInvalidInput
		}
		return s.reportApprovalPending(ctx, q, computer, pending)
	case remoteproto.ControlApprovalApplied:
		var applied remoteproto.ApprovalApplied
		if err := json.Unmarshal(message.Data, &applied); err != nil {
			return ErrInvalidInput
		}
		return s.reportApprovalApplied(ctx, q, computer, applied)
	case remoteproto.ControlApprovedKeys:
		var keys remoteproto.ApprovedKeys
		if err := json.Unmarshal(message.Data, &keys); err != nil || keys.JKTs == nil {
			return ErrInvalidInput
		}
		return q.SetComputerApprovedKeys(ctx, computer, keys.JKTs, now)
	default:
		return ErrInvalidInput
	}
}
