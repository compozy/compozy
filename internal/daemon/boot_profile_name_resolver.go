package daemon

import (
	"context"
	"errors"

	profilepkg "github.com/compozy/compozy/internal/profile"
)

type bootProfileNameResolver struct {
	state *bootState
}

type bootProfileCatalog struct {
	state *bootState
}

func (c bootProfileCatalog) List(ctx context.Context) ([]profilepkg.WithCounts, error) {
	if c.state == nil || c.state.profiles == nil {
		return nil, nil
	}
	return c.state.profiles.List(ctx)
}

func (r bootProfileNameResolver) ProfileName(ctx context.Context, profileID string) (string, error) {
	if r.state == nil || r.state.profiles == nil {
		return "", errors.New("daemon: profile manager is not booted")
	}
	return r.state.profiles.ProfileName(ctx, profileID)
}
