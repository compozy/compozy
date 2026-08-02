package deadentity

import (
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/redact"
)

const maxPersistedReasonBytes = 512

func boundedReason(reason string) string {
	bounded := strings.TrimSpace(redact.String(reason))
	if bounded == "" {
		bounded = "permanent runtime failure"
	}
	for len(bounded) > maxPersistedReasonBytes {
		_, size := utf8.DecodeLastRuneInString(bounded)
		if size == 0 {
			break
		}
		bounded = bounded[:len(bounded)-size]
	}
	return bounded
}
