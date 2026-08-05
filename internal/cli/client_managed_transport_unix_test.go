//go:build !windows

package cli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
)

func TestNewClientUsesManagedAgentTransport(t *testing.T) {
	// not parallel: t.Setenv binds the managed transport for this process.
	t.Run("Should reuse the session socket for the complete skill view flow", func(t *testing.T) {
		socketDir, err := os.MkdirTemp("/tmp", "cz-cli-")
		if err != nil {
			t.Fatalf("os.MkdirTemp(short socket path) error = %v", err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(socketDir); err != nil {
				t.Errorf("os.RemoveAll(short socket path) error = %v", err)
			}
		})
		socketPath := filepath.Join(socketDir, "agent.sock")
		var listenConfig net.ListenConfig
		listener, err := listenConfig.Listen(t.Context(), "unix", socketPath)
		if err != nil {
			t.Fatalf("net.Listen(unix) error = %v", err)
		}
		server := &http.Server{
			Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				var body string
				switch request.URL.Path {
				case "/api/skills/release-signal":
					body = `{"skill":{"name":"release-signal","source":"user","enabled":true}}`
				case "/api/skills/release-signal/content":
					body = `{"content":"HELIX-SKILL-314"}`
				default:
					http.NotFound(writer, request)
					return
				}
				if _, err := writer.Write([]byte(body)); err != nil {
					t.Errorf("writer.Write() error = %v", err)
				}
			}),
			ReadHeaderTimeout: time.Second,
		}
		serveDone := make(chan error, 1)
		go func() {
			serveDone <- server.Serve(listener)
		}()
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				t.Errorf("server.Close() error = %v", err)
			}
			if err := <-serveDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("server.Serve() error = %v", err)
			}
		})

		t.Setenv(agentidentity.EnvTransportSocket, socketPath)
		client, err := NewClient("/not/reachable/from/provider.sock")
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		query := SkillQuery{Caller: agentidentity.Credentials{SessionID: "sess-managed", AgentName: "general"}}
		skill, err := client.GetSkill(context.Background(), "release-signal", query)
		if err != nil {
			t.Fatalf("GetSkill() error = %v", err)
		}
		if skill.Name != "release-signal" {
			t.Fatalf("GetSkill().Name = %q, want release-signal", skill.Name)
		}
		content, err := client.GetSkillContent(context.Background(), "release-signal", query)
		if err != nil {
			t.Fatalf("GetSkillContent() error = %v", err)
		}
		if content != "HELIX-SKILL-314" {
			t.Fatalf("GetSkillContent() = %q, want marker", content)
		}
	})
}
