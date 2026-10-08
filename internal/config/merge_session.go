package config

import (
	"log/slog"
	"strings"
)

func (o sessionOverlay) Apply(dst *SessionConfig) {
	if o.Stop.CooperativeGrace != nil {
		dst.Stop.CooperativeGrace = *o.Stop.CooperativeGrace
	}
	o.Limits.Apply(&dst.Limits)
	o.Supervision.Apply(&dst.Supervision)
	o.BusyInput.Apply(&dst.BusyInput)
	o.Attachments.Apply(&dst.Attachments)
	o.Derive.Apply(&dst.Derive)
}

func (o sessionLimitsOverlay) Apply(dst *SessionLimitsConfig) {
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
}

func (o sessionSupervisionOverlay) Apply(dst *SessionSupervisionConfig) {
	if o.ActivityHeartbeatInterval != nil {
		dst.ActivityHeartbeatInterval = *o.ActivityHeartbeatInterval
	}
	if o.ProgressNotifyInterval != nil {
		dst.ProgressNotifyInterval = *o.ProgressNotifyInterval
	}
	if o.PromptDeadline != nil {
		dst.PromptDeadline = *o.PromptDeadline
	}
	applySupervisionCompatibility(o, dst)
	if o.QuietAfter != nil {
		dst.QuietAfter = *o.QuietAfter
	}
	if o.StopGrace != nil {
		dst.StopGrace = *o.StopGrace
	}
	if o.TimeoutCancelGrace != nil {
		dst.TimeoutCancelGrace = *o.TimeoutCancelGrace
	}
}

func (o sessionBusyInputOverlay) Apply(dst *SessionBusyInputConfig) {
	if o.DefaultMode != nil {
		dst.DefaultMode = *o.DefaultMode
		// Retain the released config value through v0.4; remove it in v0.5.0.
		if strings.TrimSpace(dst.DefaultMode) == "interrupt" {
			slog.Warn("session.busy_input.default_mode=interrupt is deprecated; use steer or queue before v0.5.0")
		}
	}
	if o.QueueCap != nil {
		dst.QueueCap = *o.QueueCap
	}
	if o.MaxTextBytes != nil {
		dst.MaxTextBytes = *o.MaxTextBytes
	}
}
