package contract

import "time"

const IngressBindingsPath = "/api/ingress/bindings"

type IngressPayload struct {
	SubjectKind                 string     `json:"subject_kind"`
	SubjectID                   string     `json:"subject_id"`
	ScopeKind                   string     `json:"scope_kind"`
	WorkspaceID                 string     `json:"workspace_id,omitempty"`
	URL                         string     `json:"url,omitempty"`
	Reachability                string     `json:"reachability"`
	EndpointGeneration          uint64     `json:"endpoint_generation"`
	ConfirmedEndpointGeneration uint64     `json:"confirmed_endpoint_generation,omitempty"`
	ConfirmedAt                 *time.Time `json:"confirmed_at,omitempty"`
	EnablePath                  string     `json:"enable_path,omitempty"`
}

type IngressBindRequest struct {
	SubjectKind string `json:"subject_kind" binding:"required"`
	SubjectID   string `json:"subject_id"   binding:"required"`
	Confirmed   bool   `json:"confirmed"    binding:"required"`
}

type IngressBindingResponse struct {
	Binding IngressPayload `json:"binding"`
	Changed bool           `json:"changed"`
}
