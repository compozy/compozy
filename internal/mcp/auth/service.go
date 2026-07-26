package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	serviceBearerValue     = "Bearer"
	serviceAccessTokenKey  = "access_token"
	serviceRefreshTokenKey = "refresh_token"
)

// ServiceOption configures the OAuth service.
type ServiceOption func(*Service)

// Service executes OAuth 2.1 authorization-code flows for remote MCP servers.
type Service struct {
	store      TokenStore
	client     *http.Client
	random     io.Reader
	now        func() time.Time
	generation *MutationGeneration
}

// NewService constructs an MCP auth service.
func NewService(store TokenStore, opts ...ServiceOption) (*Service, error) {
	if store == nil {
		return nil, errors.New("mcp auth: token store is required")
	}
	service := &Service{
		store:      store,
		client:     &http.Client{Timeout: defaultMetadataClientTimeout},
		generation: NewMutationGeneration(),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	if service.client == nil {
		service.client = &http.Client{Timeout: defaultMetadataClientTimeout}
	}
	if service.now == nil {
		service.now = func() time.Time { return time.Now().UTC() }
	}
	if service.generation == nil {
		service.generation = NewMutationGeneration()
	}
	return service, nil
}

// WithHTTPClient overrides the HTTP client used for metadata and token calls.
func WithHTTPClient(client *http.Client) ServiceOption {
	return func(service *Service) {
		service.client = client
	}
}

// WithRandom overrides the entropy source for tests.
func WithRandom(random io.Reader) ServiceOption {
	return func(service *Service) {
		service.random = random
	}
}

// WithNow overrides the clock for tests.
func WithNow(now func() time.Time) ServiceOption {
	return func(service *Service) {
		service.now = now
	}
}

// LoginState holds the short-lived in-memory authorization flow state.
type LoginState struct {
	Target           Target
	RedirectURL      string
	State            string
	Verifier         string
	AuthorizationURL string
	Metadata         Metadata
	Config           ServerConfig
}

// BeginLogin discovers metadata, generates PKCE state, and returns the URL the
// operator must open. The returned verifier is sensitive and must stay in memory.
func (s *Service) BeginLogin(
	ctx context.Context,
	cfg ServerConfig,
	redirectURL string,
) (LoginState, error) {
	if err := cfg.Validate(); err != nil {
		return LoginState{}, err
	}
	if err := validateAbsoluteHTTPURL("redirect URL", redirectURL); err != nil {
		return LoginState{}, err
	}

	metadata, err := discoverMetadata(ctx, s.client, cfg)
	if err != nil {
		return LoginState{}, err
	}
	if !supportsS256(metadata.CodeChallengeMethodsSupported) {
		return LoginState{}, errors.New("mcp auth: OAuth server must support S256 PKCE")
	}
	pkce, err := newPKCEPair(s.random)
	if err != nil {
		return LoginState{}, err
	}
	if err := validateVerifier(pkce.Verifier); err != nil {
		return LoginState{}, err
	}
	state, err := newState(s.random)
	if err != nil {
		return LoginState{}, err
	}

	authURL, err := authorizationURL(metadata.AuthorizationEndpoint, cfg, redirectURL, state, pkce)
	if err != nil {
		return LoginState{}, err
	}
	return LoginState{
		Target:           cfg.Target.Normalize(),
		RedirectURL:      strings.TrimSpace(redirectURL),
		State:            state,
		Verifier:         pkce.Verifier,
		AuthorizationURL: authURL,
		Metadata:         metadata,
		Config:           cfg,
	}, nil
}

func authorizationURL(
	endpoint string,
	cfg ServerConfig,
	redirectURL string,
	state string,
	pkce PKCEPair,
) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", fmt.Errorf("mcp auth: parse authorization endpoint: %w", err)
	}
	values := parsed.Query()
	values.Set("response_type", "code")
	values.Set("client_id", strings.TrimSpace(cfg.ClientID))
	values.Set("redirect_uri", strings.TrimSpace(redirectURL))
	values.Set("state", state)
	values.Set("code_challenge", pkce.Challenge)
	values.Set("code_challenge_method", pkce.Method)
	if scopes := strings.Join(trimStrings(cfg.Scopes), " "); scopes != "" {
		values.Set("scope", scopes)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func authorizationCodeFromCallback(callbackURL string, wantState string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("mcp auth: callback URL is invalid")
	}
	values := parsed.Query()
	if gotState := values.Get("state"); gotState == "" || gotState != wantState {
		return "", errors.New("mcp auth: OAuth callback state mismatch")
	}
	if oauthErr := strings.TrimSpace(values.Get("error")); oauthErr != "" {
		return "", errors.New("mcp auth: OAuth callback reported an error")
	}
	code := strings.TrimSpace(values.Get("code"))
	if code == "" {
		return "", errors.New("mcp auth: OAuth callback code is required")
	}
	return code, nil
}

