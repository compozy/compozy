package contract

import "time"

// MarketplaceSourcePayload reports one global source, including disabled presets.
type MarketplaceSourcePayload struct {
	Name         string           `json:"name"`
	Kind         string           `json:"kind"`
	Source       string           `json:"source"`
	Enabled      bool             `json:"enabled"`
	State        string           `json:"state"`
	Plugins      int              `json:"plugins"`
	Installable  int              `json:"installable"`
	LastReadAt   *time.Time       `json:"last_read_at,omitempty"`
	ErrorClass   string           `json:"error_class,omitempty"`
	Error        string           `json:"error,omitempty"`
	DocumentPath string           `json:"document_path,omitempty"`
	Owner        string           `json:"owner,omitempty"`
	Stability    string           `json:"stability"`
	Diagnostics  []DiagnosticItem `json:"diagnostics"`
}

type MarketplaceSourcesResponse struct {
	Sources []MarketplaceSourcePayload `json:"sources"`
}
type MarketplaceSourceResponse struct {
	Source MarketplaceSourcePayload `json:"source"`
}
type AddMarketplaceSourceRequest struct {
	Ref  string `json:"ref"`
	Name string `json:"name,omitempty"`
}
type UpdateMarketplaceSourceRequest struct {
	Enabled *bool `json:"enabled"`
}
type MarketplaceSourcePreviewPayload struct {
	Name         string           `json:"name"`
	Owner        string           `json:"owner,omitempty"`
	DocumentPath string           `json:"document_path"`
	Plugins      int              `json:"plugins"`
	Installable  int              `json:"installable"`
	Diagnostics  []DiagnosticItem `json:"diagnostics"`
}
type MarketplaceSourceErrorPayload struct {
	Error         string   `json:"error"`
	Code          string   `json:"code"`
	SuggestedName string   `json:"suggested_name,omitempty"`
	RetainedBy    []string `json:"retained_by,omitempty"`
	Checked       []string `json:"checked,omitempty"`
}
