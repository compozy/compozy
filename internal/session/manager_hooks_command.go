package session

import (
	"strings"

	shellquote "github.com/kballard/go-shellquote"
)

func splitCommand(command string) (string, []string) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", nil
	}

	parts, err := shellquote.Split(trimmed)
	if err != nil || len(parts) == 0 {
		return trimmed, nil
	}

	return parts[0], append([]string(nil), parts[1:]...)
}

func joinCommand(command string, args []string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	if len(args) == 0 {
		return trimmed
	}

	parts := make([]string, 0, len(args)+1)
	parts = append(parts, trimmed)
	for _, arg := range args {
		if item := strings.TrimSpace(arg); item != "" {
			parts = append(parts, item)
		}
	}
	return shellquote.Join(parts...)
}

func agentCommandAndArgs(proc *AgentProcess) (string, []string) {
	if proc == nil {
		return "", nil
	}
	if len(proc.Args) > 0 {
		return strings.TrimSpace(proc.Command), append([]string(nil), proc.Args...)
	}
	return splitCommand(proc.Command)
}

func agentCwd(proc *AgentProcess) string {
	if proc == nil {
		return ""
	}
	return strings.TrimSpace(proc.Cwd)
}

func agentPID(proc *AgentProcess) int {
	if proc == nil {
		return 0
	}
	return proc.PID
}
