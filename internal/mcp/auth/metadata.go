package auth

import (
	"context"
	"errors"
	"strings"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func (s *Service) authServerMetadata(
	ctx context.Context,
	cfg ServerConfig,
	issuer string,
) (*oauthex.AuthServerMeta, error) {
	metadata, err := sdkauth.GetAuthServerMetadata(
		ctx,
		strings.TrimSpace(issuer),
		s.sdkHTTPClientFor(cfg),
	)
	if err != nil {
		return nil, errors.New("mcp auth: authorization server metadata discovery failed")
	}
	if metadata == nil {
		return nil, errors.New("mcp auth: authorization server metadata is required")
	}
	return metadata, nil
}
