package providerusage

import (
	"cmp"
	"context"
	"slices"
	"time"
)

type ZaiCollector struct{ collectorHTTP }

var _ Collector = (*ZaiCollector)(nil)

func NewZaiCollector(options ...CollectorOption) *ZaiCollector {
	return &ZaiCollector{newCollectorHTTP("https://api.z.ai", options)}
}
func (*ZaiCollector) ProviderID() string { return "zai" }
func (*ZaiCollector) Identity(_ context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	return keyIdentity(t)
}

type zaiLimit struct {
	Type       string           `json:"type"`
	Unit       int64            `json:"unit"`
	Number     int64            `json:"number"`
	Percentage *float64         `json:"percentage"`
	Usage      collectorDecimal `json:"usage"`
	Current    collectorDecimal `json:"currentValue"`
	Remaining  collectorDecimal `json:"remaining"`
	Reset      *int64           `json:"nextResetTime"`
}

func (c *ZaiCollector) Collect(ctx context.Context, t Target, _ Intent, cred CredentialIdentity) (Snapshot, error) {
	key, err := checkedKey(t, cred)
	if err != nil {
		return Snapshot{}, err
	}
	var response struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    *struct {
			Limits   []zaiLimit `json:"limits"`
			PlanName string     `json:"planName"`
			Plan     string     `json:"plan"`
			PlanType string     `json:"plan_type"`
			Package  string     `json:"packageName"`
			Level    string     `json:"level"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/monitor/usage/quota/limit", key, "z.ai", &response); err != nil {
		return Snapshot{}, err
	}
	if !response.Success || response.Code != 200 || response.Data == nil {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "z.ai")
	}
	snapshot := c.snapshot(t, Source{Kind: SourceKindAPIKeyHTTP, Label: "z.ai API key"})
	plans := []string{
		response.Data.PlanName, response.Data.Plan, response.Data.PlanType, response.Data.Package, response.Data.Level,
	}
	for _, plan := range plans {
		if plan != "" {
			snapshot.Account = &Account{Plan: plan}
			break
		}
	}
	for _, limit := range response.Data.Limits {
		if limit.Type != "TOKENS_LIMIT" && limit.Type != "CREDIT_LIMIT" {
			continue
		}
		w, err := c.window(limit)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Windows = append(snapshot.Windows, w)
	}
	if len(snapshot.Windows) == 0 {
		return snapshot, NewUsageError(ErrorCodeNoPlan, "z.ai")
	}
	slices.SortStableFunc(snapshot.Windows, func(a, b Window) int {
		if a.DurationSeconds == nil {
			if b.DurationSeconds == nil {
				return 0
			}
			return 1
		}
		if b.DurationSeconds == nil {
			return -1
		}
		return cmp.Compare(*a.DurationSeconds, *b.DurationSeconds)
	})
	return snapshot, nil
}
func (c *ZaiCollector) window(limit zaiLimit) (Window, error) {
	if limit.Percentage == nil || !validCollectorPercent(*limit.Percentage) || limit.Number < 0 || limit.Number > 366 {
		return Window{}, NewUsageError(ErrorCodeParseError, "z.ai")
	}
	seconds := limit.Number * map[int64]int64{1: 86400, 3: 3600, 5: 60, 6: 604800}[limit.Unit]
	w := collectorWindow(seconds)
	w.UsedPercent = limit.Percentage
	used := limit.Current
	if limit.Usage != "" && limit.Remaining != "" {
		difference := collectorSubtract(limit.Usage, limit.Remaining, 0)
		if used == "" || difference.rat().Cmp(used.rat()) > 0 {
			used = difference
		}
	}
	if percent := collectorPercent(used, limit.Usage); percent != nil {
		w.UsedPercent = percent
	}
	w.Used = used.amount(creditsUnit)
	w.Limit = limit.Usage.amount(creditsUnit)
	if limit.Reset != nil {
		reset := time.UnixMilli(*limit.Reset)
		if seconds != 18000 || !reset.After(c.now().Add(5*time.Hour+time.Minute)) {
			w.ResetsAt = &reset
		}
	}
	return w, nil
}
