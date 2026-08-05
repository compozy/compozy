package sessiondb

type readOnlyPoolQuiescenceDependency struct {
	registered       bool
	selfDependent    bool
	state            *readOnlyPoolQuiescence
	closings         []*readOnlyPoolClose
	openings         []*readOnlyPoolOpening
	responsibilities []*readOnlyPoolQuiescenceResponsibility
}

func (p *ReadOnlyPool) beginCloseDependency(
	executing *readOnlyPoolClose,
	target *readOnlyPoolClose,
) (bool, bool, error) {
	if executing == nil {
		return true, false, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if target == executing {
		return false, false, nil
	}
	if p.closeDependsOnLocked(target, executing) {
		return false, false, errReadOnlyPoolCloseDependencyCycle
	}
	if executing.waitingForClosings == nil {
		executing.waitingForClosings = make(map[*readOnlyPoolClose]int)
	}
	executing.waitingForClosings[target]++
	return true, !target.started, nil
}

func (p *ReadOnlyPool) endCloseDependency(executing *readOnlyPoolClose, target *readOnlyPoolClose) {
	p.mu.Lock()
	defer p.mu.Unlock()
	remaining := executing.waitingForClosings[target] - 1
	if remaining > 0 {
		executing.waitingForClosings[target] = remaining
		return
	}
	delete(executing.waitingForClosings, target)
}

func (p *ReadOnlyPool) beginOpeningDependency(
	executing *readOnlyPoolClose,
	opening *readOnlyPoolOpening,
) (bool, error) {
	if executing == nil {
		return true, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if opening.closeOp != nil && p.closeDependsOnLocked(opening.closeOp, executing) {
		return false, errReadOnlyPoolCloseDependencyCycle
	}
	if executing.waitingForOpenings == nil {
		executing.waitingForOpenings = make(map[*readOnlyPoolOpening]int)
	}
	executing.waitingForOpenings[opening]++
	return true, nil
}

func (p *ReadOnlyPool) endOpeningDependency(
	executing *readOnlyPoolClose,
	opening *readOnlyPoolOpening,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	remaining := executing.waitingForOpenings[opening] - 1
	if remaining > 0 {
		executing.waitingForOpenings[opening] = remaining
		return
	}
	delete(executing.waitingForOpenings, opening)
}

func (p *ReadOnlyPool) beginQuiescenceDependency(
	executing *readOnlyPoolClose,
	state *readOnlyPoolQuiescence,
) (readOnlyPoolQuiescenceDependency, error) {
	var dependency readOnlyPoolQuiescenceDependency
	if executing == nil {
		return dependency, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if state.completed {
		return dependency, nil
	}
	dependency.state = state

	closePeers := make(map[*readOnlyPoolClose]struct{}, len(state.closings))
	openingPeers := make(map[*readOnlyPoolOpening]struct{}, len(state.openings))
	responsibleClosings := make(map[*readOnlyPoolClose]struct{}, len(state.entries))
	dependency.selfDependent = executing.quiescence == state || executing.deferredQuiescence == state
	if p.collectQuiescenceResponsibilitiesLocked(
		executing,
		state,
		&dependency,
		responsibleClosings,
	) {
		return readOnlyPoolQuiescenceDependency{}, errReadOnlyPoolCloseDependencyCycle
	}
	for closeOp := range state.closings {
		if closeOp == executing {
			dependency.selfDependent = true
			continue
		}
		if _, represented := responsibleClosings[closeOp]; represented {
			continue
		}
		closePeers[closeOp] = struct{}{}
	}
	for opening := range state.openings {
		if opening == executing.opening || opening.closeOp == executing {
			dependency.selfDependent = true
			continue
		}
		openingPeers[opening] = struct{}{}
	}

	for closeOp := range closePeers {
		if p.closeDependsOnLocked(closeOp, executing) {
			return readOnlyPoolQuiescenceDependency{}, errReadOnlyPoolCloseDependencyCycle
		}
		dependency.closings = append(dependency.closings, closeOp)
	}
	for opening := range openingPeers {
		if opening.closeOp != nil && p.closeDependsOnLocked(opening.closeOp, executing) {
			return readOnlyPoolQuiescenceDependency{}, errReadOnlyPoolCloseDependencyCycle
		}
		dependency.openings = append(dependency.openings, opening)
	}

	if executing.waitingForQuiescences == nil {
		executing.waitingForQuiescences = make(map[*readOnlyPoolQuiescence]int)
	}
	executing.waitingForQuiescences[state]++
	dependency.registered = true
	return dependency, nil
}

func (p *ReadOnlyPool) collectQuiescenceResponsibilitiesLocked(
	executing *readOnlyPoolClose,
	state *readOnlyPoolQuiescence,
	dependency *readOnlyPoolQuiescenceDependency,
	responsibleClosings map[*readOnlyPoolClose]struct{},
) bool {
	for _, responsibility := range state.entries {
		if responsibility.completed {
			continue
		}
		if responsibility.closeOp == executing {
			dependency.selfDependent = true
			continue
		}
		dependency.responsibilities = append(dependency.responsibilities, responsibility)
		if responsibility.closeOp == nil {
			continue
		}
		responsibleClosings[responsibility.closeOp] = struct{}{}
		if p.closeDependsOnLocked(responsibility.closeOp, executing) {
			return true
		}
	}
	return false
}

func (p *ReadOnlyPool) endQuiescenceDependency(
	executing *readOnlyPoolClose,
	state *readOnlyPoolQuiescence,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	remaining := executing.waitingForQuiescences[state] - 1
	if remaining > 0 {
		executing.waitingForQuiescences[state] = remaining
		return
	}
	delete(executing.waitingForQuiescences, state)
}

func (p *ReadOnlyPool) closeDependsOnLocked(
	from *readOnlyPoolClose,
	target *readOnlyPoolClose,
) bool {
	pending := []*readOnlyPoolClose{from}
	visited := make(map[*readOnlyPoolClose]struct{})
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		if current == target {
			return true
		}
		if _, ok := visited[current]; ok {
			continue
		}
		visited[current] = struct{}{}
		for dependency, count := range current.waitingForClosings {
			if count > 0 {
				pending = append(pending, dependency)
			}
		}
		for opening, count := range current.waitingForOpenings {
			if count > 0 && opening.closeOp != nil {
				pending = append(pending, opening.closeOp)
			}
		}
		for state, count := range current.waitingForQuiescences {
			if count <= 0 || state.completed {
				continue
			}
			for dependency := range state.closings {
				if dependency != current {
					pending = append(pending, dependency)
				}
			}
			for opening := range state.openings {
				if opening.closeOp != nil && opening.closeOp != current {
					pending = append(pending, opening.closeOp)
				}
			}
			for _, responsibility := range state.entries {
				if responsibility.closeOp != nil && responsibility.closeOp != current {
					pending = append(pending, responsibility.closeOp)
				}
			}
		}
	}
	return false
}
