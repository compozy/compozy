//go:build integration && !windows

package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/windowmanager"
	"golang.org/x/sys/execabs"
)

const viewProgramFixtureExtensionName = "notes-ts"

func TestDaemonE2EExtensionViewProgramFixture(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate programmable view sessions across clients and extension restarts", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		repoRoot := extensionAuthoringE2ERepoRoot(t)
		binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			BinaryPath: binaryPath,
			Env:        map[string]string{"PATH": viewProgramNodePath(t)},
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Extensions.Trust.AllowUnverified = true
			}},
			StartTimeout: 30 * time.Second,
		})

		fixtureDir := filepath.Join(repoRoot, "internal", "extension", "testdata", "view-program-ts")
		var build extensionpkg.BuildResult
		runExtensionAuthoringCLI(t, ctx, harness, &build, "extension", "build", fixtureDir, "-o", "json")
		var installed compozycontract.ExtensionPayload
		runExtensionAuthoringCLI(
			t, ctx, harness, &installed,
			"extension", "install", build.GenerationDir, "--allow-unverified", "--yes", "-o", "json",
		)
		var enabled compozycontract.ExtensionEnablementPayload
		runExtensionAuthoringCLI(
			t, ctx, harness, &enabled,
			"extension", "enable", viewProgramFixtureExtensionName, "-o", "json",
		)
		if !enabled.Enabled || enabled.Profile != "default" {
			t.Fatalf("enabled view fixture = %#v, want active extension", enabled)
		}

		clientA := registerViewProgramClient(t, ctx, harness, "view-client-a")
		clientB := registerViewProgramClient(t, ctx, harness, "view-client-b")
		openedA := openViewProgram(t, ctx, harness, clientA.AttachmentToken, nil)
		openedB := openViewProgram(t, ctx, harness, clientB.AttachmentToken, nil)
		if openedA.ViewSession == openedB.ViewSession || openedA.StreamToken == openedB.StreamToken {
			t.Fatalf("two client sessions share identity: A=%#v B=%#v", openedA, openedB)
		}
		assertViewProgramForeignOwnership(t, ctx, harness, openedA, clientB)

		streamResponse, streamReader := openViewProgramStream(t, ctx, harness, openedA)
		defer closeViewProgramBody(t, streamResponse.Body)
		replayed := readViewProgramFrame(t, streamReader)
		searchHandler := viewProgramSearchHandler(t, replayed)
		postViewProgramEvent(
			t,
			ctx,
			harness,
			openedA,
			clientA.AttachmentToken,
			compozycontract.CmdPaletteViewSessionEventRequest{
				Handler: searchHandler, Args: []any{"standup", 1}, Revision: replayed.Revision, Seq: 1,
			},
		)
		searched := readViewProgramFrame(t, streamReader)
		if searched.Revision == replayed.Revision {
			t.Fatalf("search frame revision = %q, want advancement from %q", searched.Revision, replayed.Revision)
		}

		actionHandler := viewProgramActionHandler(t, searched, replayed)
		postViewProgramEvent(
			t,
			ctx,
			harness,
			openedA,
			clientA.AttachmentToken,
			compozycontract.CmdPaletteViewSessionEventRequest{
				Handler: actionHandler, Revision: searched.Revision, Seq: 2,
			},
		)
		effectFrame := readViewProgramFrame(t, streamReader)
		if len(effectFrame.Effects) != 1 || effectFrame.Effects[0].ID == "" {
			t.Fatalf("action frame effects = %#v, want one stable effect", effectFrame.Effects)
		}
		postViewProgramEvent(
			t,
			ctx,
			harness,
			openedA,
			clientA.AttachmentToken,
			compozycontract.CmdPaletteViewSessionEventRequest{
				Handler: searchHandler, Args: []any{"standup", 2}, Revision: effectFrame.Revision, Seq: 3,
				AckEffects: []string{effectFrame.Effects[0].ID},
			},
		)
		replayResponse, replayReader := openViewProgramStream(t, ctx, harness, openedA)
		defer closeViewProgramBody(t, replayResponse.Body)
		ackReplay := readViewProgramFrame(t, replayReader)
		if len(ackReplay.Effects) != 0 {
			t.Fatalf("reconnected replay effects = %#v, want at-most-once fence", ackReplay.Effects)
		}

		assertViewProgramSlowSessionIsolation(t, ctx, harness, clientA)
		closeViewProgram(t, ctx, harness, openedA.ViewSession, clientA.AttachmentToken)
		assertViewProgramSessionGone(t, ctx, harness, openedA)

		var disabled compozycontract.ExtensionEnablementPayload
		runExtensionAuthoringCLI(
			t, ctx, harness, &disabled,
			"extension", "disable", viewProgramFixtureExtensionName, "-o", "json",
		)
		assertViewProgramSessionGone(t, ctx, harness, openedB)
		runExtensionAuthoringCLI(
			t, ctx, harness, &enabled,
			"extension", "enable", viewProgramFixtureExtensionName, "-o", "json",
		)
		fresh := openViewProgram(t, ctx, harness, clientB.AttachmentToken, nil)
		if fresh.ViewSession == openedB.ViewSession {
			t.Fatalf("fresh session = %q, want a new identity after restart", fresh.ViewSession)
		}
		closeViewProgram(t, ctx, harness, fresh.ViewSession, clientB.AttachmentToken)
	})
}

