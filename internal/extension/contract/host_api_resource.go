package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/resources"
)

// ResourceRecord is the generic Host API desired-state shape exposed to extensions.
type ResourceRecord struct {
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
