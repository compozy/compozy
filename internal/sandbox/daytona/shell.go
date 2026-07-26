package daytona

import (
	"fmt"
	"path"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/kballard/go-shellquote"
)

const defaultRemoteAdditionalBase = "dir"

func remoteLaunchCommand(spec sandbox.LaunchSpec) string {
	cwd := spec.Cwd
	if cwd == "" {
		cwd = defaultRuntimeRoot
	}
	env := make([]string, 0, len(spec.Env))
	for _, entry := range spec.Env {
		if key, _, ok := strings.Cut(entry, "="); ok && key != "" {
			env = append(env, shellquote.Join(entry))
		}
	}
	parts := []string{"cd", shellquote.Join(cwd), "&&"}
	if len(env) > 0 {
		parts = append(parts, "env")
		parts = append(parts, env...)
	}
	parts = append(parts, "sh", "-lc", shellquote.Join(spec.Command))
	return strings.Join(parts, " ")
}

func remoteTerminalCommand(root string, req acpsdk.CreateTerminalRequest) string {
	cwd := root
	if req.Cwd != nil && strings.TrimSpace(*req.Cwd) != "" {
		cwd = *req.Cwd
	}
	command := strings.TrimSpace(req.Command)
	if len(req.Args) > 0 {
		args := make([]string, 0, len(req.Args)+1)
		args = append(args, command)
		args = append(args, req.Args...)
		command = shellquote.Join(args...)
	}
	env := make([]string, 0, len(req.Env))
	for _, entry := range req.Env {
		if entry.Name == "" || isBlockedRemoteEnv(entry.Name) {
			continue
		}
		env = append(env, shellquote.Join(fmt.Sprintf("%s=%s", entry.Name, entry.Value)))
	}
	parts := []string{"cd", shellquote.Join(cwd), "&&"}
	if len(env) > 0 {
		parts = append(parts, "env")
		parts = append(parts, env...)
	}
	parts = append(parts, "sh", "-lc", shellquote.Join(command))
	return strings.Join(parts, " ")
}

func remoteExtractCommand(dest string, payloadBytes int64) string {
	return fmt.Sprintf(
		"mkdir -p %s && head -c %d | tar -xpf - -C %s",
		shellquote.Join(dest),
		payloadBytes,
		shellquote.Join(dest),
	)
}

func remoteArchiveCommand(src string) string {
	return "tar -cpf - -C " + shellquote.Join(src) + " ."
}

func remoteAdditionalDirs(runtimeRoot string, localAdditionalDirs []string) []string {
	if len(localAdditionalDirs) == 0 {
		return nil
	}
	baseRoot := path.Join(runtimeRoot, ".agh-additional")
	dirs := make([]string, 0, len(localAdditionalDirs))
	for i, localDir := range localAdditionalDirs {
		base := path.Base(strings.TrimRight(localDir, "/"))
		if base == "." || base == "/" || base == "" {
			base = defaultRemoteAdditionalBase
		}
		dirs = append(dirs, path.Join(baseRoot, fmt.Sprintf("%02d-%s", i+1, sanitizeRemoteBase(base))))
	}
	return dirs
}

func sanitizeRemoteBase(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return defaultRemoteAdditionalBase
	}
	var builder strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	cleaned := strings.Trim(builder.String(), ".-")
	if cleaned == "" {
		return defaultRemoteAdditionalBase
	}
	return cleaned
}
