package mcpfixture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const (
	defaultScope            = "tools.read"
	resourceParameter       = "resource"
	authorizationCodeGrant  = "authorization_code"
	authorizationCodeParam  = "code"
	refreshTokenGrant       = "refresh_token"
	registrationAccessToken = "mcpfixture-registration-access-token" // #nosec G101 -- deterministic test-fixture value.

	// AuthorizationCode is the deterministic authorization code returned by the fixture.
	AuthorizationCode = "mcpfixture-authorization-code"
	// AccessToken is the deterministic access token returned by the fixture.
	// #nosec G101 -- deterministic test-fixture value, never a credential.
	AccessToken = "mcpfixture-access-token"
	// RefreshToken is the deterministic refresh token returned by the fixture.
	// #nosec G101 -- deterministic test-fixture value, never a credential.
	RefreshToken = "mcpfixture-refresh-token"
)

// OAuthConfig configures the optional local OAuth surface around an MCP fixture.
type OAuthConfig struct {
	MCPPath                                    string
	Scopes                                     []string
	StepUpScope                                string
	CallbackIssuer                             string
	AuthorizationResponseIssParameterSupported bool
	ClientIDMetadataDocumentSupported          bool
}

// OAuthEndpoints identifies the local, dynamically generated OAuth endpoints.
type OAuthEndpoints struct {
	MCPURL               string
	ProtectedResourceURL string
	IssuerURL            string
	AuthorizationURL     string
	TokenURL             string
	RegistrationURL      string
	ClientMetadataURL    string
}

// OAuthRequests records observable OAuth boundary inputs without emitting them to logs.
type OAuthRequests struct {
	Authorization []url.Values
	Token         []url.Values
	Registration  []map[string]any
}

// OAuthHTTPServer combines a fixture MCP endpoint and a local OAuth authority.
type OAuthHTTPServer struct {
	Server    *httptest.Server
	Endpoints OAuthEndpoints

	fixture  *Fixture
	config   OAuthConfig
	mu       sync.Mutex
	requests OAuthRequests
}

// StartOAuthHTTP starts a fixture with protected-resource metadata, authorization,
// token, dynamic-registration, client-metadata, refresh, mix-up, and step-up surfaces.
func StartOAuthHTTP(t testing.TB, fixture *Fixture, config OAuthConfig) *OAuthHTTPServer {
	t.Helper()
	if fixture == nil {
		t.Fatal("StartOAuthHTTP() fixture is nil")
	}
	config = normalizeOAuthConfig(config)
	server := &OAuthHTTPServer{fixture: fixture, config: config}
	server.Server = httptest.NewServer(http.HandlerFunc(server.serveHTTP))
	server.Endpoints = server.endpoints(server.Server.URL)
	t.Cleanup(server.Server.Close)
	return server
}

// Requests returns independent snapshots of every received OAuth boundary request.
func (s *OAuthHTTPServer) Requests() OAuthRequests {
	s.mu.Lock()
	defer s.mu.Unlock()
	return OAuthRequests{
		Authorization: cloneValuesSlice(s.requests.Authorization),
		Token:         cloneValuesSlice(s.requests.Token),
		Registration:  cloneRegistrationSlice(s.requests.Registration),
	}
}

func normalizeOAuthConfig(config OAuthConfig) OAuthConfig {
	config.MCPPath = "/" + strings.Trim(strings.TrimSpace(config.MCPPath), "/")
	if config.MCPPath == "/" {
		config.MCPPath = "/mcp"
	}
	if len(config.Scopes) == 0 {
		config.Scopes = []string{defaultScope}
	} else {
		config.Scopes = append([]string(nil), config.Scopes...)
	}
	config.StepUpScope = strings.TrimSpace(config.StepUpScope)
	config.CallbackIssuer = strings.TrimSpace(config.CallbackIssuer)
	return config
}

