package cli

import (
	"strconv"
	"strings"
	"time"
)

func sessionBundle(info SessionRecord, now func() time.Time) outputBundle {
	return outputBundle{
		jsonValue: info,
		human:     func() (string, error) { return renderSessionHuman(info, now) },
		toon:      func() (string, error) { return renderSessionToon(info) },
	}
}

func renderSessionHuman(info SessionRecord, now func() time.Time) (string, error) {
	base := renderHumanSection(sessionSessionValue, []keyValue{
		{Label: "ID", Value: stringOrDash(info.ID)},
		{Label: sessionNameValue, Value: stringOrDash(info.Name)},
		{Label: sessionAgentValue, Value: stringOrDash(info.AgentName)},
		{Label: sessionProviderValue, Value: stringOrDash(info.Provider)},
		{Label: sessionWorkspaceValue, Value: stringOrDash(displaySessionWorkspace(info))},
		{Label: sessionChannelValue, Value: sessionResolvedChannel(info)},
		{Label: sessionStateValue, Value: stringOrDash(string(info.State))},
		{Label: sessionBadgeValue, Value: stringOrDash(string(info.Badge))},
		{Label: "Attached To", Value: stringOrDash(info.AttachedTo)},
		{Label: "Attach Expires", Value: stringOrDash(formatTimePtr(info.AttachExpiresAt))},
		{Label: "Stop Reason", Value: stringOrDash(string(info.StopReason))},
		{Label: "Stop Detail", Value: stringOrDash(info.StopDetail)},
		{Label: "Failure Kind", Value: stringOrDash(sessionFailureKind(info))},
		{Label: "Failure Summary", Value: stringOrDash(sessionFailureSummary(info))},
		{Label: "Crash Bundle", Value: stringOrDash(sessionCrashBundlePath(info))},
		{Label: "ACP Session", Value: stringOrDash(info.ACPSessionID)},
		{Label: sessionCreatedValue, Value: stringOrDash(formatTime(info.CreatedAt))},
		{Label: sessionUpdatedValue, Value: stringOrDash(formatTime(info.UpdatedAt))},
		{Label: "Age", Value: stringOrDash(formatAge(now, info.CreatedAt))},
	})

	blocks := []string{base}
	blocks = appendSessionSandboxBlock(blocks, info)
	blocks = appendSessionCapsBlock(blocks, info)
	return renderHumanBlocks(blocks...), nil
}

func appendSessionSandboxBlock(blocks []string, info SessionRecord) []string {
	if info.Sandbox == nil {
		return blocks
	}
	return append(blocks, renderHumanSection("Sandbox", []keyValue{
		{Label: sessionBackendValue, Value: stringOrDash(info.Sandbox.Backend)},
		{Label: sessionProfileValue, Value: stringOrDash(info.Sandbox.Profile)},
		{Label: "Sandbox ID", Value: stringOrDash(info.Sandbox.SandboxID)},
		{Label: "Instance ID", Value: stringOrDash(info.Sandbox.InstanceID)},
		{Label: sessionStateValue, Value: stringOrDash(info.Sandbox.State)},
		{Label: "Last Sync Error", Value: stringOrDash(info.Sandbox.LastSyncError)},
	}))
}

func appendSessionCapsBlock(blocks []string, info SessionRecord) []string {
	if info.ACPCaps == nil {
		return blocks
	}
	return append(blocks, renderHumanSection("Capabilities", []keyValue{
		{Label: "Supports Load", Value: strconv.FormatBool(info.ACPCaps.SupportsLoadSession)},
		{Label: "Modes", Value: stringOrDash(strings.Join(info.ACPCaps.SupportedModes, ", "))},
	}))
}

func renderSessionToon(info SessionRecord) (string, error) {
	return renderToonObject(sessionSessionKey, []string{
		"id",
		sessionNameKey,
		sessionAgentNameKey,
		sessionProviderKey,
		"sandbox_backend",
		workspaceSkillSource,
		sessionChannelKey,
		sessionStateKey,
		sessionBadgeKey,
		"attached_to",
		"attach_expires_at",
		"stop_reason",
		taskFailureKindKey,
		"failure_summary",
		"crash_bundle_path",
		"acp_session_id",
		sessionCreatedAtKey,
		sessionUpdatedAtKey,
	}, []string{
		info.ID,
		info.Name,
		info.AgentName,
		info.Provider,
		sessionSandboxBackend(info),
		displaySessionWorkspace(info),
		sessionResolvedChannelRaw(info),
		string(info.State),
		string(info.Badge),
		info.AttachedTo,
		formatTimePtr(info.AttachExpiresAt),
		string(info.StopReason),
		sessionFailureKind(info),
		sessionFailureSummary(info),
		sessionCrashBundlePath(info),
		info.ACPSessionID,
		formatTime(info.CreatedAt),
		formatTime(info.UpdatedAt),
	}), nil
}

