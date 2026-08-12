package contract

import "time"

const (
	ForgeCauseRateLimited       = "rate_limited"
	ForgeCauseCredentialExpired = "credential_expired"
	ForgeCauseUnsupportedRemote = "unsupported_remote"
	ForgeCauseCredentialAbsent  = "credential_absent"
)

type ForgeCapabilitiesRequest struct {
	RemoteURLs []string `json:"remote_urls"`
}

type ForgeCapabilitiesResponse struct {
	Served             bool     `json:"served"`
	Available          bool     `json:"available"`
	Winner             string   `json:"winner,omitempty"`
	Provider           string   `json:"provider,omitempty"`
	ServedRemote       string   `json:"served_remote,omitempty"`
	RequestNoun        string   `json:"request_noun,omitempty"`
	OpenActionLabel    string   `json:"open_action_label,omitempty"`
	ViewActionLabel    string   `json:"view_action_label,omitempty"`
	SupportsDraft      bool     `json:"supports_draft,omitempty"`
	CompareURLTemplate string   `json:"compare_url_template,omitempty"`
	TemplatePaths      []string `json:"template_paths,omitempty"`
	CredentialSource   string   `json:"credential_source,omitempty"`
	DefaultBranch      string   `json:"default_branch,omitempty"`
	Cause              string   `json:"cause,omitempty"`
}

type ForgeStatusRequest struct {
	RemoteURLs []string `json:"remote_urls"`
	Branch     string   `json:"branch"`
}

type ForgeStatusResponse struct {
	Provider  string    `json:"provider"`
	PRNumber  *int      `json:"pr_number,omitempty"`
	PRState   *string   `json:"pr_state,omitempty"`
	PRURL     string    `json:"pr_url,omitempty"`
	Merged    *bool     `json:"merged,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
	Cause     string    `json:"cause,omitempty"`
}

type ForgePRCreateRequest struct {
	RemoteURLs []string `json:"remote_urls"`
	Head       string   `json:"head"`
	Base       string   `json:"base"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Draft      bool     `json:"draft,omitempty"`
}

type ForgePRCreateResponse struct {
	Status string `json:"status"`
	Number int    `json:"number,omitempty"`
	URL    string `json:"url,omitempty"`
	Cause  string `json:"cause,omitempty"`
}