type tokenEndpointResponse struct {
	AccessToken      string         `json:"access_token"`
	RefreshToken     string         `json:"refresh_token"`
	TokenType        string         `json:"token_type"`
	ExpiresIn        tokenExpiresIn `json:"expires_in"`
	Scope            string         `json:"scope"`
	Error            string         `json:"error"`
	ErrorDescription string         `json:"error_description"`
}

type tokenExpiresIn struct {
	present bool
	value   int64
}

func (e *tokenExpiresIn) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.New("mcp auth: token response expires_in decoder is nil")
	}
	e.present = true
	if strings.TrimSpace(string(data)) == "null" {
		return errors.New("mcp auth: token response expires_in must be integer seconds")
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("mcp auth: token response expires_in must be integer seconds: %w", err)
	}
	e.value = value
	return nil
}

func (s *Service) exchangeCode(ctx context.Context, state LoginState, code string) (TokenRecord, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", strings.TrimSpace(state.RedirectURL))
	values.Set("client_id", strings.TrimSpace(state.Config.ClientID))
	values.Set("code_verifier", state.Verifier)
	if strings.TrimSpace(state.Config.ClientSecret) != "" {
		values.Set("client_secret", state.Config.ClientSecret)
	}

	resp, err := s.postForm(ctx, state.Metadata.TokenEndpoint, values)
	if err != nil {
		return TokenRecord{}, err
	}
	return s.tokenRecordFromResponse(state.Config, state.Metadata, resp, TokenRecord{})
}

func (s *Service) refreshToken(
	ctx context.Context,
	cfg ServerConfig,
	metadata Metadata,
	current TokenRecord,
) (TokenRecord, error) {
	values := url.Values{}
	values.Set("grant_type", serviceRefreshTokenKey)
	values.Set(serviceRefreshTokenKey, current.RefreshToken)
	values.Set("client_id", strings.TrimSpace(cfg.ClientID))
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		values.Set("client_secret", cfg.ClientSecret)
	}

	resp, err := s.postForm(ctx, metadata.TokenEndpoint, values)
	if err != nil {
		return TokenRecord{}, err
	}
	return s.tokenRecordFromResponse(cfg, metadata, resp, current)
}

func (s *Service) postForm(
	ctx context.Context,
	endpoint string,
	values url.Values,
) (payload tokenEndpointResponse, err error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimSpace(endpoint),
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return tokenEndpointResponse{}, fmt.Errorf("mcp auth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return tokenEndpointResponse{}, fmt.Errorf("mcp auth: call token endpoint: %w", err)
	}
	defer func() { err = errors.Join(err, drainAndCloseResponseBody(resp.Body)) }()

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return tokenEndpointResponse{}, fmt.Errorf("mcp auth: decode token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenEndpointResponse{}, fmt.Errorf(
			"mcp auth: token endpoint rejected request with HTTP %d",
			resp.StatusCode,
		)
	}
	if strings.TrimSpace(payload.Error) != "" {
		return tokenEndpointResponse{}, errors.New("mcp auth: token endpoint returned an error")
	}
	return payload, nil
}

func (s *Service) tokenRecordFromResponse(
	cfg ServerConfig,
	metadata Metadata,
	resp tokenEndpointResponse,
	current TokenRecord,
) (TokenRecord, error) {
	if strings.TrimSpace(resp.AccessToken) == "" {
		return TokenRecord{}, errors.New("mcp auth: token response access_token is required")
	}
	tokenType := strings.TrimSpace(resp.TokenType)
	if tokenType == "" {
		tokenType = serviceBearerValue
	}
	if !strings.EqualFold(tokenType, serviceBearerValue) {
		return TokenRecord{}, errors.New("mcp auth: token response token_type must be Bearer")
	}

	now := s.now().UTC()
	refreshToken := strings.TrimSpace(resp.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(current.RefreshToken)
	}
	scopes := trimStrings(strings.Fields(resp.Scope))
	if len(scopes) == 0 {
		scopes = trimStrings(cfg.Scopes)
	}
	expiresAt := time.Time{}
	if resp.ExpiresIn.present {
		parsedExpiresAt, err := tokenResponseExpiresAt(now, resp.ExpiresIn.value)
		if err != nil {
			return TokenRecord{}, err
		}
		expiresAt = parsedExpiresAt
	}
	obtainedAt := current.ObtainedAt
	if obtainedAt.IsZero() {
		obtainedAt = now
	}
	fingerprint, err := ServerDefinitionFingerprint(cfg)
	if err != nil {
		return TokenRecord{}, err
	}

	return TokenRecord{
		Target:                cfg.Target.Normalize(),
		DefinitionFingerprint: fingerprint,
		Issuer:                strings.TrimSpace(metadata.Issuer),
		ClientID:              strings.TrimSpace(cfg.ClientID),
		Scopes:                scopes,
		AccessToken:           strings.TrimSpace(resp.AccessToken),
		RefreshToken:          refreshToken,
		TokenType:             tokenType,
		ExpiresAt:             expiresAt,
		ObtainedAt:            obtainedAt,
		UpdatedAt:             now,
	}, nil
}

