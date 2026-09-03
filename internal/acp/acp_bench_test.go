package acp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func BenchmarkHandleSessionUpdateAgentMessage(b *testing.B) {
	proc := &AgentProcess{}
	active, err := proc.beginPrompt("turn-bench", 1)
	if err != nil {
		b.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	payload := mustMarshalJSON(wireSessionNotification{
		SessionID: "sess-bench",
		Update: mustMarshalJSON(map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]any{"type": "text", "text": "hello from bench"},
		}),
	})
	if len(payload) == 0 {
		b.Fatal("benchmark payload must not be empty")
	}

	b.ReportAllocs()

	for b.Loop() {
		if err := proc.handleSessionUpdate(payload); err != nil {
			b.Fatalf("handleSessionUpdate() error = %v", err)
		}
		<-active.events
	}
}

func BenchmarkHandleInboundReadTextFile(b *testing.B) {
	proc := &AgentProcess{
		toolHost: contextAwareToolHost{},
	}
	payload := mustMarshalJSON(acpsdk.ReadTextFileRequest{
		SessionId: "sess-bench",
		Path:      "notes.txt",
	})
	if len(payload) == 0 {
		b.Fatal("benchmark payload must not be empty")
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, reqErr := proc.handleInbound(
			context.Background(),
			acpsdk.ClientMethodFsReadTextFile,
			payload,
		); reqErr != nil {
			b.Fatalf("handleInbound() error = %v", reqErr)
		}
	}
}

func BenchmarkPermissionPolicyResolvePathExistingRelative(b *testing.B) {
	root := b.TempDir()
	target := filepath.Join(root, "nested", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		b.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(target, []byte("bench"), 0o600); err != nil {
		b.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveReads, root)
	if err != nil {
		b.Fatalf("newPermissionPolicy() error = %v", err)
	}

	relative := filepath.Join("nested", "notes.txt")
	b.ReportAllocs()

	for b.Loop() {
		if _, err := policy.resolvePath(relative); err != nil {
			b.Fatalf("resolvePath() error = %v", err)
		}
	}
}
