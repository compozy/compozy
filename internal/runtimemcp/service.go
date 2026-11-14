package runtimemcp

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
)

var ErrInvalidRuntime = errors.New("invalid_runtime")

type Reader struct{}

func (Reader) List(ctx context.Context, req Request) (Inventory, []string, error) {
	req.Context = ResolveContext(req.Context)
	descriptors := Descriptors()
	if req.Runtime != "" &&
		!slices.ContainsFunc(descriptors, func(d Descriptor) bool { return d.Runtime == req.Runtime }) {
		return Inventory{}, nil, ErrInvalidRuntime
	}
	result := Inventory{Runtimes: []Snapshot{}, GeneratedAt: time.Now().UTC()}
	paths := []string{}
	for _, d := range descriptors {
		if err := ctx.Err(); err != nil {
			return Inventory{}, nil, err
		}
		if req.Runtime != "" && req.Runtime != d.Runtime {
			continue
		}
		snapshot, files := read(req.Context, d)
		paths = append(paths, files...)
		if mode := req.Modes[d.Runtime]; mode != "" {
			snapshot.RuntimeMCP.Mode = mode
		}
		for i := range snapshot.Servers {
			snapshot.Servers[i].AlsoInCompozy = slices.Contains(req.CompozyNames, snapshot.Servers[i].Name)
		}
		if len(snapshot.ConfigFiles) > 0 || len(snapshot.Servers) > 0 || snapshot.ReadState != "ok" {
			result.Runtimes = append(result.Runtimes, snapshot)
		}
	}
	slices.Sort(paths)
	return result, slices.Compact(paths), nil
}

func Environment(environ []string) map[string]string {
	result := map[string]string{}
	allowed := []string{
		"PATH",
		"CLAUDE_CONFIG_DIR",
		"CODEX_HOME",
		"XDG_CONFIG_HOME",
		"OPENCODE_CONFIG_DIR",
		"KIMI_CODE_HOME",
		"COPILOT_HOME",
		"PI_CODING_AGENT_DIR",
		"GEMINI_CLI_SYSTEM_SETTINGS_PATH",
		"CURSOR_DATA_DIR",
	}
	for _, value := range environ {
		key, text, ok := strings.Cut(value, "=")
		if ok && slices.Contains(allowed, key) {
			result[key] = text
		}
	}
	return result
}
