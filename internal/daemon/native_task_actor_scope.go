package daemon

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func bindNativeTaskActorProfile(
	actor taskpkg.ActorContext,
	scope toolspkg.Scope,
) (taskpkg.ActorContext, error) {
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		if actor.Actor.Kind.Normalize() == taskpkg.ActorKindAgentSession {
			return taskpkg.ActorContext{}, errors.New("daemon: agent task actor profile is required")
		}
		return actor, nil
	}
	actor.ReadScope = store.ReadScope{ProfileID: profileID}
	if err := actor.Validate(); err != nil {
		return taskpkg.ActorContext{}, err
	}
	return actor, nil
}
