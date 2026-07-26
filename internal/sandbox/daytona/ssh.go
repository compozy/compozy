package daytona

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"
	"os"

	"strconv"

	"sync"
	"time"
)

const (
	defaultSSHPort            = "22"
	defaultSSHAccessExpiresIn = time.Hour
	defaultSSHKeepAlive       = 30 * time.Second
	defaultSSHDialTimeout     = 30 * time.Second
)

type sshAccess struct {
	Token     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type sshTokenSource interface {
	FetchSSHAccess(ctx context.Context, apiURL string, sandboxID string, expiresIn time.Duration) (sshAccess, error)
}

type restSSHTokenSource struct {
	httpClient *http.Client
	apiKey     func() string
	now        func() time.Time
}

func newRESTSSHTokenSource(now func() time.Time) sshTokenSource {
	if now == nil {
		now = time.Now
	}
	return &restSSHTokenSource{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     func() string { return os.Getenv("DAYTONA_API_KEY") },
		now:        now,
	}
}

func (s *restSSHTokenSource) FetchSSHAccess(
	ctx context.Context,
	apiURL string,
	sandboxID string,
	expiresIn time.Duration,
) (_ sshAccess, err error) {
	key := ""
	if s.apiKey != nil {
		key = s.apiKey()
	}
	if key == "" {
		return sshAccess{}, errors.New("sandbox/daytona: DAYTONA_API_KEY is required for SSH access")
	}
	endpoint, err := url.Parse(normalizeAPIURL(apiURL) + "/sandbox/" + url.PathEscape(sandboxID) + "/ssh-access")
	if err != nil {
		return sshAccess{}, fmt.Errorf("sandbox/daytona: build SSH access URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("expiresInMinutes", strconv.Itoa(int(expiresIn.Minutes())))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), http.NoBody)
	if err != nil {
		return sshAccess{}, fmt.Errorf("sandbox/daytona: build SSH access request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return sshAccess{}, fmt.Errorf("sandbox/daytona: fetch SSH access token: %w", err)
	}
	defer mergeHTTPResponseCloseError(&err, resp, "SSH access token")
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return sshAccess{}, fmt.Errorf(
				"sandbox/daytona: fetch SSH access token status %d and read error body: %w",
				resp.StatusCode,
				readErr,
			)
		}
		return sshAccess{}, fmt.Errorf(
			"sandbox/daytona: fetch SSH access token status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var raw struct {
		Token          string `json:"token"`
		ExpiresAt      string `json:"expiresAt"`
		ExpiresAtSnake string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return sshAccess{}, fmt.Errorf("sandbox/daytona: decode SSH access response: %w", err)
	}
	if raw.Token == "" {
		return sshAccess{}, errors.New("sandbox/daytona: SSH access response missing token")
	}
	now := s.now().UTC()
	expiresAt := now.Add(expiresIn)
	if raw.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339, raw.ExpiresAt); err == nil {
			expiresAt = parsed.UTC()
		}
	}
	if raw.ExpiresAtSnake != "" {
		if parsed, err := time.Parse(time.RFC3339, raw.ExpiresAtSnake); err == nil {
			expiresAt = parsed.UTC()
		}
	}
	return sshAccess{Token: raw.Token, IssuedAt: now, ExpiresAt: expiresAt}, nil
}

type sshTokenManager struct {
	source    sshTokenSource
	now       func() time.Time
	expiresIn time.Duration
	mu        sync.Mutex
	tokens    map[string]sshAccess
}

func newSSHTokenManager(source sshTokenSource, now func() time.Time) *sshTokenManager {
	if now == nil {
		now = time.Now
	}
	return &sshTokenManager{
		source:    source,
		now:       now,
		expiresIn: defaultSSHAccessExpiresIn,
		tokens:    make(map[string]sshAccess),
	}
}

func (m *sshTokenManager) Ensure(
	ctx context.Context,
	apiURL string,
	sandboxID string,
	force bool,
) (sshAccess, error) {
	if m == nil || m.source == nil {
		return sshAccess{}, errors.New("sandbox/daytona: SSH token manager is not configured")
	}
	key := tokenCacheKey(apiURL, sandboxID)
	m.mu.Lock()
	cached, ok := m.tokens[key]
	if ok && !force && !m.shouldRefresh(cached) {
		m.mu.Unlock()
		return cached, nil
	}
	m.mu.Unlock()

	access, err := m.source.FetchSSHAccess(ctx, apiURL, sandboxID, m.expiresIn)
	if err != nil {
		return sshAccess{}, err
	}
	m.mu.Lock()
	m.tokens[key] = access
	m.mu.Unlock()
	return access, nil
}

func (m *sshTokenManager) shouldRefresh(access sshAccess) bool {
	if access.Token == "" || access.ExpiresAt.IsZero() {
		return true
	}
	now := m.now().UTC()
	if !now.Before(access.ExpiresAt) {
		return true
	}
	issuedAt := access.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = access.ExpiresAt.Add(-m.expiresIn)
	}
	refreshAt := issuedAt.Add(access.ExpiresAt.Sub(issuedAt) / 2)
	return !now.Before(refreshAt)
}

func tokenCacheKey(apiURL string, sandboxID string) string {
	return normalizeAPIURL(apiURL) + "\x00" + sandboxID
}
