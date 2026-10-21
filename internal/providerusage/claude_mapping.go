package providerusage

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type claudeWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

func decodeClaudeUsage(data []byte) (Snapshot, error) {
	var fields map[string]jsontext.Value
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Claude")
	}
	snapshot := Snapshot{Windows: []Window{}, Balances: []Balance{}, SpendCaps: []SpendCap{}}
	for _, lane := range []struct {
		key, id, label string
		kind           WindowKind
		seconds        int64
	}{
		{"five_hour", string(WindowKindSession), sessionWindowLabel, WindowKindSession, 18000},
		{"seven_day", string(WindowKindWeekly), weeklyWindowLabel, WindowKindWeekly, 604800},
		{"seven_day_opus", "weekly_opus", "Weekly · Opus", WindowKindModel, 604800},
		{"seven_day_sonnet", "model:sonnet:weekly", "Weekly · Sonnet", WindowKindModel, 604800},
	} {
		if window, ok := decodeClaudeWindow(fields[lane.key]); ok {
			snapshot.Windows = append(
				snapshot.Windows,
				Window{
					ID:              lane.id,
					Label:           lane.label,
					Kind:            lane.kind,
					DurationSeconds: new(lane.seconds),
					UsedPercent:     window.Utilization,
					ResetsAt:        claudeReset(window.ResetsAt),
					ResetText:       claudeResetText(window.ResetsAt),
				},
			)
		}
	}
	for _, key := range []string{
		"seven_day_routines", "seven_day_claude_routines", "claude_routines",
		"routines", "routine", "seven_day_cowork", "cowork",
	} {
		if window, ok := decodeClaudeWindow(fields[key]); ok {
			snapshot.Windows = append(
				snapshot.Windows,
				Window{
					ID:          "routines",
					Label:       "Daily Routines",
					Kind:        WindowKindDaily,
					UsedPercent: window.Utilization,
					ResetsAt:    claudeReset(window.ResetsAt),
					ResetText:   claudeResetText(window.ResetsAt),
				},
			)
			break
		}
	}
	appendClaudeScoped(&snapshot, fields["limits"])
	if spendCap, ok := decodeClaudeExtra(fields["extra_usage"]); ok {
		snapshot.SpendCaps = append(snapshot.SpendCaps, spendCap)
	}
	if len(snapshot.Windows) == 0 && len(snapshot.SpendCaps) == 0 {
		return Snapshot{}, NewUsageError(ErrorCodeParseError, "Claude")
	}
	return snapshot, nil
}
func decodeClaudeWindow(data jsontext.Value) (claudeWindow, bool) {
	var window claudeWindow
	if len(data) == 0 || string(data) == jsonNull || json.Unmarshal(data, &window) != nil {
		return window, false
	}
	if window.Utilization != nil && *window.Utilization < 0 {
		window.Utilization = nil
	}
	return window, window.Utilization != nil || window.ResetsAt != "" ||
		strings.Contains(string(data), "\"utilization\"")
}
func claudeReset(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return &parsed
}
func claudeResetText(value string) string {
	if claudeReset(value) == nil {
		return value
	}
	return ""
}
func decodeClaudeExtra(data jsontext.Value) (SpendCap, bool) {
	var extra struct {
		Enabled     *bool          `json:"is_enabled"`
		Limit       jsontext.Value `json:"monthly_limit"`
		Used        jsontext.Value `json:"used_credits"`
		Utilization *float64       `json:"utilization"`
		Currency    string         `json:"currency"`
	}
	if len(data) == 0 || json.Unmarshal(data, &extra) != nil {
		return SpendCap{}, false
	}
	used, ok := claudeCents(extra.Used)
	if !ok {
		return SpendCap{}, false
	}
	currency := extra.Currency
	if currency == "" {
		currency = "USD"
	}
	spendCap := SpendCap{
		ID:          "extra_usage",
		Label:       "Extra usage",
		Used:        Amount{Value: used, Unit: currency},
		Enabled:     extra.Enabled,
		UsedPercent: extra.Utilization,
	}
	if value, ok := claudeCents(extra.Limit); ok {
		spendCap.Limit = &Amount{Value: value, Unit: currency}
	}
	if spendCap.UsedPercent != nil && *spendCap.UsedPercent < 0 {
		spendCap.UsedPercent = nil
	}
	return spendCap, true
}
func claudeCents(raw jsontext.Value) (string, bool) {
	if len(raw) == 0 || len(raw) > 128 || raw.Kind() != '0' {
		return "", false
	}
	if _, exponent, found := strings.Cut(strings.ToLower(string(raw)), "e"); found {
		value, err := strconv.Atoi(exponent)
		if err != nil || value < -32 || value > 32 {
			return "", false
		}
	}
	number, ok := new(big.Rat).SetString(string(raw))
	if !ok || number.Sign() < 0 {
		return "", false
	}
	number.Quo(number, big.NewRat(100, 1))
	// Preserve sub-cent precision and exponent notation without floating point.
	scale := 2
	for ; scale <= 32; scale++ {
		scaled := new(
			big.Rat,
		).Mul(number, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)))
		if scaled.IsInt() {
			return number.FloatString(scale), true
		}
	}
	return "", false
}

func appendClaudeScoped(snapshot *Snapshot, data jsontext.Value) {
	var entries []jsontext.Value
	if len(data) == 0 || json.Unmarshal(data, &entries) != nil {
		return
	}
	seen := make(map[string]bool)
	for _, window := range snapshot.Windows {
		seen[window.ID] = true
	}
	for _, raw := range entries {
		var entry struct {
			Kind     string   `json:"kind"`
			Group    string   `json:"group"`
			Percent  *float64 `json:"percent"`
			ResetsAt string   `json:"resets_at"`
			Scope    struct {
				Model struct {
					ID          string `json:"id"`
					DisplayName string `json:"display_name"`
				} `json:"model"`
			} `json:"scope"`
		}
		if json.Unmarshal(raw, &entry) != nil {
			continue
		}
		name := strings.TrimSpace(entry.Scope.Model.DisplayName)
		if entry.Kind != "weekly_scoped" || entry.Group != string(WindowKindWeekly) || name == "" ||
			entry.Percent == nil ||
			*entry.Percent < 0 {
			continue
		}
		identity := entry.Scope.Model.ID
		if identity == "" {
			identity = name
		}
		slug := claudeModelSlug(identity)
		if slug == "" || claudeModelSlug(name) == "all-models" || strings.HasSuffix(slug, "all-models") {
			continue
		}
		id := "model:" + slug + ":weekly"
		if slug == "opus" {
			id = "weekly_opus"
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		snapshot.Windows = append(
			snapshot.Windows,
			Window{
				ID:              id,
				Label:           "Weekly · " + name,
				Kind:            WindowKindModel,
				DurationSeconds: new(int64(604800)),
				UsedPercent:     entry.Percent,
				ResetsAt:        claudeReset(entry.ResetsAt),
				ResetText:       claudeResetText(entry.ResetsAt),
			},
		)
	}
}
func claudeModelSlug(value string) string {
	parts := strings.FieldsFunc(
		strings.ToLower(value),
		func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) },
	)
	return strings.Join(parts, "-")
}
