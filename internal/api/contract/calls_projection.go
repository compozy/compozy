package contract

// NewCallProvenancePayload projects result provenance only after the result has an admission verdict.
func NewCallProvenancePayload(producedBy, sessionID, admitted string) *CallProvenancePayload {
	if admitted == "" {
		return nil
	}
	return &CallProvenancePayload{
		ProducedBy: producedBy,
		SessionID:  sessionID,
		Admitted:   admitted,
	}
}
