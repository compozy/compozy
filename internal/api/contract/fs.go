package contract

// FSEntryPayload describes one filesystem entry returned by the directory browser.
type FSEntryPayload struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// FSBrowseResponse lists the contents of one directory plus navigation anchors.
type FSBrowseResponse struct {
	Path    string           `json:"path"`
	Parent  string           `json:"parent,omitempty"`
	Home    string           `json:"home,omitempty"`
	Entries []FSEntryPayload `json:"entries"`
}
