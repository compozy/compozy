package providerusage

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	agyProviderID = "agy"
	agyLabel      = "Antigravity"
	// The Antigravity CLI stores its Google login through go-keyring under this generic-password item.
	agyKeychainService = "gemini"
	agyKeychainAccount = "antigravity"
	agyKeyringPrefix   = "go-keyring-base64:"
	// The Code Assist backend serves quota only to Antigravity clients; the suffix names this reader.
	agyUserAgent = "antigravity compozy-provider-usage"
)

type AgyOption func(*AgyCollector)

// AgyCollector reads the Antigravity CLI quota summary with the CLI's own Keychain login.
// It never refreshes or writes that login: an expired token waits for the CLI to renew it.
type AgyCollector struct {
	collectorHTTP
	keychain KeychainReader
	platform string
}

var _ Collector = (*AgyCollector)(nil)

func NewAgyCollector(options []CollectorOption, agyOptions ...AgyOption) *AgyCollector {
	c := &AgyCollector{
		collectorHTTP: newCollectorHTTP("https://cloudcode-pa.googleapis.com", options),
		keychain:      defaultClaudeKeychain(),
		platform:      runtime.GOOS,
	}
	for _, option := range agyOptions {
		option(c)
	}
	return c
}

func WithAgyKeychain(reader KeychainReader) AgyOption {
	return func(c *AgyCollector) {
		if reader != nil {
			c.keychain = reader
		}
	}
}

// WithAgyPlatform overrides the host platform for collectors with an injected Keychain reader.
func WithAgyPlatform(platform string) AgyOption {
	return func(c *AgyCollector) { c.platform = platform }
}

func (*AgyCollector) ProviderID() string { return agyProviderID }

type agyCredential struct{ token, email string }

func (c agyCredential) identity() CredentialIdentity {
	return NewCredentialIdentity(SourceKindOAuthHTTP, []byte(c.token), c.email)
}

// credential reads the one machine-wide Keychain login; aliases cannot select another account.
func (c *AgyCollector) credential(ctx context.Context, t Target, intent Intent) (agyCredential, error) {
	if t.ProviderID != agyProviderID {
		usage := NewUsageError(ErrorCodeNoCollector, t.ProviderID)
		usage.Message = "Usage is available only for the agy provider, not its aliases"
		return agyCredential{}, usage
	}
	if c.platform != "darwin" {
		usage := NewUsageError(ErrorCodeNoCollector, agyProviderID)
		usage.Message = "agy usage is available only on macOS"
		return agyCredential{}, usage
	}
	if t.Route.NativeCommand == "" {
		usage := NewUsageError(ErrorCodeNoCollector, agyProviderID)
		usage.Message = "Install the Antigravity CLI (agy) on the daemon host to read usage"
		return agyCredential{}, usage
	}
	if !t.Route.KeychainAccess {
		return agyCredential{}, NewUsageError(ErrorCodeKeychainConsentRequired, agyProviderID)
	}
	data, err := c.keychain.Read(ctx, agyKeychainService, agyKeychainAccount, intent == IntentExplicit)
	defer clear(data)
	if err != nil {
		return agyCredential{}, agyKeychainError(err)
	}
	return parseAgyCredential(data, c.now())
}

func agyKeychainError(err error) *UsageError {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return NewUsageError(ErrorCodeAuthMissing, agyProviderID)
	case errors.Is(err, os.ErrPermission):
		return NewUsageError(ErrorCodeKeychainDenied, agyProviderID)
	}
	if usage, ok := errors.AsType[*UsageError](err); ok && usage.Code == ErrorCodeKeychainReadTimeout {
		return usage
	}
	return NewUsageError(ErrorCodeProviderUnavailable, agyProviderID)
}

func parseAgyCredential(data []byte, now time.Time) (agyCredential, error) {
	payload := bytes.TrimSpace(data)
	if encoded, ok := bytes.CutPrefix(payload, []byte(agyKeyringPrefix)); ok {
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
		n, err := base64.StdEncoding.Decode(decoded, encoded)
		defer clear(decoded)
		if err != nil {
			return agyCredential{}, NewUsageError(ErrorCodeParseError, agyProviderID)
		}
		payload = decoded[:n]
	}
	var stored struct {
		Token struct {
			Access string `json:"access_token"`
			Expiry string `json:"expiry"`
		} `json:"token"`
		Email string `json:"id_token_email"`
	}
	if err := json.Unmarshal(payload, &stored); err != nil {
		return agyCredential{}, NewUsageError(ErrorCodeParseError, agyProviderID)
	}
	if stored.Token.Access == "" {
		return agyCredential{}, NewUsageError(ErrorCodeAuthMissing, agyProviderID)
	}
	if expiry, err := time.Parse(time.RFC3339Nano, stored.Token.Expiry); err != nil ||
		expiredCollectorToken(expiry.Unix(), now) {
		return agyCredential{}, NewUsageError(ErrorCodeAuthExpired, agyProviderID)
	}
	return agyCredential{token: stored.Token.Access, email: stored.Email}, nil
}

func (c *AgyCollector) Identity(ctx context.Context, t Target, intent Intent) (CredentialIdentity, error) {
	value, err := c.credential(ctx, t, intent)
	if err != nil {
		return CredentialIdentity{}, err
	}
	return value.identity(), nil
}

