package runtimemcp

import (
	"cmp"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/pelletier/go-toml/v2"
)

const maxFileBytes int64 = 8 << 20

type source struct{ path, scope string }
type reader struct {
	context ReadContext
	paths   map[string]struct{}
	files   []string
	failure string
}

func (r *reader) load(path string) (any, bool) {
	path = filepath.Clean(path)
	r.paths[path] = struct{}{}
	data, _, err := fileutil.ReadRegularFile(path, maxFileBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	r.files = append(r.files, path)
	if err != nil {
		r.failure = "Couldn't read " + r.label(path) + ": file unavailable or exceeds 8 MiB"
		return nil, false
	}
	var value any
	if strings.HasSuffix(path, ".toml") {
		var table object
		err = toml.Unmarshal(data, &table)
		value = table
	} else {
		data = stripJSONComments(data)
		err = json.Unmarshal(data, &value)
		if err != nil { // Runtime files may be replaced while they are being read.
			if retry, _, readErr := fileutil.ReadRegularFile(path, maxFileBytes); readErr == nil {
				err = json.Unmarshal(stripJSONComments(retry), &value)
			}
		}
	}
	if err != nil {
		r.failure = "Couldn't read " + r.label(path) + ": invalid configuration"
		return nil, false
	}
	return value, true
}

func (r *reader) label(path string) string {
	rel, err := filepath.Rel(r.context.Home, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join("~", rel)
	}
	return path
}

func (c ReadContext) homeDir(key, fallback string) string {
	if value := c.Env[key]; value != "" {
		return value
	}
	if filepath.IsAbs(fallback) {
		return fallback
	}
	return filepath.Join(c.Home, fallback)
}

func ancestorDirs(cwd, stop string) []string {
	if cwd == "" {
		return nil
	}
	result := []string{}
	for dir := filepath.Clean(cwd); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		result = append(result, dir)
		if dir == stop {
			break
		}
	}
	slices.Reverse(result)
	return result
}

func (c ReadContext) systemDir(name string) string {
	if c.SystemRoot != "" {
		return filepath.Join(c.SystemRoot, name)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(string(filepath.Separator), "Library", "Application Support", name)
	case "windows":
		if name == runtimeCodex {
			return "C:/ProgramData/OpenAI/Codex"
		}
		return filepath.Join("C:"+string(filepath.Separator), "Program Files", name)
	default:
		switch name {
		case "ClaudeCode":
			return "/etc/claude-code"
		case "GeminiCli":
			return "/etc/gemini-cli"
		}
		return filepath.Join(string(filepath.Separator), "etc", strings.ToLower(name))
	}
}

func sources(c ReadContext, id string) []source {
	home, cwd := c.Home, c.CWD
	user := func(path string) source { return source{path, scopeUser} }
	project := func(path string) source { return source{path, scopeProject} }
	var list []source
	switch id {
	case runtimeClaude:
		return claudeSources(c)
	case runtimeCodex:
		return codexSources(c)
	case runtimeOpenCode:
		return opencodeSources(c)
	case runtimeGemini, runtimeQwenCode:
		name := runtimeGemini
		if id == runtimeQwenCode {
			name = runtimeQwen
		}
		if name == runtimeGemini {
			list = append(list, source{filepath.Join(c.systemDir("GeminiCli"), "system-defaults.json"), scopeManaged})
		}
		list = append(list, user(filepath.Join(home, "."+name, "settings.json")))
		if cwd != "" {
			list = append(list, project(filepath.Join(cwd, "."+name, "settings.json")))
			if name == runtimeQwen {
				list = append(list, project(filepath.Join(cwd, ".mcp.json")))
			}
		}
		if name == runtimeGemini {
			system := cmp.Or(
				c.Env["GEMINI_CLI_SYSTEM_SETTINGS_PATH"],
				filepath.Join(c.systemDir("GeminiCli"), "settings.json"),
			)
			list = append(list, source{system, scopeManaged})
		}
	case "kimi-cli":
		list = append(
			list,
			user(filepath.Join(home, ".kimi", "mcp.json")),
			user(filepath.Join(c.homeDir("KIMI_CODE_HOME", ".kimi-code"), "mcp.json")),
		)
		if cwd != "" {
			list = append(
				list,
				project(filepath.Join(cmp.Or(c.WorktreeRoot, c.GitRoot, cwd), ".mcp.json")),
				source{filepath.Join(cwd, ".kimi-code", "mcp.json"), scopeLocal},
			)
		}
	case runtimeCopilot:
		list = append(list, user(filepath.Join(c.homeDir("COPILOT_HOME", ".copilot"), "mcp-config.json")))
		for _, dir := range ancestorDirs(cwd, cmp.Or(c.WorktreeRoot, c.GitRoot)) {
			for _, name := range []string{".mcp.json", ".github/mcp.json"} {
				list = append(list, project(filepath.Join(dir, name)))
			}
		}
	case runtimeCursor:
		list = append(list, user(filepath.Join(home, ".cursor", "mcp.json")))
		if cwd != "" {
			list = append(list, project(filepath.Join(cmp.Or(c.WorktreeRoot, c.GitRoot, cwd), ".cursor", "mcp.json")))
		}
	case runtimeAntigravity:
		list = append(list, user(filepath.Join(home, ".gemini", "config", "mcp_config.json")))
	case "pi":
		list = append(list, user(filepath.Join(c.homeDir("PI_CODING_AGENT_DIR", ".pi/agent"), "mcp.json")))
		if cwd != "" {
			list = append(list, project(filepath.Join(cwd, ".pi", "mcp.json")))
		}
	}
	return list
}
