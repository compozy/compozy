package control

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

type Computer struct {
	ID            remoteproto.ComputerID `json:"id"`
	AccountID     string                 `json:"account_id"`
	Name          string                 `json:"name"`
	Platform      string                 `json:"platform"`
	Version       string                 `json:"version"`
	TLSRootSHA256 string                 `json:"tls_root_sha256"`
	RemoteEnabled bool                   `json:"remote_enabled"`
	PlanLimited   bool                   `json:"plan_limited"`
	Online        bool                   `json:"online"`
	Endpoints     []string               `json:"endpoints"`
	LinkedAt      time.Time              `json:"linked_at"`
}

type LinkRequest struct {
	Name          string          `json:"name"`
	Platform      string          `json:"platform"`
	Version       string          `json:"version"`
	ComputerJWK   remoteproto.JWK `json:"computer_jwk"`
	TLSRootSHA256 string          `json:"tls_root_sha256"`
}

type ComputerPatch struct {
	Name          *string   `json:"name,omitempty"`
	RemoteEnabled *bool     `json:"remote_enabled,omitempty"`
	Endpoints     *[]string `json:"endpoints,omitempty"`
}

type ComputerRequestAuth struct {
	JKT      string
	JTI      string
	IssuedAt int64
}

func computerView(row store.ControlComputer, now time.Time) Computer {
	return Computer{
		ID: row.ID, AccountID: row.AccountID, Name: row.Name, Platform: row.Platform, Version: row.Version,
		TLSRootSHA256: row.TLSRootSHA256, RemoteEnabled: row.RemoteEnabled, PlanLimited: row.PlanLimited,
		Online:    row.RemoteEnabled && row.LinkID != "" && now.Before(row.LastControlAt.Add(remoteproto.LeaseTTL)),
		Endpoints: row.Endpoints, LinkedAt: row.LinkedAt,
	}
}

func (s *Service) LinkTokenAccount(ctx context.Context, raw string) (string, error) {
	if !strings.HasPrefix(raw, LinkTokenPrefix) || len(raw) > 128 {
		return "", errLinkTokenInvalid
	}
	hash := sha256.Sum256([]byte(raw))
	token, err := s.db.ControlLinkToken(ctx, hash[:])
	if errors.Is(err, store.ErrNotFound) {
		return "", errLinkTokenInvalid
	}
	return token.AccountID, err
}

func (s *Service) LinkWithToken(ctx context.Context, accountID, raw string, request LinkRequest) (Computer, error) {
	computer, err := request.computer()
	if err != nil {
		return Computer{}, err
	}
	hash := sha256.Sum256([]byte(raw))
	err = s.accountTransaction(ctx, accountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		token, err := q.ControlLinkToken(ctx, hash[:])
		if errors.Is(err, store.ErrNotFound) {
			return errLinkTokenInvalid
		}
		if err != nil {
			return err
		}
		if token.AccountID != account.ID {
			return errLinkTokenInvalid
		}
		if _, err := findDevice(ctx, q, account.ID, token.DeviceID); err != nil {
			return err
		}
		computer, err = s.redeemLinkToken(ctx, q, token, hash[:], computer)
		return err
	})
	return computerView(computer, s.config.Now()), err
}

func (s *Service) redeemLinkToken(
	ctx context.Context, q *store.Queries, token store.ControlLinkToken, hash []byte, computer store.ControlComputer,
) (store.ControlComputer, error) {
	jkt, err := computer.Key.Thumbprint()
	if err != nil {
		return store.ControlComputer{}, ErrInvalidInput
	}
	if !token.UsedAt.IsZero() {
		if token.UsedByJKT != jkt {
			return store.ControlComputer{}, errLinkTokenUsed
		}
		return q.ControlComputer(ctx, token.ComputerID)
	}
	if !s.config.Now().Before(token.ExpiresAt) {
		return store.ControlComputer{}, errLinkTokenInvalid
	}
	computer.AccountID = token.AccountID
	result, err := s.linkComputer(ctx, q, computer, token.DeviceID, token.ReservationID)
	if err != nil {
		return store.ControlComputer{}, err
	}
	return result, q.UseControlLinkToken(ctx, hash, result.ID, jkt, result.LinkedAt)
}

func (s *Service) LinkWithDevice(
	ctx context.Context, principal DevicePrincipal, request LinkRequest, proof ComputerRequestAuth,
) (Computer, error) {
	computer, err := request.computer()
	if err != nil {
		return Computer{}, err
	}
	if err := s.consumeComputerProof(ctx, computer, proof); err != nil {
		return Computer{}, err
	}
	err = s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			computer.AccountID = account.ID
			computer, err = s.linkComputer(ctx, q, computer, device.ID, "")
			return err
		},
	)
	return computerView(computer, s.config.Now()), err
}