func viewProgramNodePath(t *testing.T) string {
	t.Helper()
	nodePath, err := execabs.LookPath("node")
	if err != nil {
		t.Fatalf("resolve node executable error = %v", err)
	}
	if strings.Contains(filepath.ToSlash(nodePath), "/mise/shims/") {
		command := execabs.Command("mise", "which", "node")
		output, commandErr := command.Output()
		if commandErr != nil {
			t.Fatalf("resolve mise node executable error = %v", commandErr)
		}
		nodePath = strings.TrimSpace(string(output))
	}
	if nodePath == "" {
		t.Fatal("resolved node executable is empty")
	}
	return filepath.Dir(nodePath) + string(os.PathListSeparator) + os.Getenv("PATH")
}

func registerViewProgramClient(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	clientID string,
) compozycontract.WindowManagerClientView {
	t.Helper()
	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) + "/window-manager/clients"
	request := compozycontract.WindowManagerClientRegistration{
		WorkspaceID: windowmanager.WorkspaceID(harness.WorkspaceID),
		ClientID:    windowmanager.ClientID(clientID),
		Kind:        windowmanager.ClientKindBrowser,
		Context:     compozycontract.WindowManagerClientContextInput{WorkspaceTrusted: true},
	}
	var client compozycontract.WindowManagerClientView
	doViewProgramJSON(t, ctx, harness, http.MethodPost, path, request, "", http.StatusCreated, &client)
	if strings.TrimSpace(client.AttachmentToken) == "" {
		t.Fatalf("registered client %q has no attachment token", clientID)
	}
	return client
}

func openViewProgram(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	token string,
	args map[string]any,
) compozycontract.CmdPaletteViewSessionOpenResponse {
	t.Helper()
	var opened compozycontract.CmdPaletteViewSessionOpenResponse
	doViewProgramJSON(
		t, ctx, harness, http.MethodPost,
		"/api/cmd-palette/views/ext.notes-ts.browser/open",
		compozycontract.CmdPaletteViewSessionOpenRequest{Workspace: harness.WorkspaceID, Args: args},
		token, http.StatusOK, &opened,
	)
	if opened.ViewSession == "" || opened.StreamToken == "" || opened.FirstFrame.Revision == "" {
		t.Fatalf("opened view program = %#v, want complete session", opened)
	}
	return opened
}

func openViewProgramStream(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	opened compozycontract.CmdPaletteViewSessionOpenResponse,
) (*http.Response, *bufio.Reader) {
	t.Helper()
	path := "/api/cmd-palette/view-sessions/" + url.PathEscape(opened.ViewSession) +
		"/stream?token=" + url.QueryEscape(opened.StreamToken)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
	if err != nil {
		t.Fatalf("create view stream request error = %v", err)
	}
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatalf("open view stream error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if joined := errors.Join(readErr, closeErr); joined != nil {
			t.Fatalf("read failed view stream error = %v", joined)
		}
		t.Fatalf("view stream status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, body)
	}
	return response, bufio.NewReader(response.Body)
}

func readViewProgramFrame(t *testing.T, reader *bufio.Reader) cmdpalette.ViewFrame {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read view stream frame error = %v", err)
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var frame cmdpalette.ViewFrame
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data: "))), &frame); err != nil {
			t.Fatalf("decode view stream frame error = %v; line=%s", err, line)
		}
		return frame
	}
}

func viewProgramSearchHandler(t *testing.T, frame cmdpalette.ViewFrame) string {
	t.Helper()
	if frame.Payload == nil || frame.Payload.Chrome == nil || frame.Payload.Chrome.OnSearch == "" {
		t.Fatalf("view frame = %#v, want search handler", frame)
	}
	return frame.Payload.Chrome.OnSearch
}

func viewProgramActionHandler(t *testing.T, frames ...cmdpalette.ViewFrame) string {
	t.Helper()
	for _, frame := range frames {
		if frame.Payload == nil {
			continue
		}
		for _, section := range frame.Payload.Sections {
			for _, row := range section.Rows {
				for _, action := range row.Actions {
					if action.Destructive && action.Handler != "" {
						return action.Handler
					}
				}
			}
		}
	}
	t.Fatal("view frames contain no destructive action handler")
	return ""
}

