//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"golang.org/x/sys/execabs"
)

const extensionAuthoringE2EName = "authoring-dev-e2e"

func TestDaemonE2EExtensionAuthoringShouldCompleteTheDevelopmentLoopWithoutTrustPrompts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath:   binaryPath,
		StartTimeout: 30 * time.Second,
	})
	identity, err := workspacepkg.EnsureIdentity(ctx, harness.WorkspaceRoot)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	sourceDir := filepath.Join(harness.WorkspaceRoot, extensionAuthoringE2EName)

	var scaffold extensionpkg.ScaffoldResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&scaffold,
		"extension", "init", extensionAuthoringE2EName,
		"--template", "tool-provider-go",
		"--dir", sourceDir,
		"-o", "json",
	)
	if scaffold.Directory != sourceDir || scaffold.Name != extensionAuthoringE2EName {
		t.Fatalf("extension init result = %#v, want source %q", scaffold, sourceDir)
	}
	configureExtensionAuthoringSDKReplace(t, ctx, sourceDir, repoRoot)
	rewriteExtensionAuthoringGeneration(t, sourceDir, "No results for ", "generation-one:", "generation-one-observed")

	var firstBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&firstBuild,
		"extension", "build", sourceDir, "-o", "json",
	)
	assertExtensionAuthoringValidation(t, ctx, harness, firstBuild.GenerationDir)

	var linked compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&linked,
		"extension", "dev", sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if !linked.Dev || linked.WorkspaceID != identity.WorkspaceID || linked.GenerationHash != firstBuild.GenerationHash {
		t.Fatalf("extension dev result = %#v, want workspace dev generation %q", linked, firstBuild.GenerationHash)
	}
	assertExtensionAuthoringInvocation(t, ctx, harness, "generation-one:alpha")
	assertExtensionAuthoringLogs(t, ctx, harness, "generation-one-observed")

	rewriteExtensionAuthoringGeneration(
		t,
		sourceDir,
		"generation-one:",
		"generation-two:",
		"generation-two-observed",
	)
	var reloaded compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&reloaded,
		"extension", "reload", extensionAuthoringE2EName, sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if reloaded.GenerationHash == firstBuild.GenerationHash || !reloaded.Dev {
		t.Fatalf("extension reload result = %#v, want a new dev generation", reloaded)
	}
	assertExtensionAuthoringInvocation(t, ctx, harness, "generation-two:alpha")
	assertExtensionAuthoringLogs(t, ctx, harness, "generation-two-observed")

	var removed compozycontract.ManagedExtensionRemovePayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&removed,
		"extension", "remove", extensionAuthoringE2EName,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if removed.Name != extensionAuthoringE2EName || removed.Status != "removed" || removed.Path != sourceDir {
		t.Fatalf("extension remove result = %#v, want removed dev link", removed)
	}
}

func runExtensionAuthoringCLI(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	destination any,
	args ...string,
) {
	t.Helper()
	if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, destination, args...); err != nil {
		t.Fatalf("CLI %q error = %v", strings.Join(args, " "), err)
	}
}

func assertExtensionAuthoringValidation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	generationDir string,
) {
	t.Helper()
	var report extensionpkg.ValidationReport
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&report,
		"extension", "validate", generationDir, "-o", "json",
	)
	if len(report.Issues) != 0 {
		t.Fatalf("extension validation issues = %#v, want none", report.Issues)
	}
}

func assertExtensionAuthoringInvocation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	toolID, err := toolspkg.CanonicalToolID("ext", extensionAuthoringE2EName, "search")
	if err != nil {
		t.Fatalf("CanonicalToolID() error = %v", err)
	}
	var response compozycontract.ToolInvokeResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		"/api/tools/"+url.PathEscape(string(toolID))+"/invoke",
		compozycontract.ToolInvokeRequest{
			WorkspaceID: harness.WorkspaceID,
			Input:       json.RawMessage(`{"query":"alpha"}`),
		},
		&response,
	); err != nil {
		t.Fatalf("invoke extension tool %q error = %v", toolID, err)
	}
	if len(response.Result.Content) != 1 || response.Result.Content[0].Text != want {
		t.Fatalf("invoke extension result = %#v, want %q", response.Result.Content, want)
	}
}

func assertExtensionAuthoringLogs(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	var logs []compozycontract.ExtensionLogPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&logs,
		"extension", "logs", extensionAuthoringE2EName,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	for _, entry := range logs {
		if strings.Contains(entry.Message, want) {
			return
		}
	}
	t.Fatalf("extension logs = %#v, want message containing %q", logs, want)
}

func rewriteExtensionAuthoringGeneration(
	t *testing.T,
	sourceDir string,
	oldResult string,
	newResult string,
	logMarker string,
) {
	t.Helper()
	mainPath := filepath.Join(sourceDir, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", mainPath, err)
	}
	content := string(data)
	content = strings.Replace(content, oldResult, newResult, 1)
	needle := "return compozysdk.TextResult("
	if !strings.Contains(content, needle) {
		t.Fatalf("scaffold source %q is missing tool result handler", mainPath)
	}
	content = strings.Replace(
		content,
		needle,
		fmt.Sprintf("fmt.Fprintln(os.Stderr, %q)\n\t\t\t%s", logMarker, needle),
		1,
	)
	if err := os.WriteFile(mainPath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", mainPath, err)
	}
}

func configureExtensionAuthoringSDKReplace(
	t *testing.T,
	ctx context.Context,
	sourceDir string,
	repoRoot string,
) {
	t.Helper()
	// #nosec G204 -- the isolated fixture intentionally points its SDK dependency at this checkout.
	command := execabs.CommandContext(
		ctx,
		"go",
		"mod",
		"edit",
		"-replace",
		"github.com/compozy/compozy/sdk/go="+filepath.Join(repoRoot, "sdk", "go"),
	)
	command.Dir = sourceDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit SDK replace error = %v; output=%s", err, strings.TrimSpace(string(output)))
	}
}

func buildStampedExtensionAuthoringBinary(t *testing.T, ctx context.Context, repoRoot string) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "compozy")
	// #nosec G204 -- the test builds the current checkout with an explicit release-compatible version stamp.
	command := execabs.CommandContext(
		ctx,
		"go",
		"build",
		"-ldflags",
		"-X github.com/compozy/compozy/internal/version.Version=v0.3.0-beta.1",
		"-o",
		binaryPath,
		"./cmd/compozy",
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build stamped compozy binary error = %v; output=%s", err, strings.TrimSpace(string(output)))
	}
	return binaryPath
}

func extensionAuthoringE2ERepoRoot(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repo root) error = %v", err)
	}
	return repoRoot
}
