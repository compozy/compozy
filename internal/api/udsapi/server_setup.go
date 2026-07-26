package udsapi

import (
	"errors"
	"fmt"
	"log/slog"

	"strings"

	"time"

	"github.com/compozy/agh/internal/api/ginutil"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/gin-gonic/gin"
)

func newDefaultServer(homePaths aghconfig.HomePaths) *Server {
	return &Server{
		homePaths: homePaths,
		config:    aghconfig.DefaultWithHome(homePaths),
		logger:    slog.Default(),
		now: func() time.Time {
			return time.Now().UTC()
		},
		pollInterval: defaultPollInterval,
		agentLoader:  aghconfig.LoadAgentDef,
	}
}

func applyOptions(server *Server, opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(server)
		}
	}
}

func registerRoutes(engine *gin.Engine, handlers *Handlers) {
	ginutil.QuietDebug(func() {
		RegisterRoutes(engine, handlers)
	})
}

func (s *Server) finalize() error {
	s.applyDefaults()
	if err := s.validateRequired(); err != nil {
		return err
	}
	return s.configureSocketPath()
}

func (s *Server) applyDefaults() {
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if s.now == nil {
		s.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if s.pollInterval <= 0 {
		s.pollInterval = defaultPollInterval
	}
	if s.startedAt.IsZero() {
		s.startedAt = s.now()
	}
	if s.agentLoader == nil {
		s.agentLoader = aghconfig.LoadAgentDef
	}
	if strings.TrimSpace(s.config.Daemon.Socket) == "" {
		s.config.Daemon.Socket = s.homePaths.DaemonSocket
	}
}

func (s *Server) validateRequired() error {
	switch {
	case s.sessions == nil:
		return ErrSessionManagerRequired
	case s.tasks == nil:
		return ErrTaskServiceRequired
	case s.observer == nil:
		return ErrObserverRequired
	case s.workspaces == nil:
		return ErrWorkspaceResolverRequired
	default:
		return nil
	}
}

func (s *Server) configureSocketPath() error {
	if strings.TrimSpace(s.socketPath) == "" {
		s.socketPath = strings.TrimSpace(s.config.Daemon.Socket)
	}
	if strings.TrimSpace(s.socketPath) == "" {
		return errors.New("udsapi: socket path is required")
	}
	if length := len([]byte(s.socketPath)); length > maxSocketPathBytes {
		return fmt.Errorf(
			"%w: %q is %d bytes; use %d bytes or fewer",
			ErrSocketPathTooLong,
			s.socketPath,
			length,
			maxSocketPathBytes,
		)
	}
	return nil
}

func (s *Server) ensureEngine() {
	if s.engine != nil {
		return
	}

	s.engine = ginutil.NewEngine()
	s.engine.Use(gin.Recovery())
}
