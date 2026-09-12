package terminal

import "strings"

// processEnvironment copies caller overrides and enforces the originating agent identity fields.
func processEnvironment(actor Actor, overrides map[string]string) map[string]string {
	env := cloneStringMap(overrides)
	if actor.Kind != ActorKindAgent {
		return env
	}
	if env == nil {
		env = make(map[string]string)
	}
	// Windows environment keys are case-insensitive. Remove alternate spellings
	// before binding the same managed identity used by the provider process.
	for key := range env {
		switch strings.ToUpper(key) {
		case "COMPOZY_SESSION_ID", "COMPOZY_AGENT", "COMPOZY_AGENT_NAME":
			delete(env, key)
		}
	}
	env["COMPOZY_SESSION_ID"] = actor.SessionID
	env["COMPOZY_AGENT"] = actor.ID
	env["COMPOZY_AGENT_NAME"] = actor.ID
	return env
}

func sameActor(left, right Actor) bool {
	return left.Kind == right.Kind && left.ID == right.ID && left.ProfileID == right.ProfileID &&
		left.SessionID == right.SessionID && left.RunID == right.RunID && left.Generation == right.Generation
}

func sameRun(left, right Actor) bool {
	return left.Kind == ActorKindAgent && right.Kind == ActorKindAgent && left.ProfileID == right.ProfileID &&
		left.SessionID == right.SessionID && left.RunID == right.RunID
}
