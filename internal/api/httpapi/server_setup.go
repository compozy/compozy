package httpapi

import (
	"errors"

	"log/slog"

	"strings"

	"time"

	"github.com/compozy/compozy/internal/api/ginutil"
	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/gin-gonic/gin"
)

func newDefaultServer(homePaths compozyconfig.HomePaths) *Server {
	return &Server{
		homePaths: homePaths,
		config:    compozyconfig.DefaultWithHome(homePaths),
		logger:    slog.Default(),
		now: func() time.Time {
			return time.Now().UTC()
		},
		pollInterval: defaultPollInterval,
		agentLoader:  compozyconfig.LoadAgentDef,
		httpExtendedServices: httpExtendedServices{
			surfaceSet: SurfaceSetLocal,
		},
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
	s.configureAddress()
	if err := s.validateRequired(); err != nil {
		return err
	}
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
		s.agentLoader = compozyconfig.LoadAgentDef
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
	case s.surfaceSet.validate() != nil:
		return s.surfaceSet.validate()
	case s.surfaceSet != SurfaceSetLocal && !isLoopbackHost(canonicalHost(s.host)):
		return errors.New("httpapi: tier listeners must bind a loopback host")
	case s.surfaceSet.hasOperatorRoutes() && s.deviceAuth == nil:
		return errors.New("httpapi: device authentication is required for operator tier listeners")
	case s.surfaceSet.hasOperatorRoutes() && s.gateway == nil:
		return errors.New("httpapi: gateway service is required for operator tier listeners")
	case s.surfaceSet != SurfaceSetLocal && s.gatewayAdmission == nil:
		return errors.New("httpapi: gateway admission control is required for tier listeners")
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
	if !s.portSet {
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
	s.engine.Use(corsMiddlewareWithForwardedTarget(s.host, s.surfaceSet != SurfaceSetLocal))
	s.engine.Use(requestBodyLimitMiddleware(maxAPIRequestBodyBytes))
	s.engine.Use(errorMiddleware())
}
