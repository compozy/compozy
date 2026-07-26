package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/resources"
)

// PutResourceRequest is the shared desired-state upsert payload.
type PutResourceRequest struct {
	Scope           resources.ResourceScope `json:"scope"`
	ExpectedVersion int64                   `json:"expected_version,omitempty"`
	Spec            json.RawMessage         `json:"spec"`
}

// DeleteResourceRequest is the shared desired-state delete payload.
type DeleteResourceRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

// ResourceRecordPayload is the shared desired-state record response shape.
type ResourceRecordPayload struct {
	Kind      resources.ResourceKind   `json:"kind"`
	ID        string                   `json:"id"`
	Version   int64                    `json:"version"`
	Scope     resources.ResourceScope  `json:"scope"`
	Owner     resources.ResourceOwner  `json:"owner"`
	Source    resources.ResourceSource `json:"source"`
	Spec      json.RawMessage          `json:"spec"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
}
