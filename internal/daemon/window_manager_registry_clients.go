package daemon

import (
	"context"
	"errors"
	"slices"
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

// clientProfileScope names whose attached clients a lookup may reach. Clients
// are registered per profile, so a lookup reaches one profile's manager unless
// the caller explicitly asked for the All profiles aggregate.
type clientProfileScope struct {
	profileID   string
	allProfiles bool
}

func clientsOfProfile(profileID string) clientProfileScope {
	return clientProfileScope{profileID: strings.TrimSpace(profileID)}
}

func clientsOfAllProfiles() clientProfileScope {
	return clientProfileScope{allProfiles: true}
}

// clientRuntimes returns the live runtimes inside scope. A profile without a
// live window manager has no attached clients, so it yields no runtimes rather
// than starting one.
func (r *windowManagerRegistry) clientRuntimes(scope clientProfileScope) ([]*windowManagerProfileRuntime, error) {
	if scope.allProfiles {
		return r.liveRuntimes(), nil
	}
	if scope.profileID == "" {
		return nil, errors.New("daemon: window-manager client profile is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	runtime := r.runtimes[scope.profileID]
	if runtime == nil {
		return nil, nil
	}
	return []*windowManagerProfileRuntime{runtime}, nil
}

// ClientsInWorkspace lists the clients attached to the managers inside scope.
func (r *windowManagerRegistry) ClientsInWorkspace(
	ctx context.Context,
	scope clientProfileScope,
	workspaceID windowmanager.WorkspaceID,
) ([]windowmanager.ClientView, error) {
	runtimes, err := r.clientRuntimes(scope)
	if err != nil {
		return nil, err
	}
	views := make([]windowmanager.ClientView, 0)
	for _, runtime := range runtimes {
		attached, err := runtime.manager.Clients(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		views = append(views, attached...)
	}
	return views, nil
}

// CommandClientsInWorkspace lists the command-capable clients inside scope.
func (r *windowManagerRegistry) CommandClientsInWorkspace(
	ctx context.Context,
	scope clientProfileScope,
	workspaceID windowmanager.WorkspaceID,
) ([]windowmanager.ClientView, error) {
	runtimes, err := r.clientRuntimes(scope)
	if err != nil {
		return nil, err
	}
	views := make([]windowmanager.ClientView, 0)
	for _, runtime := range runtimes {
		connected, err := runtime.manager.CommandClients(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		views = append(views, connected...)
	}
	return views, nil
}

// ManagerForClient resolves the manager inside scope that owns one client. A
// client registered under a profile outside scope is not found.
func (r *windowManagerRegistry) ManagerForClient(
	ctx context.Context,
	scope clientProfileScope,
	workspaceID windowmanager.WorkspaceID,
	clientID windowmanager.ClientID,
) (*windowmanager.Manager, error) {
	runtimes, err := r.clientRuntimes(scope)
	if err != nil {
		return nil, err
	}
	for _, runtime := range runtimes {
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
	slices.Sort(otherIDs)
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