func (s *OAuthHTTPServer) endpoints(baseURL string) OAuthEndpoints {
	return OAuthEndpoints{
		MCPURL: baseURL + s.config.MCPPath,
		ProtectedResourceURL: baseURL + "/.well-known/oauth-protected-resource/" + strings.TrimPrefix(
			s.config.MCPPath,
			"/",
		),
		IssuerURL:         baseURL,
		AuthorizationURL:  baseURL + "/authorize",
		TokenURL:          baseURL + "/token",
		RegistrationURL:   baseURL + "/register",
		ClientMetadataURL: baseURL + "/.well-known/mcp-client.json",
	}
}

func (s *OAuthHTTPServer) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	baseURL := requestBaseURL(request)
	endpoints := s.endpoints(baseURL)
	switch request.URL.Path {
	case s.config.MCPPath:
		s.serveProtectedMCP(writer, request, endpoints)
	case "/.well-known/oauth-protected-resource/" + strings.TrimPrefix(s.config.MCPPath, "/"):
		s.writeJSON(writer, http.StatusOK, map[string]any{
			resourceParameter:       endpoints.MCPURL,
			"authorization_servers": []string{endpoints.IssuerURL},
			"scopes_supported":      s.config.Scopes,
		})
	case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
		s.writeAuthorizationServerMetadata(writer, endpoints)
	case "/authorize":
		s.serveAuthorize(writer, request, endpoints)
	case "/token":
		s.serveToken(writer, request, endpoints)
	case "/register":
		s.serveRegistration(writer, request, endpoints)
	case "/.well-known/mcp-client.json":
		s.writeJSON(writer, http.StatusOK, map[string]any{
			"client_name":    "Compozy MCP fixture client",
			"grant_types":    []string{authorizationCodeGrant, refreshTokenGrant},
			"response_types": []string{authorizationCodeParam},
		})
	default:
		http.NotFound(writer, request)
	}
}

func (s *OAuthHTTPServer) serveProtectedMCP(
	writer http.ResponseWriter,
	request *http.Request,
	endpoints OAuthEndpoints,
) {
	if request.Header.Get("Authorization") != "Bearer "+AccessToken {
		s.writeBearerChallenge(writer, endpoints.ProtectedResourceURL, "invalid_token", "")
		return
	}
	if s.config.StepUpScope != "" {
		s.writeBearerChallenge(writer, endpoints.ProtectedResourceURL, "insufficient_scope", s.config.StepUpScope)
		return
	}
	s.fixture.StreamableHTTPHandler().ServeHTTP(writer, request)
}

func (s *OAuthHTTPServer) writeBearerChallenge(writer http.ResponseWriter, metadataURL, reason, scope string) {
	challenge := fmt.Sprintf(`Bearer resource_metadata=%q, error=%q`, metadataURL, reason)
	if scope != "" {
		challenge += fmt.Sprintf(`, scope=%q`, scope)
	}
	writer.Header().Set("WWW-Authenticate", challenge)
	http.Error(writer, reason, http.StatusUnauthorized)
}

func (s *OAuthHTTPServer) writeAuthorizationServerMetadata(writer http.ResponseWriter, endpoints OAuthEndpoints) {
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"issuer":                                         endpoints.IssuerURL,
		"authorization_endpoint":                         endpoints.AuthorizationURL,
		"token_endpoint":                                 endpoints.TokenURL,
		"jwks_uri":                                       endpoints.IssuerURL + "/jwks",
		"registration_endpoint":                          endpoints.RegistrationURL,
		"scopes_supported":                               s.config.Scopes,
		"response_types_supported":                       []string{authorizationCodeParam},
		"grant_types_supported":                          []string{authorizationCodeGrant, refreshTokenGrant},
		"code_challenge_methods_supported":               []string{"S256"},
		"client_id_metadata_document_supported":          s.config.ClientIDMetadataDocumentSupported,
		"authorization_response_iss_parameter_supported": s.config.AuthorizationResponseIssParameterSupported,
	})
}

