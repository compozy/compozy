package runtimemcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"

	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/config"
)

func fixtureContext(t *testing.T) ReadContext {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := ReadContext{
		Home:       filepath.Join(root, "home"),
		CWD:        filepath.Join(root, "project"),
		SystemRoot: filepath.Join(root, "system"),
		Env:        map[string]string{},
	}
	for _, dir := range []string{c.Home, c.CWD, c.SystemRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	c.GitRoot = c.CWD
	c.WorktreeRoot = c.CWD
	return c
}

func putFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func putFixture(t *testing.T, c ReadContext, fixture, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, path, strings.ReplaceAll(string(data), "$PROJECT", filepath.ToSlash(c.GitRoot)))
}

func mustReadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func groupFor(t *testing.T, c ReadContext, id string) Snapshot {
	t.Helper()
	inventory, _, err := (Reader{}).List(t.Context(), Request{Context: c, Runtime: id})
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Runtimes) != 1 {
		t.Fatalf("groups=%d, want 1", len(inventory.Runtimes))
	}
	return inventory.Runtimes[0]
}

func serverFor(t *testing.T, group Snapshot, name string) Server {
	t.Helper()
	for _, s := range group.Servers {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("missing server %q in %+v", name, group)
	return Server{}
}

func TestRuntimeDecoders(t *testing.T) {
	t.Parallel()
	// Invariant: native files produce effective, secret-free runtime rows. Owner: runtimemcp; canonical suite: read_test.go.
	t.Run("Should read Claude user local approval and disabled scopes UT-040 UT-041 UT-042 UT-043", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFixture(t, c, "claude/user.json", filepath.Join(c.Home, ".claude.json"))
		group := groupFor(t, c, "claude")
		if len(group.Servers) != 3 || serverFor(t, group, "local").Scope != "local" ||
			serverFor(t, group, "remote").Scope != "user" ||
			serverFor(t, group, "remote").Transport != "http" {
			t.Fatalf("unexpected servers: %+v", group)
		}
		if serverFor(t, group, "paper").EnabledInRuntime {
			t.Fatal("disabled server enabled")
		}
		if len(group.Notes) != 1 || group.Notes[0] != "claude.ai connectors appear inside sessions" {
			t.Fatalf("notes=%v", group.Notes)
		}
		putFixture(t, c, "claude/project.json", filepath.Join(c.CWD, ".mcp.json"))
		putFixture(t, c, "claude/settings.json", filepath.Join(c.Home, ".claude", "settings.json"))
		group = groupFor(t, c, "claude")
		if serverFor(t, group, "project").Approval != "pending" {
			t.Fatal("project auto-approved")
		}
	})
	cases := []struct {
		name, runtime, fixture, path, server, transport string
		enabled                                         bool
	}{
		{
			"Should decode Codex disabled stdio UT-044",
			"codex",
			"codex/config.toml",
			".codex/config.toml",
			"computer-use",
			"stdio",
			false,
		},
		{
			"Should decode OpenCode v1 UT-046",
			"opencode",
			"opencode/v1.json",
			".config/opencode/opencode.json",
			"y",
			"http",
			false,
		},
		{
			"Should decode OpenCode v2 JSONC UT-046",
			"opencode",
			"opencode/v2.jsonc",
			".config/opencode/opencode.jsonc",
			"x",
			"stdio",
			true,
		},
		{
			"Should decode Kimi current config UT-048",
			"kimi-cli",
			"kimi-cli/mcp.json",
			".kimi-code/mcp.json",
			"kimi",
			"stdio",
			true,
		},
		{
			"Should decode Kimi legacy config UT-048",
			"kimi-cli",
			"kimi-cli/mcp.json",
			".kimi/mcp.json",
			"kimi",
			"stdio",
			true,
		},
		{
			"Should decode Qwen exclusions UT-049",
			"qwen-code",
			"qwen-code/settings.json",
			".qwen/settings.json",
			"qwen",
			"stdio",
			false,
		},
		{
			"Should decode Copilot user scope UT-050",
			"copilot",
			"copilot/mcp-config.json",
			".copilot/mcp-config.json",
			"copilot",
			"stdio",
			true,
		},
		{
			"Should decode Antigravity UT-052",
			"agy",
			"agy/mcp_config.json",
			".gemini/config/mcp_config.json",
			"agy",
			"stdio",
			true,
		},
		{"Should decode Pi disablement UT-052", "pi", "pi/mcp.json", ".pi/agent/mcp.json", "pi", "stdio", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := fixtureContext(t)
			putFixture(t, c, tc.fixture, filepath.Join(c.Home, tc.path))
			group := groupFor(t, c, tc.runtime)
			s := serverFor(t, group, tc.server)
			if group.ReadState != "ok" || s.Scope != "user" || s.Transport != tc.transport ||
				s.EnabledInRuntime != tc.enabled {
				t.Fatalf("group=%+v server=%+v", group, s)
			}
		})
	}
	t.Run("Should decode Gemini transports and enablement UT-047", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFixture(t, c, "gemini/settings.json", filepath.Join(c.Home, ".gemini", "settings.json"))
		putFixture(t, c, "gemini/enablement.json", filepath.Join(c.Home, ".gemini", "mcp-server-enablement.json"))
		group := groupFor(t, c, "gemini")
		for name, transport := range map[string]string{"http": "http", "sse": "sse", "local": "stdio"} {
			if serverFor(t, group, name).Transport != transport {
				t.Fatalf("%s transport wrong", name)
			}
		}
		if serverFor(t, group, "http").EnabledInRuntime {
			t.Fatal("enablement ignored")
		}
	})
	t.Run("Should decode Copilot workspace scope UT-050", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFixture(t, c, "claude/project.json", filepath.Join(c.CWD, ".mcp.json"))
		if serverFor(t, groupFor(t, c, "copilot"), "project").Scope != "project" {
			t.Fatal("scope wrong")
		}
	})
	t.Run("Should preserve Cursor project approvals UT-051", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		slug := cursorProjectSlug(c.WorktreeRoot)
		putFixture(t, c, "cursor/mcp.json", filepath.Join(c.Home, ".cursor", "mcp.json"))
		path, err := json.Marshal(c.WorktreeRoot)
		if err != nil {
			t.Fatal(err)
		}
		// Native Cursor 2026.09.26 preimage; independent of the inventory's approval-key helper.
		sum := sha256.Sum256([]byte(`{"path":` + string(path) + `,"server":{"command":"echo"}}`))
		approval := strings.ReplaceAll(
			string(mustReadFixture(t, "cursor/mcp-approvals.json")),
			"$HASH",
			hex.EncodeToString(sum[:8]),
		)
		putFile(t, filepath.Join(c.Home, ".cursor", "projects", slug, "mcp-approvals.json"), approval)
		group := groupFor(t, c, "cursor")
		if serverFor(t, group, "approved").Approval != "approved" ||
			serverFor(t, group, "pending").Approval != "pending" {
			t.Fatalf("approvals=%+v", group.Servers)
		}
		putFile(t, filepath.Join(c.Home, ".cursor", "mcp.json"), `{"mcpServers":{"approved":{"command":"changed"}}}`)
		if serverFor(t, groupFor(t, c, "cursor"), "approved").Approval != "pending" {
			t.Fatal("approval survived a definition change")
		}
	})
}

