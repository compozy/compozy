package terminal

func sameActor(left, right Actor) bool {
	return left.Kind == right.Kind && left.ID == right.ID && left.ProfileID == right.ProfileID &&
		left.SessionID == right.SessionID && left.RunID == right.RunID && left.Generation == right.Generation
}

func sameRun(left, right Actor) bool {
	return left.Kind == ActorKindAgent && right.Kind == ActorKindAgent && left.ProfileID == right.ProfileID &&
		left.SessionID == right.SessionID && left.RunID == right.RunID
}
