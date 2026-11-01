package heartbeat

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"time"
)

// CollectorScope is resolved by the daemon from the instance's effective reach.
type CollectorScope struct {
	ProfileID           string
	WorkspaceID         string
	Unrestricted        bool
	ReachableWorkspaces []string
	MainSessionID       string
	ChainSessionIDs     []string
	LastCompletedAt     time.Time
	AutomationCursor    string
	Now                 time.Time
}

type TaskFact struct {
	ID, RunID, ProfileID, WorkspaceID, OwnerKind, OwnerSessionID, Status string
}
type SessionFact struct {
	ID, ProfileID, WorkspaceID, Health string
	Hidden                             bool
	LastEventAt                        time.Time
	LastSeq                            int64
}
type AutomationFact struct {
	ID, ProfileID, WorkspaceID, Status, Cursor string
}
type MonitorFact struct {
	ID, SessionID string
	Fired         bool
}
type CollectorFacts struct {
	Tasks       []TaskFact
	Sessions    []SessionFact
	Automations []AutomationFact
	Monitors    []MonitorFact
}
type DigestItem struct {
	Key string `json:"key"`
}
type CheckDigest struct {
	Items            []DigestItem `json:"items"`
	Fingerprint      string       `json:"fingerprint"`
	AutomationCursor string       `json:"automation_cursor"`
	Diagnostics      []string     `json:"diagnostics,omitempty"`
}

var collectorNames = []string{"task", "session_health", "sessions_idle", "automations_failed", "monitors_fired"}

// CollectDigest accepts daemon-owned facts; callers must fetch complete eligible populations before applying caps.
func CollectDigest(scope CollectorScope, facts CollectorFacts, include []string, policyDigest, instructions string) (CheckDigest, error) {
	result := CheckDigest{Items: []DigestItem{}, AutomationCursor: scope.AutomationCursor}
	if include == nil {
		include = collectorNames
	}
	selected := map[string]bool{}
	for _, name := range include {
		if !slices.Contains(collectorNames, name) {
			result.Diagnostics = append(result.Diagnostics, "unknown context.include: "+name)
			continue
		}
		selected[name] = true
	}
	chain := append(slices.Clone(scope.ChainSessionIDs), scope.MainSessionID)
	buckets := map[string][]string{}
	for _, task := range facts.Tasks {
		if !selected["task"] || !scope.contains(task.ProfileID, task.WorkspaceID) ||
			task.OwnerKind != "agent_session" || task.OwnerSessionID == "" || !slices.Contains(chain, task.OwnerSessionID) {
			continue
		}
		if task.RunID == "" && task.Status == "blocked" {
			buckets["task"] = append(buckets["task"], "task:"+task.ID+":blocked")
		}
		if task.RunID != "" && task.Status == "queued" {
			buckets["task"] = append(buckets["task"], "run:"+task.RunID+":queued")
		}
	}
	for _, session := range facts.Sessions {
		if session.Hidden || slices.Contains(chain, session.ID) || !scope.contains(session.ProfileID, session.WorkspaceID) {
			continue
		}
		if selected["session_health"] && slices.Contains([]string{"degraded", "stale", "dead", "unknown"}, session.Health) {
			buckets["session_health"] = append(buckets["session_health"], "health:"+session.ID+":"+session.Health)
		}
		if selected["sessions_idle"] && session.LastEventAt.After(scope.LastCompletedAt) &&
			!session.LastEventAt.After(scope.Now.Add(-time.Hour)) && session.LastSeq > 0 {
			buckets["sessions_idle"] = append(buckets["sessions_idle"], fmt.Sprintf("idle:%s:%d", session.ID, session.LastSeq))
		}
	}
	if selected["automations_failed"] {
		eligible := make([]AutomationFact, 0)
		for _, run := range facts.Automations {
			if scope.contains(run.ProfileID, run.WorkspaceID) && run.Status == "failed" && run.Cursor > scope.AutomationCursor {
				eligible = append(eligible, run)
			}
		}
		slices.SortFunc(eligible, func(a, b AutomationFact) int { return cmp.Or(cmp.Compare(a.Cursor, b.Cursor), cmp.Compare(a.ID, b.ID)) })
		seen := map[string]bool{}
		for _, run := range eligible {
			if seen[run.ID] {
				continue
			}
			if len(buckets["automations_failed"]) == 10 {
				break
			}
			seen[run.ID] = true
			buckets["automations_failed"] = append(buckets["automations_failed"], "auto:"+run.ID)
			result.AutomationCursor = run.Cursor
		}
	}
	if selected["monitors_fired"] {
		for _, monitor := range facts.Monitors {
			if monitor.Fired && scope.MainSessionID != "" && monitor.SessionID == scope.MainSessionID {
				buckets["monitors_fired"] = append(buckets["monitors_fired"], "monitor:"+monitor.ID)
			}
		}
	}
	for _, name := range collectorNames {
		keys := buckets[name]
		slices.Sort(keys)
		keys = slices.Compact(keys)
		cap := 10
		if name == "task" || name == "sessions_idle" {
			cap = 20
		}
		for _, key := range keys[:min(len(keys), cap)] {
			result.Items = append(result.Items, DigestItem{Key: key})
		}
	}
	slices.SortFunc(result.Items, func(a, b DigestItem) int { return cmp.Compare(a.Key, b.Key) })
	slices.Sort(result.Diagnostics)
	result.Diagnostics = slices.Compact(result.Diagnostics)
	payload, err := json.Marshal(struct {
		Items        []DigestItem
		Policy       string
		Instructions string
	}{result.Items, policyDigest, strings.TrimSpace(instructions)})
	if err != nil {
		return CheckDigest{}, fmt.Errorf("encode heartbeat digest: %w", err)
	}
	sum := sha256.Sum256(payload)
	result.Fingerprint = hex.EncodeToString(sum[:])
	return result, nil
}

func (scope CollectorScope) contains(profile, workspace string) bool {
	if scope.ProfileID == "" || profile != scope.ProfileID {
		return false
	}
	if scope.WorkspaceID != "" {
		return workspace == scope.WorkspaceID
	}
	return workspace == "" || scope.Unrestricted && slices.Contains(scope.ReachableWorkspaces, workspace)
}
