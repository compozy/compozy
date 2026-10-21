package providerusage

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const codexProviderID = "codex"

type CodexOptions struct {
	BaseURL   string
	Transport http.RoundTripper
	Now       func() time.Time
}
type CodexCollector struct{ options CodexOptions }

var _ Collector = (*CodexCollector)(nil)

func NewCodexCollector(options CodexOptions) *CodexCollector {
	if options.Now == nil {
		options.Now = time.Now
	}
	return &CodexCollector{options: options}
}
func (*CodexCollector) ProviderID() string { return codexProviderID }

type codexAuth struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}
type codexConfig struct {
	Store   string `toml:"cli_auth_credentials_store"`
	BaseURL string `toml:"chatgpt_base_url"`
}

func codexHome(t Target) string {
	if home := routeEnv(t, "CODEX_HOME"); home != "" {
		return home
	}
	if home := routeEnv(t, "HOME"); home != "" {
		return filepath.Join(home, ".codex")
	}
	return ""
}
func loadCodex(t Target) (codexConfig, codexAuth, CredentialIdentity, error) {
	var config codexConfig
	var auth codexAuth
	home := codexHome(t)
	if home == "" {
		return config, auth, CredentialIdentity{}, NewUsageError(ErrorCodeAuthMissing, "Codex")
	}
	data, err := readCredentialFile(filepath.Join(home, "config.toml"), "Codex")
	if err != nil && !os.IsNotExist(err) {
		return config, auth, CredentialIdentity{}, NewUsageError(ErrorCodeParseError, "Codex")
	}
	if len(data) > 0 {
		if _, err := toml.Decode(string(data), &config); err != nil {
			return config, auth, CredentialIdentity{}, NewUsageError(ErrorCodeParseError, "Codex")
		}
	}
	if config.Store != "" && config.Store != "file" {
		return config, auth, CredentialIdentity{}, NewUsageError(ErrorCodeAuthStoreUnsupported, "Codex")
	}
	data, err = readCredentialFile(filepath.Join(home, "auth.json"), "Codex")
	if err != nil {
		return config, auth, CredentialIdentity{}, NewUsageError(ErrorCodeAuthMissing, "Codex")
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return config, auth, NewCredentialIdentity(
				SourceKindOAuthHTTP,
				data,
				"",
			), NewUsageError(
				ErrorCodeParseError,
				"Codex",
			)
	}
	identity := NewCredentialIdentity(SourceKindOAuthHTTP, data, auth.Tokens.AccountID)
	if auth.Tokens.AccessToken == "" {
		return config, auth, identity, NewUsageError(ErrorCodeAuthMissing, "Codex")
	}
	return config, auth, identity, nil
}
func (*CodexCollector) Identity(ctx context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	if err := ctx.Err(); err != nil {
		return CredentialIdentity{}, err
	}
	_, _, identity, err := loadCodex(t)
	return identity, err
}

func (c *CodexCollector) Collect(
	ctx context.Context,
	t Target,
	intent Intent,
	cred CredentialIdentity,
) (Snapshot, error) {
	config, auth, identity, err := loadCodex(t)
	if err != nil {
		return Snapshot{}, err
	}
	if !cred.Same(identity) {
		return Snapshot{}, NewUsageError(ErrorCodeAuthMissing, "Codex")
	}
	if t.Route.NativeCommand != "" {
		snapshot, rpcErr := collectCodexRPC(ctx, t)
		if rpcErr == nil {
			after, probeErr := c.Identity(ctx, t, intent)
			if probeErr != nil {
				return Snapshot{}, probeErr
			}
			if !cred.Same(after) {
				return Snapshot{}, NewUsageError(ErrorCodeAuthMissing, "Codex")
			}
			return snapshot, nil
		}
		if ctx.Err() != nil {
			return Snapshot{}, ctx.Err()
		}
		// The owner CLI may have renewed the file even if the quota RPC failed.
		config, auth, identity, err = loadCodex(t)
		if err != nil {
			return Snapshot{}, err
		}
		if !cred.Same(identity) {
			return Snapshot{}, NewUsageError(ErrorCodeAuthMissing, "Codex")
		}
	}
	if tokenExpired(auth.Tokens.AccessToken, c.options.Now()) {
		return Snapshot{}, NewUsageError(ErrorCodeAuthExpired, "Codex")
	}
	base := c.options.BaseURL
	if base == "" {
		base = config.BaseURL
	}
	if base == "" {
		base = "https://chatgpt.com/backend-api"
	}
	base = strings.TrimRight(base, "/")
	usagePath := "/api/codex/usage"
	if strings.HasSuffix(base, "/backend-api") {
		usagePath = "/wham/usage"
	}
	headers := http.Header{
		headerAuthorization: []string{"Bearer " + auth.Tokens.AccessToken},
		headerAccept:        []string{collectorJSONType},
	}
	if auth.Tokens.AccountID != "" {
		headers.Set("ChatGPT-Account-Id", auth.Tokens.AccountID)
	}
	data, err := NewHTTPClient(
		base,
		c.options.Transport,
		c.options.Now,
	).Do(ctx, http.MethodGet, usagePath, headers, nil, "Codex")
	if err != nil {
		return Snapshot{}, err
	}
	return decodeCodexHTTP(data)
}

func tokenExpired(token string, now time.Time) bool {
	claims, err := readCollectorClaims(token)
	return err != nil || claims.Expiry == 0 || !now.Add(5*time.Minute).Before(time.Unix(claims.Expiry, 0))
}
