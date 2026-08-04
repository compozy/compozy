package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func loopInspectOutputBundle(response *contract.LoopResponse) outputBundle {
	summary := fmt.Sprintf("Loop %s v%d", response.Loop.Name, response.Loop.Version)
	bundle := loopOutputBundle(*response, summary)
	if response.Loop.EffectiveLifecycle == nil {
		return bundle
	}
	lifecycle := response.Loop.EffectiveLifecycle
	bundle.human = func() (string, error) {
		definition := renderHumanSection("Loop", []keyValue{
			{Label: automationNameValue, Value: response.Loop.Name},
			{Label: versionValue, Value: strconv.Itoa(response.Loop.Version)},
			{Label: "Source", Value: string(response.Loop.Source)},
		})
		defaults := renderHumanSection("Effective lifecycle defaults", []keyValue{
			{Label: "Retry attempts", Value: strconv.Itoa(lifecycle.RetryMaxAttempts)},
			{Label: "Retry backoff", Value: lifecycle.RetryBackoffBase + " .. " + lifecycle.RetryBackoffMax},
			{Label: "Silence window", Value: lifecycle.LivenessSilenceWindow},
			{Label: "Resume death streak", Value: strconv.Itoa(lifecycle.ResumeDeathStreakLimit)},
			{Label: "Predicate cost", Value: strconv.FormatUint(lifecycle.PredicateCostLimit, 10)},
			{Label: "Wait admissions", Value: strconv.Itoa(lifecycle.WaitAdmissionAttempts)},
			{Label: "Wait retry interval", Value: lifecycle.WaitAdmissionInterval},
			{Label: "Admission horizon", Value: lifecycle.AdmissionHorizon},
			{Label: "Sources", Value: formatLoopLifecycleSources(lifecycle.Sources)},
		})
		return renderHumanBlocks(definition, defaults), nil
	}
	return bundle
}

func formatLoopLifecycleSources(sources map[string]string) string {
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, key+"="+sources[key])
	}
	return strings.Join(items, ", ")
}
