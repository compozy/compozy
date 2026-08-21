package daemon

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/windowmanager"
)

// clientClaimKey identifies the attachment a claim competes for. The profile is
// intentionally absent: claims for different profiles must contend.
type clientClaimKey struct {
	workspaceID windowmanager.WorkspaceID
	clientID    windowmanager.ClientID
}

type clientClaim struct {
	sync.Mutex
	waiters int
}

// ClaimClient attaches one client id to one profile atomically.
//
// Retiring and registering as separate calls allows two competing registrations
// to both retire before either registers. Serializing the complete operation per
// (workspace, client) makes the last claim the only surviving attachment.
func (r *windowManagerRegistry) ClaimClient(
	ctx context.Context,
	profileID string,
	registration windowmanager.ClientRegistration,
) (windowmanager.ClientView, error) {
	clientID := windowmanager.ClientID(strings.TrimSpace(string(registration.ClientID)))
	if clientID == "" {
		manager, err := r.For(profileID)
		if err != nil {
			return windowmanager.ClientView{}, err
		}
		return manager.RegisterClient(ctx, registration)
	}

	key := clientClaimKey{workspaceID: registration.WorkspaceID, clientID: clientID}
	claim := r.claimFor(key)
	claim.Lock()
	defer r.releaseClaim(key, claim)

	manager, err := r.For(profileID)
	if err != nil {
		return windowmanager.ClientView{}, err
	}
	if err := r.retireClientElsewhere(ctx, registration.WorkspaceID, clientID, profileID); err != nil {
		return windowmanager.ClientView{}, err
	}
	return manager.RegisterClient(ctx, registration)
}

// ClientsInWorkspace unions the clients attached across every live profile.
func (r *windowManagerRegistry) ClientsInWorkspace(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) ([]windowmanager.ClientView, error) {
	views := make([]windowmanager.ClientView, 0)
	for _, runtime := range r.liveRuntimes() {
		attached, err := runtime.manager.Clients(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		views = append(views, attached...)
	}
	return views, nil
}

// ManagerForClient resolves the sole live manager that owns one client.
func (r *windowManagerRegistry) ManagerForClient(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
	clientID windowmanager.ClientID,
) (*windowmanager.Manager, error) {
	for _, runtime := range r.liveRuntimes() {
		attached, err := runtime.manager.Clients(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for index := range attached {
			if attached[index].ClientID == clientID {
				return runtime.manager, nil
			}
		}
	}
	return nil, windowmanager.ErrClientNotFound
}

func (r *windowManagerRegistry) retireClientElsewhere(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
	clientID windowmanager.ClientID,
	profileID string,
) error {
	keep := strings.TrimSpace(profileID)
	r.mu.Lock()
	otherIDs := make([]string, 0, len(r.runtimes))
	for id := range r.runtimes {
		if id != keep {
			otherIDs = append(otherIDs, id)
		}
	}
	sort.Strings(otherIDs)
	others := make([]*windowManagerProfileRuntime, 0, len(otherIDs))
	for _, id := range otherIDs {
		others = append(others, r.runtimes[id])
	}
	r.mu.Unlock()

	var errs []error
	for _, runtime := range others {
		err := runtime.manager.UnregisterClient(ctx, workspaceID, clientID)
		if err != nil && !errors.Is(err, windowmanager.ErrClientNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *windowManagerRegistry) claimFor(key clientClaimKey) *clientClaim {
	r.mu.Lock()
	defer r.mu.Unlock()
	claim := r.claims[key]
	if claim == nil {
		claim = &clientClaim{}
		r.claims[key] = claim
	}
	claim.waiters++
	return claim
}

func (r *windowManagerRegistry) releaseClaim(key clientClaimKey, claim *clientClaim) {
	claim.Unlock()
	r.mu.Lock()
	claim.waiters--
	if claim.waiters == 0 && r.claims[key] == claim {
		delete(r.claims, key)
	}
	r.mu.Unlock()
}
