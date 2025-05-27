package nats

import (
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// ServerOptions and NatsServer structs remain unchanged from the original
type ServerOptions struct {
	EnableLogging   bool
	ServerName      string
	EnableJetStream bool
	JetStreamDomain string
	Port            int
	StoreDir        string
}

func DefaultServerOptions() ServerOptions {
	storeDir := filepath.Join(core.GetStoreDir(), "nats")
	return ServerOptions{
		EnableLogging:   false,
		ServerName:      "compozy_embedded_server",
		EnableJetStream: false,
		JetStreamDomain: fmt.Sprintf("compozy_%s", core.GetVersion()),
		Port:            0,
		StoreDir:        storeDir,
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

	if options.EnableJetStream {
		if options.JetStreamDomain != "" {
			serverOpts.JetStreamDomain = options.JetStreamDomain
		}
		// Configure JetStream store directory
		serverOpts.StoreDir = options.StoreDir
	}

	ns, err := server.NewServer(serverOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating NATS server: %w", err)
	}

	if options.EnableLogging {
		ns.ConfigureLogger()
	}

	go ns.Start()

	if !ns.ReadyForConnections(15 * time.Second) {
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
