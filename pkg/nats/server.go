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

type Server struct {
	NatsServer *server.Server
	Conn       *nats.Conn
	Options    ServerOptions
}

func NewNatsServer(options ServerOptions) (*Server, error) {
	nc, ns, err := runEmbeddedServer(options)
	if err != nil {
		return nil, fmt.Errorf("failed to start embedded NATS server: %w", err)
	}

	return &Server{
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
	addr, ok := ns.Addr().(*net.TCPAddr)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get server address: unexpected address type")
	}
	port := addr.Port
	clientOpts := []nats.Option{}
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port), clientOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to NATS server: %w", err)
	}

	return nc, ns, nil
}

func (s *Server) Shutdown() error {
	if s.Conn != nil {
		s.Conn.Close()
	}

	if s.NatsServer != nil {
		s.NatsServer.Shutdown()
		s.NatsServer.WaitForShutdown()
	}

	return nil
}

func (s *Server) WaitForShutdown() {
	if s.NatsServer != nil {
		s.NatsServer.WaitForShutdown()
	}
}

func (s *Server) IsRunning() bool {
	return s.NatsServer != nil && s.NatsServer.Running()
}

// request sends a request message and waits for a response of the specified type
func (s *Server) request(
	execID string,
	subject string,
	msgType MessageType,
	payload interface{},
	timeout time.Duration,
	expectedResponseType MessageType,
	response interface{},
) error {
	msg, err := NewMessage(execID, msgType, payload)
	if err != nil {
		return fmt.Errorf("failed to create request message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal request message: %w", err)
	}

	respMsg, err := s.Conn.Request(subject, data, timeout)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	var respMessage Message
	if err := json.Unmarshal(respMsg.Data, &respMessage); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	switch respMessage.Type {
	case expectedResponseType:
		if err := respMessage.UnmarshalPayload(response); err != nil {
			return fmt.Errorf("failed to unmarshal response payload: %w", err)
		}
		return nil
	case TypeError:
		var errMsg ErrorMessage
		if err := respMessage.UnmarshalPayload(&errMsg); err != nil {
			return fmt.Errorf("failed to unmarshal error message: %w", err)
		}
		return fmt.Errorf("error from worker: %s", errMsg.Message)
	default:
		return fmt.Errorf("unexpected response type: %s", respMessage.Type)
	}
}

// RequestAgent sends an AgentRequest and waits for an AgentResponse or ErrorMessage
func (s *Server) RequestAgent(execID string, req *AgentRequest, timeout time.Duration) (*AgentResponse, error) {
	subject := GenAgentRequestSubject(execID, req.AgentID)
	var resp AgentResponse
	err := s.request(execID, subject, TypeAgentRequest, req, timeout, TypeAgentResponse, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to send agent request: %w", err)
	}
	return &resp, nil
}

// RequestTool sends a ToolRequest and waits for a ToolResponse or ErrorMessage
func (s *Server) RequestTool(execID string, req *ToolRequest, timeout time.Duration) (*ToolResponse, error) {
	subject := GenToolRequestSubject(execID, req.ToolID)
	var resp ToolResponse
	err := s.request(execID, subject, TypeToolRequest, req, timeout, TypeToolResponse, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to send tool request: %w", err)
	}
	return &resp, nil
}

// PublishLog publishes a log message to the appropriate subject
func (s *Server) PublishLog(execID string, logMsg *LogMessage) error {
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
func (s *Server) SubscribeToLogs(execID string, handler func(*LogMessage)) (*nats.Subscription, error) {
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
func (s *Server) SubscribeToLogLevel(
	execID string,
	level LogLevel,
	handler func(*LogMessage),
) (*nats.Subscription, error) {
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
