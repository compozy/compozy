package providerusage

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var collectorDecimalPattern = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)

type collectorDecimal string

func (d *collectorDecimal) UnmarshalJSON(data []byte) error {
	if string(data) == jsonNull {
		return nil
	}
	value := string(data)
	if strings.HasPrefix(value, `"`) {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	}
	if len(value) > 256 || !collectorDecimalPattern.MatchString(value) {
		return fmt.Errorf("invalid decimal")
	}
	*d = collectorDecimal(value)
	return nil
}
func (d collectorDecimal) amount(unit string) *Amount {
	if d == "" {
		return nil
	}
	return &Amount{Value: string(d), Unit: unit}
}
func (d collectorDecimal) rat() *big.Rat {
	value, _ := new(big.Rat).SetString(string(d))
	return value
}
func collectorPercent(used, limit collectorDecimal) *float64 {
	if used == "" || limit == "" || limit.rat().Sign() <= 0 {
		return nil
	}
	value, _ := new(big.Rat).Mul(new(big.Rat).Quo(used.rat(), limit.rat()), big.NewRat(100, 1)).Float64()
	if !validCollectorPercent(value) {
		return nil
	}
	return &value
}
func validCollectorPercent(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
func collectorSubtract(a, b collectorDecimal, scale int) collectorDecimal {
	return collectorDecimal(new(big.Rat).Sub(a.rat(), b.rat()).FloatString(scale))
}
func collectorCents(d collectorDecimal) *Amount {
	if d == "" {
		return nil
	}
	scale := 2
	if _, fraction, ok := strings.Cut(string(d), "."); ok {
		scale += len(fraction)
	}
	value := new(big.Rat).Quo(d.rat(), big.NewRat(100, 1)).FloatString(scale)
	return &Amount{Value: value, Unit: "USD"}
}
func collectorReset(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return &parsed
}
func collectorWindow(seconds int64) Window {
	w := Window{Kind: WindowKindOther, ID: "window:" + strconv.FormatInt(seconds, 10), Label: "Usage"}
	if seconds > 0 {
		w.DurationSeconds = &seconds
	}
	switch seconds {
	case 18000:
		w.ID = string(WindowKindSession)
		w.Label = sessionWindowLabel
		w.Kind = WindowKindSession
	case 86400:
		w.ID = "daily"
		w.Label = "Daily"
		w.Kind = WindowKindDaily
	case 604800:
		w.ID = string(WindowKindWeekly)
		w.Label = weeklyWindowLabel
		w.Kind = WindowKindWeekly
	}
	return w
}

func collectorMoneyScale(values ...collectorDecimal) int {
	scale := 2
	for _, value := range values {
		if _, fraction, ok := strings.Cut(string(value), "."); ok {
			scale = max(scale, len(fraction))
		}
	}
	return scale
}
