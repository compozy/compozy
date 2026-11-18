package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/resources"
)

type compactResource struct {
	Kind      resources.ResourceKind   `json:"kind"`
	ID        string                   `json:"id"`
	Version   int64                    `json:"version"`
	Scope     resources.ResourceScope  `json:"scope"`
	Owner     resources.ResourceOwner  `json:"owner"`
	Source    resources.ResourceSource `json:"source"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
}

func projectCompactResource(p contract.ResourceRecordPayload) compactResource {
	return compactResource{
		Kind:      p.Kind,
		ID:        p.ID,
		Version:   p.Version,
		Scope:     p.Scope,
		Owner:     p.Owner,
		Source:    p.Source,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

type resourceListResult struct {
	Count   *int                             `json:"count,omitempty"`
	Records []contract.ResourceRecordPayload `json:"records"`
}

func compactResources(p resourceListResult) any {
	var records []compactResource
	if p.Records != nil {
		records = make([]compactResource, len(p.Records))
		for i := range p.Records {
			records[i] = projectCompactResource(p.Records[i])
		}
	}
	return struct {
		Count   *int              `json:"count,omitempty"`
		Records []compactResource `json:"records"`
	}{p.Count, records}
}
