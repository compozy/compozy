package deadentity

func probeDecision(state *entityState, allowed bool, recovery bool) ProbeDecision {
	return ProbeDecision{
		Allowed:  allowed,
		Recovery: recovery,
		Dead:     state.dead,
		Reason:   state.entity.Reason,
		MarkedAt: state.entity.MarkedAt,
	}
}
