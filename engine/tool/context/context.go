package toolcontext

import (
	"context"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
)

type ctxKey string

const (
	appStateKey      ctxKey = "toolcontext.app_state"
	taskRepoKey      ctxKey = "toolcontext.task_repo"
	resourceStoreKey ctxKey = "toolcontext.resource_store"
)

func WithAppState(ctx context.Context, state *appstate.State) context.Context {
	if ctx == nil || state == nil {
		return ctx
	}
	return context.WithValue(ctx, appStateKey, state)
}

func GetAppState(ctx context.Context) (*appstate.State, bool) {
	if ctx == nil {
		return nil, false
	}
	state, ok := ctx.Value(appStateKey).(*appstate.State)
	if !ok || state == nil {
		return nil, false
	}
	return state, true
}

func WithTaskRepo(ctx context.Context, repo task.Repository) context.Context {
	if ctx == nil || repo == nil {
		return ctx
	}
	return context.WithValue(ctx, taskRepoKey, repo)
}

func GetTaskRepo(ctx context.Context) (task.Repository, bool) {
	if ctx == nil {
		return nil, false
	}
	repo, ok := ctx.Value(taskRepoKey).(task.Repository)
	if !ok || repo == nil {
		return nil, false
	}
	return repo, true
}

func WithResourceStore(ctx context.Context, store resources.ResourceStore) context.Context {
	if ctx == nil || store == nil {
		return ctx
	}
	return context.WithValue(ctx, resourceStoreKey, store)
}

func GetResourceStore(ctx context.Context) (resources.ResourceStore, bool) {
	if ctx == nil {
		return nil, false
	}
	store, ok := ctx.Value(resourceStoreKey).(resources.ResourceStore)
	if !ok || store == nil {
		return nil, false
	}
	return store, true
}
