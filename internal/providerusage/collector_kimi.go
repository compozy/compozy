package providerusage

import (
	"context"
	"net/http"
	"path/filepath"
	"time"
)

type KimiCollector struct {
	collectorHTTP
	cli bool
}

var _ Collector = (*KimiCollector)(nil)

func NewKimiCollector(options ...CollectorOption) *KimiCollector {
	return &KimiCollector{collectorHTTP: newCollectorHTTP("https://api.kimi.com", options)}
}
func NewKimiCLICollector(options ...CollectorOption) *KimiCollector {
	c := NewKimiCollector(options...)
	c.cli = true
	return c
}
func (c *KimiCollector) ProviderID() string {
	if c.cli {
		return "kimi-cli"
	}
	return "moonshot"
}
func (c *KimiCollector) credential(t Target) (collectorCredential, error) {
	home, err := collectorHomePath(t, "KIMI_CODE_HOME", ".kimi-code")
	if err != nil {
		return collectorCredential{}, err
	}
	var data struct {
		Access string   `json:"access_token"`
		Expiry *float64 `json:"expires_at"`
	}
	if err := readCollectorCredential(
		filepath.Join(home, "credentials", "kimi-code.json"),
		"Kimi Coding",
		&data,
	); err != nil {
		return collectorCredential{}, err
	}
	if data.Access == "" {
		return collectorCredential{}, NewUsageError(ErrorCodeAuthMissing, "Kimi Coding")
	}
	claims, claimsErr := readCollectorClaims(data.Access)
	expiry := claims.Expiry
	if data.Expiry != nil {
		expiry = int64(*data.Expiry)
	}
	if expiredCollectorToken(expiry, c.now()) {
		return collectorCredential{}, NewUsageError(ErrorCodeAuthExpired, "Kimi Coding")
	}
	credential := collectorCredential{token: data.Access}
	if claimsErr == nil {
		credential.account = claims.Subject
	}
	return credential, nil
}
func (c *KimiCollector) Identity(_ context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	if !c.cli {
		return keyIdentity(t)
	}
	credential, err := c.credential(t)
	if err != nil {
		return CredentialIdentity{}, err
	}
	return credential.identity(), nil
}

type kimiRatio struct {
	Used  *float64 `json:"used_ratio"`
	Reset string   `json:"reset_time"`
}
type kimiDetail struct {
	Limit     collectorDecimal `json:"limit"`
	Used      collectorDecimal `json:"used"`
	Remaining collectorDecimal `json:"remaining"`
	Reset     string           `json:"resetTime"`
}

