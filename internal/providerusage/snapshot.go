package providerusage

import "math"

func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func cloneSnapshot(s Snapshot) Snapshot {
	s.Source = clonePointer(s.Source)
	s.Account = clonePointer(s.Account)
	s.MeasuredAt = clonePointer(s.MeasuredAt)
	s.CheckedAt = clonePointer(s.CheckedAt)
	s.NextRefreshAt = clonePointer(s.NextRefreshAt)
	s.Error = clonePointer(s.Error)
	if s.Error != nil {
		s.Error.RetryAfter = clonePointer(s.Error.RetryAfter)
	}
	windows := make([]Window, len(s.Windows))
	copy(windows, s.Windows)
	s.Windows = windows
	for i := range s.Windows {
		w := &s.Windows[i]
		w.DurationSeconds = clonePointer(w.DurationSeconds)
		w.UsedPercent = clonePointer(w.UsedPercent)
		w.Used = clonePointer(w.Used)
		w.Limit = clonePointer(w.Limit)
		w.ResetsAt = clonePointer(w.ResetsAt)
	}
	balances := make([]Balance, len(s.Balances))
	copy(balances, s.Balances)
	s.Balances = balances
	for i := range s.Balances {
		s.Balances[i].ExpiresAt = clonePointer(s.Balances[i].ExpiresAt)
	}
	caps := make([]SpendCap, len(s.SpendCaps))
	copy(caps, s.SpendCaps)
	s.SpendCaps = caps
	for i := range s.SpendCaps {
		c := &s.SpendCaps[i]
		c.Limit = clonePointer(c.Limit)
		c.UsedPercent = clonePointer(c.UsedPercent)
		c.ResetsAt = clonePointer(c.ResetsAt)
		c.Enabled = clonePointer(c.Enabled)
	}
	return s
}

func SelectTray(targets []Snapshot) *TraySignal {
	var signal *TraySignal
	for _, target := range targets {
		if target.Status != StatusOK && target.Status != StatusStale {
			continue
		}
		for _, w := range target.Windows {
			if w.UsedPercent == nil || math.IsNaN(*w.UsedPercent) || math.IsInf(*w.UsedPercent, 0) ||
				*w.UsedPercent < 0 {
				continue
			}
			percent := *w.UsedPercent
			if signal != nil && !tighterWindow(percent, w, signal) {
				continue
			}

			severity := SeverityNeutral
			if percent >= 100 {
				severity = SeverityExhausted
			} else if percent >= 80 {
				severity = SeverityWarning
			}
			signal = &TraySignal{
				ProviderID:  target.ProviderID,
				WindowID:    w.ID,
				Label:       target.ProviderName + " · " + w.Label,
				UsedPercent: percent,
				ResetsAt:    clonePointer(w.ResetsAt),
				Severity:    severity,
				Stale:       target.Status == StatusStale,
			}
		}
	}
	return signal
}

func tighterWindow(percent float64, w Window, signal *TraySignal) bool {
	if percent != signal.UsedPercent {
		return percent > signal.UsedPercent
	}
	return w.ResetsAt != nil && (signal.ResetsAt == nil || w.ResetsAt.Before(*signal.ResetsAt))
}
