package providerusage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

type claudeCredential struct {
	token           []byte
	expires         time.Time
	plan            string
	identity        CredentialIdentity
	route           string
	evidencePath    string
	evidenceAccount string
	expired         bool
	keychain        bool
	singleUse       bool
}

type claudeRead struct{ cancel context.CancelFunc }

func (r *claudeCredential) expire() { clear(r.token); r.token = nil; r.expired = true }

func claudeExpiryError() *UsageError {
	return NewUsageError(ErrorCodeAuthExpired, "Claude")
}

type claudeOAuthFile struct {
	OAuth *struct {
		AccessToken      string   `json:"accessToken"`
		ExpiresAt        int64    `json:"expiresAt"`
		Scopes           []string `json:"scopes"`
		SubscriptionType string   `json:"subscriptionType"`
		RateLimitTier    string   `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// resolve keeps a usable credential file authoritative. On macOS an unusable leftover file
// falls through to the unchanged consented Keychain path, so it cannot hide the live login.
func (c *ClaudeCollector) resolve(ctx context.Context, target Target, intent Intent) (*claudeCredential, error) {
	record, err := loadClaudeFile(target, c.now())
	fileAbsent := isClaudeFileAbsent(err)
	keychain := claudeCredentialPath(target) != "" && c.platform == "darwin"
	if !fileAbsent && (err == nil || !keychain) {
		c.clearLocked(claudeKeyOf(target))
		if record != nil {
			c.records[claudeKeyOf(target)] = record
		}
		return record, err
	}
	if !keychain {
		c.clearLocked(claudeKeyOf(target))
		return nil, NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	if !target.Route.KeychainAccess {
		c.clearLocked(claudeKeyOf(target))
		return nil, NewUsageError(ErrorCodeKeychainConsentRequired, "Claude")
	}
	missing := NewUsageError(ErrorCodeAuthMissing, "Claude")
	if !fileAbsent {
		// Without a Keychain login, the unusable file remains the accurate diagnosis.
		missing = AsUsageError(err, "Claude")
	}
	if intent != IntentExplicit {
		record = c.records[claudeKeyOf(target)]
		if record != nil && c.validateLocked(target, record.identity) == nil {
			if !record.singleUse && !record.expired && c.now().Before(record.expires) {
				return record, nil
			}
		}
	}
	return c.readKeychainLocked(ctx, target, intent, missing)
}

// readKeychainLocked releases the collector lock during OS access; the reader enforces refresh intent.
// missing is returned when the routed Keychain item does not exist.
func (c *ClaudeCollector) readKeychainLocked(
	ctx context.Context,
	target Target,
	intent Intent,
	missing *UsageError,
) (*claudeCredential, error) {
	c.clearLocked(claudeKeyOf(target))
	path, account := claudeAccountEvidence(target)
	service, username := claudeKeychainItem(target)
	readCtx, cancel := context.WithCancel(ctx)
	pending := &claudeRead{cancel: cancel}
	c.pending[claudeKeyOf(target)] = pending
	c.mu.Unlock()
	data, err := c.keychain.Read(readCtx, service, username, intent == IntentExplicit)
	c.mu.Lock()
	defer cancel()
	defer clear(data)
	if c.closed || c.pending[claudeKeyOf(target)] != pending {
		return nil, NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	delete(c.pending, claudeKeyOf(target))
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil, missing
		case errors.Is(err, os.ErrPermission):
			return nil, NewUsageError(ErrorCodeKeychainDenied, "Claude")
		default:
			return nil, AsUsageError(err, "Claude")
		}
	}
	record, err := parseClaudeCredential(data, c.now())
	if record == nil {
		return nil, err
	}
	record.keychain, record.singleUse = true, account == ""
	record.route, record.evidencePath, record.evidenceAccount = target.RouteFingerprint, path, account
	record.identity = claudeIdentity(record.token, account)
	if record.expired {
		record.expire()
		err = claudeExpiryError()
	}
	c.records[claudeKeyOf(target)] = record
	if err := c.validateLocked(target, record.identity); err != nil {
		return nil, err
	}
	return record, err
}

func loadClaudeFile(target Target, now time.Time) (*claudeCredential, error) {
	data, err := readCredentialFile(claudeCredentialPath(target), "Claude")
	defer clear(data)
	if err != nil {
		if isClaudeFileAbsent(err) {
			return nil, err
		}
		return nil, NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	record, err := parseClaudeCredential(data, now)
	if record == nil {
		return nil, err
	}
	path, account := claudeAccountEvidence(target)
	record.route, record.evidencePath, record.evidenceAccount = target.RouteFingerprint, path, account
	record.identity = claudeIdentity(record.token, account)
	if record.expired {
		record.expire()
	}
	return record, err
}

func parseClaudeCredential(data []byte, now time.Time) (*claudeCredential, error) {
	var file claudeOAuthFile
	if json.Unmarshal(data, &file) != nil {
		return nil, NewUsageError(ErrorCodeParseError, "Claude")
	}
	oauth := file.OAuth
	if oauth == nil || oauth.AccessToken == "" {
		return nil, NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	if !slices.Contains(oauth.Scopes, "user:profile") {
		return nil, NewUsageError(ErrorCodeAuthScope, "Claude")
	}
	expires := time.UnixMilli(oauth.ExpiresAt)
	plan := oauth.SubscriptionType
	if oauth.SubscriptionType == "max" {
		switch oauth.RateLimitTier {
		case "default_claude_max_20x":
			plan = "max_20x"
		case "default_claude_max_5x":
			plan = "max_5x"
		}
	}
	if plan == "" {
		plan = oauth.RateLimitTier
	}
	record := &claudeCredential{
		token:   []byte(oauth.AccessToken),
		expires: expires,
		plan:    plan,
		expired: !now.Before(expires),
	}
	if record.expired {
		return record, claudeExpiryError()
	}
	return record, nil
}

func claudeIdentity(token []byte, account string) CredentialIdentity {
	material := make([]byte, 0, len(token)+len(account)+1)
	material = append(material, token...)
	material = append(material, 0)
	material = append(material, account...)
	defer clear(material)
	return NewCredentialIdentity(SourceKindOAuthHTTP, material, account)
}

func claudeHome(target Target) string {
	home, _ := routeEnvValue(target, "HOME")
	if home == "" {
		home, _ = routeEnvValue(target, "USERPROFILE")
	}
	return home
}

const (
	claudeConfigDirKey     = "CLAUDE_CONFIG_DIR"
	claudeSecureStorageKey = "CLAUDE_SECURESTORAGE_CONFIG_DIR"
)

// SelectorKey implements AccountSelector: a present CLAUDE_SECURESTORAGE_CONFIG_DIR takes
// precedence over CLAUDE_CONFIG_DIR for the credential lookup.
func (*ClaudeCollector) SelectorKey(env []string) string { return claudeSelectorKey(env) }

func claudeSelectorKey(env []string) string {
	if _, set := envValue(env, claudeSecureStorageKey); set {
		return claudeSecureStorageKey
	}
	return claudeConfigDirKey
}
func claudeConfigDir(target Target) string {
	dir, _ := routeEnvValue(target, claudeConfigDirKey)
	if dir == "" {
		if home := claudeHome(target); home != "" {
			dir = filepath.Join(home, ".claude")
		}
	}
	return norm.NFC.String(dir)
}
func claudeCredentialPath(target Target) string {
	dir := claudeConfigDir(target)
	if claudeSelectorKey(target.Route.Env) == claudeSecureStorageKey {
		override, _ := routeEnvValue(target, claudeSecureStorageKey)
		dir = norm.NFC.String(override)
		if dir == "" && claudeHome(target) != "" {
			dir = filepath.Join(claudeHome(target), ".claude")
		}
	}
	if !filepath.IsAbs(dir) {
		return ""
	}
	return filepath.Join(dir, ".credentials.json")
}
func isClaudeFileAbsent(err error) bool { return errors.Is(err, os.ErrNotExist) }
func claudeAccountPaths(target Target) (string, string) {
	dir := claudeConfigDir(target)
	if !filepath.IsAbs(dir) {
		return "", ""
	}
	legacy := filepath.Join(dir, ".config.json")
	modernDir, _ := routeEnvValue(target, claudeConfigDirKey)
	if modernDir == "" {
		modernDir = claudeHome(target)
	}
	modern := filepath.Join(norm.NFC.String(modernDir), ".claude.json")
	return legacy, modern
}
func claudeAccountEvidence(target Target) (string, string) {
	legacy, modern := claudeAccountPaths(target)
	if legacy == "" {
		return "", ""
	}
	path := legacy
	if _, err := os.Stat(legacy); isClaudeFileAbsent(err) {
		path = modern
	}
	data, err := readCredentialFile(path, "Claude")
	defer clear(data)
	if err != nil {
		return path, ""
	}
	var config struct {
		OAuthAccount struct {
			AccountUUID string `json:"accountUuid"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(data, &config) != nil {
		return path, ""
	}
	return path, config.OAuthAccount.AccountUUID
}
func claudeKeychainItem(target Target) (string, string) {
	dir, _ := routeEnvValue(target, "CLAUDE_CONFIG_DIR")
	if override, set := routeEnvValue(target, "CLAUDE_SECURESTORAGE_CONFIG_DIR"); set {
		dir = override
	}
	service := "Claude Code-credentials"
	if dir != "" {
		digest := sha256.Sum256([]byte(norm.NFC.String(dir)))
		service += "-" + hex.EncodeToString(digest[:4])
	}
	account, _ := routeEnvValue(target, "USER")
	if account == "" {
		if current, err := user.Current(); err == nil {
			account = current.Username
		}
	}
	if account == "" || strings.ContainsFunc(account, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '_' && r != '-'
	}) {
		account = "claude-code-user"
	}
	return service, account
}
