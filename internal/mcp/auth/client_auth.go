package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	tokenEndpointAuthMethodNone  = "none"
	tokenEndpointAuthMethodBasic = "client_secret_basic"
	tokenEndpointAuthMethodPost  = "client_secret_post"
)

func registrationTokenEndpointAuthMethod(method string, clientSecret string) (string, error) {
	normalized := strings.TrimSpace(method)
	if normalized == "" {
		if strings.TrimSpace(clientSecret) == "" {
			normalized = tokenEndpointAuthMethodNone
		} else {
			normalized = tokenEndpointAuthMethodBasic
		}
	}
	if err := validateTokenEndpointAuthMethod(normalized, clientSecret); err != nil {
		return "", err
	}
	return normalized, nil
}

func effectiveTokenEndpointAuthMethod(cfg ServerConfig) (string, error) {
	method := strings.TrimSpace(cfg.TokenEndpointAuthMethod)
	if method == "" {
		if strings.TrimSpace(cfg.ClientSecret) == "" {
			method = tokenEndpointAuthMethodNone
		} else {
			method = tokenEndpointAuthMethodPost
		}
	}
	if err := validateTokenEndpointAuthMethod(method, cfg.ClientSecret); err != nil {
		return "", err
	}
	return method, nil
}

func validateTokenEndpointAuthMethod(method string, clientSecret string) error {
	switch method {
	case tokenEndpointAuthMethodNone:
		return nil
	case tokenEndpointAuthMethodBasic, tokenEndpointAuthMethodPost:
		if strings.TrimSpace(clientSecret) == "" {
			return fmt.Errorf("mcp auth: token endpoint auth method %q requires a client secret", method)
		}
		return nil
	default:
		return fmt.Errorf("mcp auth: unsupported token endpoint auth method %q", method)
	}
}

func newAuthenticatedFormRequest(
	ctx context.Context,
	endpoint string,
	cfg ServerConfig,
	values url.Values,
) (*http.Request, error) {
	method, err := effectiveTokenEndpointAuthMethod(cfg)
	if err != nil {
		return nil, err
	}
	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		return nil, errors.New("mcp auth: token endpoint client id is required")
	}
	values.Del("client_id")
	values.Del("client_secret")
	switch method {
	case tokenEndpointAuthMethodNone:
		values.Set("client_id", clientID)
	case tokenEndpointAuthMethodPost:
		values.Set("client_id", clientID)
		values.Set("client_secret", cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimSpace(endpoint),
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if method == tokenEndpointAuthMethodBasic {
		req.SetBasicAuth(clientID, cfg.ClientSecret)
	}
	return req, nil
}
