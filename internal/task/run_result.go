package task

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// DefaultRunResultPageBytes is the default exact-byte page size for task results.
	DefaultRunResultPageBytes int64 = 64 << 10
	// MaxRunResultPageBytes is the largest task-result page exposed by one read.
	MaxRunResultPageBytes int64 = 64 << 10
)

var (
	// ErrTaskRunResultNotFound masks absent, unowned, and non-result task runs.
	ErrTaskRunResultNotFound = errors.New("task: run result not found")
	// ErrTaskRunResultInvalidRange reports an invalid task-result byte range.
	ErrTaskRunResultInvalidRange = errors.New("task: run result range is invalid")
	// ErrTaskRunResultCorrupt reports a descriptor, digest, or byte-size mismatch.
	ErrTaskRunResultCorrupt = errors.New("task: run result is corrupt")
)

// RunResultPage is one exact bounded byte page from a task-run result.
type RunResultPage struct {
	RunID      string `json:"run_id"`
	ResultRef  string `json:"result_ref,omitempty"`
	Offset     int64  `json:"offset"`
	Bytes      int64  `json:"bytes"`
	TotalBytes int64  `json:"total_bytes"`
	DataBase64 string `json:"data_base64"`
	NextOffset int64  `json:"next_offset,omitempty"`
	EOF        bool   `json:"eof"`
}

// ResultValue returns a cloned terminal result, or nil when the run has none.
func (r Run) ResultValue() json.RawMessage {
	if r.RunResultState == nil {
		return nil
	}
	return cloneRawJSON(rawJSONValue(r.Result))
}

// SetResult replaces the terminal result with an isolated copy.
func (r *Run) SetResult(result json.RawMessage) {
	r.RunResultState = &RunResultState{
		Result:              rawJSONPointer(result),
		RunResultDescriptor: RunResultDescriptor{ResultBytes: int64(len(result))},
	}
}

// SetExternalResult replaces the inline value with its durable opaque descriptor.
func (r *Run) SetExternalResult(ref string, bytes int64) {
	r.RunResultState = &RunResultState{
		RunResultDescriptor: RunResultDescriptor{ResultRef: ref, ResultBytes: bytes},
	}
}

// ClearResult removes both inline and external terminal result state.
func (r *Run) ClearResult() {
	r.RunResultState = nil
}

// ResultReference returns the external result reference, if present.
func (r Run) ResultReference() string {
	if r.RunResultState == nil {
		return ""
	}
	return r.ResultRef
}

// ResultByteCount returns the declared result size, if present.
func (r Run) ResultByteCount() int64 {
	if r.RunResultState == nil {
		return 0
	}
	return r.ResultBytes
}

func cloneRunResultState(state *RunResultState) *RunResultState {
	if state == nil {
		return nil
	}
	cloned := *state
	cloned.Result = cloneRawJSONPointer(state.Result)
	return &cloned
}

// PageRunResult returns one exact byte page for a validated inline or external result.
func PageRunResult(
	runID string,
	resultRef string,
	content []byte,
	offset int64,
	limit int64,
) (RunResultPage, error) {
	if offset < 0 {
		return RunResultPage{}, fmt.Errorf(
			"%w: offset must be greater than or equal to zero",
			ErrTaskRunResultInvalidRange,
		)
	}
	if limit == 0 {
		limit = DefaultRunResultPageBytes
	}
	if limit < 1 || limit > MaxRunResultPageBytes {
		return RunResultPage{}, fmt.Errorf(
			"%w: limit must be between 1 and %d",
			ErrTaskRunResultInvalidRange,
			MaxRunResultPageBytes,
		)
	}
	total := int64(len(content))
	if offset > total {
		return RunResultPage{}, fmt.Errorf(
			"%w: offset %d exceeds total bytes %d",
			ErrTaskRunResultInvalidRange,
			offset,
			total,
		)
	}
	end := min(total, offset+limit)
	page := content[offset:end]
	return RunResultPage{
		RunID:      runID,
		ResultRef:  resultRef,
		Offset:     offset,
		Bytes:      int64(len(page)),
		TotalBytes: total,
		DataBase64: base64.StdEncoding.EncodeToString(page),
		NextOffset: end,
		EOF:        end == total,
	}, nil
}
