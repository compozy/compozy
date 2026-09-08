package cli

import "strconv"

func sessionStopBundle(result SessionStopRecord) outputBundle {
	keys := []string{
		"session_id",
		"state",
		"verified",
		"escalated",
		"stop_cause",
		"phase",
		"stopped_after",
		"attention",
	}
	values := []string{
		result.SessionID,
		string(result.State),
		strconv.FormatBool(result.Verified),
		strconv.FormatBool(result.Escalated),
		result.StopCause,
		string(result.Phase),
		result.StoppedAfter,
		result.Attention,
	}
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			return renderHumanSection("Session Stop", []keyValue{
				{Label: agentKernelSessionValue, Value: result.SessionID},
				{Label: "State", Value: string(result.State)},
				{Label: "Verified", Value: strconv.FormatBool(result.Verified)},
				{Label: "Escalated", Value: strconv.FormatBool(result.Escalated)},
				{Label: "Cause", Value: stringOrDash(result.StopCause)},
				{Label: "Phase", Value: stringOrDash(string(result.Phase))},
				{Label: "Elapsed", Value: stringOrDash(result.StoppedAfter)},
				{Label: "Attention", Value: stringOrDash(result.Attention)},
			}), nil
		},
		toon: func() (string, error) { return renderToonObject("session_stop", keys, values), nil },
	}
}
