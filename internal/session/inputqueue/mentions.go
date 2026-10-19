package inputqueue

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/mention"
	"github.com/compozy/compozy/internal/store"
)

// MentionAdmission is what admission verified for a replacement's authored text: its mentions and
// whether the human operator authored it.
type MentionAdmission struct {
	Mentions     []mention.Mention
	HumanIngress bool
}

func (a MentionAdmission) clone() MentionAdmission {
	a.Mentions = cloneMentions(a.Mentions)
	return a
}

// EncodeMentions renders verified mentions as the JSON array persisted beside queued input and
// prompt admissions. No mentions persist as the empty array.
func EncodeMentions(mentions []mention.Mention) (string, error) {
	if len(mentions) == 0 {
		return store.EmptyMentionsJSON, nil
	}
	encoded, err := json.Marshal(mentions)
	if err != nil {
		return "", fmt.Errorf("inputqueue: encode mentions: %w", err)
	}
	return string(encoded), nil
}

// DecodeMentions reads a persisted mentions array; blank and empty arrays decode as none.
func DecodeMentions(raw string) ([]mention.Mention, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == store.EmptyMentionsJSON {
		return nil, nil
	}
	var mentions []mention.Mention
	if err := json.Unmarshal([]byte(trimmed), &mentions); err != nil {
		return nil, fmt.Errorf("inputqueue: decode mentions: %w", err)
	}
	if len(mentions) == 0 {
		return nil, nil
	}
	return mentions, nil
}

// HasMentions reports whether a persisted mentions array carries at least one mention.
func HasMentions(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return trimmed != "" && trimmed != store.EmptyMentionsJSON && trimmed != "null"
}
