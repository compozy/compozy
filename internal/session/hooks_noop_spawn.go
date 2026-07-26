package session

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type noopSpawnHooks struct{}

func (noopSpawnHooks) DispatchSpawnPreCreate(
	_ context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	return payload, nil
}

func (noopSpawnHooks) DispatchSpawnCreated(
	_ context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return payload, nil
}

func (noopSpawnHooks) DispatchSpawnParentStopped(
	_ context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return payload, nil
}

func (noopSpawnHooks) DispatchSpawnTTLExpired(
	_ context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return payload, nil
}

func (noopSpawnHooks) DispatchSpawnReaped(
	_ context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return payload, nil
}

type noopAuthoredContextHooks struct{}

func (noopAuthoredContextHooks) DispatchSessionHealthUpdateAfter(
	_ context.Context,
	payload hookspkg.SessionHealthUpdateAfterPayload,
) (hookspkg.SessionHealthUpdateAfterPayload, error) {
	return payload, nil
}