func (r LinkRequest) computer() (store.ControlComputer, error) {
	public, err := r.ComputerJWK.PublicKey()
	if _, ok := public.(ed25519.PublicKey); err != nil || !ok {
		return store.ControlComputer{}, ErrInvalidInput
	}
	root, err := hex.DecodeString(r.TLSRootSHA256)
	if err != nil || len(root) != sha256.Size {
		return store.ControlComputer{}, ErrInvalidInput
	}
	name := strings.TrimSpace(r.Name)
	if name == "" || len(name) > 128 || !validPlatform(r.Platform) || r.Version == "" || len(r.Version) > 64 {
		return store.ControlComputer{}, ErrInvalidInput
	}
	return store.ControlComputer{
		Name: name, Platform: r.Platform, Version: r.Version, Key: r.ComputerJWK,
		TLSRootSHA256: strings.ToLower(r.TLSRootSHA256),
	}, nil
}

func validPlatform(platform string) bool {
	switch platform {
	case "linux_x86_64", "linux_arm64", "darwin_x86_64", "darwin_arm64", "windows_x86_64":
		return true
	default:
		return false
	}
}

func (s *Service) linkComputer(
	ctx context.Context,
	q *store.Queries,
	computer store.ControlComputer,
	creator remoteproto.DeviceID,
	reservation string,
) (store.ControlComputer, error) {
	now := s.config.Now()
	tier, err := s.reconcilePlan(ctx, q, computer.AccountID, now)
	if err != nil {
		return store.ControlComputer{}, err
	}
	if reservation != "" {
		consumed, err := q.ConsumeControlReservation(ctx, reservation, computer.AccountID, creator, now)
		if err != nil {
			return store.ControlComputer{}, err
		}
		if !consumed {
			return store.ControlComputer{}, errReservationExpired
		}
	}
	if err := admitSlot(ctx, q, computer.AccountID, tier, now); err != nil {
		return store.ControlComputer{}, err
	}
	computer.LinkedAt = now
	for range 3 {
		computer.ID = remoteproto.ComputerID("cmp_" + strings.ToLower(rand.Text()[:8]))
		result, err := q.CreateControlComputer(ctx, computer)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return store.ControlComputer{}, err
		}
		if err := s.securityEvent(ctx, q, computer.AccountID, creator, "computer_linked",
			map[string]any{auditComputerID: result.ID}); err != nil {
			return store.ControlComputer{}, err
		}
		return result, notifyComputerChanged(ctx, q, result)
	}
	return store.ControlComputer{}, errors.New("could not allocate a computer ID")
}

func (s *Service) ListComputers(ctx context.Context, principal DevicePrincipal) ([]Computer, error) {
	var computers []Computer
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, _ store.ControlDevice) error {
			now := s.config.Now()
			if _, err := s.reconcilePlan(ctx, q, account.ID, now); err != nil {
				return err
			}
			rows, err := q.ListControlComputers(ctx, account.ID)
			if err != nil {
				return err
			}
			computers = make([]Computer, 0, len(rows))
			for _, row := range rows {
				computers = append(computers, computerView(row, now))
			}
			return nil
		},
	)
	return computers, err
}

func (s *Service) PatchComputer(
	ctx context.Context, principal DevicePrincipal, id remoteproto.ComputerID, patch ComputerPatch,
) (Computer, error) {
	var result Computer
	if patch.Endpoints != nil {
		return result, ErrInvalidInput
	}
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, _ store.ControlDevice) error {
			computer, err := ownedComputer(ctx, q, account.ID, id)
			if err != nil {
				return err
			}
			result, err = s.patchComputer(ctx, q, computer, patch, principal.DeviceID)
			return err
		},
	)
	return result, err
}

func (s *Service) ComputerIdentity(ctx context.Context, id remoteproto.ComputerID) (store.ControlComputer, error) {
	computer, err := s.db.ControlComputer(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.ControlComputer{}, ErrUnauthorized
	}
	return computer, err
}

func (s *Service) PatchComputerWithKey(
	ctx context.Context, computer store.ControlComputer, proof ComputerRequestAuth, patch ComputerPatch,
) (Computer, error) {
	var result Computer
	if err := s.consumeComputerProof(ctx, computer, proof); err != nil {
		return result, err
	}
	err := s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		current, err := ownedComputer(ctx, q, account.ID, computer.ID)
		if err != nil {
			return err
		}
		result, err = s.patchComputer(ctx, q, current, patch, "")
		return err
	})
	return result, err
}

func (s *Service) consumeComputerProof(
	ctx context.Context,
	computer store.ControlComputer,
	proof ComputerRequestAuth,
) error {
	jkt, err := computer.Key.Thumbprint()
	if err != nil || proof.JKT != jkt || proof.JTI == "" {
		return ErrProofInvalid
	}
	fresh, err := s.db.UseComputerProof(ctx, jkt, proof.JTI, time.Unix(proof.IssuedAt, 0).Add(remoteproto.DPoPLeeway))
	if err != nil {
		return err
	}
	if !fresh {
		return errProofReplayed
	}
	return nil
}

