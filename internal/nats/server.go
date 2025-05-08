package nats

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// ServerOptions and NatsServer structs remain unchanged from the original
type ServerOptions struct {
	EnableLogging   bool
	ServerName      string
	EnableJetStream bool
	JetStreamDomain string
	Port            int // Port to listen on, 0 means random port
}

func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		EnableLogging:   true,
		ServerName:      "compozy_embedded_server",
		EnableJetStream: false,
		JetStreamDomain: "compozy",
		Port:            0, // Use random port by default
	}
}

type NatsServer struct {
	NatsServer *server.Server
	Conn       *nats.Conn
	Options    ServerOptions
}

func NewNatsServer(options ServerOptions) (*NatsServer, error) {
	nc, ns, err := runEmbeddedServer(options)
	if err != nil {
		return nil, fmt.Errorf("failed to start embedded NATS server: %w", err)
	}

	return &NatsServer{
		NatsServer: ns,
		Conn:       nc,
		Options:    options,
	}, nil
}

func runEmbeddedServer(options ServerOptions) (*nats.Conn, *server.Server, error) {
	serverOpts := &server.Options{
		ServerName: options.ServerName,
		JetStream:  options.EnableJetStream,
		Host:       "127.0.0.1", // Bind to localhost only
		Port:       options.Port,
	}

	if options.EnableJetStream && options.JetStreamDomain != "" {
		serverOpts.JetStreamDomain = options.JetStreamDomain
	}

	ns, err := server.NewServer(serverOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating NATS server: %w", err)
	}

	if options.EnableLogging {
		ns.ConfigureLogger()
	}

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, nil, fmt.Errorf("server failed to start in time")
	}

	// Get the actual port the server is listening on
	port := ns.Addr().(*net.TCPAddr).Port
	clientOpts := []nats.Option{}
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port), clientOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to NATS server: %w", err)
	}

	return nc, ns, nil
}

func (s *NatsServer) Shutdown() error {
	if s.Conn != nil {
		s.Conn.Close()
	}

	if s.NatsServer != nil {
		s.NatsServer.Shutdown()
		s.NatsServer.WaitForShutdown()
	}

	return nil
}

func (s *NatsServer) WaitForShutdown() {
	if s.NatsServer != nil {
		s.NatsServer.WaitForShutdown()
	}
}

func (s *NatsServer) IsRunning() bool {
	return s.NatsServer != nil && s.NatsServer.Running()
}

// RequestAgent sends an AgentRequest and waits for an AgentResponse or ErrorMessage
func (s *NatsServer) RequestAgent(execID string, req *AgentRequest, timeout time.Duration) (*AgentResponse, error) {
	msg, err := NewMessage(execID, TypeAgentRequest, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent request message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent request message: %w", err)
	}

	subject := GenAgentRequestSubject(execID, req.AgentID)
	respMsg, err := s.Conn.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to send agent request: %w", err)
	}

	var respMessage Message
	if err := json.Unmarshal(respMsg.Data, &respMessage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	switch respMessage.Type {
	case TypeAgentResponse:
		var resp AgentResponse
		if err := respMessage.UnmarshalPayload(&resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent response: %w", err)
		}
		return &resp, nil
	case TypeError:
		var errMsg ErrorMessage
		if err := respMessage.UnmarshalPayload(&errMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error message: %w", err)
		}
		return nil, fmt.Errorf("error from worker: %s", errMsg.Message)
	default:
		return nil, fmt.Errorf("unexpected response type: %s", respMessage.Type)
	}
}

// RequestTool sends a ToolRequest and waits for a ToolResponse or ErrorMessage
func (s *NatsServer) RequestTool(execID string, req *ToolRequest, timeout time.Duration) (*ToolResponse, error) {
	msg, err := NewMessage(execID, TypeToolRequest, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool request message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool request message: %w", err)
	}

	subject := GenToolRequestSubject(execID, req.ToolID)
	respMsg, err := s.Conn.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to send tool request: %w", err)
	}

	var respMessage Message
	if err := json.Unmarshal(respMsg.Data, &respMessage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	switch respMessage.Type {
	case TypeToolResponse:
		var resp ToolResponse
		if err := respMessage.UnmarshalPayload(&resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool response: %w", err)
		}
		return &resp, nil
	case TypeError:
		var errMsg ErrorMessage
		if err := respMessage.UnmarshalPayload(&errMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error message: %w", err)
		}
		return nil, fmt.Errorf("error from worker: %s", errMsg.Message)
	default:
		return nil, fmt.Errorf("unexpected response type: %s", respMessage.Type)
	}
}

// PublishLog publishes a log message to the appropriate subject
func (s *NatsServer) PublishLog(execID string, logMsg *LogMessage) error {
	msg, err := NewMessage(execID, TypeLog, logMsg)
	if err != nil {
		return fmt.Errorf("failed to create log message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal log message: %w", err)
	}

	subject := GenLogSubject(execID, logMsg.Level)
	return s.Conn.Publish(subject, data)
}

// SubscribeToLogs subscribes to log messages and calls the handler for each message
func (s *NatsServer) SubscribeToLogs(execID string, handler func(*LogMessage)) (*nats.Subscription, error) {
	subject := GenLogWildcard(execID)

	sub, err := s.Conn.Subscribe(subject, func(msg *nats.Msg) {
		var message Message
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			return // Silently ignore invalid messages
		}

		if message.Type != TypeLog {
			return // Ignore non-log messages
		}

		var logMsg LogMessage
		if err := message.UnmarshalPayload(&logMsg); err != nil {
			return // Silently ignore invalid log messages
		}

		handler(&logMsg)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to logs: %w", err)
	}

	return sub, nil
}

// SubscribeToLogLevel subscribes to log messages of a specific level
func (s *NatsServer) SubscribeToLogLevel(execID string, level LogLevel, handler func(*LogMessage)) (*nats.Subscription, error) {
	subject := GenLogLevelWildcard(execID, level)

	sub, err := s.Conn.Subscribe(subject, func(msg *nats.Msg) {
		var message Message
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			return // Silently ignore invalid messages
		}

		if message.Type != TypeLog {
			return // Ignore non-log messages
		}

		var logMsg LogMessage
		if err := message.UnmarshalPayload(&logMsg); err != nil {
			return // Silently ignore invalid log messages
		}

		handler(&logMsg)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to log level %s: %w", level, err)
	}

	return sub, nil
}
