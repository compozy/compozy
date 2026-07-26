package clawhub

import (
	"encoding/json"

	"io"

	"github.com/compozy/agh/internal/registry"
)

func decodeListings(body io.Reader) ([]registry.Listing, error) {
	payload, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var direct []clawhubListing
	if err := json.Unmarshal(payload, &direct); err == nil {
		if direct == nil {
			return []registry.Listing{}, nil
		}
		return normalizeClawHubListings(direct), nil
	}

	var envelope struct {
		Skills  []clawhubListing `json:"skills"`
		Results []clawhubListing `json:"results"`
		Items   []clawhubListing `json:"items"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}

	switch {
	case envelope.Skills != nil:
		return normalizeClawHubListings(envelope.Skills), nil
	case envelope.Results != nil:
		return normalizeClawHubListings(envelope.Results), nil
	case envelope.Items != nil:
		return normalizeClawHubListings(envelope.Items), nil
	default:
		return []registry.Listing{}, nil
	}
}

func decodeDetail(payload []byte, fallbackSlug string) (*registry.Detail, error) {
	var envelope clawhubDetailEnvelope
	if err := json.Unmarshal(payload, &envelope); err == nil && envelope.Skill != nil {
		detail := envelope.Skill.registryDetail(
			fallbackSlug,
			envelope.Owner.Handle,
			envelope.LatestVersion.Version,
			envelope.LatestVersion.License,
		)
		return &detail, nil
	}

	var detail registry.Detail
	if err := json.Unmarshal(payload, &detail); err != nil {
		return nil, err
	}
	return normalizeDetail(detail, fallbackSlug), nil
}
