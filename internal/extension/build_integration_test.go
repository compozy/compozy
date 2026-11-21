//go:build integration

package extensionpkg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestBuildBundleDescribeRoundTrip(t *testing.T) {
	withDaemonVersion(t, "0.6.0")
	fixture := unicodeSeparatorSchemaFixture(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repo root) error = %v", err)
	}

	t.Run("Should build and register a Go SDK bundle with its published schema identity", func(t *testing.T) {
		sourceDir := t.TempDir()
		writeGoRoundTripSource(t, sourceDir, repoRoot, fixture.Schema)
		testBuildBundleDescribeRoundTrip(t, sourceDir, "bin", fixture.IdentitySHA256, fixture.IdentitySHA256)
	})
	t.Run("Should build and register a TypeScript SDK bundle with separator schemas", func(t *testing.T) {
		sourceDir := t.TempDir()
		writeTypeScriptRoundTripSource(t, sourceDir, repoRoot, fixture.Schema)
		testBuildBundleDescribeRoundTrip(t, sourceDir, "index.cjs", fixture.SHA256, fixture.IdentitySHA256)
	})
}

func writeGoRoundTripSource(t *testing.T, sourceDir, repoRoot string, schema json.RawMessage) {
	t.Helper()

	writeFile(t, filepath.Join(sourceDir, "go.mod"), `module example.com/roundtrip

go 1.26.4

require github.com/compozy/compozy/sdk/go v0.0.0

replace github.com/compozy/compozy/sdk/go => `+filepath.ToSlash(filepath.Join(repoRoot, "sdk", "go"))+"\n")
	writeFile(t, filepath.Join(sourceDir, "main.go"), `package main

import (
  "context"
  "encoding/json"
  "fmt"
  "os"

  compozysdk "github.com/compozy/compozy/sdk/go"
)

type searchInput struct {
  Query string `+"`json:\"query\"`"+`
}

func main() {
  extension := compozysdk.NewExtension(compozysdk.ExtensionDefinition{
    Name: "roundtrip",
    Version: "0.1.0",
    Description: "Real build round trip",
    Subprocess: compozysdk.DescribeSubprocess{Command: "./bin"},
  })
  err := compozysdk.Tool[searchInput](extension, "search", compozysdk.ToolOptions{
    Description: "Search fixture data",
    ReadOnly: true,
    InputSchema: json.RawMessage(`+strconv.Quote(string(schema))+`),
  }, func(_ context.Context, req compozysdk.ToolRequest[searchInput]) (compozysdk.ToolResult, error) {
    return compozysdk.TextResult("found " + req.Input.Query), nil
  })
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
  if err := extension.Run(context.Background()); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}
`)
}

func writeTypeScriptRoundTripSource(t *testing.T, sourceDir, repoRoot string, schema json.RawMessage) {
	t.Helper()

	sdk := filepath.ToSlash(filepath.Join(repoRoot, "sdk", "typescript", "src", "index.ts"))
	writeFile(t, filepath.Join(sourceDir, "package.json"), `{
  "scripts": {
    "build": "bun build index.ts --target=node --format=cjs --outfile=dist/index.cjs",
    "start": "node dist/index.cjs"
  }
}`)
	writeFile(t, filepath.Join(sourceDir, "index.ts"), `
import { Extension } from `+strconv.Quote(sdk)+`;

const extension = new Extension({
  name: "roundtrip",
  version: "0.1.0",
  description: "Real build round trip",
  subprocess: { command: "node", args: ["index.cjs"] },
});
extension.tool<{ query: string }>("search", {
  description: "Search fixture data",
  readOnly: true,
  inputSchema: `+string(schema)+`,
}, async ({ input }) => ({
  content: [{ type: "text", text: "found " + input.query }],
  truncated: false,
  bytes: 0,
  duration_ms: 0,
}));
void extension.start();
`)
}

func testBuildBundleDescribeRoundTrip(t *testing.T, sourceDir, binary, runtimeDigest, identityDigest string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testutil.Context(t), 60*time.Second)
	defer cancel()
	result, err := BuildBundle(ctx, BuildRequest{SourceDir: sourceDir})
	if err != nil {
		t.Fatalf("BuildBundle() error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(result.GenerationDir, binary)); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("built binary stat = (%#v, %v), want regular file", info, err)
	}
	if checksum, err := ComputeDirectoryChecksum(result.GenerationDir); err != nil {
		t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
	} else if checksum != result.GenerationHash {
		t.Fatalf("generation checksum = %q, want %q", checksum, result.GenerationHash)
	}

	env := newRegistryTestEnv(t)
	if err := env.registry.Install(result.Manifest, result.GenerationDir, result.GenerationHash); err != nil {
		t.Fatalf("Registry.Install() error = %v", err)
	}
	installed, err := env.registry.Get("roundtrip")
	if err != nil {
		t.Fatalf("Registry.Get() error = %v", err)
	}
	installedManifest, err := LoadManifest(filepath.Dir(installed.ManifestPath))
	if err != nil {
		t.Fatalf("LoadManifest(installed) error = %v", err)
	}
	tools, err := ResolveManifestToolDescriptors(installedManifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if len(tools) != 1 || tools[0].Tool.ID.String() != "ext__roundtrip__search" {
		t.Fatalf("installed tools = %#v, want ext__roundtrip__search", tools)
	}
	if got := tools[0].Tool.InputSchemaDigest; got != identityDigest {
		t.Fatalf("installed tool digest = %q, want published identity %q", got, identityDigest)
	}

	manager := NewManager(
		env.registry,
		WithHealthCheckTimeout(20*time.Millisecond),
		WithSubprocessSignalGrace(15*time.Millisecond),
	)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Manager.Stop() cleanup error = %v", err)
		}
	})
	waitForManagerCondition(t, time.Second, func() bool {
		extension, getErr := manager.Get("roundtrip")
		return getErr == nil && extension.Status.Active && extension.Status.Healthy
	})
	descriptors, err := manager.ProvideTools(testutil.Context(t), "roundtrip")
	if err != nil {
		t.Fatalf("Manager.ProvideTools() error = %v", err)
	}
	if len(descriptors) != 1 || descriptors[0].InputSchemaDigest != runtimeDigest {
		t.Fatalf("runtime descriptors = %#v, want one with digest %q", descriptors, runtimeDigest)
	}

	toolRegistry := newExtensionToolRegistry(t, env.registry, manager, extensionToolPolicyAllowAll())
	scope := toolspkg.Scope{Operator: true, SessionID: "session-1"}
	view, err := toolRegistry.Get(testutil.Context(t), scope, tools[0].Tool.ID)
	if err != nil {
		t.Fatalf("tool registry Get() error = %v", err)
	}
	if !view.Availability.Executable {
		t.Fatalf("tool availability = %#v, want executable", view.Availability)
	}
	if got := view.Descriptor.InputSchemaDigest; got != identityDigest {
		t.Fatalf("tool registry digest = %q, want published identity %q", got, identityDigest)
	}
	callResult, err := toolRegistry.Call(
		testutil.Context(t),
		scope,
		toolspkg.CallRequest{
			ToolID: tools[0].Tool.ID,
			Input:  json.RawMessage(`{"query":"alpha"}`),
		},
	)
	if err != nil {
		t.Fatalf("tool registry Call() error = %v", err)
	}
	if len(callResult.Content) != 1 || callResult.Content[0].Text != "found alpha" {
		t.Fatalf("tool result content = %#v, want found alpha", callResult.Content)
	}
}
