package tools

// ToolPresentation carries optional, non-executable descriptor presentation hints.
type ToolPresentation struct {
	FriendlyVerb string `json:"friendly_verb,omitempty"`
	Preview      string `json:"preview,omitempty"`
}

// NewToolPresentation returns nil when both optional presentation fields are empty.
func NewToolPresentation(friendlyVerb string, preview string) *ToolPresentation {
	if friendlyVerb == "" && preview == "" {
		return nil
	}
	return &ToolPresentation{FriendlyVerb: friendlyVerb, Preview: preview}
}

// CloneToolPresentation returns an independent presentation payload.
func CloneToolPresentation(p *ToolPresentation) *ToolPresentation {
	if p == nil {
		return nil
	}
	cloned := *p
	return &cloned
}

func presentationValue(p *ToolPresentation) ToolPresentation {
	if p == nil {
		return ToolPresentation{}
	}
	return *p
}

// Presentation returns a nil-safe copy of the cold tool presentation metadata.
func (t Tool) Presentation() ToolPresentation {
	return presentationValue(t.ToolPresentation)
}

// Presentation returns a nil-safe copy of the runtime descriptor presentation metadata.
func (d Descriptor) Presentation() ToolPresentation {
	return presentationValue(d.ToolPresentation)
}
