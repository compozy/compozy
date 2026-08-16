package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func encodeSessionInputAttachments(attachments []store.SessionInputAttachment) (string, error) {
	cloned := make([]store.SessionInputAttachment, 0, len(attachments))
	cloned = append(cloned, attachments...)
	raw, err := json.Marshal(cloned)
	if err != nil {
		return "", fmt.Errorf("store: encode session input attachments: %w", err)
	}
	return string(raw), nil
}

func decodeSessionInputAttachments(raw string) ([]store.SessionInputAttachment, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []store.SessionInputAttachment{}, nil
	}
	var attachments []store.SessionInputAttachment
	if err := json.Unmarshal([]byte(trimmed), &attachments); err != nil {
		return nil, fmt.Errorf("store: decode session input attachments: %w", err)
	}
	if attachments == nil {
		return []store.SessionInputAttachment{}, nil
	}
	return attachments, nil
}