func sessionResumeEmptyBundle() outputBundle {
	payload := struct {
		Resumed *SessionRecord `json:"resumed"`
		Reason  string         `json:"reason"`
	}{
		Reason: "no_eligible_sessions",
	}
	return outputBundle{
		jsonValue: payload,
		human: func() (string, error) {
			return "No resumable sessions; start a new one with 'agh session new'.", nil
		},
		toon: func() (string, error) {
			return renderToonObject("resume", []string{"resumed", "reason"}, []string{"", payload.Reason}), nil
		},
	}
}

func sessionRecapBundle(record *SessionRecapRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			blocks := []string{
				renderHumanSection("Session Recap", []keyValue{
					{Label: sessionSessionValue, Value: stringOrDash(record.Session.ID)},
					{Label: sessionBadgeValue, Value: stringOrDash(string(record.Session.Badge))},
					{Label: "Markers", Value: strconv.Itoa(len(record.RecentMarkers))},
					{Label: "Messages", Value: strconv.Itoa(len(record.RecentMessages))},
					{Label: "Event Cursor", Value: strconv.FormatInt(record.Snapshot.EventCursor, 10)},
					{Label: "Consistency", Value: stringOrDash(record.Snapshot.Consistency)},
				}),
			}
			if len(record.RecentMarkers) > 0 {
				items := make([]keyValue, 0, len(record.RecentMarkers))
				for _, marker := range record.RecentMarkers {
					items = append(items, keyValue{
						Label: stringOrDash(marker.Kind),
						Value: stringOrDash(marker.Summary),
					})
				}
				blocks = append(blocks, renderHumanSection("Recent Markers", items))
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			return renderToonObject("recap", []string{
				sessionSessionIDKey,
				sessionBadgeKey,
				"markers",
				sessionMessagesKey,
				"event_cursor",
				"consistency",
			}, []string{
				record.Session.ID,
				string(record.Session.Badge),
				strconv.Itoa(len(record.RecentMarkers)),
				strconv.Itoa(len(record.RecentMessages)),
				strconv.FormatInt(record.Snapshot.EventCursor, 10),
				record.Snapshot.Consistency,
			}), nil
		},
	}
}

func sessionSandboxBackend(info SessionRecord) string {
	if info.Sandbox == nil {
		return ""
	}
	return strings.TrimSpace(info.Sandbox.Backend)
}

func sessionRepairBundle(record SessionRepairRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Session Repair", []keyValue{
				{Label: sessionSessionValue, Value: stringOrDash(record.SessionID)},
				{Label: "Persisted", Value: strconv.FormatBool(record.Persisted)},
				{Label: "Issues", Value: stringOrDash(sessionRepairIssueSummary(record.Issues))},
				{Label: "Actions", Value: stringOrDash(sessionRepairActionSummary(record.Actions))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("repair", []string{
				sessionSessionIDKey,
				"persisted",
				"issues",
				"actions",
			}, []string{
				record.SessionID,
				strconv.FormatBool(record.Persisted),
				sessionRepairIssueSummary(record.Issues),
				sessionRepairActionSummary(record.Actions),
			}), nil
		},
	}
}

func sessionApprovalBundle(sessionID string, record SessionApprovalRecord) outputBundle {
	payload := struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
	}{
		SessionID: strings.TrimSpace(sessionID),
		Status:    strings.TrimSpace(record.Status),
	}
	return outputBundle{
		jsonValue: payload,
		human: func() (string, error) {
			return renderHumanSection("Session Approval", []keyValue{
				{Label: sessionSessionValue, Value: stringOrDash(payload.SessionID)},
				{Label: sessionStatusValue, Value: stringOrDash(payload.Status)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"session_approval",
				[]string{sessionSessionIDKey, sessionStatusKey},
				[]string{payload.SessionID, payload.Status},
			), nil
		},
	}
}

func sessionRepairIssueSummary(items []SessionRepairIssueRecord) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, repairSummaryPart(item.Code, item.TurnID, item.EventID))
	}
	return strings.Join(parts, ", ")
}

func sessionRepairActionSummary(items []SessionRepairActionRecord) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		ref := item.EventID
		if ref == "" {
			ref = item.ToolCallID
		}
		parts = append(parts, repairSummaryPart(item.Code, item.TurnID, ref))
	}
	return strings.Join(parts, ", ")
}

func repairSummaryPart(code string, turnID string, ref string) string {
	value := strings.TrimSpace(code)
	if trimmedTurn := strings.TrimSpace(turnID); trimmedTurn != "" {
		value += ":" + trimmedTurn
	}
	if trimmedRef := strings.TrimSpace(ref); trimmedRef != "" {
		value += ":" + trimmedRef
	}
	return value
}

func sessionFailureKind(info SessionRecord) string {
	if info.Failure == nil {
		return ""
	}
	return strings.TrimSpace(string(info.Failure.Kind))
}

func sessionFailureSummary(info SessionRecord) string {
	if info.Failure == nil {
		return ""
	}
	return strings.TrimSpace(info.Failure.Summary)
}

func sessionCrashBundlePath(info SessionRecord) string {
	if info.Failure == nil {
		return ""
	}
	return strings.TrimSpace(info.Failure.CrashBundlePath)
}
