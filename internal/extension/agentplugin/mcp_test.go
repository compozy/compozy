package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadMCPGoldenTranslation(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize stdio and Streamable HTTP servers", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "golden-mcp")
		dataDir := canonicalPathForTest(t, filepath.Join(t.TempDir(), "plugin-data"))
		writeMCPFile(t, root, map[string]any{
			"validator": map[string]any{
				"type": "stdio", "command": "./bin/validator",
				"args": []string{"--data", pluginDataPlaceholder + "/validator"},
				"env":  map[string]string{"CONFIG": pluginRootPlaceholder + "/config.json"},
				"cwd":  pluginRootPlaceholder,
			},
			"remote": map[string]any{
				"type": "streamable-http", "url": "https://deploy.example.com/mcp",
				"headers": map[string]string{"X-Tenant": "public"},
			},
		})
		pkg, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got, want := serverNames(pkg.Servers), []string{"remote", "validator"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() server names = %#v, want %#v", got, want)
		}
		validator := pkg.Servers[1]
		if got, want := validator.Command, filepath.Join(root, "bin", "validator"); got != want {
			t.Fatalf("Load() command = %q, want %q", got, want)
		}
		if got, want := validator.CWD, root; got != want {
			t.Fatalf("Load() cwd = %q, want %q", got, want)
		}
		if got, want := validator.Args, []string{
			"--data",
			filepath.Join(dataDir, "validator"),
		}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("Load() args = %#v, want %#v", got, want)
		}
		if validator.Env[pluginRootVariable] != root || validator.Env[pluginDataVariable] != dataDir {
			t.Fatalf("Load() env = %#v, want injected roots", validator.Env)
		}
		if got, want := validator.Env["CONFIG"], filepath.Join(root, "config.json"); got != want {
			t.Fatalf("Load() CONFIG = %q, want %q", got, want)
		}
	})
}

func TestLoadMCPWholeFileFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  any
	}{
		{
			name: "Should disable MCP for an unsupported schema",
			doc: map[string]any{
				"$schema":    "https://agent-plugins.org/schemas/2.0.0/mcp.schema.json",
				"mcpServers": map[string]any{},
			},
		},
		{name: "Should disable MCP for a non-object root", doc: []any{"not", "an", "object"}},
		{name: "Should disable MCP when mcpServers is missing", doc: map[string]any{"$schema": MCPSchemaID}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := newPackageRoot(t, "whole-file")
			writeFile(t, filepath.Join(root, "skills", "review", "SKILL.md"), validSkill("review", "Review code."))
			writeJSONFile(t, filepath.Join(root, "mcp.json"), test.doc)
			pkg := loadForTest(t, root)
			if len(pkg.Servers) != 0 || len(pkg.Skills) != 1 {
				t.Fatalf(
					"Load() = servers %#v, skills %#v; want MCP disabled and skill retained",
					pkg.Servers,
					pkg.Skills,
				)
			}
			if len(pkg.Diagnostics) != 1 || pkg.Diagnostics[0].Scope != "mcp" {
				t.Fatalf("Load() diagnostics = %#v, want one mcp diagnostic", pkg.Diagnostics)
			}
		})
	}
}

func TestLoadMCPSSEDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should distinguish supported-shape SSE from malformed SSE", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "sse-cases")
		writeMCPFile(t, root, map[string]any{
			"legacy-events": map[string]any{"type": "sse", "url": "https://example.com/events"},
			"broken-events": map[string]any{"type": "sse"},
		})
		pkg := loadForTest(t, root)
		if !hasDiagnostic(pkg.Diagnostics, "mcp:legacy-events", "sse transport is not supported") {
			t.Fatalf("Load() diagnostics = %#v, want unsupported SSE", pkg.Diagnostics)
		}
		if !hasDiagnostic(pkg.Diagnostics, "mcp:broken-events", "invalid mcp server entry") {
			t.Fatalf("Load() diagnostics = %#v, want malformed SSE", pkg.Diagnostics)
		}
	})
}

