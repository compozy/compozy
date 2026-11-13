package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/compozy/internal/store/globaldb"
	globalseed "github.com/compozy/compozy/internal/testutil/storeseed/global"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var sessionTestStoreSeed *globalseed.Seed

func TestMain(m *testing.M) {
	os.Exit(runSessionTests(m))
}

func runSessionTests(m *testing.M) (code int) {
	if os.Getenv(hostedToolsHelperEnv) == "1" {
		return serveHostedToolsHelper()
	}
	if isSessionTestHelperProcess() {
		return m.Run()
	}
	seed, err := globalseed.New(context.Background())
	if err != nil {
		reportSessionTestMainError("create store seed: %v", err)
		return 1
	}
	defer func() {
		if err := seed.Close(); err != nil {
			reportSessionTestMainError("close store seed: %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	sessionTestStoreSeed = seed
	return m.Run()
}

func isSessionTestHelperProcess() bool {
	return os.Getenv("COMPOZY_TEST_SESSION_STOP_HELPER") == "1" ||
		os.Getenv("COMPOZY_TEST_SESSION_STOP_WRAPPER") == "1"
}

// hostedToolsHelperEnv re-executes the test binary as a stdio MCP server standing in for the daemon's hosted tools.
const hostedToolsHelperEnv = "COMPOZY_TEST_SESSION_HOSTED_TOOLS"

// hostedToolsHelperTool is the one native tool the helper serves.
const hostedToolsHelperTool = "compozy__session_list"

func serveHostedToolsHelper() int {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "session-hosted-tools", Version: "1"}, nil)
	server.AddTool(
		&sdkmcp.Tool{Name: hostedToolsHelperTool, InputSchema: json.RawMessage(`{"type":"object"}`)},
		func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: `{"sessions":[]}`}}}, nil
		},
	)
	if err := server.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
		reportSessionTestMainError("serve hosted tools: %v", err)
		return 1
	}
	return 0
}

func openSessionTestGlobalDB(ctx context.Context, path string) (*globaldb.GlobalDB, error) {
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat session test global database %q: %w", path, err)
		}
		if err := sessionTestStoreSeed.Clone(path); err != nil {
			return nil, fmt.Errorf("clone session test store seed to %q: %w", path, err)
		}
	}
	return globaldb.OpenGlobalDB(ctx, path)
}

func reportSessionTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "session tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
