package contract

// LoopInputDefaultsScope selects the file-backed config layer managed by an input-default request.
type LoopInputDefaultsScope string

const (
	LoopInputDefaultsScopeGlobal    LoopInputDefaultsScope = "global"
	LoopInputDefaultsScopeWorkspace LoopInputDefaultsScope = "workspace"
)

// LoopInputDefaultsResponse returns one Loop's configured values at one explicit scope.
type LoopInputDefaultsResponse struct {
	LoopName string                 `json:"loop_name"`
	Scope    LoopInputDefaultsScope `json:"scope"`
	Values   map[string]any         `json:"values"`
}

// LoopInputDefaultResponse returns one configured key without collapsing explicit zero values.
type LoopInputDefaultResponse struct {
	LoopName string                 `json:"loop_name"`
	Key      string                 `json:"key"`
	Scope    LoopInputDefaultsScope `json:"scope"`
	Present  bool                   `json:"present"`
	Value    any                    `json:"value,omitempty"`
}

// PutLoopInputDefaultsRequest replaces one Loop's complete configured table at one scope.
type PutLoopInputDefaultsRequest struct {
	Scope  LoopInputDefaultsScope `json:"scope"`
	Values map[string]any         `json:"values"`
}

// PutLoopInputDefaultRequest sets one configured Loop input at one scope.
type PutLoopInputDefaultRequest struct {
	Scope LoopInputDefaultsScope `json:"scope"`
	Value any                    `json:"value"`
}

// DeleteLoopInputDefaultResponse reports whether one configured key was present before deletion.
type DeleteLoopInputDefaultResponse struct {
	LoopName string                 `json:"loop_name"`
	Key      string                 `json:"key"`
	Scope    LoopInputDefaultsScope `json:"scope"`
	Deleted  bool                   `json:"deleted"`
}
