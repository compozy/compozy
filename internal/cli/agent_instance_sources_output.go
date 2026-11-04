package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
)

func sourceItemCount(n int64) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

func instanceSourcesText(instance contract.AgentInstancePayload) string {
	if len(instance.Sources) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Sources\n")
	for _, source := range instance.Sources {
		fmt.Fprintf(&b, "  %s  %s  %s", source.ID, instanceSourceRef(source), source.Status)
		if source.Error != nil {
			fmt.Fprintf(&b, "  %s: %s", source.Error.Code, source.Error.Detail)
			if source.FailureStreak > 0 {
				fmt.Fprintf(&b, " (%d in a row)", source.FailureStreak)
			}
		} else if source.Status == "ok" {
			fmt.Fprintf(&b, "  %s", sourceItemCount(source.LastItemCount))
			if polled, err := time.Parse(time.RFC3339, source.LastPolledAt); err == nil {
				fmt.Fprintf(&b, "  polled %s", polled.Format("15:04"))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func instanceSourceRef(source heartbeat.SourceStatus) string {
	if source.Script != "" {
		return "script " + source.Script
	}
	return source.Kind
}

func instanceCheckText(
	result contract.AgentInstanceCheckResponse,
	target string,
	states []heartbeat.SourceStatus,
) string {
	if result.Gate == "source_poll_in_flight" {
		return fmt.Sprintf("Not started: a source check is already running for %s (source_poll_in_flight)\n", target)
	}
	var b strings.Builder
	if len(result.Sources) == 0 {
		fmt.Fprintf(&b, "%s%s\n", result.Gate, result.OccurrenceID)
	} else {
		wouldRun := "no"
		if result.Gate == "would_run" || result.OccurrenceID != "" {
			wouldRun = yesFlagName
		}
		fmt.Fprintf(&b, "Would run: %s\nSources:\n", wouldRun)
		refs := make(map[string]string, len(states))
		for _, state := range states {
			refs[state.ID] = instanceSourceRef(state)
		}
		for _, source := range result.Sources {
			fmt.Fprintf(&b, "  %s  %s  %s  %s  (%d ms)\n",
				source.ID, refs[source.ID], source.Result, sourceItemCount(int64(source.Items)), source.DurationMS)
			if source.Error != nil {
				fmt.Fprintf(&b, "    %s: %s\n", source.Error.Code, source.Error.Detail)
			}
		}
		b.WriteString("Digest:\n")
	}
	for _, item := range result.Digest {
		fmt.Fprintf(&b, "  %s  %s\n", item.Key, item.Summary)
	}
	return b.String()
}
