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

func Test_Server(t *testing.T) {
	// Helper function to start a NATS server for tests
	startNATSServer := func(t *testing.T) *Server {
		opts := DefaultServerOptions()
		server, err := NewNatsServer(opts)
		if err != nil {
			t.Fatalf("Failed to start NATS server: %v", err)
		}
		return server
	}

	// Helper function to connect to a NATS server
	connectToServer := func(t *testing.T, server *Server) *nats.Conn {
		port := server.NatsServer.Addr().(*net.TCPAddr).Port
		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port))
		if err != nil {
			t.Fatalf("Failed to connect to NATS server: %v", err)
		}
		return nc
	}

	t.Run("Should create new NATS server successfully", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()
		assert.NotNil(t, server.NatsServer)
		assert.NotNil(t, server.Conn)
		assert.True(t, server.NatsServer.ReadyForConnections(5*time.Second))
	})

	t.Run("Should shutdown server gracefully", func(t *testing.T) {
		server := startNATSServer(t)
		err := server.Shutdown()
		assert.NoError(t, err)
		assert.False(t, server.IsRunning())
	})

	t.Run("Should handle successful agent request", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding to the request
		agentID := "agent123"
		execID := uuid.New().String()
		subject := GenAgentRequestSubject(execID, agentID)

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
				respMsg, _ := NewMessage(execID, TypeAgentResponse, resp)
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
		resp, err := server.RequestAgent(execID, req, 2*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, req.ID, resp.ID)
		assert.Equal(t, agentID, resp.AgentID)
		assert.Equal(t, StatusSuccess, resp.Status)
		var output map[string]string
		assert.NoError(t, json.Unmarshal(resp.Output, &output))
		assert.Equal(t, "success", output["result"])
	})

	t.Run("Should handle agent request error response", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding with an error
		agentID := "agent123"
		execID := uuid.New().String()
		subject := GenAgentRequestSubject(execID, agentID)

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe and signal when ready
			sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
				errMsg, _ := NewErrorMessage("Worker failed", "", nil)
				respMsg, _ := NewMessage(execID, TypeError, errMsg)
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
		_, err := server.RequestAgent(execID, req, 2*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error from worker: Worker failed")
	})

	t.Run("Should handle agent request timeout", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// No worker responding, expect timeout
		execID := uuid.New().String()
		req := NewAgentRequest("agent123", "Process data", AgentActionRequest{ActionID: "action1"}, nil, nil)
		_, err := server.RequestAgent(execID, req, 1*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send agent request")
	})

	t.Run("Should handle successful tool request", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding to the request
		toolID := "tool123"
		execID := uuid.New().String()
		subject := GenToolRequestSubject(execID, toolID)

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
				respMsg, _ := NewMessage(execID, TypeToolResponse, resp)
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
		resp, err := server.RequestTool(execID, req, 2*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, req.ID, resp.ID)
		assert.Equal(t, toolID, resp.ToolID)
		assert.Equal(t, StatusSuccess, resp.Status)
		var output map[string]string
		assert.NoError(t, json.Unmarshal(resp.Output, &output))
		assert.Equal(t, "tool success", output["result"])
	})

	t.Run("Should handle tool request error response", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Simulate a Deno worker responding with an error
		toolID := "tool123"
		execID := uuid.New().String()
		subject := GenToolRequestSubject(execID, toolID)

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe and signal when ready
			sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
				errMsg, _ := NewErrorMessage("Tool failed", "", nil)
				respMsg, _ := NewMessage(execID, TypeError, errMsg)
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
		_, err = server.RequestTool(execID, req, 2*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error from worker: Tool failed")
	})

	t.Run("Should handle tool request timeout", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// No worker responding, expect timeout
		execID := uuid.New().String()
		req, err := NewToolRequest("tool123", "Tool desc", nil, nil, nil)
		assert.NoError(t, err)
		_, err = server.RequestTool(execID, req, 1*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send tool request")
	})

	t.Run("Should handle log message publishing and subscribing", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		// Create a channel to receive log messages
		logs := make(chan *LogMessage, 10)
		defer close(logs)

		// Generate a single execID for the test
		execID := uuid.New().String()

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe to all log messages
			sub, err := server.SubscribeToLogs(execID, func(msg *LogMessage) {
				logs <- msg
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

		// Publish different log messages
		logMessages := []struct {
			level   LogLevel
			message string
			context map[string]any
		}{
			{InfoLevel, "Info message", map[string]any{"info": true}},
			{ErrorLevel, "Error message", map[string]any{"error": true}},
			{DebugLevel, "Debug message", map[string]any{"debug": true}},
		}

		for _, lm := range logMessages {
			logMsg, err := NewLogLevel(lm.level, lm.message, lm.context, time.Now())
			assert.NoError(t, err)
			err = server.PublishLog(execID, logMsg)
			assert.NoError(t, err)
		}

		// Verify all messages were received
		for i := range logMessages {
			select {
			case msg := <-logs:
				found := false
				for _, lm := range logMessages {
					if msg.Level == lm.level && msg.Message == lm.message {
						assert.Equal(t, lm.context, msg.Context)
						found = true
						break
					}
				}
				assert.True(t, found, "Received unexpected log message: %+v", msg)
			case <-time.After(2 * time.Second):
				t.Fatalf("Timeout waiting for log message %d", i+1)
			}
		}
	})

	t.Run("Should handle level-specific log subscriptions", func(t *testing.T) {
		server := startNATSServer(t)
		defer server.Shutdown()

		// Create channels for synchronization
		ready := make(chan struct{})
		done := make(chan struct{})
		defer close(done)

		// Create a channel to receive error logs
		errorLogs := make(chan *LogMessage, 5)
		defer close(errorLogs)

		// Generate a single execID for the test
		execID := uuid.New().String()

		go func() {
			nc := connectToServer(t, server)
			defer nc.Close()

			// Subscribe only to error logs
			sub, err := server.SubscribeToLogLevel(execID, ErrorLevel, func(msg *LogMessage) {
				errorLogs <- msg
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

		// Publish different log messages
		logMessages := []struct {
			level   LogLevel
			message string
		}{
			{InfoLevel, "Info message"},
			{ErrorLevel, "Error message"},
			{DebugLevel, "Debug message"},
			{ErrorLevel, "Another error"},
		}

		for _, lm := range logMessages {
			logMsg, err := NewLogLevel(lm.level, lm.message, nil, time.Now())
			assert.NoError(t, err)
			err = server.PublishLog(execID, logMsg)
			assert.NoError(t, err)
		}

		// Verify only error messages were received
		receivedCount := 0
		for i := 0; i < 2; i++ { // We expect 2 error messages
			select {
			case msg := <-errorLogs:
				assert.Equal(t, ErrorLevel, msg.Level)
				receivedCount++
			case <-time.After(2 * time.Second):
				t.Fatalf("Timeout waiting for error log message %d", i+1)
			}
		}

		assert.Equal(t, 2, receivedCount, "Expected to receive exactly 2 error messages")
	})
}
