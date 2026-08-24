package task

import "encoding/json"

// ResultValue returns a cloned terminal result, or nil when the run has none.
func (r Run) ResultValue() json.RawMessage {
	return cloneRawJSON(rawJSONValue(r.Result))
}

// SetResult replaces the terminal result with an isolated copy.
func (r *Run) SetResult(result json.RawMessage) {
	r.Result = rawJSONPointer(result)
}