func TestRuntimeInventorySafety(t *testing.T) {
	t.Parallel()
	t.Run("Should keep auth unknown and never execute file commands UT-045 UT-055 UT-058", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		marker := filepath.Join(c.Home, "executed")
		for _, d := range Descriptors() {
			src := sources(c, d.Runtime)
			if len(src) == 0 {
				t.Fatal("missing sources")
			}
			var path string
			for _, candidate := range src {
				if candidate.scope == "user" {
					path = candidate.path
					break
				}
			}
			switch d.Runtime {
			case "codex":
				putFixture(t, c, "codex/config.toml", path)
			case "opencode":
				putFixture(t, c, "opencode/v1.json", path)
			default:
				putFile(
					t,
					path,
					`{"mcpServers":{"safe":{"command":"`+filepath.ToSlash(
						marker,
					)+`","env":{"TOKEN":"s3cr3t"},"headers":{"Authorization":"Bearer abc"}}}}`,
				)
			}
			group := groupFor(t, c, d.Runtime)
			if group.Installed {
				t.Fatalf("uninstalled %s marked installed", d.Runtime)
			}
			for _, s := range group.Servers {
				if s.Auth != "unknown" {
					t.Fatalf("auth=%q", s.Auth)
				}
			}
			data, err := json.Marshal(group)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"s3cr3t", "Bearer abc", "private-query"} {
				if strings.Contains(string(data), secret) {
					t.Fatalf("secret leaked: %s", secret)
				}
			}
		}
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("runtime command executed: %v", err)
		}
	})
	t.Run("Should isolate parse failures to one runtime UT-056", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFile(t, filepath.Join(c.Home, ".claude.json"), `{"mcpServers":`)
		putFixture(t, c, "codex/config.toml", filepath.Join(c.Home, ".codex", "config.toml"))
		inventory, _, err := (Reader{}).List(t.Context(), Request{Context: c})
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range inventory.Runtimes {
			if g.Runtime == "claude" {
				if g.ReadState != "unreadable" || !strings.Contains(g.ReadError, ".claude.json") {
					t.Fatalf("failure=%+v", g)
				}
			} else if g.ReadState != "ok" {
				t.Fatalf("other runtime failed: %+v", g)
			}
		}
	})
	t.Run("Should identify servers also configured in CompozyOS UT-057", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFixture(t, c, "codex/config.toml", filepath.Join(c.Home, ".codex", "config.toml"))
		inventory, _, err := (Reader{}).List(
			t.Context(),
			Request{Context: c, Runtime: "codex", CompozyNames: []string{"savee"}},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !serverFor(t, inventory.Runtimes[0], "savee").AlsoInCompozy {
			t.Fatal("missing duplicate marker")
		}
	})
	t.Run("Should bound file reads and safely follow symlinked config files", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		path := filepath.Join(c.Home, ".claude.json")
		putFile(t, path, strings.Repeat(" ", int(maxFileBytes)+1))
		if groupFor(t, c, "claude").ReadState != "unreadable" {
			t.Fatal("oversized file accepted")
		}
		target := filepath.Join(c.Home, "outside.json")
		putFile(t, target, `{"mcpServers":{"linked":{"command":"echo"}}}`)
		link := filepath.Join(c.Home, ".pi", "agent", "mcp.json")
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		group := groupFor(t, c, "pi")
		if group.ReadState != "ok" || serverFor(t, group, "linked").Name != "linked" {
			t.Fatalf("symlinked config=%+v", group)
		}
	})
}