func (c *KimiCollector) Collect(ctx context.Context, t Target, _ Intent, cred CredentialIdentity) (Snapshot, error) {
	token, kind, label, err := c.token(t, cred)
	if err != nil {
		return Snapshot{}, err
	}
	var response struct {
		Pools *struct {
			Session *kimiRatio `json:"limit_5h"`
			Weekly  *kimiRatio `json:"limit_7d"`
			Monthly *kimiRatio `json:"limit_month_total"`
		} `json:"usages"`
		Usage  *kimiDetail `json:"usage"`
		Limits []struct {
			Window struct {
				Duration int64  `json:"duration"`
				Unit     string `json:"timeUnit"`
			} `json:"window"`
			Detail kimiDetail `json:"detail"`
		} `json:"limits"`
		User struct {
			Membership struct {
				Level string `json:"level"`
			} `json:"membership"`
		} `json:"user"`
	}
	if err := c.getUsage(ctx, token, &response); err != nil {
		return Snapshot{}, err
	}
	snapshot := c.snapshot(t, Source{Kind: kind, Label: label})
	snapshot.ProviderName = "Kimi Coding"
	if response.User.Membership.Level != "" {
		snapshot.Account = &Account{Plan: response.User.Membership.Level}
	}
	if response.Pools != nil {
		for i, pool := range []*kimiRatio{response.Pools.Session, response.Pools.Weekly, response.Pools.Monthly} {
			if pool == nil {
				continue
			}
			if pool.Used == nil || !validCollectorPercent(*pool.Used) {
				return Snapshot{}, NewUsageError(ErrorCodeParseError, "Kimi Coding")
			}
			w := collectorWindow([]int64{18000, 604800, 0}[i])
			if i == 2 {
				w.ID = string(WindowKindMonthly)
				w.Label = "Monthly"
				w.Kind = WindowKindMonthly
			}
			percent := *pool.Used * 100
			w.UsedPercent = &percent
			w.ResetsAt = collectorReset(pool.Reset)
			snapshot.Windows = append(snapshot.Windows, w)
		}
	}
	if response.Usage != nil {
		w, err := kimiCountWindow(*response.Usage, 604800)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Windows = mergeKimiWindow(snapshot.Windows, w)
	}
	for _, limit := range response.Limits {
		multipliers := map[string]int64{
			"TIME_UNIT_MINUTE": 60, "TIME_UNIT_HOUR": 3600, "TIME_UNIT_DAY": 86400, "TIME_UNIT_SECOND": 1,
		}
		multiplier := multipliers[limit.Window.Unit]
		if multiplier == 0 || limit.Window.Duration <= 0 || limit.Window.Duration > 366*86400 {
			return Snapshot{}, NewUsageError(ErrorCodeParseError, "Kimi Coding")
		}
		w, err := kimiCountWindow(limit.Detail, limit.Window.Duration*multiplier)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Windows = mergeKimiWindow(snapshot.Windows, w)
	}
	if len(snapshot.Windows) == 0 {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Kimi Coding")
	}
	return snapshot, nil
}
func (c *KimiCollector) token(t Target, cred CredentialIdentity) (string, SourceKind, string, error) {
	if !c.cli {
		key, err := checkedKey(t, cred)
		return key, SourceKindAPIKeyHTTP, "Kimi Coding API key", err
	}
	value, err := c.credential(t)
	if err == nil {
		err = checkedCollectorCredential(value, cred, "Kimi Coding")
	}
	return value.token, SourceKindOAuthHTTP, "Kimi CLI OAuth", err
}
func kimiCountWindow(detail kimiDetail, seconds int64) (Window, error) {
	used := detail.Used
	if used == "" && detail.Limit != "" && detail.Remaining != "" {
		used = collectorSubtract(detail.Limit, detail.Remaining, 0)
	}
	if used == "" || detail.Limit == "" {
		return Window{}, NewUsageError(ErrorCodeParseError, "Kimi Coding")
	}
	w := collectorWindow(seconds)
	w.Used = used.amount("requests")
	w.Limit = detail.Limit.amount("requests")
	w.UsedPercent = collectorPercent(used, detail.Limit)
	w.ResetsAt = collectorReset(detail.Reset)
	return w, nil
}
func mergeKimiWindow(windows []Window, w Window) []Window {
	for i, existing := range windows {
		if existing.ID == w.ID {
			if existing.ResetsAt != nil && w.ResetsAt != nil &&
				existing.ResetsAt.Sub(*w.ResetsAt).Abs() <= 2*time.Second &&
				w.UsedPercent != nil &&
				(existing.UsedPercent == nil || *w.UsedPercent > *existing.UsedPercent) {
				windows[i] = w
			}
			return windows
		}
	}
	return append(windows, w)
}

func (c *KimiCollector) getUsage(ctx context.Context, token string, out any) error {
	headers := bearerHeaders(token)
	if c.cli {
		headers.Set("X-Msh-Platform", "kimi_code_cli")
	}
	data, err := c.client().Do(ctx, http.MethodGet, "/coding/v1/usages", headers, nil, "Kimi Coding")
	if err != nil {
		return err
	}
	return decodeCollectorJSON(data, out, "Kimi Coding")
}
