package heartbeat

import (
	"errors"
	"fmt"

	"path/filepath"
	"sort"

	"strings"

	"unicode"

	"github.com/compozy/agh/internal/diagnostics"
)

func safeSourcePath(sourcePath string, workspaceRoot string) (string, *Diagnostic) {
	trimmed := strings.TrimSpace(sourcePath)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsRune(trimmed, 0) {
		return FileName, &Diagnostic{
			Code:       heartbeatHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    "HEARTBEAT.md path contains an invalid NUL byte",
			SourcePath: FileName,
		}
	}

	cleanSource := filepath.Clean(trimmed)
	if strings.TrimSpace(workspaceRoot) == "" {
		return safePathWithoutRoot(cleanSource), nil
	}

	absRoot, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return safePathWithoutRoot(cleanSource), &Diagnostic{
			Code:       heartbeatHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve workspace root: %v", err), 300),
			SourcePath: safePathWithoutRoot(cleanSource),
		}
	}
	sourceForRoot := cleanSource
	if !filepath.IsAbs(sourceForRoot) {
		sourceForRoot = filepath.Join(absRoot, sourceForRoot)
	}
	absSource, err := filepath.Abs(sourceForRoot)
	if err != nil {
		return safePathWithoutRoot(cleanSource), &Diagnostic{
			Code:       heartbeatHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    diagnostics.RedactAndBound(fmt.Sprintf("resolve HEARTBEAT.md path: %v", err), 300),
			SourcePath: safePathWithoutRoot(cleanSource),
		}
	}

	safePath, within := relativePathWithinRoot(absRoot, absSource)
	if !within {
		return safePath, &Diagnostic{
			Code:       heartbeatHeartbeatPathEscapeKey,
			Severity:   diagnosticError,
			Message:    "HEARTBEAT.md path must stay inside the workspace root",
			SourcePath: safePath,
		}
	}
	if resolvedRoot, rootErr := filepath.EvalSymlinks(absRoot); rootErr == nil {
		if resolvedSource, sourceErr := filepath.EvalSymlinks(absSource); sourceErr == nil {
			safeResolved, resolvedWithin := relativePathWithinRoot(resolvedRoot, resolvedSource)
			if !resolvedWithin {
				return safePath, &Diagnostic{
					Code:       heartbeatHeartbeatPathEscapeKey,
					Severity:   diagnosticError,
					Message:    "HEARTBEAT.md symlink target must stay inside the workspace root",
					SourcePath: safeResolved,
				}
			}
		}
	}
	return safePath, nil
}

func relativePathWithinRoot(root string, target string) (string, bool) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return safePathWithoutRoot(target), false
	}
	if rel == "." {
		return ".", true
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return safePathWithoutRoot(target), false
	}
	return filepath.ToSlash(rel), true
}

func safePathWithoutRoot(path string) string {
	slashed := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(slashed, "/")
	for idx := 0; idx < len(parts)-2; idx++ {
		if parts[idx] == ".agh" && parts[idx+1] == "agents" {
			return strings.Join(parts[idx:], "/")
		}
	}
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	if slashed == "." {
		return FileName
	}
	return strings.TrimPrefix(slashed, "/")
}

func heartbeatPathForAgent(agentPath string) (string, error) {
	trimmed := strings.TrimSpace(agentPath)
	if trimmed == "" {
		return "", errors.New("agent path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if strings.EqualFold(filepath.Base(cleaned), "AGENT.md") {
		return filepath.Join(filepath.Dir(cleaned), FileName), nil
	}
	return filepath.Join(cleaned, FileName), nil
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
		return "AGH Network membership"
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
