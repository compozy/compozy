package providerusage

import "context"

type OpenRouterCollector struct{ collectorHTTP }

var _ Collector = (*OpenRouterCollector)(nil)

func NewOpenRouterCollector(options ...CollectorOption) *OpenRouterCollector {
	return &OpenRouterCollector{newCollectorHTTP("https://openrouter.ai", options)}
}
func (*OpenRouterCollector) ProviderID() string { return "openrouter" }
func (*OpenRouterCollector) Identity(_ context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	return keyIdentity(t)
}

func (c *OpenRouterCollector) Collect(
	ctx context.Context,
	t Target,
	_ Intent,
	cred CredentialIdentity,
) (Snapshot, error) {
	key, err := checkedKey(t, cred)
	if err != nil {
		return Snapshot{}, err
	}
	var response struct {
		Data *struct {
			Limit     collectorDecimal `json:"limit"`
			Usage     collectorDecimal `json:"usage"`
			Remaining collectorDecimal `json:"limit_remaining"`
			Reset     string           `json:"limit_reset"`
			Daily     collectorDecimal `json:"usage_daily"`
			Weekly    collectorDecimal `json:"usage_weekly"`
			Monthly   collectorDecimal `json:"usage_monthly"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/v1/key", key, "OpenRouter", &response); err != nil {
		return Snapshot{}, err
	}
	if response.Data == nil || response.Data.Usage == "" {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "OpenRouter")
	}
	data := response.Data
	used := data.Usage
	switch data.Reset {
	case "daily":
		if data.Daily != "" {
			used = data.Daily
		}
	case "weekly":
		if data.Weekly != "" {
			used = data.Weekly
		}
	case string(WindowKindMonthly):
		if data.Monthly != "" {
			used = data.Monthly
		}
	}
	percent := collectorPercent(used, data.Limit)
	if data.Limit != "" && data.Remaining != "" {
		percent = collectorPercent(collectorSubtract(data.Limit, data.Remaining, 18), data.Limit)
	}
	snapshot := c.snapshot(t, Source{Kind: SourceKindAPIKeyHTTP, Label: "OpenRouter API key"})
	snapshot.SpendCaps = []SpendCap{
		{
			ID:          "key_limit",
			Label:       "Key spending cap",
			Used:        *used.amount("USD"),
			Limit:       data.Limit.amount("USD"),
			UsedPercent: percent,
		},
	}
	var credits struct {
		Data *struct {
			Total collectorDecimal `json:"total_credits"`
			Used  collectorDecimal `json:"total_usage"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/v1/credits", key, "OpenRouter", &credits); err != nil {
		return snapshot, err
	}
	if credits.Data == nil || credits.Data.Total == "" || credits.Data.Used == "" {
		return snapshot, NewUsageError(ErrorCodeParseError, "OpenRouter")
	}

	scale := collectorMoneyScale(credits.Data.Total, credits.Data.Used)
	remaining := collectorSubtract(credits.Data.Total, credits.Data.Used, scale)
	snapshot.Balances = []Balance{
		{
			ID:        "credits",
			Label:     "Account balance",
			Remaining: *remaining.amount("USD"),
		},
	}
	return snapshot, nil
}