func (s *Service) patchComputer(
	ctx context.Context,
	q *store.Queries,
	computer store.ControlComputer,
	patch ComputerPatch,
	actor remoteproto.DeviceID,
) (Computer, error) {
	now := s.config.Now()
	tier, err := s.reconcilePlan(ctx, q, computer.AccountID, now)
	if err != nil {
		return Computer{}, err
	}
	updated, err := applyComputerPatch(ctx, q, computer, patch, now)
	if err != nil {
		return Computer{}, err
	}
	if updated.RemoteEnabled != computer.RemoteEnabled {
		if updated.RemoteEnabled {
			if err := admitSlot(ctx, q, computer.AccountID, tier, now); err != nil {
				return Computer{}, err
			}
		}
		if err := q.AppendSecurityEvent(ctx, store.SecurityEvent{
			AccountID: computer.AccountID,
			Actor:     auditActor(actor),
			Type:      "computer_remote_access_changed",
			At:        now,
			Details:   map[string]any{auditComputerID: computer.ID, "remote_enabled": updated.RemoteEnabled},
		}); err != nil {
			return Computer{}, err
		}
	}
	return s.saveComputerPatch(ctx, q, computer, updated, patch.Endpoints != nil, now)
}

func applyComputerPatch(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, patch ComputerPatch, now time.Time,
) (store.ControlComputer, error) {
	if patch.Name != nil {
		computer.Name = strings.TrimSpace(*patch.Name)
		if computer.Name == "" || len(computer.Name) > 128 {
			return store.ControlComputer{}, ErrInvalidInput
		}
	}
	if patch.RemoteEnabled != nil {
		computer.RemoteEnabled = *patch.RemoteEnabled
	}
	if patch.Endpoints != nil {
		raw := *patch.Endpoints
		if len(raw) > 16 || slices.ContainsFunc(raw, invalidEndpoint) {
			return store.ControlComputer{}, ErrInvalidInput
		}
		computer.Endpoints = slices.Compact(slices.Sorted(slices.Values(raw)))
		if err := q.SetControlEndpoints(ctx, computer.ID, computer.Endpoints, now); err != nil {
			return store.ControlComputer{}, err
		}
	}
	return computer, nil
}

func (s *Service) saveComputerPatch(
	ctx context.Context, q *store.Queries, before, updated store.ControlComputer, endpointsChanged bool, now time.Time,
) (Computer, error) {
	row, err := q.UpdateControlComputer(ctx, updated)
	if err != nil {
		return Computer{}, err
	}
	if before.RemoteEnabled != row.RemoteEnabled || before.Name != row.Name || endpointsChanged {
		if _, err := s.reconcilePlan(ctx, q, row.AccountID, now); err != nil {
			return Computer{}, err
		}
		if err := notifyComputerChanged(ctx, q, row); err != nil {
			return Computer{}, err
		}
		if before.Name != row.Name || before.RemoteEnabled != row.RemoteEnabled {
			if err := q.NotifyControlEvent(ctx, store.ControlEvent{
				Type: remoteproto.ControlPeers, AccountID: row.AccountID,
			}); err != nil {
				return Computer{}, err
			}
		}
		row, err = q.ControlComputer(ctx, row.ID)
		if err != nil {
			return Computer{}, err
		}
	}
	row.Endpoints = append(row.Endpoints, updated.Endpoints...)
	return computerView(row, now), nil
}

func invalidEndpoint(address string) bool {
	host, port, err := net.SplitHostPort(address)
	number, portErr := strconv.ParseUint(port, 10, 16)
	return err != nil || portErr != nil || number == 0 || host == "" || len(address) > 255 ||
		strings.ContainsAny(host, " /\\?#@\r\n\t")
}

func (s *Service) UnlinkComputer(ctx context.Context, principal DevicePrincipal, id remoteproto.ComputerID) error {
	return s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			computer, err := ownedComputer(ctx, q, account.ID, id)
			if err != nil {
				return err
			}
			now := s.config.Now()
			if err := q.UnlinkControlComputer(ctx, computer, now); err != nil {
				return err
			}
			if _, err := s.reconcilePlan(ctx, q, account.ID, now); err != nil {
				return err
			}
			if err := s.securityEvent(ctx, q, account.ID, device.ID, "computer_unlinked",
				map[string]any{auditComputerID: id}); err != nil {
				return err
			}
			_, err = q.AdvanceSecurity(ctx, store.ControlEvent{
				Type: remoteproto.ControlUnlink, AccountID: account.ID, ComputerID: id,
			})
			return err
		},
	)
}

func notifyComputerChanged(ctx context.Context, q *store.Queries, computer store.ControlComputer) error {
	return q.NotifyControlEvent(ctx, store.ControlEvent{
		Type: remoteproto.ControlComputersChanged, AccountID: computer.AccountID, ComputerID: computer.ID,
	})
}
