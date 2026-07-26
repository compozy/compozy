package daemon

import (
	"strings"
	"sync"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
)

type managedGoalUsageReporterEntry struct {
	revision uint64
	reporter looppkg.ActionUsageReporter
}

type managedGoalUsageReporters struct {
	mu       sync.Mutex
	next     uint64
	byTicket map[string]managedGoalUsageReporterEntry
}

func (r *managedGoalUsageReporters) install(
	queueEntryID string,
	reporter looppkg.ActionUsageReporter,
) func() {
	key := strings.TrimSpace(queueEntryID)
	if r == nil || key == "" || reporter == nil {
		return func() {}
	}
	r.mu.Lock()
	if r.byTicket == nil {
		r.byTicket = make(map[string]managedGoalUsageReporterEntry)
	}
	r.next++
	revision := r.next
	r.byTicket[key] = managedGoalUsageReporterEntry{revision: revision, reporter: reporter}
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		entry, ok := r.byTicket[key]
		if ok && entry.revision == revision {
			delete(r.byTicket, key)
		}
		r.mu.Unlock()
	}
}

func (r *managedGoalUsageReporters) lookup(queueEntryID string) looppkg.ActionUsageReporter {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byTicket[strings.TrimSpace(queueEntryID)].reporter
}

func (r *managedGoalUsageReporters) remove(queueEntryID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.byTicket, strings.TrimSpace(queueEntryID))
	r.mu.Unlock()
}

func (l *loopGoalManagedInputLifecycle) usageReporter(queueEntryID string) session.ManagedInputUsageReporter {
	if l == nil || l.binder == nil {
		return nil
	}
	return l.binder.usageReporters.lookup(queueEntryID)
}

func (l *loopGoalManagedInputLifecycle) clearUsageReporter(queueEntryID string) {
	if l == nil || l.binder == nil {
		return
	}
	l.binder.usageReporters.remove(queueEntryID)
}
