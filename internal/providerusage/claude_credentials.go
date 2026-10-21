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
	token        []byte
	expires      time.Time
	plan         string
	identity     CredentialIdentity
	route        string
	evidencePath string
	keychain     bool
	singleUse    bool
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

func (c *ClaudeCollector) resolve(ctx context.Context, target Target, intent Intent) (*claudeCredential, error) {
	record, err := loadClaudeFile(target, c.now())
	if !isClaudeFileAbsent(err) {
		c.clearLocked(target.ProfileID)
		if err != nil {
			return nil, err
		}
		c.records[target.ProfileID] = record
		return record, nil
	}
	if claudeCredentialPath(target) == "" || c.platform != "darwin" {
		c.clearLocked(target.ProfileID)
		return nil, NewUsageError(ErrorCodeAuthMissing, "Claude")
	}
	if !target.Route.KeychainAccess {
		c.clearLocked(target.ProfileID)
		return nil, NewUsageError(ErrorCodeKeychainConsentRequired, "Claude")
	}
	if intent != IntentExplicit {
		record = c.records[target.ProfileID]
		if record == nil {
			return nil, NewUsageError(ErrorCodeKeychainConsentRequired, "Claude")
		}
		if err := c.validateLocked(target, record.identity); err != nil {
			return nil, err
		}
		if record.singleUse {
			c.clearLocked(target.ProfileID)
			return nil, NewUsageError(ErrorCodeKeychainTokenExpired, "Claude")
		}
		return record, nil
	}
	c.clearLocked(target.ProfileID)
	path, account := claudeAccountEvidence(target)
	service, username := claudeKeychainItem(target)
	data, err := c.keychain.Read(ctx, service, username)
	defer clear(data)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil, NewUsageError(ErrorCodeAuthMissing, "claude")
		case errors.Is(err, os.ErrPermission):
			return nil, NewUsageError(ErrorCodeKeychainDenied, "Claude")
		default:
			return nil, AsUsageError(err, "Claude")
		}
	}
	record, err = parseClaudeCredential(data, c.now())
	if err != nil {
		return nil, err
	}
	record.keychain, record.singleUse = true, account == ""
	record.route, record.evidencePath = target.RouteFingerprint, path
	record.identity = claudeIdentity(record.token, account)
	c.records[target.ProfileID] = record
	if err := c.validateLocked(target, record.identity); err != nil {
		return nil, err
	}
	return record, nil
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
	if err != nil {
		return nil, err
	}
	path, account := claudeAccountEvidence(target)
	record.route, record.evidencePath = target.RouteFingerprint, path
	record.identity = claudeIdentity(record.token, account)
	return record, nil
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
	if !now.Before(expires) {
		return nil, NewUsageError(ErrorCodeAuthExpired, "Claude")
	}
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
	return &claudeCredential{token: []byte(oauth.AccessToken), expires: expires, plan: plan}, nil
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
func claudeConfigDir(target Target) string {
	dir, _ := routeEnvValue(target, "CLAUDE_CONFIG_DIR")
	if dir == "" {
		if home := claudeHome(target); home != "" {
			dir = filepath.Join(home, ".claude")
		}
	}
	return norm.NFC.String(dir)
}
func claudeCredentialPath(target Target) string {
	dir := claudeConfigDir(target)
	if override, set := routeEnvValue(target, "CLAUDE_SECURESTORAGE_CONFIG_DIR"); set {
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
func claudeAccountEvidence(target Target) (string, string) {
	dir := claudeConfigDir(target)
	if !filepath.IsAbs(dir) {
		return "", ""
	}
	legacy := filepath.Join(dir, ".config.json")
	modernDir, _ := routeEnvValue(target, "CLAUDE_CONFIG_DIR")
	if modernDir == "" {
		modernDir = claudeHome(target)
	}
	modern := filepath.Join(norm.NFC.String(modernDir), ".claude.json")
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
