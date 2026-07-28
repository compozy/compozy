package participation

// CloneSpec returns an independently addressable immutable participation snapshot.
func CloneSpec(spec Spec) *Spec {
	return new(spec)
}

// CloneRequest returns a deep copy of one unresolved participation intent.
func CloneRequest(request *Request) *Request {
	if request == nil {
		return nil
	}
	cloned := *request
	if request.Mode != nil {
		cloned.Mode = new(*request.Mode)
	}
	if request.ChannelStrategy != nil {
		cloned.ChannelStrategy = new(*request.ChannelStrategy)
	}
	if request.ChannelID != nil {
		cloned.ChannelID = new(*request.ChannelID)
	}
	cloned.Bounds = cloneBoundsRequest(request.Bounds)
	return &cloned
}

func cloneBoundsRequest(bounds *BoundsRequest) *BoundsRequest {
	if bounds == nil {
		return nil
	}
	cloned := *bounds
	if bounds.MaxWakes != nil {
		cloned.MaxWakes = new(*bounds.MaxWakes)
	}
	if bounds.MaxWakeWallTime != nil {
		cloned.MaxWakeWallTime = new(*bounds.MaxWakeWallTime)
	}
	if bounds.MaxTotalWallTime != nil {
		cloned.MaxTotalWallTime = new(*bounds.MaxTotalWallTime)
	}
	if bounds.MaxInputTokens != nil {
		cloned.MaxInputTokens = new(*bounds.MaxInputTokens)
	}
	if bounds.MaxOutputTokens != nil {
		cloned.MaxOutputTokens = new(*bounds.MaxOutputTokens)
	}
	if bounds.MaxWakeDepth != nil {
		cloned.MaxWakeDepth = new(*bounds.MaxWakeDepth)
	}
	if bounds.CoalesceWindow != nil {
		cloned.CoalesceWindow = new(*bounds.CoalesceWindow)
	}
	return &cloned
}
