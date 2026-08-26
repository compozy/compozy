package calls

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

const completionPreviewBytes = 512

func RenderCompletionWake(call CallRecord, payload []byte) string {
	agent := strings.TrimSpace(call.AgentName)
	if agent == "" {
		agent = "agent"
	}
	if call.State == StateCompleted && call.ResultRef != "" {
		preview := escapeUntrustedFrameText(truncateUTF8(string(payload), completionPreviewBytes))
		resultSummary := fmt.Sprintf("%d B", call.ResultBytes)
		digest := strings.TrimPrefix(call.ExpectDigest, "sha256:")
		if digest != "" {
			if len(digest) > 4 {
				digest = digest[:4] + "…"
			}
			resultSummary += ", contract sha256:" + digest
		}
		return fmt.Sprintf(
			"Call completed: %s (%s) → %s.\nResult preview (%s):\n<untrusted-call-result>\n%s\n</untrusted-call-result>\nFetch the full result with compozy__call_result.",
			agent, call.CallID, call.State, resultSummary, preview,
		)
	}
	reason := strings.TrimSpace(call.FailureDetail)
	if clean, _, reject := contracts.SanitizeText(reason); reject {
		reason = ""
	} else {
		reason = strings.TrimSpace(clean)
	}
	if reason == "" {
		reason = strings.TrimSpace(call.FailureCode)
	}
	if reason == "" {
		reason = "the call ended without a result"
	}
	return fmt.Sprintf(
		"Call failed: %s (%s) → %s.\nReason: %s.\nInspect with compozy__call_result (attempts and errors are recorded).",
		agent, call.CallID, call.State, strings.TrimSuffix(reason, "."),
	)
}
