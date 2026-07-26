package observe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

const (
	healthTimeoutKey = "timeout"
)

const (
	sessionStateOrphaned               = "orphaned"
	sessionActivityHealthStatusActive  = "active"
	sessionActivityHealthStatusWarning = "warning"
	sessionActivityHealthStatusStalled = "stalled"
	maxFailureHealthBytes              = 2048
	observeHealthStatusOK              = "ok"
	observeHealthStatusDegraded        = "degraded"
)

// Health is the daemon-local observability health snapshot.
type Health struct {
	Status             string                  `json:"status"`
	UptimeSeconds      int64                   `json:"uptime_seconds"`
	ActiveSessions     int                     `json:"active_sessions"`
	ActiveAgents       int                     `json:"active_agents"`
	GlobalDBSizeBytes  int64                   `json:"global_db_size_bytes"`
	SessionDBSizeBytes int64                   `json:"session_db_size_bytes"`
	Persistence        PersistenceHealth       `json:"persistence"`
	Retention          RetentionHealth         `json:"retention"`
	Failures           FailureHealth           `json:"failures"`
	AgentProbes        []acp.ProbeResult       `json:"agent_probes,omitempty"`
	Bridges            BridgeAggregateHealth   `json:"bridges"`
	Tasks              TaskHealth              `json:"tasks"`
	Activities         []SessionActivityHealth `json:"activities,omitempty"`
	Version            string                  `json:"version"`
}

// FailureHealth summarizes persisted lifecycle failures across indexed sessions.
type FailureHealth struct {
	Status string                    `json:"status"`
	Total  int                       `json:"total"`
	ByKind map[store.FailureKind]int `json:"by_kind,omitempty"`
	Recent []SessionFailureHealth    `json:"recent,omitempty"`
}

