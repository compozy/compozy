package contract

// ClarifyAskParams carries daemon-issued invocation authority and extension-authored question data.
type ClarifyAskParams struct {
	InvocationID string   `json:"invocation_id"`
	Question     string   `json:"question"`
	Choices      []string `json:"choices,omitempty"`
}