func postViewProgramEvent(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	opened compozycontract.CmdPaletteViewSessionOpenResponse,
	token string,
	event compozycontract.CmdPaletteViewSessionEventRequest,
) {
	t.Helper()
	path := "/api/cmd-palette/view-sessions/" + url.PathEscape(opened.ViewSession) + "/events"
	doViewProgramJSON(t, ctx, harness, http.MethodPost, path, event, token, http.StatusAccepted, nil)
}

func closeViewProgram(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	session string,
	token string,
) {
	t.Helper()
	path := "/api/cmd-palette/view-sessions/" + url.PathEscape(session)
	doViewProgramJSON(t, ctx, harness, http.MethodDelete, path, nil, token, http.StatusOK, nil)
}

func assertViewProgramForeignOwnership(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	opened compozycontract.CmdPaletteViewSessionOpenResponse,
	foreign compozycontract.WindowManagerClientView,
) {
	t.Helper()
	path := "/api/cmd-palette/view-sessions/" + url.PathEscape(opened.ViewSession) + "/events"
	var payload compozycontract.CmdPaletteError
	doViewProgramJSON(
		t,
		ctx,
		harness,
		http.MethodPost,
		path,
		compozycontract.CmdPaletteViewSessionEventRequest{
			Handler:  "foreign",
			Revision: opened.FirstFrame.Revision,
			Seq:      1,
		},
		foreign.AttachmentToken,
		http.StatusForbidden,
		&payload,
	)
	if payload.Error != "session_forbidden" {
		t.Fatalf("foreign ownership error = %#v, want session_forbidden", payload)
	}
}

func assertViewProgramSlowSessionIsolation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	client compozycontract.WindowManagerClientView,
) {
	t.Helper()
	slow := openViewProgram(t, ctx, harness, client.AttachmentToken, map[string]any{"mode": "slow"})
	fast := openViewProgram(t, ctx, harness, client.AttachmentToken, nil)
	fastResponse, fastReader := openViewProgramStream(t, ctx, harness, fast)
	defer closeViewProgramBody(t, fastResponse.Body)
	fastFirst := readViewProgramFrame(t, fastReader)
	postViewProgramEvent(
		t,
		ctx,
		harness,
		slow,
		client.AttachmentToken,
		compozycontract.CmdPaletteViewSessionEventRequest{
			Handler: viewProgramSearchHandler(t, slow.FirstFrame), Args: []any{"release", 1},
			Revision: slow.FirstFrame.Revision, Seq: 1,
		},
	)
	postViewProgramEvent(
		t,
		ctx,
		harness,
		fast,
		client.AttachmentToken,
		compozycontract.CmdPaletteViewSessionEventRequest{
			Handler: viewProgramSearchHandler(t, fastFirst), Args: []any{"release", 1},
			Revision: fastFirst.Revision, Seq: 1,
		},
	)
	if next := readViewProgramFrame(t, fastReader); next.Revision == fastFirst.Revision {
		t.Fatalf("fast session revision = %q, want progress while sibling is slow", next.Revision)
	}
	closeViewProgram(t, ctx, harness, slow.ViewSession, client.AttachmentToken)
	closeViewProgram(t, ctx, harness, fast.ViewSession, client.AttachmentToken)
}

func assertViewProgramSessionGone(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	opened compozycontract.CmdPaletteViewSessionOpenResponse,
) {
	t.Helper()
	path := "/api/cmd-palette/view-sessions/" + url.PathEscape(opened.ViewSession) +
		"/stream?token=" + url.QueryEscape(opened.StreamToken)
	var payload compozycontract.CmdPaletteError
	doViewProgramJSON(t, ctx, harness, http.MethodGet, path, nil, "", http.StatusGone, &payload)
	if payload.Error != "session_gone" {
		t.Fatalf("gone session error = %#v, want session_gone", payload)
	}
}

func doViewProgramJSON(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	method string,
	path string,
	payload any,
	token string,
	wantStatus int,
	result any,
) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode view program request error = %v", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, harness.HTTPURL(path), body)
	if err != nil {
		t.Fatalf("create view program request error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("X-Compozy-Client-Token", token)
	}
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatalf("view program request error = %v", err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read view program response error = %v", err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("view program status = %d, want %d; body=%s", response.StatusCode, wantStatus, responseBody)
	}
	if result != nil {
		if err := json.Unmarshal(responseBody, result); err != nil {
			t.Fatalf("decode view program response error = %v; body=%s", err, responseBody)
		}
	}
}

func closeViewProgramBody(t *testing.T, body io.Closer) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Errorf("close view stream body error = %v", err)
	}
}
