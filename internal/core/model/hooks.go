package model

import "context"

type observerHookWaiter interface {
	WaitForObserverHooks(context.Context) error
}

type mutableHookStatusDispatcher interface {
	DispatchMutableHookWithStatus(context.Context, string, any) (any, bool, error)
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
) (T, bool, error) {
	if manager == nil {
		return payload, false, nil
	}

	statusDispatcher, ok := manager.(mutableHookStatusDispatcher)
	if !ok {
		updated, err := DispatchMutableHook(ctx, manager, hook, payload)
		return updated, updated != payload, err
	}

	updated, applied, err := statusDispatcher.DispatchMutableHookWithStatus(ctx, hook, payload)
	if err != nil {
		return payload, false, err
	}
	typed, ok := updated.(T)
	if !ok {
		return payload, false, nil
	}
	return typed, applied, nil
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
