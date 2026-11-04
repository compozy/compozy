package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (e *instanceCheckEnvironment) PollSources(
	ctx context.Context,
	target *heartbeat.CheckTarget,
	dry bool,
) (heartbeat.PollBatch, error) {
	cfg := e.sourceConfig
	now := time.Now()
	if e.sourceNow != nil {
		now = e.sourceNow()
	}
	states, err := e.instances.SourceStates(
		ctx,
		target.Instance.ID,
		target.Policy.Preferences.Context.Sources,
		dry,
		now,
	)
	if err != nil {
		return heartbeat.PollBatch{}, err
	}
	token, err := store.NewID("poll")
	if err != nil {
		return heartbeat.PollBatch{}, err
	}
	batch := heartbeat.PollBatch{Token: token}
	runner := &source.ScriptRunner{
		DefinitionDir:  target.DefinitionDir,
		ProfileID:      target.Instance.ProfileID,
		Agent:          target.Instance.AgentName,
		ScriptsEnabled: cfg.ScriptsEnabled && !target.UntrustedScripts,
		Timeout:        cfg.Timeout,
		MaxOutputBytes: cfg.MaxOutputBytes,
		EnvAllow:       cfg.EnvAllow,
		Approvals:      e.instances,
	}
	batch.Sources = make([]heartbeat.PolledSource, len(states))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, cfg.MaxPerAgent))
	for i, ref := range target.Policy.Preferences.Context.Sources {
		wg.Go(func() {
			state := states[i]
			p := heartbeat.PolledSource{
				Ref:           ref,
				RefDigest:     state.RefDigest,
				Generation:    state.Generation,
				FailureStreak: state.FailureStreak,
			}
			defer func() { batch.Sources[i] = p }()
			if !dry && state.FailureStreak > 0 && now.Before(state.NextPollAt) {
				p.Err = state.Error
				return
			}
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			e.pollHeartbeatSource(ctx, target, state, runner, &p, now, dry)
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return heartbeat.PollBatch{}, err
	}
	for i := range batch.Sources {
		p := &batch.Sources[i]
		if !p.Polled {
			continue
		}
		if !dry {
			if err := e.updateHeartbeatSourceStatus(ctx, target, states[i], p, now); err != nil {
				return heartbeat.PollBatch{}, err
			}
		}
		logHeartbeatSourcePoll(target, *p, token)
	}
	return batch, nil
}

func (e *instanceCheckEnvironment) pollHeartbeatSource(
	ctx context.Context,
	target *heartbeat.CheckTarget,
	state heartbeat.SourceState,
	runner source.Runner,
	p *heartbeat.PolledSource,
	now time.Time,
	dry bool,
) {
	cfg := e.sourceConfig
	p.Polled = true
	start := time.Now()
	req := source.Request{
		Version:  1,
		SourceID: p.Ref.ID,
		Kind:     p.Ref.Kind,
		Params:   p.Ref.With,
		State:    state.State,
		Limits: source.Limits{
			MaxItems:        cfg.MaxItems,
			MaxOutputBytes:  cfg.MaxOutputBytes,
			MaxSummaryBytes: source.SummaryBytes,
			MaxStateBytes:   cfg.MaxStateBytes,
		},
		Workspace: source.WorkspaceRef{ID: target.Instance.WorkspaceID, RootDir: target.RootDir},
		Now:       now,
		DryRun:    dry,
	}
	var pollErr error
	if p.Ref.Kind != "" {
		if e.sourceExtensions == nil {
			pollErr = source.Failure("source_kind_unavailable", "source kind is not enabled in this workspace")
		} else {
			pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()
			p.Result, pollErr = e.sourceExtensions.PollSource(
				pollCtx,
				source.CatalogScope{ProfileID: target.Instance.ProfileID, WorkspaceID: target.Instance.WorkspaceID},
				p.Ref.Kind,
				req,
			)
		}
	} else {
		p.Result, pollErr = runner.Poll(ctx, p.Ref, req)
	}
	p.DurationMS = time.Since(start).Milliseconds()
	if pollErr != nil {
		p.Err, _ = errors.AsType[*source.Error](pollErr)
		if p.Err == nil {
			p.Err = source.Failure("source_output_invalid", diagnostics.RedactAndBound(pollErr.Error(), 500))
		}
	}

	p.FailureStreak = 0
	if heartbeat.SourceResultStatus(p.Err) == "failing" {
		p.FailureStreak = state.FailureStreak + 1
	}
}

func (e *instanceCheckEnvironment) updateHeartbeatSourceStatus(
	ctx context.Context,
	target *heartbeat.CheckTarget,
	state heartbeat.SourceState,
	p *heartbeat.PolledSource,
	now time.Time,
) error {
	cadence := target.Policy.Preferences.MinInterval
	var next time.Time
	if p.FailureStreak > 0 {
		next = now.Add(heartbeat.CheckBackoff(cadence, e.sourceMaxInterval, p.FailureStreak-1))
	}
	if p.Err != nil && p.Err.Code == "source_script_unapproved" &&
		(state.Error == nil || state.Error.Code != "source_script_unapproved") {
		_, notifyErr := e.sessions.PublishOperatorNotification(
			ctx,
			session.NotifyRequest{
				Kind:        session.NotifyKindHeartbeatSourceUnapproved,
				SessionID:   target.Instance.MainSessionID,
				ProfileID:   target.Instance.ProfileID,
				WorkspaceID: target.Instance.WorkspaceID,
				Title:       "Check-in script needs approval",
				Body:        "Review and approve source script " + p.Ref.Script + " for " + target.Instance.AgentName + ".",
			},
		)
		if notifyErr != nil {
			slog.Warn(
				"heartbeat source approval notification failed",
				"instance_id",
				target.Instance.ID,
				"source_id",
				p.Ref.ID,
				"error",
				notifyErr,
			)
		}
	}
	if err := e.instances.SourcePollStatus(ctx, target.Instance.ID, *p, now, next); err != nil {
		return err
	}
	return nil
}

func logHeartbeatSourcePoll(target *heartbeat.CheckTarget, p heartbeat.PolledSource, token string) {
	ref := p.Ref.Kind
	if ref == "" {
		ref = "script:" + p.Ref.Script
	}
	code := source.ErrorCode("")
	if p.Err != nil {
		code = p.Err.Code
	}
	slog.Info(
		"heartbeat.source.poll",
		"instance_id",
		target.Instance.ID,
		"source_id",
		p.Ref.ID,
		"ref",
		ref,
		"duration_ms",
		p.DurationMS,
		"items",
		len(p.Result.Items),
		"more",
		p.Result.More,
		"result",
		heartbeat.SourceResultStatus(p.Err),
		"error_code",
		code,
		"poll_token",
		token,
	)
}
