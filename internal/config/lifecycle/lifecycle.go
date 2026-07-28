// Package lifecycle owns the config-apply lifecycle matrix shared by runtime,
// API, CLI, web, docs, and codegen checks.
package lifecycle

import (
	"fmt"
	"sort"
	"strings"
)

// Lifecycle names how a desired config change becomes daemon runtime truth.
type Lifecycle string

const (
	// Live applies in-place and advances the active config generation.
	Live Lifecycle = "live"
	// LiveAdd applies a new catalog entry in-place and advances the active generation.
	LiveAdd Lifecycle = "live-add"
	// LiveRemoveIfUnused applies a removal only when the removed resource has no active users.
	LiveRemoveIfUnused Lifecycle = "live-remove-if-unused"
	// RestartRequired writes desired state but cannot update daemon runtime truth.
	RestartRequired Lifecycle = "restart-required"
	// SessionRebind applies to new sessions while existing sessions keep their bound runtime values.
	SessionRebind Lifecycle = "session-rebind"
)

// Status names the persisted apply-record state.
type Status string

const (
	StatusPendingApply Status = "pending_apply"
	StatusApplied      Status = "applied"
	StatusBlocked      Status = "blocked"
	StatusFailed       Status = "failed"
)

// DiffClass summarizes the broad subsystem touched by one apply attempt.
type DiffClass string

const (
	DiffClassLive               DiffClass = DiffClass(Live)
	DiffClassLiveAdd            DiffClass = DiffClass(LiveAdd)
	DiffClassLiveRemoveIfUnused DiffClass = DiffClass(LiveRemoveIfUnused)
	DiffClassRestartRequired    DiffClass = DiffClass(RestartRequired)
	DiffClassSessionRebind      DiffClass = DiffClass(SessionRebind)
)

// NextAction is the user/agent-facing recovery or continuation hint.
type NextAction string

const (
	NextActionNone          NextAction = "none"
	NextActionRestartDaemon NextAction = "restart-daemon"
	NextActionNewSession    NextAction = "new-session"
	NextActionRetry         NextAction = "retry"
)

const (
	pathDaemonReloadTimeoutProviders = "daemon.reload_timeouts.providers"
	pathDaemonReloadTimeoutMCP       = "daemon.reload_timeouts.mcp"
	pathDaemonReloadTimeoutBridges   = "daemon.reload_timeouts.bridges"
	pathRoles                        = "roles"
)

// Rule records the lifecycle for a config path pattern.
type Rule struct {
	Pattern   string
	Lifecycle Lifecycle
	DiffClass DiffClass
}

// Matrix is the canonical config lifecycle matrix. Patterns use "." path
// segments and "*" wildcards for one segment.
var Matrix = []Rule{
	{Pattern: "skills.disabled_skills", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: pathDaemonReloadTimeoutProviders, Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: pathDaemonReloadTimeoutMCP, Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: pathDaemonReloadTimeoutBridges, Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "providers.*.models", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "providers.*.models.*", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "marketplace.catalog.*", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "extensions.marketplace.allow_unverified", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: pathRoles, Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: pathRoles + ".*", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "providers.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "mcp-servers.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "sandboxes.*", Lifecycle: SessionRebind, DiffClass: DiffClassSessionRebind},
	{Pattern: "hooks.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "extensions.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "defaults.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "limits.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "session.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "permissions.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "tools.clarify.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "tools.artifacts.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "http.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "daemon.memory_report_interval", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{
		Pattern:   "daemon.subprocess_health_escalation_threshold",
		Lifecycle: RestartRequired,
		DiffClass: DiffClassRestartRequired,
	},
	{Pattern: "daemon.socket", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "memory.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "automation.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "loops.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "goals.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "task.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "network.enabled", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "network.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "window_manager.*", Lifecycle: Live, DiffClass: DiffClassLive},
	{Pattern: "observability.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "log.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "redact.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
	{Pattern: "skills.*", Lifecycle: RestartRequired, DiffClass: DiffClassRestartRequired},
}

// ClassifyPath returns the matrix rule for one changed config path.
func ClassifyPath(path string) (Rule, error) {
	normalized := normalizePath(path)
	if normalized == "" {
		return Rule{}, fmt.Errorf("config lifecycle: path is required")
	}
	for _, rule := range Matrix {
		if pathMatches(rule.Pattern, normalized) {
			return rule, nil
		}
	}
	return Rule{}, fmt.Errorf("config lifecycle: unsupported path %q", normalized)
}

// ClassifyPaths returns the aggregate lifecycle and diff class for a mutation.
func ClassifyPaths(paths []string) (Lifecycle, DiffClass, error) {
	if len(paths) == 0 {
		return Live, DiffClassLive, nil
	}
	var (
		lifecycle Lifecycle
		diffClass DiffClass
		seen      bool
	)
	for _, path := range paths {
		rule, err := ClassifyPath(path)
		if err != nil {
			return "", "", err
		}
		if !seen {
			lifecycle = rule.Lifecycle
			diffClass = rule.DiffClass
			seen = true
			continue
		}
		lifecycle = dominantLifecycle(lifecycle, rule.Lifecycle)
		diffClass = DiffClass(lifecycle)
	}
	return lifecycle, diffClass, nil
}

// DiffClassForRoot maps a settings section or collection name onto a diff class.
func DiffClassForRoot(root string) DiffClass {
	switch strings.TrimSpace(root) {
	case "skills", pathRoles, "window-manager":
		return DiffClassLive
	case "sandboxes":
		return DiffClassSessionRebind
	default:
		return DiffClassRestartRequired
	}
}

// NextActionForLifecycle returns the canonical user-visible next action.
func NextActionForLifecycle(lifecycle Lifecycle, status Status) NextAction {
	switch {
	case status == StatusFailed:
		return NextActionRetry
	case lifecycle == RestartRequired && status == StatusBlocked:
		return NextActionRestartDaemon
	case lifecycle == SessionRebind && status == StatusApplied:
		return NextActionNewSession
	default:
		return NextActionNone
	}
}

// SortedMatrix returns a stable copy for docs and tests.
func SortedMatrix() []Rule {
	rules := append([]Rule(nil), Matrix...)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Pattern < rules[j].Pattern
	})
	return rules
}

func normalizePath(path string) string {
	parts := strings.Split(strings.TrimSpace(path), ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, ".")
}

func pathMatches(pattern string, path string) bool {
	patternParts := strings.Split(normalizePath(pattern), ".")
	pathParts := strings.Split(normalizePath(path), ".")
	if len(patternParts) > 0 && patternParts[len(patternParts)-1] == "*" && len(pathParts) >= len(patternParts) {
		for i := range patternParts[:len(patternParts)-1] {
			if patternParts[i] == "*" {
				continue
			}
			if patternParts[i] != pathParts[i] {
				return false
			}
		}
		return true
	}
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, part := range patternParts {
		if part == "*" {
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}
	return true
}

func dominantLifecycle(current Lifecycle, next Lifecycle) Lifecycle {
	rank := map[Lifecycle]int{
		Live:               0,
		LiveAdd:            1,
		LiveRemoveIfUnused: 2,
		SessionRebind:      3,
		RestartRequired:    4,
	}
	if rank[next] > rank[current] {
		return next
	}
	return current
}
