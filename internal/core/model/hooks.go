package model

import "context"

type observerHookWaiter interface {
	WaitForObserverHooks(context.Context) error
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
