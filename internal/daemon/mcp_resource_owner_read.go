package daemon

import (
	"context"
	"sync"
)

type mcpOwnerReadKey struct{}
type mcpOwnerProfileKey struct {
	state *bootState
	name  string
}
type mcpOwnerProfile struct {
	id  string
	err error
}
type mcpOwnerRead struct {
	mu       sync.Mutex
	profiles map[mcpOwnerProfileKey]mcpOwnerProfile
}

func withMCPOwnerRead(ctx context.Context) context.Context {
	if ctx.Value(mcpOwnerReadKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, mcpOwnerReadKey{}, &mcpOwnerRead{
		profiles: make(map[mcpOwnerProfileKey]mcpOwnerProfile),
	})
}

func mcpOwnerProfileID(ctx context.Context, state *bootState, name string) (string, error) {
	read, ok := ctx.Value(mcpOwnerReadKey{}).(*mcpOwnerRead)
	if !ok {
		profile, err := state.profiles.GetByName(ctx, name)
		return profile.ID, err
	}
	read.mu.Lock()
	defer read.mu.Unlock()
	key := mcpOwnerProfileKey{state, name}
	if cached, ok := read.profiles[key]; ok {
		return cached.id, cached.err
	}
	profile, err := state.profiles.GetByName(ctx, name)
	id := profile.ID
	read.profiles[key] = mcpOwnerProfile{id, err}
	return id, err
}
