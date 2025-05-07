package nats

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	// Helper function to start a NATS server for tests
	startNATSServer := func(t *testing.T) *NatsServer {
		opts := DefaultServerOptions()
		server, err := NewNatsServer(opts)
		if err != nil {
			t.Fatalf("Failed to start NATS server: %v", err)
		}
		return server
	}

	// Helper function to connect to a NATS server
	connectToServer := func(t *testing.T, server *NatsServer) *nats.Conn {
		port := server.NatsServer.Addr().(*net.TCPAddr).Port
		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port))
		if err != nil {
			t.Fatalf("Failed to connect to NATS server: %v", err)
		}
		return nc
	}

	t.Run("NewNatsServer", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()
		assert.NotNil(t, server.NatsServer)
		assert.NotNil(t, server.Conn)
		assert.True(t, server.NatsServer.ReadyForConnections(5*time.Second))
	})

	t.Run("Shutdown", func(t *testing.T) {
		server := startNATSServer(t)
		err := server.Shutdown()
		assert.NoError(t, err)
		assert.False(t, server.IsRunning())
	})

	t.Run("RequestAgent_Success", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding to the request
		agentID := "agent123"
		requestID := uuid.New().String()
		subject := GenAgentRequestSubject(agentID, requestID)

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe and signal when ready
			sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
				var reqMsg Message
				if err := json.Unmarshal(msg.Data, &reqMsg); err != nil {
					t.Error(err)
					return
				}
				var req AgentRequest
				if err := reqMsg.UnmarshalPayload(&req); err != nil {
					t.Error(err)
					return
				}
				resp := AgentResponse{
					ID:      req.ID,
					AgentID: req.AgentID,
					Output:  []byte(`{"result": "success"}`),
					Status:  StatusSuccess,
				}
				respMsg, _ := NewMessage(TypeAgentResponse, resp)
				data, _ := json.Marshal(respMsg)
				msg.Respond(data)
			})
			if err != nil {
				t.Error(err)
				return
			}
			defer sub.Unsubscribe()

			nc.Flush()   // Ensure subscription is registered
			close(ready) // Signal that we're ready

			// Keep the goroutine alive until test completes
			<-done
		}()

		// Wait for subscription to be ready
		select {
		case <-ready:
			// Subscription is ready, proceed with test
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for subscription to be ready")
		}

		// Send request
		req := NewAgentRequest(agentID, "Process data", AgentActionRequest{ActionID: "action1"}, nil, nil)
		req.ID = requestID // Ensure ID matches subscription
		resp, err := server.RequestAgent(req, 2*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, requestID, resp.ID)
		assert.Equal(t, agentID, resp.AgentID)
		assert.Equal(t, StatusSuccess, resp.Status)
		var output map[string]string
		assert.NoError(t, json.Unmarshal(resp.Output, &output))
		assert.Equal(t, "success", output["result"])
	})

	t.Run("RequestAgent_ErrorResponse", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding with an error
		agentID := "agent123"
		requestID := uuid.New().String()
		subject := GenAgentRequestSubject(agentID, requestID)

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe and signal when ready
			sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
				errMsg, _ := NewErrorMessage("Worker failed", requestID, "", nil)
				respMsg, _ := NewMessage(TypeError, errMsg)
				data, _ := json.Marshal(respMsg)
				msg.Respond(data)
			})
			if err != nil {
				t.Error(err)
				return
			}
			defer sub.Unsubscribe()

			nc.Flush()   // Ensure subscription is registered
			close(ready) // Signal that we're ready

			// Keep the goroutine alive until test completes
			<-done
		}()

		// Wait for subscription to be ready
		select {
		case <-ready:
			// Subscription is ready, proceed with test
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for subscription to be ready")
		}

		// Send request
		req := NewAgentRequest(agentID, "Process data", AgentActionRequest{ActionID: "action1"}, nil, nil)
		req.ID = requestID
		_, err := server.RequestAgent(req, 2*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error from worker: Worker failed")
	})

	t.Run("RequestAgent_Timeout", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// No worker responding, expect timeout
		req := NewAgentRequest("agent123", "Process data", AgentActionRequest{ActionID: "action1"}, nil, nil)
		_, err := server.RequestAgent(req, 1*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send agent request")
	})

	t.Run("RequestTool_Success", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding to the request
		toolID := "tool123"
		requestID := uuid.New().String()
		subject := GenToolRequestSubject(toolID, requestID)

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe and signal when ready
			sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
				var reqMsg Message
				if err := json.Unmarshal(msg.Data, &reqMsg); err != nil {
					t.Error(err)
					return
				}
				var req ToolRequest
				if err := reqMsg.UnmarshalPayload(&req); err != nil {
					t.Error(err)
					return
				}
				resp := ToolResponse{
					ID:     req.ID,
					ToolID: req.ToolID,
					Output: []byte(`{"result": "tool success"}`),
					Status: StatusSuccess,
				}
				respMsg, _ := NewMessage(TypeToolResponse, resp)
				data, _ := json.Marshal(respMsg)
				msg.Respond(data)
			})
			if err != nil {
				t.Error(err)
				return
			}
			defer sub.Unsubscribe()

			nc.Flush()   // Ensure subscription is registered
			close(ready) // Signal that we're ready

			// Keep the goroutine alive until test completes
			<-done
		}()

		// Wait for subscription to be ready
		select {
		case <-ready:
			// Subscription is ready, proceed with test
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for subscription to be ready")
		}

		// Send request
		req, err := NewToolRequest(toolID, "Tool desc", nil, nil, nil)
		assert.NoError(t, err)
		req.ID = requestID
		resp, err := server.RequestTool(req, 2*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, requestID, resp.ID)
		assert.Equal(t, toolID, resp.ToolID)
		assert.Equal(t, StatusSuccess, resp.Status)
		var output map[string]string
		assert.NoError(t, json.Unmarshal(resp.Output, &output))
		assert.Equal(t, "tool success", output["result"])
	})

	t.Run("RequestTool_ErrorResponse", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding with an error
		toolID := "tool123"
		requestID := uuid.New().String()
		subject := GenToolRequestSubject(toolID, requestID)

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe and signal when ready
			sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
				errMsg, _ := NewErrorMessage("Tool failed", requestID, "", nil)
				respMsg, _ := NewMessage(TypeError, errMsg)
				data, _ := json.Marshal(respMsg)
				msg.Respond(data)
			})
			if err != nil {
				t.Error(err)
				return
			}
			defer sub.Unsubscribe()

			nc.Flush()   // Ensure subscription is registered
			close(ready) // Signal that we're ready

			// Keep the goroutine alive until test completes
			<-done
		}()

		// Wait for subscription to be ready
		select {
		case <-ready:
			// Subscription is ready, proceed with test
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for subscription to be ready")
		}

		// Send request
		req, err := NewToolRequest(toolID, "Tool desc", nil, nil, nil)
		assert.NoError(t, err)
		req.ID = requestID
		_, err = server.RequestTool(req, 2*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error from worker: Tool failed")
	})

	t.Run("RequestTool_Timeout", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// No worker responding, expect timeout
		req, err := NewToolRequest("tool123", "Tool desc", nil, nil, nil)
		assert.NoError(t, err)
		_, err = server.RequestTool(req, 1*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send tool request")
	})
}
