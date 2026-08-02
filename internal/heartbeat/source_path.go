package heartbeat

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/managedsidecar"
)

func safeSourcePath(sourcePath string, workspaceRoot string) (string, *Diagnostic) {
	safePath, issue := managedsidecar.SafeSourcePath(sourcePath, workspaceRoot)
	if issue == nil {
		return safePath, nil
	}
	message := "HEARTBEAT.md path must stay inside the workspace root"
	switch issue.Kind {
	case managedsidecar.IssueInvalidNUL:
		safePath = FileName
		message = "HEARTBEAT.md path contains an invalid NUL byte"
	case managedsidecar.IssueResolveRoot:
		message = diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root: %v", issue.Err), 300)
	case managedsidecar.IssueResolveTarget:
		message = diagnostics.RedactAndBound(fmt.Sprintf("resolve HEARTBEAT.md path: %v", issue.Err), 300)
	case managedsidecar.IssueSymlinkTargetOutside:
		message = "HEARTBEAT.md symlink target must stay inside the workspace root"
	}
	return safePath, &Diagnostic{
		Code:       heartbeatHeartbeatPathEscapeKey,
		Severity:   diagnosticError,
		Message:    message,
		SourcePath: firstNonEmpty(issue.SourcePath, safePath),
	}
}

func safePathWithoutRoot(path string) string {
	safe := managedsidecar.SafePathWithoutRoot(path)
	if safe == "" {
		return FileName
	}
	return safe
}

func heartbeatPathForAgent(agentPath string) (string, error) {
	return managedsidecar.SidecarPath(agentPath, FileName)
}

func isAllowedField(key string) bool {
	switch key {
	case heartbeatVersionKey, heartbeatEnabledKey, "summary", "preferences", "context":
		return true
	default:
		return false
	}
}

func forbiddenOwner(key string) string {
	switch normalizeKey(key) {
	case "session", "session_health", "session_liveness", "liveness", "health",
		"supervision", "session_supervision", "activity", "activity_heartbeat",
		"activity_heartbeat_interval", "inactivity_warning_after", "inactivity_timeout":
		return "[session.supervision] and daemon session health"
	case "scheduler", "schedule", "schedules", "cadence", "interval", "every",
		"default_interval", "sweep", "wake_loop", "run_loop", "loop", "cron":
		return "scheduler config"
	case "task", "tasks", "task_runs", "task_run", "claim_next_run", "claimnextrun",
		"claim", "claim_token", "claim_token_hash", "ownership", "owner", "queue", "queues":
		return "task runtime and ClaimNextRun"
	case "lease", "leases", "lease_duration", "task_lease", "lease_heartbeat",
		"heartbeat_run_lease", "heartbeatrunlease", "heartbeat_at":
		return "task lease heartbeat"
	case "network", "greet", "presence", "peer_presence", "peers",
		"channels", "channel":
		return "Compozy Network membership"
	case "provider", "providers", "model", "command", "tools", "toolsets", "deny_tools",
		"permissions", "capabilities", "capability", "hooks", "mcp_servers", "env", "config":
		return "agent definition or runtime config"
	default:
		return ""
	}
}

func markdownHeadingKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmed) || !unicode.IsSpace(rune(trimmed[level])) {
		return "", false
	}
	title := strings.TrimSpace(trimmed[level:])
	title = strings.Trim(title, "# ")
	if title == "" {
		return "", false
	}
	return normalizeKey(title), true
}

func bodyDeclarationKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimLeft(trimmed, "-*+0123456789. \t")
	before, _, hasColon := strings.Cut(trimmed, ":")
	if !hasColon {
		var hasEquals bool
		before, _, hasEquals = strings.Cut(trimmed, "=")
		if !hasEquals {
			return "", false
		}
	}
	key := normalizeKey(before)
	if key == "" {
		return "", false
	}
	return key, true
}

func normalizeKey(value string) string {
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func sortedKeys(raw map[string]any) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func frontmatterKeyLocation(metadata []byte, key string) (int, int) {
	lines := strings.Split(string(metadata), "\n")
	for idx, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		col := len(line) - len(trimmed) + 1
		before, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(before) == key {
			return idx + 2, col
		}
	}
	return 2, 1
}

func bodyStartLine(metadata []byte) int {
	if len(metadata) == 0 {
		return 3
	}
	return 3 + strings.Count(string(metadata), "\n")
}
