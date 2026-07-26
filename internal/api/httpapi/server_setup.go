package httpapi

import (
	"errors"

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

func (s *Server) finalize() error {
	s.applyDefaults()
	if err := s.validateRequired(); err != nil {
		return err
	}
	s.configureAddress()
	return nil
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
	if strings.TrimSpace(s.config.HTTP.Host) == "" {
		s.config.HTTP.Host = serverLocalhostKey
	}
	if s.config.HTTP.Port <= 0 {
		s.config.HTTP.Port = 2123
	}
}

func (s *Server) validateRequired() error {
	switch {
	case len(s.resourceAuth) > 0 && s.resources == nil:
		return errors.New("httpapi: resource service is required when resource operator auth is configured")
	case s.sessions == nil:
		return errors.New("httpapi: session manager is required")
	case s.tasks == nil:
		return errors.New("httpapi: task service is required")
	case s.observer == nil:
		return errors.New("httpapi: observer is required")
	case s.workspaces == nil:
		return errors.New("httpapi: workspace resolver is required")
	default:
		return nil
	}
}

func (s *Server) configureAddress() {
	if strings.TrimSpace(s.host) == "" {
		s.host = strings.TrimSpace(s.config.HTTP.Host)
	}
	if s.port <= 0 {
		s.port = s.config.HTTP.Port
	}
}

func (s *Server) ensureEngine() {
	if s.engine != nil {
		return
	}

	s.engine = ginutil.NewEngine()
	s.engine.Use(gin.Recovery())
	s.engine.Use(requestLoggingMiddleware(s.logger))
	s.engine.Use(corsMiddleware(s.host))
	s.engine.Use(requestBodyLimitMiddleware(maxAPIRequestBodyBytes))
	s.engine.Use(errorMiddleware())
}
