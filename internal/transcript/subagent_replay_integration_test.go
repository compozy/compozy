//go:build integration

package transcript_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/transcript"
)

func TestNativeSubagentFixtureReplay(t *testing.T) {
	// IT-020 transcript side: real ACP subprocess -> canonical SQLite ledger -> UI parts.
	t.Run("Should persist and project parent tool attribution from adapter frames", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile("../acp/testdata/claude_agent_subagent.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		var steps []acpmock.Step
		for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
			steps = append(
				steps,
				acpmock.Step{Kind: acpmock.StepKindDriverControl, DriverControl: &acpmock.DriverControlStep{
					Action:     acpmock.DriverControlWriteRawJSONRPC,
					RawJSONRPC: strings.ReplaceAll(line, `"session-native"`, `"native-session-1"`),
				}},
			)
		}
		fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{
			{
				Name:     "native",
				Provider: "claude",
				Turns:    []acpmock.TurnFixture{{Match: acpmock.TurnMatch{UserText: "replay"}, Steps: steps}},
			},
		}}
		encoded, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		fixturePath := filepath.Join(dir, "fixture.json")
		if err := os.WriteFile(fixturePath, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		driver := acp.New()
		proc, err := driver.Start(ctx, acp.StartOpts{
			AgentName: "native",
			Cwd:       dir,
			Command: acpmock.BuildCommand(
				acpmock.RequireDriver(t),
				fixturePath,
				"native",
				filepath.Join(dir, "diagnostics.jsonl"),
			),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := driver.Stop(context.Background(), proc); err != nil {
				t.Errorf("stop: %v", err)
			}
		})
		db, err := sessiondb.OpenSessionDB(
			ctx,
			store.SessionDBOwner{SessionID: "parent", WorkspaceID: "workspace"},
			filepath.Join(dir, "events.db"),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close database: %v", err)
			}
		})
		stream, err := driver.Prompt(ctx, proc, acp.PromptRequest{TurnID: "turn", Message: "replay"})
		if err != nil {
			t.Fatal(err)
		}
		for event := range stream {
			if event.Type == acp.EventTypeError {
				t.Fatalf("provider error: %s", event.Error)
			}
			payload, err := transcript.MarshalAgentEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Record(
				ctx,
				store.SessionEvent{
					SessionID: "parent",
					AgentName: "native",
					TurnID:    event.TurnID,
					Type:      event.Type,
					Timestamp: event.Timestamp,
					Content:   payload,
				},
			); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := db.Query(ctx, store.EventQuery{})
		if err != nil {
			t.Fatal(err)
		}
		inner, native := 0, 0
		for _, row := range rows {
			event, err := transcript.UnmarshalAgentEvent(row.Content)
			if err != nil {
				t.Fatal(err)
			}
			if event.ParentToolCallID() == "toolu_agent" {
				inner++
			}
			if event.ProviderToolName() == "Agent" {
				native++
			}
		}
		if inner != 3 || native != 2 {
			t.Fatalf("persisted inner/native events = %d/%d, want 3/2", inner, native)
		}
		messages, err := transcript.ToUIMessages(rows)
		if err != nil {
			t.Fatal(err)
		}
		cards, childText := 0, 0
		for _, message := range messages {
			for _, part := range message.Parts {
				if part.Type == "data-compozy-subagent" {
					cards++
					var payload transcript.UISubagentPayload
					if err := json.Unmarshal(part.Data, &payload); err != nil {
						t.Fatal(err)
					}
					if payload.TurnID != "turn" {
						t.Fatalf("native card turn = %q, want turn", payload.TurnID)
					}
				}
				if part.Type == "text" && part.Text == "Inspecting the diff." &&
					part.ParentToolCallID == "toolu_agent" {
					childText++
				}
			}
		}
		if cards != 1 || childText != 1 {
			t.Fatalf("cards/child text = %d/%d, want 1/1", cards, childText)
		}
	})
}
