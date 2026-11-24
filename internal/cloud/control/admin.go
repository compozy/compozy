// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/remoteproto"
)

var errAdminResourceChanged = &Error{
	http.StatusConflict, "admin_resource_changed", "This resource changed. Refresh the account and confirm again.",
}

func (s *Service) AdminRevokeDevice(
	ctx context.Context, accountID, operatorEmail string, id remoteproto.DeviceID,
) (int64, error) {
	actor, err := adminActor(operatorEmail)
	if err != nil {
		return 0, err
	}
	var eventID int64
	err = s.accountTransaction(ctx, accountID, func(q *store.Queries, account store.ControlAccount) error {
		if account.State == accountDeleting {
			return errAccountDeleting
		}
		if _, err := findDevice(ctx, q, accountID, id); err != nil {
			return err
		}
		eventID, err = s.revokeDevice(ctx, q, accountID, id, actor)
		return err
	})
	return eventID, err
}

func (s *Service) AdminRevokeSession(
	ctx context.Context, accountID, operatorEmail, sessionID string,
) (int64, error) {
	return s.adminAccountChange(ctx, accountID, operatorEmail, "device_session_revoked",
		map[string]any{"session_id": sessionID}, func(q *store.Queries) (bool, error) {
			updated, err := q.RevokeAdminDeviceSession(ctx, accountID, sessionID, s.config.Now())
			if err != nil || !updated {
				return updated, err
			}
			return true, q.NotifyControlEvent(ctx, store.ControlEvent{
				Type: remoteproto.ControlDevicesChanged, AccountID: accountID,
			})
		})
}

func (s *Service) AdminSetHostedRouteCap(
	ctx context.Context, accountID, operatorEmail string, routeCap int32,
) (int64, error) {
	if routeCap < 0 {
		return 0, ErrInvalidInput
	}
	return s.adminAccountChange(ctx, accountID, operatorEmail, "hosted_route_cap_changed",
		map[string]any{"cloud_routes_max": routeCap}, func(q *store.Queries) (bool, error) {
			current, err := q.AdminAccount(ctx, accountID)
			if err != nil {
				return false, err
			}
			if routeCap < current.CloudRoutesMax {
				return false, errAdminResourceChanged
			}
			return q.SetAdminHostedRouteCap(ctx, accountID, routeCap)
		})
}

func (s *Service) AdminDetachIdentity(
	ctx context.Context, accountID, operatorEmail, provider, subject, request string,
) (int64, error) {
	request = strings.TrimSpace(request)
	if (provider != "google" && provider != "github") || subject == "" || len(subject) > 256 ||
		!safeWrittenRequest(request) {
		return 0, ErrInvalidInput
	}
	return s.adminAccountChange(ctx, accountID, operatorEmail, "identity_detached",
		map[string]any{"provider": provider, "subject": subject, "written_request": request},
		func(q *store.Queries) (bool, error) {
			return q.DetachAdminIdentity(ctx, accountID, provider, subject)
		})
}

func safeWrittenRequest(request string) bool {
	if request == "" || len(request) > 4096 || redact.New(redact.Options{}).RedactString(request) != request {
		return false
	}
	for word := range strings.FieldsFuncSeq(request, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(`"'(){}<>,;=/?`, r)
	}) {
		word = strings.TrimRight(word, ".")
		if _, err := netip.ParseAddr(strings.Trim(word, "[]")); err == nil {
			return false
		}
		if _, err := netip.ParseAddrPort(word); err == nil {
			return false
		}
	}
	return true
}

func (s *Service) adminAccountChange(
	ctx context.Context, accountID, operatorEmail, kind string, details map[string]any,
	change func(*store.Queries) (bool, error),
) (int64, error) {
	actor, err := adminActor(operatorEmail)
	if err != nil {
		return 0, err
	}
	var eventID int64
	err = s.accountTransaction(ctx, accountID, func(q *store.Queries, account store.ControlAccount) error {
		if account.State == accountDeleting {
			return errAccountDeleting
		}
		changed, err := change(q)
		if err != nil || !changed {
			return err
		}
		eventID, err = q.AppendSecurityEventID(ctx, store.SecurityEvent{
			AccountID: accountID, Actor: actor, At: s.config.Now(), Type: kind, Details: details,
		})
		return err
	})
	return eventID, err
}

func (s *Service) AdminTakeDownTunnel(
	ctx context.Context, target store.AdminTunnel, operatorEmail string,
) (int64, error) {
	if s.config.HostedTunnels == nil {
		return 0, errHostedUnavailable
	}
	eventID, err := s.adminAccountChange(
		ctx,
		target.AccountID,
		operatorEmail,
		"hosted_tunnel_takedown",
		map[string]any{
			auditComputerID: target.ComputerID,
			"hostname":      target.Hostname,
			"generation":    target.Generation,
		},
		func(q *store.Queries) (bool, error) {
			tunnel, err := q.LockHostedTunnel(ctx, target.ComputerID)
			if errors.Is(err, store.ErrNotFound) {
				return false, errAdminResourceChanged
			}
			if err != nil {
				return false, err
			}
			if tunnel.TunnelRef != target.TunnelRef || tunnel.Hostname != target.Hostname {
				return false, errAdminResourceChanged
			}
			if tunnel.TakenDown {
				return false, nil
			}
			computer, err := q.ControlComputer(ctx, tunnel.ComputerID)
			if err != nil {
				return false, err
			}
			if tunnel.State != tunnelDeleted && tunnel.State != tunnelDeleting {
				if err := markHostedDeleting(ctx, q, computer, tunnel); err != nil {
					return false, err
				}
			}
			return true, q.TakeDownAdminTunnel(ctx, tunnel.TunnelRef, s.config.Now())
		},
	)
	// A committed deletion is visible to the existing sweeper even if this attempt is interrupted.
	if err == nil {
		s.finishHostedDelete(ctx, target.TunnelRef)
	}
	return eventID, err
}
