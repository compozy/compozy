package recovery

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

const recoveryLogTailBytes = 12 * 1024

//go:embed guidance/*.md
var recoveryGuidanceFS embed.FS

func buildRecoverySystemPrompt(in RemediationInput) (string, error) {
	systematic, err := recoveryGuidanceFS.ReadFile("guidance/systematic-debugging.md")
	if err != nil {
		return "", fmt.Errorf("read systematic debugging guidance: %w", err)
	}
	noWorkarounds, err := recoveryGuidanceFS.ReadFile("guidance/no-workarounds.md")
	if err != nil {
		return "", fmt.Errorf("read no-workarounds guidance: %w", err)
	}
	sections := []string{
		strings.Join([]string{
			"You are the Compozy recovery agent.",
			"Triage the failed run, fix only project-side root causes,",
			"and return a constrained JSON verdict.",
		}, " "),
		strings.TrimSpace(string(systematic)),
		strings.TrimSpace(string(noWorkarounds)),
		buildFailureContextSection(in),
		`Verdict contract: finish with exactly one JSON object like {"decision":"fixed","reason":"what changed","changed_files":["path"]} or {"decision":"reject","reason":"why this is not safely fixable","changed_files":[]}.`,
	}
	return strings.Join(nonEmptySections(sections), "\n\n"), nil
}

func buildRecoveryPrompt() string {
	infraRejectLine := "If the failure is infrastructure, cancellation, missing credentials, " +
		"or otherwise outside this workspace, do not edit files and return a reject verdict."
	return strings.Join([]string{
		"Inspect the failure context in your system prompt.",
		"Make only the smallest production-code fix needed for the failed project-side invariant.",
		infraRejectLine,
		"Return the constrained JSON verdict as the final visible output.",
	}, "\n")
}

func buildFailureContextSection(in RemediationInput) string {
	var b strings.Builder
	b.WriteString("Failure context:\n")
	writeContextLine(&b, "failed_run_id", in.Outcome.RunID)
	writeContextLine(&b, "failed_run_status", string(in.Outcome.Status))
	writeContextLine(&b, "failed_run_artifacts", in.Outcome.ArtifactsDir)
	writeRuntimeScope(&b, in.FailedConfig)
	failedIDs := in.Outcome.FailedJobIDs()
	if len(failedIDs) > 0 {
		writeContextLine(&b, "failed_job_ids", strings.Join(failedIDs, ", "))
	}
	for _, job := range in.Outcome.Jobs {
		if job.Status != StatusFailed {
			continue
		}
		b.WriteString("\nFailed job:\n")
		writeContextLine(&b, "safe_name", job.SafeName)
		writeContextLine(&b, "status", string(job.Status))
		writeContextLine(&b, "exit_code", fmt.Sprintf("%d", job.ExitCode))
		writeContextLine(&b, "error", job.Error)
		writeLogContext(&b, "stdout_log", job.OutLog)
		writeLogContext(&b, "stderr_log", job.ErrLog)
	}
	return strings.TrimSpace(b.String())
}

func writeRuntimeScope(b *strings.Builder, cfg *model.RuntimeConfig) {
	if cfg == nil {
		return
	}
	writeContextLine(b, "workspace_root", cfg.WorkspaceRoot)
	writeContextLine(b, "scope_name", cfg.Name)
	writeContextLine(b, "scope_mode", string(cfg.Mode))
	writeContextLine(b, "scope_pr", cfg.PR)
	writeContextLine(b, "scope_reviews_dir", cfg.ReviewsDir)
	writeContextLine(b, "scope_tasks_dir", cfg.TasksDir)
}

func writeLogContext(b *strings.Builder, label string, path string) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return
	}
	writeContextLine(b, label, trimmed)
	content, err := readLogTail(trimmed, recoveryLogTailBytes)
	if err != nil {
		writeContextLine(b, label+"_read_error", err.Error())
		return
	}
	if strings.TrimSpace(content) == "" {
		return
	}
	b.WriteString(label)
	b.WriteString("_tail:\n")
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteString("\n")
}

func readLogTail(path string, limit int64) (string, error) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", cleanPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", cleanPath)
	}
	file, err := os.Open(cleanPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", cleanPath, err)
	}
	defer file.Close()
	size := info.Size()
	offset := int64(0)
	if limit > 0 && size > limit {
		offset = size - limit
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return "", fmt.Errorf("seek %s: %w", cleanPath, err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", cleanPath, err)
	}
	return string(data), nil
}

func writeContextLine(b *strings.Builder, key string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	b.WriteString("- ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(trimmed)
	b.WriteString("\n")
}

func nonEmptySections(sections []string) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
