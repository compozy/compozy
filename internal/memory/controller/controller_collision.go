package controller

import memcontract "github.com/compozy/compozy/internal/memory/contract"

func (c *Controller) filenameCollisionDecision(
	candidate memcontract.Candidate,
	targets []Target,
	trace []memcontract.RuleHit,
) (memcontract.Decision, bool, error) {
	collision := exactFilenameTarget(candidate, targets)
	if collision == nil {
		return memcontract.Decision{}, false, nil
	}
	if isAutonomousWriteOrigin(candidate.Origin) && !hasExplicitTargetFilename(candidate) {
		decision, err := c.decision(
			candidate,
			memcontract.OpNoop,
			[]Target{*collision},
			collision.TargetFilename,
			"",
			append(
				trace,
				failedRule(
					"autonomous_generated_slug_collision",
					"generated filename does not establish update identity for autonomous writes",
					[]string{collision.ID},
				),
			),
			"autonomous generated filename collision requires an explicit target",
			nil,
		)
		return decision, true, err
	}
	decision, err := c.updateDecision(candidate, *collision, append(
		trace,
		passedRule("exact_slug_collision", "target filename already exists", collision.ID),
	))
	return decision, true, err
}
