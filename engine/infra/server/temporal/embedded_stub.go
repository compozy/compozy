//go:build !embedded_temporal

package temporal

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
)

type Server struct{}

func StartEmbedded(_ context.Context, _ *config.Config, _ string) (*Server, error) {
	return nil, fmt.Errorf("embedded temporal server not enabled (build tag 'embedded_temporal' not set)")
}

func (s *Server) HostPort() string { return "" }
func (s *Server) UIPort() int      { return 0 }

func (s *Server) Stop() error { return nil }