func (c *AgyCollector) post(ctx context.Context, path, token, body string, out any) error {
	headers := bearerHeaders(token)
	headers.Set("User-Agent", agyUserAgent)
	data, err := c.client().Do(ctx, http.MethodPost, path, headers, strings.NewReader(body), agyProviderID)
	if err != nil {
		return err
	}
	return decodeCollectorJSON(data, out, agyLabel)
}

func (c *AgyCollector) Collect(
	ctx context.Context,
	t Target,
	intent Intent,
	cred CredentialIdentity,
) (Snapshot, error) {
	value, err := c.credential(ctx, t, intent)
	if err != nil {
		return Snapshot{}, err
	}
	if !value.identity().Same(cred) {
		return Snapshot{}, NewUsageError(ErrorCodeAuthMissing, agyProviderID)
	}
	snapshot := c.snapshot(t, Source{Kind: SourceKindOAuthHTTP, Label: "Antigravity CLI OAuth"})
	snapshot.Account = &Account{Label: value.email, Plan: c.plan(ctx, value.token)}
	var summary agyQuotaSummary
	err = c.post(ctx, "/v1internal:retrieveUserQuotaSummary", value.token, "{}", &summary)
	var windows []Window
	if err == nil {
		windows, err = summary.windows()
	}
	if err != nil {
		// The account and plan stay as fenced partial data; no quota was measured.
		usage := AsUsageError(err, agyLabel)
		if usage.Code == ErrorCodeAuthScope {
			usage = NewUsageError(ErrorCodeProviderUnavailable, agyLabel)
		}
		snapshot.Status, snapshot.Error, snapshot.MeasuredAt = usage.Status(), usage, nil
		return snapshot, usage
	}
	snapshot.Windows = windows
	snapshot.Confidence = ConfidencePercentOnly
	return snapshot, nil
}

// plan is display-only: a failed tier lookup leaves it empty while the quota summary decides status.
func (c *AgyCollector) plan(ctx context.Context, token string) string {
	var assist struct {
		Current *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"currentTier"`
		Paid *struct {
			Name string `json:"name"`
		} `json:"paidTier"`
	}
	body := `{"metadata":{"ideType":"ANTIGRAVITY","pluginType":"GEMINI"}}`
	if c.post(ctx, "/v1internal:loadCodeAssist", token, body, &assist) != nil {
		return ""
	}
	if assist.Paid != nil && assist.Paid.Name != "" {
		return assist.Paid.Name
	}
	if assist.Current != nil {
		return cmp.Or(assist.Current.Name, assist.Current.ID)
	}
	return ""
}

type agyQuotaBucket struct {
	ID        string   `json:"bucketId"`
	Name      string   `json:"displayName"`
	Window    string   `json:"window"`
	Remaining *float64 `json:"remainingFraction"`
	Disabled  bool     `json:"disabled"`
	Reset     string   `json:"resetTime"`
}

type agyQuotaSummary struct {
	Buckets []agyQuotaBucket `json:"buckets"`
	Groups  []struct {
		Name    string           `json:"displayName"`
		Buckets []agyQuotaBucket `json:"buckets"`
	} `json:"groups"`
}

// windows maps each enabled bucket to one window; grouped buckets carry their group's name.
func (summary agyQuotaSummary) windows() ([]Window, error) {
	var result []Window
	seen := map[string]struct{}{}
	add := func(group string, bucket agyQuotaBucket) error {
		if bucket.Disabled {
			return nil
		}
		window, err := agyWindow(group, bucket)
		if err != nil {
			return err
		}
		if _, duplicate := seen[window.ID]; duplicate {
			return NewUsageError(ErrorCodeParseError, agyLabel)
		}
		seen[window.ID] = struct{}{}
		result = append(result, window)
		return nil
	}
	for _, bucket := range summary.Buckets {
		if err := add("", bucket); err != nil {
			return nil, err
		}
	}
	for _, group := range summary.Groups {
		for _, bucket := range group.Buckets {
			if err := add(group.Name, bucket); err != nil {
				return nil, err
			}
		}
	}
	if len(result) == 0 {
		return nil, NewUsageError(ErrorCodeParseError, agyLabel)
	}
	slices.SortFunc(result, func(a, b Window) int { return cmp.Compare(a.ID, b.ID) })
	return result, nil
}

func agyWindow(group string, bucket agyQuotaBucket) (Window, error) {
	slug := strings.Trim(collectorModelSlug.ReplaceAllString(strings.ToLower(bucket.ID), "-"), "-")
	if slug == "" {
		return Window{}, NewUsageError(ErrorCodeParseError, agyLabel)
	}
	// Known durations use the shared Session/Daily/Weekly labels; others keep the provider's name.
	window := Window{Kind: WindowKindOther, Label: cmp.Or(bucket.Name, bucket.ID)}
	if seconds, ok := agyWindowSeconds[strings.ToLower(bucket.Window)]; ok {
		window = collectorWindow(seconds)
	}
	window.ID = "bucket:" + slug
	window.ResetsAt = collectorReset(bucket.Reset)
	if group != "" {
		window.Label = group + " · " + window.Label
	}
	if bucket.Remaining == nil || !validCollectorPercent(*bucket.Remaining) || *bucket.Remaining > 1 {
		return Window{}, NewUsageError(ErrorCodeParseError, agyLabel)
	}
	window.UsedPercent = new((1 - *bucket.Remaining) * 100)
	return window, nil
}

var agyWindowSeconds = map[string]int64{"5h": 18000, "daily": 86400, "1d": 86400, "weekly": 604800, "7d": 604800}
