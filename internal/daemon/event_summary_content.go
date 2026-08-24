package daemon

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/store"
)

func daemonEventSummary(summary store.EventSummary, content json.RawMessage) store.EventSummary {
	summary.SetContent(content)
	return summary
}