func (s *OAuthHTTPServer) serveAuthorize(writer http.ResponseWriter, request *http.Request, endpoints OAuthEndpoints) {
	query := request.URL.Query()
	s.recordAuthorization(query)
	if query.Get("state") == "" || query.Get("code_challenge") == "" ||
		query.Get(resourceParameter) != endpoints.MCPURL {
		http.Error(writer, "invalid authorization request", http.StatusBadRequest)
		return
	}
	redirectURL, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || redirectURL.Scheme == "" || redirectURL.Host == "" {
		http.Error(writer, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	callbackQuery := redirectURL.Query()
	callbackQuery.Set(authorizationCodeParam, AuthorizationCode)
	callbackQuery.Set("state", query.Get("state"))
	if s.config.AuthorizationResponseIssParameterSupported {
		issuer := endpoints.IssuerURL
		if s.config.CallbackIssuer != "" {
			issuer = s.config.CallbackIssuer
		}
		callbackQuery.Set("iss", issuer)
	}
	redirectURL.RawQuery = callbackQuery.Encode()
	// #nosec G710 -- OAuth endpoints redirect to validated client callback URIs by contract.
	http.Redirect(writer, request, redirectURL.String(), http.StatusFound)
}

func (s *OAuthHTTPServer) serveToken(writer http.ResponseWriter, request *http.Request, endpoints OAuthEndpoints) {
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "parse token request", http.StatusBadRequest)
		return
	}
	values := cloneValues(request.PostForm)
	s.recordToken(values)
	switch values.Get("grant_type") {
	case authorizationCodeGrant:
		if values.Get(authorizationCodeParam) != AuthorizationCode || values.Get("code_verifier") == "" ||
			values.Get(resourceParameter) != endpoints.MCPURL {
			http.Error(writer, "invalid authorization code request", http.StatusBadRequest)
			return
		}
	case refreshTokenGrant:
		if values.Get("refresh_token") != RefreshToken || values.Get(resourceParameter) != endpoints.MCPURL {
			http.Error(writer, "invalid refresh request", http.StatusBadRequest)
			return
		}
	default:
		http.Error(writer, "unsupported grant_type", http.StatusBadRequest)
		return
	}
	s.writeJSON(writer, http.StatusOK, map[string]any{
		"access_token":    AccessToken,
		refreshTokenGrant: RefreshToken,
		"token_type":      "Bearer",
		"expires_in":      3600,
		"scope":           strings.Join(s.config.Scopes, " "),
	})
}

func (s *OAuthHTTPServer) serveRegistration(
	writer http.ResponseWriter,
	request *http.Request,
	endpoints OAuthEndpoints,
) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var registration map[string]any
	if err := json.NewDecoder(request.Body).Decode(&registration); err != nil {
		http.Error(writer, "decode registration", http.StatusBadRequest)
		return
	}
	s.recordRegistration(registration)
	s.writeJSON(writer, http.StatusCreated, map[string]any{
		"client_id":                  "mcpfixture-client-id",     // #nosec G101 -- deterministic test-fixture value.
		"client_secret":              "mcpfixture-client-secret", // #nosec G101 -- deterministic test-fixture value.
		"registration_client_uri":    endpoints.IssuerURL + "/registration/mcpfixture-client-id",
		"registration_access_token":  registrationAccessToken,
		"token_endpoint_auth_method": "client_secret_post",
	})
}

func (s *OAuthHTTPServer) writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return
	}
}

func (s *OAuthHTTPServer) recordAuthorization(values url.Values) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests.Authorization = append(s.requests.Authorization, cloneValues(values))
}

func (s *OAuthHTTPServer) recordToken(values url.Values) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests.Token = append(s.requests.Token, cloneValues(values))
}

func (s *OAuthHTTPServer) recordRegistration(registration map[string]any) {
	data, err := json.Marshal(registration)
	if err != nil {
		return
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests.Registration = append(s.requests.Registration, cloned)
}

func cloneValuesSlice(values []url.Values) []url.Values {
	cloned := make([]url.Values, len(values))
	for index, value := range values {
		cloned[index] = cloneValues(value)
	}
	return cloned
}

func cloneRegistrationSlice(values []map[string]any) []map[string]any {
	cloned := make([]map[string]any, 0, len(values))
	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		cloned = append(cloned, entry)
	}
	return cloned
}

func requestBaseURL(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + request.Host
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}