func TestRuntimeContextAndDigests(t *testing.T) {
	t.Parallel()
	// Invariant: observed definitions remain keyed and copy distinguishes edits from removal; owner: runtime reader.
	t.Run("Should key public digests and refuse an edited copy with a distinct message", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		path := filepath.Join(c.Home, ".claude.json")
		putFile(t, path, `{"mcpServers":{"a":{"command":"echo","env":{"PIN":"1234"}}}}`)
		request := Request{Context: c, Runtime: "claude"}
		firstReader, secondReader := Reader{
			DigestKey: []byte("installation-one"),
		}, Reader{
			DigestKey: []byte("installation-two"),
		}
		first, _, err := firstReader.List(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		previous := serverFor(t, first.Runtimes[0], "a")
		second, _, err := secondReader.List(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		if previous.DefinitionDigest == second.Runtimes[0].Servers[0].DefinitionDigest {
			t.Fatal("installation key omitted")
		}
		if _, err := firstReader.Resolve(t.Context(), request, "a", &previous); err != nil {
			t.Fatal(err)
		}
		putFile(t, path, `{"mcpServers":{"a":{"command":"changed"}}}`)
		_, err = firstReader.Resolve(t.Context(), request, "a", &previous)
		if failure, ok := errors.AsType[*ImportError](
			err,
		); !ok || failure.Code != "mcp_server_not_found" ||
			!strings.Contains(failure.Message, "changed in") {
			t.Fatalf("edited copy error=%v", err)
		}
		putFile(t, path, `{"mcpServers":{}}`)
		_, err = firstReader.Resolve(t.Context(), request, "a", &previous)
		if err == nil || !strings.Contains(err.Error(), "no longer in") {
			t.Fatalf("removed copy error=%v", err)
		}
	})

	// Invariant: launch inputs are scoped to provider home and execution CWD, not only workspace. Owner: runtimemcp.
	t.Run("Should separate worktrees nested CWD and Settings inventory UT-059", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFixture(t, c, "claude/user.json", filepath.Join(c.Home, ".claude.json"))
		a := c
		a.CWD = filepath.Join(c.CWD, "worktree-a")
		a.WorktreeRoot = a.CWD
		b := a
		b.CWD = filepath.Join(c.CWD, "worktree-b")
		b.WorktreeRoot = b.CWD
		putFile(t, filepath.Join(a.CWD, ".mcp.json"), `{"mcpServers":{"a":{"command":"echo"}}}`)
		putFile(t, filepath.Join(b.CWD, ".mcp.json"), `{"mcpServers":{"b":{"command":"echo"}}}`)
		ga, gb, settings := groupFor(t, a, "claude"), groupFor(t, b, "claude"), groupFor(t, c, "claude")
		if ga.ActivationDigest == gb.ActivationDigest || ga.ActivationDigest == settings.ActivationDigest {
			t.Fatal("contexts share activation digest")
		}
		nested := a
		nested.CWD = filepath.Join(a.CWD, "nested")
		putFile(t, filepath.Join(nested.CWD, ".mcp.json"), `{"mcpServers":{"a":{"command":"different"}}}`)
		if groupFor(t, nested, "claude").ActivationDigest == ga.ActivationDigest {
			t.Fatal("nested CWD ignored")
		}
		isolated := a
		isolated.Home = filepath.Join(c.Home, "isolated")
		if groupFor(t, isolated, "claude").ActivationDigest == ga.ActivationDigest {
			t.Fatal("provider home ignored")
		}
	})
	t.Run("Should hash values states approvals and sources without hashing unrelated config", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		path := filepath.Join(c.Home, ".claude.json")
		putFile(t, path, `{"counter":1,"mcpServers":{"a":{"command":"echo","env":{"TOKEN":"one"}}}}`)
		before := groupFor(t, c, "claude").ActivationDigest
		putFile(t, path, `{"counter":2,"mcpServers":{"a":{"env":{"TOKEN":"one"},"command":"echo"}}}`)
		if groupFor(t, c, "claude").ActivationDigest != before {
			t.Fatal("non-MCP rewrite changed digest")
		}
		putFile(t, path, `{"mcpServers":{"a":{"command":"echo","env":{"TOKEN":"two"}}}}`)
		if groupFor(t, c, "claude").ActivationDigest == before {
			t.Fatal("secret change omitted from digest")
		}
	})
}

