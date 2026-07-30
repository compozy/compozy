// Suite: MCP fixture OAuth surface
// Invariant: the OAuth fixture publishes consistent PRM, metadata, authorization, token, DCR, mix-up, refresh, and step-up contracts around one MCP resource.
// Boundary IN: mcpfixture's local HTTP OAuth authority and request observations.
// Boundary OUT: OAuth policy, persistence, and redaction behavior owned by internal/mcp/auth.
package mcpfixture

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestOAuthHTTPServer(t *testing.T) {
	t.Run("Should publish OAuth discovery and complete deterministic authorization lifecycle", func(t *testing.T) {
		t.Parallel()

		server := StartOAuthHTTP(t, MustNew(ProfileModern2026), OAuthConfig{
			Scopes: []string{"tools.read", "tools.write"},
			AuthorizationResponseIssParameterSupported: true,
		})
		client := noRedirectClient()

		protectedResponse := performRequest(t, client, http.MethodGet, server.Endpoints.MCPURL, nil)
		defer closeResponse(t, protectedResponse)
		if got, want := protectedResponse.StatusCode, http.StatusUnauthorized; got != want {
			t.Fatalf("protected resource status = %d, want %d", got, want)
		}
		challenge := protectedResponse.Header.Get("WWW-Authenticate")
		if !strings.Contains(challenge, server.Endpoints.ProtectedResourceURL) {
			t.Fatalf("protected resource challenge = %q, want resource metadata URL", challenge)
		}

		metadataResponse := performRequest(t, client, http.MethodGet, server.Endpoints.ProtectedResourceURL, nil)
		defer closeResponse(t, metadataResponse)
		if got, want := metadataResponse.StatusCode, http.StatusOK; got != want {
			t.Fatalf("protected resource metadata status = %d, want %d", got, want)
		}
		var protectedMetadata struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
			ScopesSupported      []string `json:"scopes_supported"`
		}
		decodeJSON(t, metadataResponse, &protectedMetadata)
		if got, want := protectedMetadata.Resource, server.Endpoints.MCPURL; got != want {
			t.Fatalf("PRM resource = %q, want %q", got, want)
		}
		if got, want := protectedMetadata.ScopesSupported, []string{
			"tools.read",
			"tools.write",
		}; !slices.Equal(got, want) {
			t.Fatalf("PRM scopes = %#v, want %#v", got, want)
		}
		if len(protectedMetadata.AuthorizationServers) == 0 {
			t.Fatal("PRM authorization servers are empty")
		}
		if got, want := protectedMetadata.AuthorizationServers[0], server.Endpoints.IssuerURL; got != want {
			t.Fatalf("PRM authorization server = %q, want %q", got, want)
		}

		authorizeURL, err := url.Parse(server.Endpoints.AuthorizationURL)
		if err != nil {
			t.Fatalf("url.Parse(authorization endpoint) error = %v", err)
		}
		authorizeQuery := authorizeURL.Query()
		authorizeQuery.Set("state", "fixture-state")
		authorizeQuery.Set("code_challenge", "fixture-challenge")
		authorizeQuery.Set("code_challenge_method", "S256")
		authorizeQuery.Set("resource", server.Endpoints.MCPURL)
		authorizeQuery.Set("scope", "tools.read")
		authorizeQuery.Set("redirect_uri", "http://127.0.0.1:45454/callback")
		authorizeURL.RawQuery = authorizeQuery.Encode()
		authorizeResponse := performRequest(t, client, http.MethodGet, authorizeURL.String(), nil)
		defer closeResponse(t, authorizeResponse)
		if got, want := authorizeResponse.StatusCode, http.StatusFound; got != want {
			t.Fatalf("authorize status = %d, want %d", got, want)
		}
		callbackURL, err := url.Parse(authorizeResponse.Header.Get("Location"))
		if err != nil {
			t.Fatalf("url.Parse(callback location) error = %v", err)
		}
		if got, want := callbackURL.Query().Get("code"), AuthorizationCode; got != want {
			t.Fatalf("callback code = %q, want %q", got, want)
		}
		if got, want := callbackURL.Query().Get("state"), "fixture-state"; got != want {
			t.Fatalf("callback state = %q, want %q", got, want)
		}
		if got, want := callbackURL.Query().Get("iss"), server.Endpoints.IssuerURL; got != want {
			t.Fatalf("callback iss = %q, want %q", got, want)
		}

		tokenResponse := postForm(t, client, server.Endpoints.TokenURL, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {AuthorizationCode},
			"code_verifier": {"fixture-verifier"},
			"resource":      {server.Endpoints.MCPURL},
		})
		defer closeResponse(t, tokenResponse)
		if got, want := tokenResponse.StatusCode, http.StatusOK; got != want {
			t.Fatalf("authorization-code token status = %d, want %d", got, want)
		}
		var token struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		decodeJSON(t, tokenResponse, &token)
		if got, want := token.AccessToken, AccessToken; got != want {
			t.Fatalf("access token = %q, want %q", got, want)
		}
		if got, want := token.RefreshToken, RefreshToken; got != want {
			t.Fatalf("refresh token = %q, want %q", got, want)
		}

		refreshResponse := postForm(t, client, server.Endpoints.TokenURL, url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {RefreshToken},
			"resource":      {server.Endpoints.MCPURL},
		})
		defer closeResponse(t, refreshResponse)
		if got, want := refreshResponse.StatusCode, http.StatusOK; got != want {
			t.Fatalf("refresh token status = %d, want %d", got, want)
		}

		registrationResponse := postJSON(t, client, server.Endpoints.RegistrationURL, map[string]any{
			"redirect_uris": []string{"http://127.0.0.1:45454/callback"},
		})
		defer closeResponse(t, registrationResponse)
		if got, want := registrationResponse.StatusCode, http.StatusCreated; got != want {
			t.Fatalf("registration status = %d, want %d", got, want)
		}
		var registration struct {
			ClientID                string `json:"client_id"`
			RegistrationClientURI   string `json:"registration_client_uri"`
			RegistrationAccessToken string `json:"registration_access_token"`
		}
		decodeJSON(t, registrationResponse, &registration)
		if registration.ClientID == "" || registration.RegistrationClientURI == "" ||
			registration.RegistrationAccessToken == "" {
			t.Fatalf("registration response = %#v, want durable-registration fields", registration)
		}

		requests := server.Requests()
		if got, want := len(requests.Authorization), 1; got != want {
			t.Fatalf("authorization request count = %d, want %d", got, want)
		}
		if got, want := requests.Authorization[0].Get("scope"), "tools.read"; got != want {
			t.Fatalf("authorization scope = %q, want %q", got, want)
		}
		if got, want := len(requests.Token), 2; got != want {
			t.Fatalf("token request count = %d, want %d", got, want)
		}
		if got, want := len(requests.Registration), 1; got != want {
			t.Fatalf("registration request count = %d, want %d", got, want)
		}
	})

	t.Run("Should expose mix-up and step-up contracts", func(t *testing.T) {
		t.Parallel()

		server := StartOAuthHTTP(t, MustNew(ProfileModern2026), OAuthConfig{
			StepUpScope:    "tools.write",
			CallbackIssuer: "https://unexpected-issuer.example",
			AuthorizationResponseIssParameterSupported: true,
			ClientIDMetadataDocumentSupported:          true,
		})
		client := noRedirectClient()

		metadataResponse := performRequest(
			t,
			client,
			http.MethodGet,
			server.Endpoints.IssuerURL+"/.well-known/oauth-authorization-server",
			nil,
		)
		defer closeResponse(t, metadataResponse)
		if got, want := metadataResponse.StatusCode, http.StatusOK; got != want {
			t.Fatalf("authorization metadata status = %d, want %d", got, want)
		}
		var metadata struct {
			CIMDSupported      bool `json:"client_id_metadata_document_supported"`
			IssuerParamSupport bool `json:"authorization_response_iss_parameter_supported"`
		}
		decodeJSON(t, metadataResponse, &metadata)
		if !metadata.CIMDSupported || !metadata.IssuerParamSupport {
			t.Fatalf("authorization metadata = %#v, want CIMD and issuer support", metadata)
		}

		stepUpRequest, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			server.Endpoints.MCPURL,
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("http.NewRequest(step-up) error = %v", err)
		}
		stepUpRequest.Header.Set("Authorization", "Bearer "+AccessToken)
		stepUpResponse, err := client.Do(stepUpRequest)
		if err != nil {
			t.Fatalf("step-up request error = %v", err)
		}
		defer closeResponse(t, stepUpResponse)
		if got, want := stepUpResponse.StatusCode, http.StatusUnauthorized; got != want {
			t.Fatalf("step-up status = %d, want %d", got, want)
		}
		challenge := stepUpResponse.Header.Get("WWW-Authenticate")
		if !strings.Contains(challenge, "insufficient_scope") || !strings.Contains(challenge, "tools.write") {
			t.Fatalf("step-up challenge = %q, want insufficient scope and requested scope", challenge)
		}

		authorizeURL, err := url.Parse(server.Endpoints.AuthorizationURL)
		if err != nil {
			t.Fatalf("url.Parse(authorization endpoint) error = %v", err)
		}
		query := authorizeURL.Query()
		query.Set("state", "mix-up-state")
		query.Set("code_challenge", "fixture-challenge")
		query.Set("resource", server.Endpoints.MCPURL)
		query.Set("redirect_uri", "http://127.0.0.1:45454/callback")
		authorizeURL.RawQuery = query.Encode()
		response := performRequest(t, client, http.MethodGet, authorizeURL.String(), nil)
		defer closeResponse(t, response)
		callbackURL, err := url.Parse(response.Header.Get("Location"))
		if err != nil {
			t.Fatalf("url.Parse(mix-up callback) error = %v", err)
		}
		if got, want := callbackURL.Query().Get("iss"), "https://unexpected-issuer.example"; got != want {
			t.Fatalf("mix-up callback issuer = %q, want %q", got, want)
		}
	})
}

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       5 * time.Second,
	}
}

func performRequest(t *testing.T, client *http.Client, method string, rawURL string, body io.Reader) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, rawURL, body)
	if err != nil {
		t.Fatalf("http.NewRequest(%s) error = %v", method, err)
	}
	return executeRequest(t, client, request)
}

func executeRequest(t *testing.T, client *http.Client, request *http.Request) *http.Response {
	t.Helper()
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP %s %s error = %v", request.Method, request.URL, err)
	}
	return response
}

func postForm(t *testing.T, client *http.Client, rawURL string, values url.Values) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequest(form post) error = %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return executeRequest(t, client, request)
}

func postJSON(t *testing.T, client *http.Client, rawURL string, value any) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("http.NewRequest(JSON post) error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	return executeRequest(t, client, request)
}

func decodeJSON(t *testing.T, response *http.Response, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("json.Decode(response body) error = %v", err)
	}
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Errorf("response body close error = %v", err)
	}
}