func TestLoadMCPPerServerIsolation(t *testing.T) {
	t.Parallel()

	t.Run("Should retain one valid server beside eight invalid entries", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "isolated-mcp")
		writeMCPFile(t, root, map[string]any{
			"unknown-type":    map[string]any{"type": "websocket", "url": "https://example.com"},
			"missing-command": map[string]any{"type": "stdio"},
			"bad-cwd":         map[string]any{"type": "stdio", "command": "tool", "cwd": "./../escape"},
			"reserved-env": map[string]any{
				"type":    "stdio",
				"command": "tool",
				"env":     map[string]string{pluginRootVariable: "bad"},
			},
			"bad-header": map[string]any{
				"type":    "streamable-http",
				"url":     "https://example.com",
				"headers": map[string]string{"Bad Header": "bad"},
			},
			"bad-url":            map[string]any{"type": "streamable-http", "url": "http://example.com"},
			"whitespace-command": map[string]any{"type": "stdio", "command": "my tool"},
			"extra-field":        map[string]any{"type": "stdio", "command": "tool", "extra": true},
			"valid":              map[string]any{"type": "streamable-http", "url": "https://example.com/mcp"},
		})
		pkg := loadForTest(t, root)
		if got, want := serverNames(pkg.Servers), []string{"valid"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() server names = %#v, want %#v", got, want)
		}
		if len(pkg.Diagnostics) != 8 {
			t.Fatalf("Load() diagnostics count = %d, want 8: %#v", len(pkg.Diagnostics), pkg.Diagnostics)
		}
	})
}

func TestLoadMCPRejectsNullCollectionEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		server any
	}{
		{
			name:   "Should reject a null server entry",
			server: nil,
		},
		{
			name:   "Should reject a null stdio argument",
			server: map[string]any{"type": "stdio", "command": "tool", "args": []any{"ok", nil}},
		},
		{
			name:   "Should reject a null stdio environment value",
			server: map[string]any{"type": "stdio", "command": "tool", "env": map[string]any{"TOKEN": nil}},
		},
		{
			name: "Should reject a null remote header value",
			server: map[string]any{
				"type": "streamable-http", "url": "https://example.com/mcp",
				"headers": map[string]any{"X-Tenant": nil},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := newPackageRoot(t, "null-collection")
			writeMCPFile(t, root, map[string]any{"subject": test.server})
			pkg := loadForTest(t, root)
			if len(pkg.Servers) != 0 ||
				!hasDiagnostic(pkg.Diagnostics, "mcp:subject", "invalid mcp server entry") {
				t.Fatalf(
					"Load() = servers %#v diagnostics %#v, want isolated null-entry rejection",
					pkg.Servers,
					pkg.Diagnostics,
				)
			}
		})
	}
}

func TestLoadMCPCommandPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		valid   bool
	}{
		{name: "Should reject whitespace", command: "my tool"},
		{name: "Should reject an unrooted slash", command: "bin/tool"},
		{name: "Should reject a drive-shaped token", command: "C:tool.exe"},
		{name: "Should reject an empty command", command: ""},
		{name: "Should reject a placeholder command", command: pluginRootPlaceholder},
		{name: "Should accept a bare executable", command: "validator", valid: true},
		{name: "Should accept a contained relative executable", command: "./bin/validator", valid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := newPackageRoot(t, "command-policy")
			writeMCPFile(t, root, map[string]any{"subject": map[string]any{"type": "stdio", "command": test.command}})
			pkg := loadForTest(t, root)
			if test.valid && (len(pkg.Servers) != 1 || len(pkg.Diagnostics) != 0) {
				t.Fatalf(
					"Load() = servers %#v diagnostics %#v, want accepted command without diagnostics",
					pkg.Servers,
					pkg.Diagnostics,
				)
			}
			if !test.valid && (len(pkg.Servers) != 0 || !hasDiagnostic(pkg.Diagnostics, "mcp:subject", "command")) {
				t.Fatalf(
					"Load() = servers %#v diagnostics %#v, want isolated command rejection",
					pkg.Servers,
					pkg.Diagnostics,
				)
			}
		})
	}
}

func TestLoadMCPCWDPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize omitted package and data-root working directories", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "cwd-normalization")
		dataDir := canonicalPathForTest(t, filepath.Join(t.TempDir(), "data"))
		writeMCPFile(t, root, map[string]any{
			"default":  map[string]any{"type": "stdio", "command": "tool"},
			"relative": map[string]any{"type": "stdio", "command": "tool", "cwd": "./sub"},
			"data":     map[string]any{"type": "stdio", "command": "tool", "cwd": pluginDataPlaceholder},
			"state":    map[string]any{"type": "stdio", "command": "tool", "cwd": pluginDataPlaceholder + "/state"},
		})
		pkg, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		cwdByName := make(map[string]string, len(pkg.Servers))
		for _, server := range pkg.Servers {
			cwdByName[server.Name] = server.CWD
		}
		want := map[string]string{
			"default":  root,
			"relative": filepath.Join(root, "sub"),
			"data":     dataDir,
			"state":    filepath.Join(dataDir, "state"),
		}
		if !reflect.DeepEqual(cwdByName, want) {
			t.Fatalf("Load() cwd map = %#v, want %#v", cwdByName, want)
		}
	})

	t.Run("Should reject lexical and mixed-root escapes", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "cwd-escapes")
		writeMCPFile(t, root, map[string]any{
			"parent": map[string]any{"type": "stdio", "command": "tool", "cwd": "./../escape"},
			"mixed": map[string]any{
				"type":    "stdio",
				"command": "tool",
				"cwd":     "./" + pluginDataPlaceholder + "/state",
			},
		})
		pkg := loadForTest(t, root)
		if len(pkg.Servers) != 0 || len(pkg.Diagnostics) != 2 {
			t.Fatalf("Load() = servers %#v diagnostics %#v, want two isolated skips", pkg.Servers, pkg.Diagnostics)
		}
	})

	t.Run("Should reject symlink escapes from both containment domains", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "cwd-symlinks")
		dataDir := filepath.Join(t.TempDir(), "data")
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", dataDir, err)
		}
		dataDir = canonicalPathForTest(t, dataDir)
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "root-link")); err != nil {
			t.Skipf("os.Symlink(package) unavailable: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(dataDir, "data-link")); err != nil {
			t.Skipf("os.Symlink(data) unavailable: %v", err)
		}
		writeMCPFile(t, root, map[string]any{
			"root-escape": map[string]any{"type": "stdio", "command": "tool", "cwd": "./root-link"},
			"data-escape": map[string]any{
				"type":    "stdio",
				"command": "tool",
				"cwd":     pluginDataPlaceholder + "/data-link",
			},
		})
		pkg, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if len(pkg.Servers) != 0 || len(pkg.Diagnostics) != 2 {
			t.Fatalf("Load() = servers %#v diagnostics %#v, want two symlink skips", pkg.Servers, pkg.Diagnostics)
		}
	})
}

func TestLoadMCPEnvironmentAndExpansion(t *testing.T) {
	t.Parallel()

	t.Run("Should reject both reserved environment variables", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "reserved-env")
		writeMCPFile(t, root, map[string]any{
			"root": map[string]any{
				"type":    "stdio",
				"command": "tool",
				"env":     map[string]string{pluginRootVariable: "bad"},
			},
			"data": map[string]any{
				"type":    "stdio",
				"command": "tool",
				"env":     map[string]string{pluginDataVariable: "bad"},
			},
		})
		pkg := loadForTest(t, root)
		if len(pkg.Servers) != 0 || len(pkg.Diagnostics) != 2 {
			t.Fatalf("Load() = servers %#v diagnostics %#v, want two reserved-env skips", pkg.Servers, pkg.Diagnostics)
		}
	})

	t.Run("Should expand once and preserve unknown placeholders", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		root := filepath.Join(parent, "literal-"+pluginDataPlaceholder)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
		}
		root = canonicalPathForTest(t, root)
		writeJSONFile(
			t,
			filepath.Join(root, "plugin.json"),
			map[string]any{"$schema": PluginSchemaID, "name": "single-pass"},
		)
		dataDir := filepath.Join(t.TempDir(), "real-data")
		writeMCPFile(t, root, map[string]any{
			"subject": map[string]any{
				"type": "stdio", "command": "tool",
				"args": []string{pluginRootPlaceholder + "/cfg", "${UNKNOWN}/value"},
				"env":  map[string]string{"ROOT_CONFIG": pluginRootPlaceholder + "/env"},
			},
		})
		pkg, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		server := pkg.Servers[0]
		if !strings.Contains(server.Args[0], pluginDataPlaceholder) || strings.Contains(server.Args[0], dataDir) {
			t.Fatalf("Load() first arg = %q, want single-pass literal token", server.Args[0])
		}
		if got, want := server.Args[1], "${UNKNOWN}/value"; got != want {
			t.Fatalf("Load() unknown placeholder = %q, want %q", got, want)
		}
	})
}