func TestRuntimeCopyability(t *testing.T) {
	t.Parallel()
	// Invariant: copying must preserve every behavior the runtime definition expresses. Owner: runtimemcp; UT-060.
	cases := []struct {
		name, definition, scope, reason string
		copyable                        bool
	}{
		{
			"Should allow literal stdio env",
			`{"command":"echo","args":["ok"],"env":{"TOKEN":"literal"}}`,
			"user",
			"",
			true,
		},
		{
			"Should allow literal HTTP headers",
			`{"type":"http","url":"https://example.test/mcp","headers":{"Authorization":"secret"}}`,
			"user",
			"",
			true,
		},
		{
			"Should reject URL queries",
			`{"type":"http","url":"https://example.test/mcp?secret=value"}`,
			"user",
			"URL query",
			false,
		},
		{"Should reject cwd", `{"command":"echo","cwd":"/tmp"}`, "user", "cwd", false},
		{
			"Should reject headers helpers",
			`{"url":"https://example.test","headersHelper":"secret command"}`,
			"user",
			"headers helper",
			false,
		},
		{
			"Should reject Codex headers helpers",
			`{"url":"https://example.test","http_headers_helper":"secret command"}`,
			"user",
			"headers helper",
			false,
		},
		{
			"Should reject env passthrough",
			`{"command":"echo","env_vars":["TOKEN"]}`,
			"user",
			"environment passthrough",
			false,
		},
		{
			"Should reject bearer passthrough",
			`{"url":"https://example.test","bearer_token_env_var":"TOKEN"}`,
			"user",
			"environment passthrough",
			false,
		},
		{"Should reject tool filters", `{"command":"echo","enabled_tools":["read"]}`, "user", "tool filters", false},
		{
			"Should reject custom OAuth clients",
			`{"url":"https://example.test","oauth":{"clientId":"custom"}}`,
			"user",
			"custom OAuth client",
			false,
		},
		{"Should reject plugin sources", `{"command":"echo"}`, "plugin", "runtime-owned", false},
		{"Should reject managed sources", `{"command":"echo"}`, "managed", "runtime-owned", false},
		{"Should reject SSE", `{"type":"sse","url":"https://example.test"}`, "user", "SSE", false},
		{"Should reject WebSocket", `{"type":"ws","url":"wss://example.test"}`, "user", "WS", false},
		{
			"Should reject environment expansion",
			`{"command":"echo","env":{"TOKEN":"${TOKEN}"}}`,
			"user",
			"environment indirection",
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var definition object
			if err := json.Unmarshal([]byte(tc.definition), &definition); err != nil {
				t.Fatal(err)
			}
			s, err := describeServer("claude", "test", tc.scope, "/config", "config", definition)
			if err != nil {
				t.Fatal(err)
			}
			if s.Copyable != tc.copyable || !strings.Contains(s.CopyBlockedReason, tc.reason) {
				t.Fatalf("copyability=%+v", s)
			}
		})
	}
}