// SessionFailureHealth is a compact, redacted lifecycle failure row.
type SessionFailureHealth struct {
	SessionID       string            `json:"session_id"`
	AgentName       string            `json:"agent_name,omitempty"`
	Provider        string            `json:"provider,omitempty"`
	WorkspaceID     string            `json:"workspace_id,omitempty"`
	State           string            `json:"state,omitempty"`
	FailureKind     store.FailureKind `json:"failure_kind"`
	Summary         string            `json:"summary,omitempty"`
	CrashBundlePath string            `json:"crash_bundle_path,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// PersistenceHealth captures store health that later hardening tracks can extend
// without overloading unrelated API payloads.
type PersistenceHealth struct {
	Status             string `json:"status"`
	GlobalDBSizeBytes  int64  `json:"global_db_size_bytes"`
	SessionDBSizeBytes int64  `json:"session_db_size_bytes"`
}

// SessionActivityHealth captures the active runtime supervision state exposed
// through the daemon health view.
type SessionActivityHealth struct {
	SessionID          string     `json:"session_id"`
	TurnID             string     `json:"turn_id,omitempty"`
	TurnSource         string     `json:"turn_source,omitempty"`
	TurnStartedAt      *time.Time `json:"turn_started_at,omitempty"`
	LastActivityAt     *time.Time `json:"last_activity_at,omitempty"`
	LastActivityKind   string     `json:"last_activity_kind,omitempty"`
	LastActivityDetail string     `json:"last_activity_detail,omitempty"`
	CurrentTool        string     `json:"current_tool,omitempty"`
	ToolCallID         string     `json:"tool_call_id,omitempty"`
	LastProgressAt     *time.Time `json:"last_progress_at,omitempty"`
	IterationCurrent   int        `json:"iteration_current,omitempty"`
	IterationMax       int        `json:"iteration_max,omitempty"`
	IdleSeconds        int64      `json:"idle_seconds,omitempty"`
	ElapsedSeconds     int64      `json:"elapsed_seconds,omitempty"`
	Status             string     `json:"status"`
	StallState         string     `json:"stall_state,omitempty"`
	StallReason        string     `json:"stall_reason,omitempty"`
}

// Health returns the current daemon-local observability health snapshot.
func (o *Observer) Health(ctx context.Context) (Health, error) {
	activeSessions, activeAgents, activities, err := o.activeSnapshot(ctx)
	if err != nil {
		return Health{}, err
	}

	globalDBSize, err := databaseSize(o.registry.Path())
	if err != nil {
		return Health{}, fmt.Errorf("observe: measure global database size: %w", err)
	}

	sessionDBSize, err := totalSessionDBSize(o.homePaths.SessionsDir)
	if err != nil {
		return Health{}, fmt.Errorf("observe: measure session database size: %w", err)
	}

	_, bridgeHealth, err := o.collectBridgeHealth(ctx)
	if err != nil {
		return Health{}, err
	}
	taskHealth, err := o.collectTaskHealth(ctx)
	if err != nil {
		return Health{}, fmt.Errorf("observe: collect task health: %w", err)
	}
	failureHealth, err := o.collectFailureHealth(ctx)
	if err != nil {
		return Health{}, err
	}
	agentProbes, err := o.collectAgentProbeHealth(ctx)
	if err != nil {
		return Health{}, err
	}

	uptimeSeconds := max(int64(o.now().Sub(o.startedAt).Seconds()), 0)
	retentionHealth := o.retentionHealthSnapshot()
	persistenceHealth := PersistenceHealth{
		Status:             persistenceStatus(retentionHealth),
		GlobalDBSizeBytes:  globalDBSize,
		SessionDBSizeBytes: sessionDBSize,
	}

	return Health{
		Status: observeHealthStatus(
			persistenceHealth.Status,
			failureHealth.Status,
			agentProbeStatus(agentProbes),
		),
		UptimeSeconds:      uptimeSeconds,
		ActiveSessions:     activeSessions,
		ActiveAgents:       activeAgents,
		GlobalDBSizeBytes:  globalDBSize,
		SessionDBSizeBytes: sessionDBSize,
		Persistence:        persistenceHealth,
		Retention:          retentionHealth,
		Failures:           failureHealth,
		AgentProbes:        agentProbes,
		Bridges:            bridgeHealth,
		Tasks:              taskHealth,
		Activities:         activities,
		Version:            o.versionSource().Version,
	}, nil
}

func persistenceStatus(retention RetentionHealth) string {
	if retention.LastSweepStatus == retentionSweepStatusError {
		return observeHealthStatusDegraded
	}
	return observeHealthStatusOK
}

func observeHealthStatus(statuses ...string) string {
	for _, status := range statuses {
		if strings.TrimSpace(status) == observeHealthStatusDegraded {
			return observeHealthStatusDegraded
		}
	}
	return observeHealthStatusOK
}

func agentProbeStatus(probes []acp.ProbeResult) string {
	for _, probe := range probes {
		if strings.TrimSpace(probe.Status) != "" && strings.TrimSpace(probe.Status) != acp.ProbeStatusOK {
			return observeHealthStatusDegraded
		}
	}
	return observeHealthStatusOK
}

func (o *Observer) collectAgentProbeHealth(ctx context.Context) ([]acp.ProbeResult, error) {
	if o.agentProbeSource == nil {
		return nil, nil
	}
	targets, err := o.agentProbeSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("observe: collect agent probe targets: %w", err)
	}
	return acp.ProbeTargets(ctx, targets, acp.ProbeOptions{
		Timeout: o.agentProbeTimeout,
		Now:     o.now,
	}), nil
}

func (o *Observer) collectFailureHealth(ctx context.Context) (FailureHealth, error) {
	sessions, err := o.registry.ListSessions(ctx, store.SessionListQuery{})
	if err != nil {
		return FailureHealth{}, fmt.Errorf("observe: list sessions for failure health: %w", err)
	}
	health := FailureHealth{
		Status: observeHealthStatusOK,
		ByKind: make(map[store.FailureKind]int),
	}
	recent := make([]SessionFailureHealth, 0, len(sessions))
	degradedTotal := 0
	for _, info := range sessions {
		if info.Failure == nil {
			continue
		}
		failure := info.Failure.Normalize()
		if failure.IsZero() {
			continue
		}
		health.Total++
		health.ByKind[failure.Kind]++
		if failureDegradesHealth(failure.Kind) {
			degradedTotal++
		}
		recent = append(recent, SessionFailureHealth{
			SessionID:       strings.TrimSpace(info.ID),
			AgentName:       strings.TrimSpace(info.AgentName),
			Provider:        strings.TrimSpace(info.Provider),
			WorkspaceID:     strings.TrimSpace(info.WorkspaceID),
			State:           strings.TrimSpace(info.State),
			FailureKind:     failure.Kind,
			Summary:         diagnostics.RedactAndBound(failure.Summary, maxFailureHealthBytes),
			CrashBundlePath: diagnostics.RedactAndBound(failure.CrashBundlePath, maxFailureHealthBytes),
			UpdatedAt:       info.UpdatedAt,
		})
	}
	if health.Total == 0 {
		health.ByKind = nil
		health.Recent = nil
	} else {
		if degradedTotal > 0 {
			health.Status = observeHealthStatusDegraded
		}
		sort.SliceStable(recent, func(i, j int) bool {
			return recent[i].UpdatedAt.After(recent[j].UpdatedAt)
		})
		if len(recent) > 10 {
			recent = recent[:10]
		}
		health.Recent = recent
	}
	return health, nil
}

func failureDegradesHealth(kind store.FailureKind) bool {
	return kind != store.FailureCanceled
}

func (o *Observer) activeSnapshot(ctx context.Context) (int, int, []SessionActivityHealth, error) {
	now := o.now()
	if o.sessionSource != nil {
		count := 0
		agents := make(map[string]struct{})
		activities := make([]SessionActivityHealth, 0)
		for _, info := range o.sessionSource.List() {
			if info == nil || info.State == session.StateStopped {
				continue
			}
			count++
			if agentName := strings.TrimSpace(info.AgentName); agentName != "" {
				agents[agentName] = struct{}{}
			}
			if activity, ok := sessionActivityHealthFromLiveness(info.ID, info.Liveness, now); ok {
				activities = append(activities, activity)
			}
		}
		return count, len(agents), activities, nil
	}

	sessions, err := o.registry.ListSessions(ctx, store.SessionListQuery{})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("observe: list sessions for health: %w", err)
	}

	count := 0
	agents := make(map[string]struct{})
	activities := make([]SessionActivityHealth, 0)
	for _, info := range sessions {
		state := strings.TrimSpace(info.State)
		if state == "" || state == string(session.StateStopped) || state == sessionStateOrphaned {
			continue
		}
		count++
		if agentName := strings.TrimSpace(info.AgentName); agentName != "" {
			agents[agentName] = struct{}{}
		}
		if activity, ok := sessionActivityHealthFromLiveness(info.ID, info.Liveness, now); ok {
			activities = append(activities, activity)
		}
	}

	return count, len(agents), activities, nil
}

func sessionActivityHealthFromLiveness(
	sessionID string,
	liveness *store.SessionLivenessMeta,
	now time.Time,
) (SessionActivityHealth, bool) {
	if liveness == nil || liveness.Activity == nil {
		return SessionActivityHealth{}, false
	}
	activity := store.CloneSessionActivityMeta(liveness.Activity)
	if activity == nil {
		return SessionActivityHealth{}, false
	}
	item := SessionActivityHealth{
		SessionID:          strings.TrimSpace(sessionID),
		TurnID:             activity.TurnID,
		TurnSource:         activity.TurnSource,
		TurnStartedAt:      cloneHealthTime(activity.TurnStartedAt),
		LastActivityAt:     cloneHealthTime(activity.LastActivityAt),
		LastActivityKind:   activity.LastActivityKind,
		LastActivityDetail: activity.LastActivityDetail,
		CurrentTool:        activity.CurrentTool,
		ToolCallID:         activity.ToolCallID,
		LastProgressAt:     cloneHealthTime(activity.LastProgressAt),
		IterationCurrent:   activity.IterationCurrent,
		IterationMax:       activity.IterationMax,
		IdleSeconds:        store.SessionActivityIdleSeconds(activity, now),
		Status:             sessionActivityHealthStatus(liveness, activity),
		StallState:         strings.TrimSpace(liveness.StallState),
		StallReason:        strings.TrimSpace(liveness.StallReason),
	}
	if !now.IsZero() && activity.TurnStartedAt != nil && !activity.TurnStartedAt.IsZero() {
		elapsed := now.UTC().Sub(activity.TurnStartedAt.UTC())
		if elapsed > 0 {
			item.ElapsedSeconds = int64(elapsed.Seconds())
		}
	}
	return item, item.SessionID != ""
}

func sessionActivityHealthStatus(liveness *store.SessionLivenessMeta, activity *store.SessionActivityMeta) string {
	if liveness != nil && strings.TrimSpace(liveness.StallState) != "" {
		return sessionActivityHealthStatusStalled
	}
	if activity != nil {
		switch strings.TrimSpace(activity.LastActivityKind) {
		case healthTimeoutKey:
			return sessionActivityHealthStatusStalled
		case "warning":
			return sessionActivityHealthStatusWarning
		}
	}
	return sessionActivityHealthStatusActive
}

func cloneHealthTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func totalSessionDBSize(sessionsDir string) (int64, error) {
	cleanDir := strings.TrimSpace(sessionsDir)
	if cleanDir == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	var total int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		size, err := databaseSize(store.SessionDBFile(filepath.Join(cleanDir, entry.Name())))
		if err != nil {
			return 0, err
		}
		total += size
	}

	return total, nil
}

func databaseSize(path string) (int64, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return 0, nil
	}

	var total int64
	for _, candidate := range []string{cleanPath, cleanPath + "-wal", cleanPath + "-shm"} {
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return 0, err
		}
		if !info.IsDir() {
			total += info.Size()
		}
	}

	return total, nil
}