func TestLoadMCPHeadersAndURLs(t *testing.T) {
	t.Parallel()

	headerCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name:    "Should reject case-insensitive duplicate headers",
			headers: map[string]string{"X-Tenant": "a", "x-tenant": "b"},
		},
		{name: "Should reject a non-token header", headers: map[string]string{"Bad Header": "value"}},
		{name: "Should reject a header line break", headers: map[string]string{"X-Test": "a\r\nb"}},
		{name: "Should reject package credentials", headers: map[string]string{"Authorization": "Bearer embedded"}},
	}
	for _, test := range headerCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := newPackageRoot(t, "header-policy")
			writeMCPFile(
				t,
				root,
				map[string]any{
					"subject": map[string]any{
						"type":    "streamable-http",
						"url":     "https://example.com",
						"headers": test.headers,
					},
				},
			)
			pkg := loadForTest(t, root)
			if len(pkg.Servers) != 0 || len(pkg.Diagnostics) != 1 {
				t.Fatalf("Load() = servers %#v diagnostics %#v, want header rejection", pkg.Servers, pkg.Diagnostics)
			}
		})
	}

	urlCases := []struct {
		name  string
		url   string
		valid bool
	}{
		{name: "Should reject non-loopback HTTP", url: "http://api.example.com"},
		{name: "Should reject URL user information", url: "https://user:pw@example.com"},
		{name: "Should reject URL fragments", url: "https://example.com/mcp#frag"},
		{name: "Should accept localhost HTTP", url: "http://localhost:3000/mcp", valid: true},
		{name: "Should accept loopback IP HTTP", url: "http://127.0.0.1/mcp", valid: true},
	}
	for _, test := range urlCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := newPackageRoot(t, "url-policy")
			writeMCPFile(t, root, map[string]any{"subject": map[string]any{"type": "streamable-http", "url": test.url}})
			pkg := loadForTest(t, root)
			if test.valid && len(pkg.Servers) != 1 {
				t.Fatalf("Load() servers = %#v, want URL accepted", pkg.Servers)
			}
			if !test.valid && len(pkg.Diagnostics) != 1 {
				t.Fatalf("Load() diagnostics = %#v, want URL rejected", pkg.Diagnostics)
			}
		})
	}
}

func TestLoadMCPDuplicateKeysAndVersion(t *testing.T) {
	t.Parallel()

	t.Run("Should use last-wins duplicate keys and accept mcpServers as a server name", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "duplicate-keys")
		document := fmt.Sprintf(
			`{"$schema":%q,"mcpServers":{"duplicate":{"type":"stdio"},"duplicate":{"type":"streamable-http","url":"https://example.com"},"mcpServers":{"type":"streamable-http","url":"https://example.org"}}}`,
			MCPSchemaID,
		)
		writeFile(t, filepath.Join(root, "mcp.json"), []byte(document))
		dataDir := filepath.Join(t.TempDir(), "data")
		first, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("first Load() error = %v", err)
		}
		second, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("second Load() error = %v", err)
		}
		if got, want := serverNames(first.Servers), []string{"duplicate", "mcpServers"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() server names = %#v, want %#v", got, want)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("Load() results differ:\nfirst=%#v\nsecond=%#v", first, second)
		}
	})

	t.Run("Should disable only MCP when document versions differ", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "version-mismatch")
		writeFile(t, filepath.Join(root, "skills", "review", "SKILL.md"), validSkill("review", "Review code."))
		writeJSONFile(t, filepath.Join(root, "mcp.json"), map[string]any{
			"$schema": "https://agent-plugins.org/schemas/2.0.0/mcp.schema.json",
			"mcpServers": map[string]any{
				"remote": map[string]any{"type": "streamable-http", "url": "https://example.com"},
			},
		})
		pkg := loadForTest(t, root)
		if len(pkg.Servers) != 0 || len(pkg.Skills) != 1 {
			t.Fatalf("Load() = servers %#v skills %#v, want MCP disabled and skills retained", pkg.Servers, pkg.Skills)
		}
		if !hasDiagnostic(pkg.Diagnostics, "mcp", "schema version must match") {
			t.Fatalf("Load() diagnostics = %#v, want version mismatch", pkg.Diagnostics)
		}
	})
}

func serverNames(servers []ServerSpec) []string {
	names := make([]string, len(servers))
	for index, server := range servers {
		names[index] = server.Name
	}
	return names
}
