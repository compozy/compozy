//go:build integration

package client_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/sdk/go/client"
)

// not parallel: this journey clears COMPOZY_URL to prove unix-socket discovery.
func TestDaemonGoldenPath(t *testing.T) {
	workspace := os.Getenv("COMPOZY_TEST_WORKSPACE")
	if workspace == "" {
		t.Skip("requires the isolated app SDK daemon harness")
	}
	home, agent, expected := os.Getenv("COMPOZY_HOME"), os.Getenv("COMPOZY_TEST_AGENT"), os.Getenv("COMPOZY_TEST_REPLY")
	if home == "" || agent == "" || expected == "" {
		t.Fatal("app SDK harness must set COMPOZY_HOME, COMPOZY_TEST_AGENT, and COMPOZY_TEST_REPLY")
	}
	socket := os.Getenv("COMPOZY_TEST_SOCKET")
	if socket == "" {
		socket = filepath.Join(home, "daemon.sock")
	}
	// This case must exercise the socket even when the harness also exports its HTTP URL.
	t.Setenv("COMPOZY_URL", "")
	t.Run("Should create a session prompt the daemon and decode its transcript over a unix socket", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		// An empty discovery home makes an accidental TCP fallback fail the journey.
		c, err := client.New(ctx, client.WithSocket(socket), client.WithHome(t.TempDir()), client.WithToken(""))
		if err != nil {
			t.Fatalf("connect through unix socket: %v", err)
		}
		resolved, err := c.ResolveWorkspace(ctx, client.ResolveWorkspaceRequest{Path: workspace})
		if err != nil {
			t.Fatalf("resolve workspace: %v", err)
		}
		if resolved.Workspace.ID == "" {
			t.Fatal("workspace response has no ID")
		}
		created, err := c.CreateSession(ctx, client.CreateSessionParams{}, client.CreateSessionRequest{
			Workspace: resolved.Workspace.ID, AgentName: agent,
		})
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		if created.Session.ID == "" || created.Session.WorkspaceID != resolved.Workspace.ID {
			t.Fatalf("created session has incorrect ownership: %+v", created.Session)
		}
		waitForGoldenSession(t, ctx, c, resolved.Workspace.ID, created.Session.ID)
		prompt, err := c.Prompt(ctx, client.PromptInput{
			WorkspaceID: resolved.Workspace.ID, SessionID: created.Session.ID,
			Message: "sdk golden", IdempotencyKey: "go-sdk-golden",
		})
		if err != nil {
			t.Fatalf("prompt session: %v", err)
		}
		t.Cleanup(func() {
			if err := prompt.Close(); err != nil {
				t.Errorf("close prompt: %v", err)
			}
		})
		if accepted, ok := prompt.Accepted(); ok {
			t.Fatalf("idle golden prompt was queued instead of streamed: %+v", accepted)
		}
		var reply strings.Builder
		finished := false
		for {
			chunk, err := prompt.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("read prompt chunk: %v", err)
			}
			reply.WriteString(chunk.TextDelta())
			finished = finished || chunk.Type == "finish"
		}
		if !finished || !strings.Contains(reply.String(), expected) {
			t.Fatalf("prompt completion = %t, reply = %q, want %q", finished, reply.String(), expected)
		}
		stream, err := c.StreamSession(ctx, client.StreamSessionParams{
			WorkspaceID: resolved.Workspace.ID, SessionID: created.Session.ID,
		})
		if err != nil {
			t.Fatalf("subscribe to session transcript: %v", err)
		}
		t.Cleanup(func() {
			if err := stream.Close(); err != nil {
				t.Errorf("close session stream: %v", err)
			}
		})
		frame, err := stream.Next()
		if err != nil {
			t.Fatalf("read transcript snapshot: %v", err)
		}
		if frame.Event != "transcript_snapshot" || frame.ID == "" {
			t.Fatalf("first transcript frame = event %q id %q", frame.Event, frame.ID)
		}
		snapshot, err := frame.Data.AsTranscriptSnapshotPayload()
		if err != nil {
			t.Fatalf("decode typed transcript snapshot: %v", err)
		}
		if snapshot.SessionID != created.Session.ID || len(snapshot.Entries) == 0 {
			t.Fatalf("transcript snapshot lost its session or entries: %+v", snapshot)
		}
	})
}

func waitForGoldenSession(t *testing.T, ctx context.Context, c *client.Client, workspaceID, sessionID string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := c.GetSession(ctx, client.GetSessionParams{WorkspaceID: workspaceID, SessionID: sessionID})
		if err != nil {
			t.Fatalf("read starting session: %v", err)
		}
		switch result.Session.State {
		case "active":
			return
		case "stopped":
			t.Fatalf("session stopped during startup: %+v", result.Session)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for active session: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