// Invariant: native precedence preserves effective definitions, approval policy and source identity.
// Owner: runtimemcp; canonical suite: read_test.go, extending UT-040/044/046/059.
func TestRuntimePrecedence(t *testing.T) {
	t.Parallel()
	t.Run("Should deep merge Codex env while keeping operator and runtime profiles separate", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		c.Profile = "operator-profile"
		putFile(
			t,
			filepath.Join(c.Home, ".codex", "config.toml"),
			"profile = \"native\"\n[mcp_servers.a]\ncommand = \"echo\"\n[mcp_servers.a.env]\nONE = \"secret1\"\nTWO = \"secret2\"\n",
		)
		putFile(t, filepath.Join(c.Home, ".codex", "native.config.toml"), "[mcp_servers.a.env]\nTWO = \"override\"\n")
		putFile(
			t,
			filepath.Join(c.Home, ".codex", "operator-profile.config.toml"),
			"[mcp_servers.wrong]\ncommand = \"wrong\"\n",
		)
		putFile(t, filepath.Join(c.CWD, ".codex", "config.toml"), "[mcp_servers.a]\nenabled = false\n")
		group := groupFor(t, c, "codex")
		server := serverFor(t, group, "a")
		if group.ReadState != "ok" || len(group.Servers) != 1 || len(server.EnvKeys) != 2 || server.EnabledInRuntime {
			t.Fatalf("effective server=%+v group=%+v", server, group)
		}
	})
	t.Run("Should parse JSONC trailing commas after comments and reject unterminated comments", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		path := filepath.Join(c.Home, ".config", "opencode", "opencode.jsonc")
		putFile(t, path, "{\"mcp\":{\"a\":{\"type\":\"local\",\"command\":[\"echo\",/* comment */],},},}")
		if groupFor(t, c, "opencode").ReadState != "ok" {
			t.Fatal("valid commented trailing comma rejected")
		}
		putFile(t, path, `{"mcp":{}} /* unterminated`)
		if groupFor(t, c, "opencode").ReadState != "unreadable" {
			t.Fatal("malformed comment accepted")
		}
	})
	t.Run("Should let managed Claude config exclusively own inventory and switch availability", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFile(t, filepath.Join(c.Home, ".claude.json"), `{"mcpServers":{"user":{"command":"echo"}}}`)
		putFile(
			t,
			filepath.Join(c.SystemRoot, "ClaudeCode", "managed-mcp.json"),
			`{"mcpServers":{"managed":{"command":"echo"}}}`,
		)
		group := groupFor(t, c, "claude")
		server := serverFor(t, group, "managed")
		if len(group.Servers) != 1 || group.RuntimeMCP.CanDisable || server.Copyable || server.Scope != "managed" {
			t.Fatalf("managed inventory=%+v", group)
		}
	})
	t.Run("Should include enabled plugin servers without leaking their credentials", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		root := filepath.Join(c.Home, ".claude", "plugins", "cache", "market", "demo", "v1")
		encodedRoot, err := json.Marshal(root)
		if err != nil {
			t.Fatal(err)
		}
		putFile(t, filepath.Join(c.Home, ".claude", "settings.json"), `{"enabledPlugins":{"demo@market":true}}`)
		putFile(
			t,
			filepath.Join(c.Home, ".claude", "plugins", "installed_plugins.json"),
			`{"plugins":{"demo@market":[{"installPath":`+string(encodedRoot)+`}]}}`,
		)
		putFile(
			t,
			filepath.Join(root, ".mcp.json"),
			`{"mcpServers":{"tool":{"url":"https://example.test/mcp","headers":{"Authorization":"private-plugin-header"}}}}`,
		)
		group := groupFor(t, c, "claude")
		server := serverFor(t, group, "plugin:demo:tool")
		if server.Scope != "plugin" || server.Copyable || len(server.HeaderKeys) != 1 {
			t.Fatalf("plugin server=%+v", server)
		}
		encoded, err := json.Marshal(group)
		if err != nil || strings.Contains(string(encoded), "private-plugin-header") {
			t.Fatal("plugin credentials leaked")
		}
	})
	t.Run(
		"Should resolve isolated native homes for Codex and Pi derived providers without creating directories",
		func(t *testing.T) {
			t.Parallel()
			c := fixtureContext(t)
			home := config.HomePaths{HomeDir: filepath.Join(c.Home, "compozy")}
			for _, provider := range []string{"codex", "openrouter"} {
				resolved := config.ProviderConfig{HomePolicy: config.ProviderHomePolicyIsolated}
				if provider == "openrouter" {
					resolved.Harness = config.ProviderHarnessPiACP
				}
				context, err := ProviderContext(
					home,
					provider,
					resolved,
					[]string{"HOME=" + c.Home},
					c.CWD,
					c.WorktreeRoot,
				)
				if err != nil {
					t.Fatal(err)
				}
				if context.Home == c.Home || !strings.HasPrefix(context.Home, home.HomeDir) {
					t.Fatalf("unisolated provider context=%+v", context)
				}
				if provider == "openrouter" &&
					context.Env["PI_CODING_AGENT_DIR"] != filepath.Join(context.Home, ".pi", "agent") {
					t.Fatal("Pi provider uses operator MCP home")
				}
				if _, err := os.Stat(context.Home); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("read-only context created home: %v", err)
				}
			}
		},
	)
}

