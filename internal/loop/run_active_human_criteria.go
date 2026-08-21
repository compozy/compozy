package loop

import "encoding/json"

// ActiveHumanCriteriaValue returns a cloned view of the active human criteria.
func (r Run) ActiveHumanCriteriaValue() json.RawMessage {
	if r.ActiveHumanCriteria == nil {
		return nil
	}
	return append(json.RawMessage(nil), (*r.ActiveHumanCriteria)...)
}

// SetActiveHumanCriteria replaces the active human criteria with an isolated copy.
func (r *Run) SetActiveHumanCriteria(criteria json.RawMessage) {
	if len(criteria) == 0 {
		r.ActiveHumanCriteria = nil
		return
	}
	cloned := append(json.RawMessage(nil), criteria...)
	r.ActiveHumanCriteria = &cloned
}
