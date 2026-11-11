package contract

type ChangeHeadPayload struct {
	Branch string `json:"branch"`
	SHA    string `json:"sha"`
}
type ChangeBasePayload struct {
	Ref          string `json:"ref"`
	Source       string `json:"source"`
	MergeBaseSHA string `json:"merge_base_sha"`
}
type ChangeTotalsPayload struct {
	Files     int  `json:"files"`
	Additions int  `json:"additions"`
	Deletions int  `json:"deletions"`
	Partial   bool `json:"partial,omitzero"`
}
type FileChangePayload struct {
	Path        string `json:"path"`
	OldPath     string `json:"old_path,omitempty"`
	Status      string `json:"status"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	Binary      bool   `json:"binary,omitzero"`
	Generated   bool   `json:"generated,omitzero"`
	Sensitive   bool   `json:"sensitive,omitzero"`
	Fingerprint string `json:"fingerprint"`
}
type SessionChangesResponse struct {
	Scope       string                     `json:"scope"`
	Root        FileRootPayload            `json:"root"`
	Head        ChangeHeadPayload          `json:"head"`
	Base        ChangeBasePayload          `json:"base"`
	PullRequest *SessionPullRequestPayload `json:"pull_request,omitempty"`
	Totals      ChangeTotalsPayload        `json:"totals"`
	Files       []FileChangePayload        `json:"files"`
	Truncated   bool                       `json:"truncated"`
	Revision    string                     `json:"revision"`
}
type ChangePatchPayload struct {
	Path        string `json:"path"`
	Patch       string `json:"patch,omitempty"`
	TooLarge    bool   `json:"too_large,omitzero"`
	Sensitive   bool   `json:"sensitive,omitzero"`
	Fingerprint string `json:"fingerprint"`
}
type SessionChangePatchesResponse struct {
	Revision string               `json:"revision"`
	Patches  []ChangePatchPayload `json:"patches"`
}
type ChangeSidePayload struct {
	Text      *string `json:"text,omitempty"`
	Sensitive bool    `json:"sensitive,omitzero"`
}
type SessionChangeContentsResponse struct {
	Revision    string             `json:"revision"`
	Path        string             `json:"path"`
	OldPath     string             `json:"old_path,omitempty"`
	Fingerprint string             `json:"fingerprint"`
	Old         *ChangeSidePayload `json:"old,omitempty"`
	New         *ChangeSidePayload `json:"new,omitempty"`
}
type ChangeBaseRefPayload struct {
	Ref       string `json:"ref"`
	Kind      string `json:"kind"`
	IsDefault bool   `json:"is_default,omitzero"`
	Source    string `json:"source,omitempty"`
}
type SessionChangeBasesResponse struct {
	Default    *ChangeBaseRefPayload  `json:"default"`
	Resolvable bool                   `json:"resolvable"`
	Refs       []ChangeBaseRefPayload `json:"refs"`
	Truncated  bool                   `json:"truncated"`
}
