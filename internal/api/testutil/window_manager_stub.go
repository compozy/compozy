package testutil

import (
	"context"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/windowmanager"
)

// SingleProfileWindowManagers shares one manager across profile selections in
// transport tests while preserving the provider's profile-aware interface.
type SingleProfileWindowManagers struct {
	Manager *windowmanager.Manager
}

func (s SingleProfileWindowManagers) WindowManagerFor(string) (core.WindowManagerService, error) {
	return s.Manager, nil
}

func (s SingleProfileWindowManagers) ClaimClient(
	ctx context.Context,
	_ string,
	registration windowmanager.ClientRegistration,
) (windowmanager.ClientView, error) {
	return s.Manager.RegisterClient(ctx, registration)
}
