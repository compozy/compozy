package loop

import "encoding/json"

// ActiveHumanCriteriaValue returns a cloned view of the active human criteria.
func (r Run) ActiveHumanCriteriaValue() json.RawMessage {
	if r.RunStartState == nil || r.activeHumanCriteria == nil {
		return nil
	}
	return append(json.RawMessage(nil), (*r.activeHumanCriteria)...)
}

// SetActiveHumanCriteria replaces the active human criteria with an isolated copy.
func (r *Run) SetActiveHumanCriteria(criteria json.RawMessage) {
	if len(criteria) == 0 {
		if r.RunStartState != nil {
			r.activeHumanCriteria = nil
		}
		return
	}
	r.ensureStartState()
	cloned := append(json.RawMessage(nil), criteria...)
	r.activeHumanCriteria = &cloned
}
