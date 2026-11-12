package config

import "strings"

const (
	// MaxAgentAccepts bounds the input forms an agent advertises to peers.
	MaxAgentAccepts = 8
	// MaxAgentAcceptRunes bounds one advertised input form.
	MaxAgentAcceptRunes = 160
)

// NormalizeAgentAccepts keeps a bounded, single-line list of input forms. It
// clamps instead of failing so an oversized list never makes a definition unloadable.
func NormalizeAgentAccepts(accepts []string) []string {
	normalized := make([]string, 0, min(len(accepts), MaxAgentAccepts))
	for _, form := range accepts {
		form = strings.Join(strings.Fields(form), " ")
		if form == "" {
			continue
		}
		if runes := []rune(form); len(runes) > MaxAgentAcceptRunes {
			form = string(runes[:MaxAgentAcceptRunes])
		}
		normalized = append(normalized, form)
		if len(normalized) == MaxAgentAccepts {
			break
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// AcceptedInputs returns the input forms peers should use for this agent: the
// authored accepts list, or, when the definition declares none, the lines of the
// first fenced block under an "Input" heading of its prompt. The fallback keeps
// definitions readable by releases that predate the accepts key.
func (a AgentDef) AcceptedInputs() []string {
	if accepts := a.AcceptsValue(); len(accepts) > 0 {
		return accepts
	}
	var forms []string
	inSection, collecting := false, false
	// fence is the opening marker of the block being read; headings inside a block are text.
	fence := ""
	for line := range strings.SplitSeq(a.Prompt, "\n") {
		trimmed := strings.TrimSpace(line)
		marker := fenceMarker(trimmed)
		switch {
		case fence != "":
			if marker != "" && len(marker) >= len(fence) && strings.Trim(trimmed, "`") == "" {
				if collecting {
					return NormalizeAgentAccepts(forms)
				}
				fence = ""
				continue
			}
			if collecting {
				forms = append(forms, trimmed)
			}
		case marker != "":
			fence, collecting = marker, inSection
		case strings.HasPrefix(trimmed, "#"):
			inSection = strings.EqualFold(strings.TrimSpace(strings.TrimLeft(trimmed, "#")), "Input")
		}
	}
	return nil
}

// fenceMarker returns the run of three or more backticks that opens or closes a fenced block.
func fenceMarker(line string) string {
	run := len(line) - len(strings.TrimLeft(line, "`"))
	if run < 3 {
		return ""
	}
	return line[:run]
}
