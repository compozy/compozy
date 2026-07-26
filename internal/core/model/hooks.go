package model

import "context"

type observerHookWaiter interface {
	WaitForObserverHooks(context.Context) error
}

type mutableHookStatusDispatcher interface {
	DispatchMutableHookWithStatus(context.Context, string, any) (any, HookMutation, error)
}

// HookMutation records whether a hook applied a patch and which JSON fields it selected.
type HookMutation struct {
	Applied bool
	Fields  map[string]struct{}
}

// HasField reports whether an applied patch selected the given JSON field path.
func (m HookMutation) HasField(path string) bool {
	if !m.Applied {
		return false
	}
	_, ok := m.Fields[path]
	return ok
}

func DispatchMutableHook[T any](
	ctx context.Context,
	manager RuntimeManager,
	hook string,
	payload T,
) (T, error) {
	if manager == nil {
		return payload, nil
	}

	updated, err := manager.DispatchMutableHook(ctx, hook, payload)
	if err != nil {
		return payload, err
	}

	typed, ok := updated.(T)
	if !ok {
		return payload, nil
	}
	return typed, nil
}

func DispatchMutableHookWithStatus[T comparable](
	ctx context.Context,
	manager RuntimeManager,
	hook string,
	payload T,
) (T, HookMutation, error) {
	if manager == nil {
		return payload, HookMutation{}, nil
	}

	statusDispatcher, ok := manager.(mutableHookStatusDispatcher)
	if !ok {
		updated, err := DispatchMutableHook(ctx, manager, hook, payload)
		return updated, HookMutation{Applied: updated != payload}, err
	}

	updated, mutation, err := statusDispatcher.DispatchMutableHookWithStatus(ctx, hook, payload)
	if err != nil {
		return payload, HookMutation{}, err
	}
	typed, ok := updated.(T)
	if !ok {
		return payload, HookMutation{}, nil
	}
	return typed, mutation, nil
}

func DispatchObserverHook(ctx context.Context, manager RuntimeManager, hook string, payload any) {
	if manager == nil {
		return
	}
	manager.DispatchObserverHook(ctx, hook, payload)
}

func WaitForObserverHooks(ctx context.Context, manager RuntimeManager) error {
	if manager == nil {
		return nil
	}
	waiter, ok := manager.(observerHookWaiter)
	if !ok {
		return nil
	}
	return waiter.WaitForObserverHooks(ctx)
}
