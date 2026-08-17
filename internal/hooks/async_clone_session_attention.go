package hooks

func (payload SessionAttentionChangedPayload) cloneForAsync() SessionAttentionChangedPayload {
	payload.SessionContext = cloneSessionContext(payload.SessionContext)
	return payload
}
