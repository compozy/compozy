package providerusage

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type CursorCollector struct{ collectorHTTP }

var _ Collector = (*CursorCollector)(nil)

func NewCursorCollector(options ...CollectorOption) *CursorCollector {
	return &CursorCollector{newCollectorHTTP("https://cursor.com", options)}
}
func (*CursorCollector) ProviderID() string { return "cursor" }
func (c *CursorCollector) credential(t Target) (collectorCredential, error) {
	home, err := collectorHomePath(t, "XDG_CONFIG_HOME", ".config")
	if err != nil {
		return collectorCredential{}, err
	}
	var data struct {
		Access string `json:"accessToken"`
	}
	if err := readCollectorCredential(filepath.Join(home, "cursor", "auth.json"), "Cursor", &data); err != nil {
		return collectorCredential{}, err
	}
	if data.Access == "" {
		return collectorCredential{}, NewUsageError(ErrorCodeAuthMissing, "Cursor")
	}
	claims, err := readCollectorClaims(data.Access)
	if err != nil || claims.Subject == "" {
		return collectorCredential{}, NewUsageError(ErrorCodeParseError, "Cursor")
	}
	if expiredCollectorToken(claims.Expiry, c.now()) {
		return collectorCredential{}, NewUsageError(ErrorCodeAuthExpired, "Cursor")
	}
	return collectorCredential{token: data.Access, account: claims.Subject, email: claims.Email}, nil
}
func (c *CursorCollector) Identity(_ context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	value, err := c.credential(t)
	if err != nil {
		return CredentialIdentity{}, err
	}
	return value.identity(), nil
}

type cursorPlan struct {
	Used    collectorDecimal `json:"used"`
	Limit   collectorDecimal `json:"limit"`
	Percent *float64         `json:"totalPercentUsed"`
	Auto    *float64         `json:"autoPercentUsed"`
	API     *float64         `json:"apiPercentUsed"`
}

func (c *CursorCollector) Collect(ctx context.Context, t Target, _ Intent, cred CredentialIdentity) (Snapshot, error) {
	value, err := c.credential(t)
	if err != nil {
		return Snapshot{}, err
	}
	if err := checkedCollectorCredential(value, cred, "Cursor"); err != nil {
		return Snapshot{}, err
	}
	// Cursor accepts its CLI JWT as a derived session cookie; no browser cookie is read or stored.
	subject := value.account
	if _, last, ok := strings.Cut(subject, "|"); ok {
		subject = last
	}
	headers := http.Header{
		"Cookie":     {"WorkosCursorSessionToken=" + url.QueryEscape(subject+"::"+value.token)},
		headerAccept: {collectorJSONType},
	}
	body, err := c.client().Do(ctx, http.MethodGet, "/api/usage-summary", headers, nil, "Cursor")
	if err != nil {
		return Snapshot{}, err
	}
	var response struct {
		End        string `json:"billingCycleEnd"`
		Membership string `json:"membershipType"`
		Individual *struct {
			Plan     *cursorPlan `json:"plan"`
			Overall  *cursorPlan `json:"overall"`
			OnDemand *cursorPlan `json:"onDemand"`
		} `json:"individualUsage"`
	}
	if err := decodeCollectorJSON(body, &response, "Cursor"); err != nil {
		return Snapshot{}, err
	}
	if response.Individual == nil {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Cursor")
	}
	snapshot := c.snapshot(t, Source{Kind: SourceKindOAuthHTTP, Label: "Cursor CLI OAuth"})
	snapshot.Account = &Account{Label: value.email, Plan: response.Membership}
	plan := response.Individual.Plan
	if plan == nil {
		plan = response.Individual.Overall
	}
	if plan != nil {
		w, err := plan.window(response.End)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Windows = []Window{w}
	}
	if extra := response.Individual.OnDemand; extra != nil {
		spend, err := extra.spendCap(response.End)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.SpendCaps = []SpendCap{spend}
	}
	if len(snapshot.Windows) == 0 && len(snapshot.SpendCaps) == 0 {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Cursor")
	}
	body, err = c.client().Do(ctx, http.MethodGet, "/api/auth/me", headers, nil, "Cursor")
	if err != nil {
		return snapshot, err
	}
	var account struct {
		Email string `json:"email"`
		Sub   string `json:"sub"`
	}
	if err := decodeCollectorJSON(body, &account, "Cursor"); err != nil {
		return snapshot, err
	}
	if account.Sub != "" && account.Sub != value.account && account.Sub != subject {
		return Snapshot{}, NewUsageError(ErrorCodeAuthMissing, "Cursor")
	}
	if account.Email != "" {
		snapshot.Account.Label = account.Email
	}
	return snapshot, nil
}

func (p *cursorPlan) percent() *float64 {
	if p.Percent != nil {
		return p.Percent
	}
	return collectorPercent(p.Used, p.Limit)
}

func (p *cursorPlan) window(end string) (Window, error) {
	percent := p.percent()
	if percent != nil && !validCollectorPercent(*percent) {
		return Window{}, NewUsageError(ErrorCodeParseError, "Cursor")
	}
	if percent == nil && p.Used == "" && p.Limit == "" && p.Auto == nil && p.API == nil {
		return Window{}, NewUsageError(ErrorCodeParseError, "Cursor")
	}
	return Window{ID: string(WindowKindMonthly), Label: "Monthly", Kind: WindowKindMonthly, UsedPercent: percent,
		Used: collectorCents(p.Used), Limit: collectorCents(p.Limit), ResetsAt: collectorReset(end)}, nil
}

func (p *cursorPlan) spendCap(end string) (SpendCap, error) {
	if p.Used == "" {
		return SpendCap{}, NewUsageError(ErrorCodeParseError, "Cursor")
	}
	return SpendCap{
		ID:          "extra_usage",
		Label:       "Extra usage",
		Used:        *collectorCents(p.Used),
		Limit:       collectorCents(p.Limit),
		UsedPercent: collectorPercent(p.Used, p.Limit),
		ResetsAt:    collectorReset(end),
	}, nil
}