func TestRuntimeCodexPluginSelection(t *testing.T) {
	t.Parallel()
	// Invariant: only the active installed version contributes plugin servers. Owner: runtimemcp; read_test.go.
	t.Run("Should choose native semver precedence and prefer a local installation", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFile(t, filepath.Join(c.Home, ".codex", "config.toml"), "[plugins.\"demo@market\"]\nenabled = true\n")
		root := filepath.Join(c.Home, ".codex", "plugins", "cache", "market", "demo")
		putFile(t, filepath.Join(root, "1.9.0", ".mcp.json"), `{"mcpServers":{"old":{"command":"echo"}}}`)
		putFile(t, filepath.Join(root, "1.10.0", ".mcp.json"), `{"mcpServers":{"current":{"command":"echo"}}}`)
		group := groupFor(t, c, "codex")
		if len(group.Servers) != 1 || serverFor(t, group, "current").Scope != "plugin" {
			t.Fatalf("inactive version loaded: %+v", group)
		}
		putFile(
			t,
			filepath.Join(root, "local", ".codex-plugin", "plugin.json"),
			`{"mcpServers":["extra.json",{"inline":{"command":"echo"}}]}`,
		)
		putFile(t, filepath.Join(root, "local", "extra.json"), `{"mcpServers":{"local":{"command":"echo"}}}`)
		group = groupFor(t, c, "codex")
		if len(group.Servers) != 2 || serverFor(t, group, "local").Copyable ||
			serverFor(t, group, "inline").Scope != "plugin" {
			t.Fatalf("local plugin precedence or manifest array wrong: %+v", group)
		}
	})
}

// Invariant: public targets contain neither command arguments nor URL credentials or paths. Owner: runtime inventory; canonical decoder suite.
func TestRuntimeMCPTarget(t *testing.T) {
	t.Parallel()
	t.Run("Should expose only executable basename and remote hostname", func(t *testing.T) {
		t.Parallel()
		c := fixtureContext(t)
		putFile(
			t,
			filepath.Join(c.Home, ".claude.json"),
			`{"mcpServers":{"local":{"command":"/private/bin/runner --token hidden","args":["hidden-arg"]},"remote":{"type":"http","url":"https://user:password@example.test:8443/private?token=hidden#fragment"}}}`,
		)
		inventory, _, err := (Reader{}).List(t.Context(), Request{Context: c, Runtime: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		targets := map[string]string{}
		for _, server := range inventory.Runtimes[0].Servers {
			targets[server.Name] = server.Target
		}
		if targets["local"] != "runner" || targets["remote"] != "example.test" {
			t.Fatalf("targets=%+v", targets)
		}
		data, err := json.Marshal(inventory)
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{"password", "hidden", "hidden-arg", "/private/bin", "8443", "fragment"} {
			if strings.Contains(string(data), private) {
				t.Fatalf("inventory leaked %q", private)
			}
		}
	})
}
