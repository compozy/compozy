package daemon

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/session/contextusage"
)

type instanceRotationStore interface {
	RotationWaitingSince(context.Context, string, string, time.Time) (time.Time, error)
	ClearRotationWait(context.Context, string, string) error
	SupersededSessions(context.Context, string) ([]string, error)
	RotationAdmissionCount(context.Context, string) (int64, error)
	RotationPersistenceReady(context.Context, string) (bool, error)
}

type instanceRotationSessions interface {
	loopSessionEventReader
	Status(context.Context, string) (*session.Info, error)
	Stop(context.Context, string) error
}

type instanceRotationMonitors interface {
	List(string) []monitor.Monitor
}

type instanceRotationStarter interface {
	StartSuccessor(context.Context, agentinstance.Instance, string) (string, error)
}

type instanceMainRotator struct {
	retryMu    sync.Mutex
	retryAfter map[string]time.Time
	retryDelay time.Duration
	changed    func(agentinstance.Instance)
	starter    instanceRotationStarter
	reports    agentinstance.ReportStore
	now        func() time.Time
	sessions   instanceRotationSessions
	store      instanceRotationStore
	monitors   instanceRotationMonitors
	threshold  float64
}

func (r *instanceMainRotator) Eligible(ctx context.Context, instance agentinstance.Instance) (decision agentinstance.RotationDecision, err error) {
	decision = agentinstance.RotationDecision{Publication: agentinstance.RotationPublication{InstanceID: instance.ID, PredecessorID: instance.MainSessionID}}
	if instance.State != "active" || instance.ArchivedAt != nil || instance.MainSessionID == "" {
		return decision, nil
	}
	if r.rotationRetryPending(instance.MainSessionID) {
		return decision, nil
	}
	count, err := r.store.RotationAdmissionCount(ctx, instance.MainSessionID)
	if err != nil {
		return decision, err
	}
	decision.Publication.AdmissionCount = count
	info, err := r.sessions.Status(ctx, instance.MainSessionID)
	if err != nil {
		return decision, err
	}
	telemetry := loopGoalContextRuntime{sessions: r.sessions}
	usage, err := telemetry.Usage(ctx, loop.ActionSessionBinding{SessionID: instance.MainSessionID})
	if err != nil {
		return decision, err
	}
	_, hasCompact := session.ResolveCompactionCommand(info.AdvertisedCommands)
	if contextusage.DecideMaintenance(usage.Known, usage.Used, usage.Size, r.threshold, hasCompact) != contextusage.MaintenanceRotate {
		return decision, r.store.ClearRotationWait(ctx, instance.ID, instance.MainSessionID)
	}
	defer func() {
		if err == nil {
			err = r.recordRotationDeferral(ctx, instance, decision.Eligible)
		}
	}()
	if info.State != session.StateActive || info.PendingPermissionCount > 0 || info.PendingClarifyCount > 0 {
		return decision, nil
	}
	if info.Liveness != nil && info.Liveness.Activity != nil && info.Liveness.Activity.TurnID != "" {
		return decision, nil
	}

	for _, item := range r.monitors.List(instance.MainSessionID) {
		if item.Status == monitor.StatusActive || item.Status == monitor.StatusDelivering {
			return decision, nil
		}
	}
	decision.Eligible, err = r.store.RotationPersistenceReady(ctx, instance.MainSessionID)
	return decision, err
}

func (r *instanceMainRotator) Cleanup(ctx context.Context, instance agentinstance.Instance) error {
	ids, err := r.store.SupersededSessions(ctx, instance.ID)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err := r.sessions.Stop(ctx, id); err != nil && !errors.Is(err, session.ErrSessionNotFound) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (r *instanceMainRotator) rotationRetryPending(id string) bool {
	r.retryMu.Lock()
	defer r.retryMu.Unlock()
	until, found := r.retryAfter[id]
	if found && r.now().Before(until) {
		return true
	}
	delete(r.retryAfter, id)
	return false
}
func (r *instanceMainRotator) deferRotationRetry(id string) {
	r.retryMu.Lock()
	defer r.retryMu.Unlock()
	now := r.now()
	for key, until := range r.retryAfter {
		if !now.Before(until) {
			delete(r.retryAfter, key)
		}
	}
	if r.retryAfter == nil {
		r.retryAfter = make(map[string]time.Time)
	}
	r.retryAfter[id] = now.Add(r.retryDelay)
}
