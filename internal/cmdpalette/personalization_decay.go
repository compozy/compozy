package cmdpalette

import (
	"math"
	"time"
)

// DecayFrecency applies exponential decay to a stored signal weight.
func DecayFrecency(weight float64, last, now time.Time, halfLife time.Duration) float64 {
	if weight <= 0 {
		return 0
	}
	if halfLife <= 0 || !now.After(last) {
		return weight
	}
	return weight * math.Pow(0.5, now.Sub(last).Seconds()/halfLife.Seconds())
}

func (w Weights) frecencyHalfLife() time.Duration {
	return time.Duration(w.FrecencyHalfLifeDays) * 24 * time.Hour
}

func (w Weights) queryHalfLife() time.Duration {
	return time.Duration(w.QueryHalfLifeDays) * 24 * time.Hour
}
