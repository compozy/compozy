package providerusage

import "context"

type VercelCollector struct{ collectorHTTP }

var _ Collector = (*VercelCollector)(nil)

func NewVercelCollector(options ...CollectorOption) *VercelCollector {
	return &VercelCollector{newCollectorHTTP("https://ai-gateway.vercel.sh", options)}
}
func (*VercelCollector) ProviderID() string { return "vercel-ai-gateway" }
func (*VercelCollector) Identity(_ context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	return keyIdentity(t)
}
func (c *VercelCollector) Collect(ctx context.Context, t Target, _ Intent, cred CredentialIdentity) (Snapshot, error) {
	key, err := checkedKey(t, cred)
	if err != nil {
		return Snapshot{}, err
	}
	var response struct {
		Balance collectorDecimal `json:"balance"`
		Used    collectorDecimal `json:"total_used"`
	}
	if err := c.get(ctx, "/v1/credits", key, "Vercel AI Gateway", &response); err != nil {
		return Snapshot{}, err
	}
	if response.Balance == "" || response.Used == "" {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Vercel AI Gateway")
	}
	snapshot := c.snapshot(t, Source{Kind: SourceKindAPIKeyHTTP, Label: "Vercel AI Gateway API key"})
	snapshot.Balances = []Balance{{ID: "credits", Label: "Team balance", Remaining: *response.Balance.amount("USD")}}
	snapshot.SpendCaps = []SpendCap{{ID: "total_used", Label: "Lifetime spend", Used: *response.Used.amount("USD")}}
	return snapshot, nil
}
