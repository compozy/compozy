package providerusage

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"time"
	"unicode"
)

type codexRPCWindow struct {
	UsedPercent *float64 `json:"usedPercent"`
	Duration    *int64   `json:"windowDurationMins"`
	ResetsAt    *int64   `json:"resetsAt"`
}
type codexHTTPWindow struct {
	UsedPercent *float64 `json:"used_percent"`
	Duration    *int64   `json:"limit_window_seconds"`
	ResetsAt    *int64   `json:"reset_at"`
}
type codexHTTPRate struct {
	Primary   *codexHTTPWindow `json:"primary_window"`
	Secondary *codexHTTPWindow `json:"secondary_window"`
}
type codexCredits struct {
	Balance json.RawMessage `json:"balance"`
}
type codexRPCLimits struct {
	Primary   *codexRPCWindow `json:"primary"`
	Secondary *codexRPCWindow `json:"secondary"`
	Credits   *codexCredits   `json:"credits"`
	PlanType  string          `json:"planType"`
}

func codexSnapshot(kind SourceKind, label, plan string) Snapshot {
	return Snapshot{
		ProviderID:   codexProviderID,
		ProviderName: "Codex",
		Status:       StatusOK,
		Source:       &Source{Kind: kind, Label: label},
		Account:      &Account{Plan: plan},
		Confidence:   ConfidencePercentOnly,
		Windows:      []Window{},
		Balances:     []Balance{},
		SpendCaps:    []SpendCap{},
	}
}

func addCodexWindow(
	snapshot *Snapshot,
	id, label string,
	kind WindowKind,
	percent *float64,
	duration *int64,
	reset *int64,
) error {
	if percent != nil && (*percent < 0 || math.IsNaN(*percent) || math.IsInf(*percent, 0)) {
		return NewUsageError(ErrorCodeParseError, "Codex")
	}
	var at *time.Time
	if reset != nil {
		v := time.Unix(*reset, 0).UTC()
		at = &v
	}
	snapshot.Windows = append(
		snapshot.Windows,
		Window{ID: id, Label: label, Kind: kind, UsedPercent: percent, DurationSeconds: duration, ResetsAt: at},
	)
	return nil
}
func addCodexCredits(snapshot *Snapshot, credits *codexCredits) error {
	if credits == nil || len(credits.Balance) == 0 || bytes.Equal(credits.Balance, []byte("null")) {
		return nil
	}
	var value string
	if credits.Balance[0] == '"' {
		if err := json.Unmarshal(credits.Balance, &value); err != nil {
			return NewUsageError(ErrorCodeParseError, "Codex")
		}
	} else {
		value = string(credits.Balance)
	}
	var number json.Number
	if err := json.Unmarshal([]byte(value), &number); err != nil {
		return NewUsageError(ErrorCodeParseError, "Codex")
	}
	snapshot.Balances = append(
		snapshot.Balances,
		Balance{ID: "credits", Label: "Credits", Remaining: Amount{Value: value, Unit: "credits"}},
	)
	return nil
}
func decodeCodexRPC(limits codexRPCLimits) (Snapshot, error) {
	snapshot := codexSnapshot(SourceKindCLIRPC, "Codex app-server", limits.PlanType)
	for _, item := range []struct {
		id, label string
		kind      WindowKind
		w         *codexRPCWindow
	}{
		{"session", "Session", WindowKindSession, limits.Primary},
		{"weekly", "Weekly", WindowKindWeekly, limits.Secondary},
	} {
		if item.w == nil {
			continue
		}
		var duration *int64
		if item.w.Duration != nil {
			duration = new(*item.w.Duration * 60)
		}
		if err := addCodexWindow(
			&snapshot,
			item.id,
			item.label,
			item.kind,
			item.w.UsedPercent,
			duration,
			item.w.ResetsAt,
		); err != nil {
			return Snapshot{}, err
		}
	}
	if err := addCodexCredits(&snapshot, limits.Credits); err != nil {
		return Snapshot{}, err
	}
	if len(snapshot.Windows) == 0 && len(snapshot.Balances) == 0 {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Codex")
	}
	return snapshot, nil
}
func decodeCodexHTTP(data []byte) (Snapshot, error) {
	var payload struct {
		PlanType   string         `json:"plan_type"`
		RateLimit  *codexHTTPRate `json:"rate_limit"`
		Credits    *codexCredits  `json:"credits"`
		Additional []struct {
			Name      string         `json:"limit_name"`
			RateLimit *codexHTTPRate `json:"rate_limit"`
		} `json:"additional_rate_limits"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Codex")
	}
	snapshot := codexSnapshot(SourceKindOAuthHTTP, "Codex OAuth", payload.PlanType)
	if err := addHTTPRate(&snapshot, payload.RateLimit, "", ""); err != nil {
		return Snapshot{}, err
	}
	for _, additional := range payload.Additional {
		slug := modelSlug(additional.Name)
		if slug == "" {
			return Snapshot{}, NewUsageError(ErrorCodeParseError, "Codex")
		}
		if err := addHTTPRate(&snapshot, additional.RateLimit, "model:"+slug+":", " · "+additional.Name); err != nil {
			return Snapshot{}, err
		}
	}
	if err := addCodexCredits(&snapshot, payload.Credits); err != nil {
		return Snapshot{}, err
	}
	if len(snapshot.Windows) == 0 && len(snapshot.Balances) == 0 {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Codex")
	}
	return snapshot, nil
}
func addHTTPRate(snapshot *Snapshot, rate *codexHTTPRate, prefix, suffix string) error {
	if rate == nil {
		return nil
	}
	for _, item := range []struct {
		id, label string
		kind      WindowKind
		w         *codexHTTPWindow
	}{{"session", "Session", WindowKindSession, rate.Primary}, {"weekly", "Weekly", WindowKindWeekly, rate.Secondary}} {
		if item.w == nil {
			continue
		}
		kind := item.kind
		if prefix != "" {
			kind = WindowKindModel
		}
		if err := addCodexWindow(
			snapshot,
			prefix+item.id,
			item.label+suffix,
			kind,
			item.w.UsedPercent,
			item.w.Duration,
			item.w.ResetsAt,
		); err != nil {
			return err
		}
	}
	return nil
}
func modelSlug(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return b.String()
}