func tokenResponseExpiresAt(now time.Time, expiresIn int64) (time.Time, error) {
	if expiresIn < 0 {
		return time.Time{}, errors.New("mcp auth: token response expires_in must not be negative")
	}
	const maxExpiresInSeconds = int64(1<<63-1) / int64(time.Second)
	if expiresIn > maxExpiresInSeconds {
		return time.Time{}, errors.New("mcp auth: token response expires_in overflows duration")
	}
	return now.UTC().Add(time.Duration(expiresIn) * time.Second), nil
}

func (s *Service) revoke(
	ctx context.Context,
	cfg ServerConfig,
	metadata Metadata,
	token TokenRecord,
) (err error) {
	revokeToken := strings.TrimSpace(token.RefreshToken)
	hint := serviceRefreshTokenKey
	if revokeToken == "" {
		revokeToken = strings.TrimSpace(token.AccessToken)
		hint = serviceAccessTokenKey
	}
	if revokeToken == "" {
		return nil
	}

	values := url.Values{}
	values.Set("token", revokeToken)
	values.Set("token_type_hint", hint)
	values.Set("client_id", strings.TrimSpace(cfg.ClientID))
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		values.Set("client_secret", cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimSpace(metadata.RevocationEndpoint),
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return fmt.Errorf("mcp auth: build revocation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("mcp auth: call revocation endpoint: %w", err)
	}
	defer func() { err = errors.Join(err, drainAndCloseResponseBody(resp.Body)) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mcp auth: revocation endpoint HTTP %d", resp.StatusCode)
	}
	return nil
}

func statusFromToken(cfg ServerConfig, token *TokenRecord, now time.Time) Status {
	return statusFromTokenWithDiagnostic(cfg, token, now, "")
}

func statusFromTokenWithDiagnostic(
	cfg ServerConfig,
	token *TokenRecord,
	now time.Time,
	diagnostic string,
) Status {
	cfg.Target = cfg.Target.Normalize()
	status := Status{
		ServerName:       cfg.Target.ServerName,
		Scope:            cfg.Target.Scope,
		WorkspaceID:      cfg.Target.WorkspaceID,
		Status:           StatusNeedsLogin,
		RemoteURL:        strings.TrimSpace(cfg.RemoteURL),
		AuthType:         strings.TrimSpace(cfg.Type),
		ClientID:         strings.TrimSpace(cfg.ClientID),
		Scopes:           trimStrings(cfg.Scopes),
		RevocationURL:    strings.TrimSpace(cfg.RevocationURL),
		AuthorizationURL: strings.TrimSpace(cfg.AuthorizationURL),
		Diagnostic:       strings.TrimSpace(diagnostic),
	}
	if strings.TrimSpace(cfg.Type) == "" {
		status.Status = StatusUnconfigured
		return status
	}
	if token == nil {
		status.Diagnostic = firstNonEmpty(status.Diagnostic, "login required")
		return status
	}

	status.TokenPresent = strings.TrimSpace(token.AccessToken) != ""
	status.Refreshable = strings.TrimSpace(token.RefreshToken) != ""
	status.Issuer = strings.TrimSpace(token.Issuer)
	if len(token.Scopes) > 0 {
		status.Scopes = append([]string(nil), token.Scopes...)
	}
	if !token.ExpiresAt.IsZero() {
		expiresAt := token.ExpiresAt.UTC()
		status.ExpiresAt = &expiresAt
	}
	if !token.UpdatedAt.IsZero() {
		updatedAt := token.UpdatedAt.UTC()
		status.UpdatedAt = &updatedAt
	}
	switch {
	case !status.TokenPresent:
		status.Status = StatusInvalid
		status.Diagnostic = firstNonEmpty(status.Diagnostic, "stored token is missing access token")
	case status.ExpiresAt != nil && !status.ExpiresAt.After(now.UTC()):
		status.Status = StatusExpired
		status.Diagnostic = firstNonEmpty(status.Diagnostic, "token is expired")
	default:
		status.Status = StatusAuthenticated
	}
	return status
}

func supportsS256(methods []string) bool {
	if len(methods) == 0 {
		return false
	}
	for _, method := range methods {
		if strings.EqualFold(strings.TrimSpace(method), "S256") {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
