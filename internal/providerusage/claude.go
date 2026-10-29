package providerusage

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// KeychainSupported reports whether the daemon host has the Claude Keychain source.
func KeychainSupported() bool { return runtime.GOOS == "darwin" }

const claudeProviderID = "claude"

type KeychainReader interface {
	// Background reads reject known prompting states and bound any check/read race.
	// Explicit reads may request OS authorization.
	Read(ctx context.Context, service, account string, interactive bool) ([]byte, error)
}

type ClaudeOption func(*ClaudeCollector)

type ClaudeCollector struct {
	mu       sync.Mutex
	now      func() time.Time
	http     *HTTPClient
	keychain KeychainReader
	platform string
	records  map[string]*claudeCredential
	pending  map[string]*claudeRead
	closed   bool
}

var _ Collector = (*ClaudeCollector)(nil)
var _ io.Closer = (*ClaudeCollector)(nil)

func NewClaudeCollector(options ...ClaudeOption) *ClaudeCollector {
	c := &ClaudeCollector{
		now:      time.Now,
		keychain: defaultClaudeKeychain(),
		platform: runtime.GOOS,
		records:  make(map[string]*claudeCredential),
		pending:  make(map[string]*claudeRead),
	}
	for _, option := range options {
		option(c)
	}
	if c.http == nil {
		c.http = newClaudeHTTP("https://api.anthropic.com", nil, c.now)
	}
	return c
}

func WithClaudeHTTP(baseURL string, transport http.RoundTripper) ClaudeOption {
	return func(c *ClaudeCollector) {
		c.http = newClaudeHTTP(baseURL, transport, func() time.Time { return c.now() })
	}
}
func WithClaudeClock(now func() time.Time) ClaudeOption {
	return func(c *ClaudeCollector) { c.now = now }
}
func WithClaudeKeychain(reader KeychainReader) ClaudeOption {
	return func(c *ClaudeCollector) { c.keychain = reader }
}

// WithClaudePlatform overrides the host platform for collectors with an injected Keychain reader.
func WithClaudePlatform(platform string) ClaudeOption {
	return func(c *ClaudeCollector) { c.platform = platform }
}
func (*ClaudeCollector) ProviderID() string { return claudeProviderID }

func (c *ClaudeCollector) ClearCredentials(profileID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if profileID == "" {
		for profile := range c.records {
			c.clearLocked(profile)
		}
		for profile := range c.pending {
			c.clearLocked(profile)
		}
		return
	}
	c.clearLocked(profileID)
}
func (c *ClaudeCollector) clearLocked(profileID string) {
	if pending := c.pending[profileID]; pending != nil {
		pending.cancel()
		delete(c.pending, profileID)
	}
	if record := c.records[profileID]; record != nil {
		clear(record.token)
		record.token = nil
	}
	delete(c.records, profileID)
}
func (c *ClaudeCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for profile := range c.records {
		c.clearLocked(profile)
	}
	for profile := range c.pending {
		c.clearLocked(profile)
	}
	return nil
}

func (c *ClaudeCollector) Identity(ctx context.Context, target Target, intent Intent) (CredentialIdentity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return CredentialIdentity{}, NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	record, err := c.resolve(ctx, target, intent)
	if record == nil {
		return CredentialIdentity{}, err
	}
	return record.identity, err
}

func (c *ClaudeCollector) Collect(
	ctx context.Context,
	target Target,
	_ Intent,
	identity CredentialIdentity,
) (Snapshot, error) {
	c.mu.Lock()
	record, err := c.forCollection(target, identity)
	if err != nil {
		c.mu.Unlock()
		return Snapshot{}, err
	}
	token := append([]byte(nil), record.token...)
	plan := record.plan
	c.mu.Unlock()
	defer clear(token)
	data, err := c.http.Do(ctx, http.MethodGet, "/api/oauth/usage?cedar_ember=1", http.Header{
		headerAuthorization: {
			"Bearer " + string(token),
		},
		headerAccept:     {collectorJSONType},
		"Content-Type":   {collectorJSONType},
		"Anthropic-Beta": {"oauth-2025-04-20"},
		"User-Agent":     {"claude-cli/2.1.0 (external, cli)"},
	}, nil, "Claude")
	c.mu.Lock()
	defer c.mu.Unlock()
	if validation := c.validateLocked(target, identity); validation != nil {
		return Snapshot{}, validation
	}
	if !c.now().Before(record.expires) {
		record.expire()
		return Snapshot{}, claudeExpiryError()
	}
	if record.singleUse {
		clear(record.token)
		record.token = nil
	}
	if err != nil {
		code := AsUsageError(err, "Claude").Code
		if code == ErrorCodeAuthMissing || code == ErrorCodeAuthExpired {
			// Claude may replace a revoked token before its recorded expiry.
			record.expire()
		}
		return Snapshot{}, err
	}
	snapshot, err := decodeClaudeUsage(data)
	if err != nil {
		return Snapshot{}, err
	}
	now := c.now()
	snapshot.ProviderID, snapshot.ProviderName = "claude", target.ProviderName
	snapshot.Status, snapshot.Confidence = StatusOK, ConfidencePercentOnly
	snapshot.Source = &Source{Kind: SourceKindOAuthHTTP, Label: "Claude Code OAuth"}
	snapshot.Account = &Account{Plan: plan}
	snapshot.MeasuredAt, snapshot.CheckedAt = &now, &now
	return snapshot, nil
}

// ValidateIdentity never reads the Keychain, including for an explicit one-shot collection.
func (c *ClaudeCollector) ValidateIdentity(_ context.Context, target Target, identity CredentialIdentity) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.validateLocked(target, identity)
}

func (c *ClaudeCollector) forCollection(
	target Target,
	identity CredentialIdentity,
) (*claudeCredential, error) {
	if err := c.validateLocked(target, identity); err != nil {
		return nil, err
	}
	record := c.records[target.ProfileID]
	if record.expired || !c.now().Before(record.expires) || len(record.token) == 0 {
		record.expire()
		return nil, claudeExpiryError()
	}
	return record, nil
}

func (c *ClaudeCollector) validateLocked(target Target, identity CredentialIdentity) error {
	record := c.records[target.ProfileID]
	if c.closed || record == nil || record.identity != identity {
		return NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	if record.route != target.RouteFingerprint {
		c.clearLocked(target.ProfileID)
		if record.keychain {
			return NewUsageError(ErrorCodeKeychainTokenExpired, "Claude")
		}
		return NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	if record.keychain {
		path, account := claudeAccountEvidence(target)
		if !target.Route.KeychainAccess || path != record.evidencePath || account != record.evidenceAccount {
			c.clearLocked(target.ProfileID)
			return NewUsageError(ErrorCodeKeychainTokenExpired, "Claude")
		}
		// A newly created credential file changes the authoritative source, even for the same account.
		fileData, fileErr := readCredentialFile(claudeCredentialPath(target), "Claude")
		clear(fileData)
		if !isClaudeFileAbsent(fileErr) {
			c.clearLocked(target.ProfileID)
			return NewUsageError(ErrorCodeKeychainTokenExpired, "Claude")
		}
		return nil
	}
	fresh, err := loadClaudeFile(target, c.now())
	if fresh == nil {
		c.clearLocked(target.ProfileID)
		return err
	}
	defer clear(fresh.token)
	if fresh.identity != identity {
		c.clearLocked(target.ProfileID)
		return NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	return nil
}
