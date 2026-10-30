package sessionprocess

import (
	"cmp"
	"context"
	"os"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func identity(p procutil.ScannedProcess) Identity {
	return Identity{PID: p.PID, StartedAt: p.StartedAt}
}

type attributionScan struct {
	byPID           map[int]procutil.ScannedProcess
	leaders         map[int]Owner
	protectedGroups map[int]bool
	known           map[string]Owner
	next            map[Identity]Observation
	groups          map[int]Observation
}

// Reconciliation requires exact identities for retention and live ancestry for new observations.
// A remembered PGID is never authority unless an earlier member still proves continuity.
func (s *Service) reconcile(
	ctx context.Context,
	snap procutil.ProcessSnapshot,
	owners []Owner,
	full bool,
) (map[Identity]Observation, []Process, error) {
	scan := newAttributionScan(snap, owners)
	if err := scan.retain(ctx, s.source, s.observations); err != nil {
		return nil, nil, err
	}
	for _, p := range snap.Processes {
		if err := scan.discover(ctx, s.source, p, snap.ScannedAt, full); err != nil {
			return nil, nil, err
		}
	}
	scan.linkGroups()
	terminals, err := s.source.Terminals(ctx)
	if err != nil {
		return nil, nil, err
	}
	terminalGroups := scan.addTerminals(terminals, snap.ScannedAt)
	return scan.next, scan.processes(terminalGroups), nil
}

func newAttributionScan(snap procutil.ProcessSnapshot, owners []Owner) *attributionScan {
	scan := &attributionScan{
		byPID: make(map[int]procutil.ScannedProcess, len(snap.Processes)), leaders: make(map[int]Owner),
		protectedGroups: make(map[int]bool), known: make(map[string]Owner),
		next: make(map[Identity]Observation), groups: make(map[int]Observation),
	}
	for _, p := range snap.Processes {
		if !p.Zombie {
			scan.byPID[p.PID] = p
		}
	}
	if self, ok := scan.byPID[os.Getpid()]; ok {
		scan.protectedGroups[self.PGID] = true
	}
	for _, owner := range owners {
		scan.known[owner.SessionID] = owner
		p, ok := scan.byPID[owner.LeaderPID]
		if !ok {
			continue
		}
		// Never expose a published leader group while its identity is uncertain.
		// Only a verified identity grants ancestry attribution to its descendants.
		scan.protectedGroups[p.PGID] = true
		matches := !owner.LeaderStartedAt.IsZero() && p.StartedAt.Sub(owner.LeaderStartedAt).Abs() < 2*time.Second
		if owner.LeaderKernelToken != "" {
			matches = owner.LeaderKernelToken == p.KernelToken
		}
		if matches {
			scan.leaders[p.PID] = owner
			scan.protectedGroups[p.PGID] = true
		}
	}
	return scan
}

// Legacy records without a kernel token retain the exact timestamp fence.
func observationMatches(old Observation, p procutil.ScannedProcess) bool {
	if old.KernelToken != "" {
		return old.KernelToken == p.KernelToken
	}
	return p.StartedAt.Equal(old.StartedAt)
}

func (scan *attributionScan) retain(ctx context.Context, source Source, previous map[Identity]Observation) error {
	for id, old := range previous {
		p, ok := scan.byPID[id.PID]
		if !ok || !observationMatches(old, p) {
			continue
		}
		owner, known := scan.known[old.Owner.SessionID]
		if !known {
			var err error
			owner, known, err = source.Owner(ctx, old.Owner.SessionID)
			if err != nil {
				return err
			}
			if !known {
				owner = old.Owner
				owner.Live = false
				owner.LeaderPID = 0
				owner.Starting = false
			}
			scan.known[old.Owner.SessionID] = owner
		}
		if !old.GroupObservedAt.IsZero() {
			old.GroupObservedAt = old.GroupObservedAt.Add(p.StartedAt.Sub(old.StartedAt))
		}
		id = identity(p)
		old.Identity = id
		old.KernelToken = p.KernelToken
		old.Owner = owner
		if old.PGID != p.PGID {
			old.GroupObservedAt = time.Time{}
		}
		old.PGID = p.PGID
		scan.next[id] = old
		if !old.GroupObservedAt.IsZero() {
			scan.groups[p.PGID] = old
		}
	}
	return nil
}

func (scan *attributionScan) envOwner(
	ctx context.Context,
	source Source,
	p procutil.ScannedProcess,
) (Owner, bool, error) {
	if p.SessionID == "" {
		return Owner{}, false, nil
	}
	if owner, ok := scan.known[p.SessionID]; ok {
		return owner, true, nil
	}
	owner, ok, err := source.Owner(ctx, p.SessionID)
	if err == nil && !ok {
		owner, ok, err = source.OrphanOwner(ctx, p)
	}
	if ok {
		scan.known[p.SessionID] = owner
	}
	return owner, ok, err
}

func (scan *attributionScan) discover(
	ctx context.Context,
	source Source,
	p procutil.ScannedProcess,
	now time.Time,
	full bool,
) error {
	if p.Zombie || p.PID <= 1 || p.PID == os.Getpid() {
		return nil
	}
	if _, leader := scan.leaders[p.PID]; leader {
		return nil
	}
	id := identity(p)
	old, retained := scan.next[id]
	owner, attached := ancestorOwner(p, scan.byPID, scan.leaders)
	attribution := attributionLineage
	envOwner, marked, err := scan.envOwner(ctx, source, p)
	if err != nil {
		return err
	}
	if p.SessionID != "" && !marked && !retained && !attached {
		return nil
	}
	switch {
	case marked:
		owner, attribution = envOwner, "env"
	case attached:
	case retained:
		owner, attribution = old.Owner, old.Attribution
		if full && p.SessionID == "" {
			attribution = attributionLineage
		}
	default:
		anchor, ok := scan.groups[p.PGID]
		if !ok || p.StartedAt.Before(anchor.GroupObservedAt) {
			return nil
		}
		owner, old.GroupObservedAt = anchor.Owner, anchor.GroupObservedAt
	}
	if owner.SessionID == "" || owner.Starting {
		return nil
	}
	scan.recordObservation(old, id, owner, attribution, p, now)
	return nil
}

func (scan *attributionScan) recordObservation(
	old Observation,
	id Identity,
	owner Owner,
	attribution string,
	p procutil.ScannedProcess,
	now time.Time,
) {
	old.Identity, old.Owner, old.PGID, old.Attribution = id, owner, p.PGID, attribution
	old.KernelToken = p.KernelToken
	if old.FirstSeen.IsZero() {
		old.FirstSeen = now
	}
	if _, direct := scan.leaders[p.PPID]; direct && p.PGID != 0 && !scan.protectedGroups[p.PGID] {
		// The shell's birth includes children already forked before the first scan.
		old.GroupObservedAt = p.StartedAt
	}
	scan.next[id] = old
}

// Carry direct-shell evidence onto observed members before the shell exits.
func (scan *attributionScan) linkGroups() {
	for _, o := range scan.next {
		if !o.GroupObservedAt.IsZero() && !scan.protectedGroups[o.PGID] {
			scan.groups[o.PGID] = o
		}
	}
	for id, o := range scan.next {
		anchor, ok := scan.groups[o.PGID]
		if ok && anchor.Owner.SessionID == o.Owner.SessionID && !o.StartedAt.Before(anchor.GroupObservedAt) {
			o.GroupObservedAt = anchor.GroupObservedAt
			scan.next[id] = o
		}
	}
}

func (scan *attributionScan) addTerminals(terminals []Terminal, now time.Time) map[int]Terminal {
	groups := make(map[int]Terminal)
	for _, terminal := range terminals {
		p, ok := scan.byPID[terminal.PID]
		if !ok || terminal.StartedAt.IsZero() || p.StartedAt.Sub(terminal.StartedAt).Abs() >= 2*time.Second {
			continue
		}
		id := identity(p)
		if terminal.Owner.ProfileID == "" {
			old, retained := scan.next[id]
			if !retained {
				continue
			}
			terminal.Owner = old.Owner
		}
		groups[p.PGID] = terminal
		observation, retained := scan.next[id]
		if !retained {
			observation = Observation{Identity: id, FirstSeen: now, Attribution: attributionLineage}
		}
		observation.Owner, observation.PGID = terminal.Owner, p.PGID
		scan.next[id] = observation
	}
	return groups
}

func (scan *attributionScan) processes(terminals map[int]Terminal) []Process {
	rows := make([]Process, 0, len(scan.next))
	for id, observation := range scan.next {
		p := scan.byPID[id.PID]
		if observation.Owner.Starting || scan.protectedGroups[p.PGID] || p.PID == os.Getpid() {
			continue
		}
		_, attached := ancestorOwner(p, scan.byPID, scan.leaders)
		kind, terminalID := "background", ""
		if attached {
			kind = "tool"
		}
		if terminal, ok := terminals[p.PGID]; ok {
			if p.PID != terminal.PID {
				continue
			}
			kind, terminalID = "terminal", terminal.ID
		}
		// Marker fields are internal authority only; raw environments never reach callers.
		p.SessionID = observation.Owner.SessionID
		rows = append(rows, Process{ScannedProcess: p, Owner: observation.Owner, Kind: kind,
			Attribution: observation.Attribution, TerminalID: terminalID})
	}
	slices.SortFunc(rows, func(a, b Process) int { return cmp.Compare(a.PID, b.PID) })
	return rows
}

func ancestorOwner(
	p procutil.ScannedProcess,
	rows map[int]procutil.ScannedProcess,
	leaders map[int]Owner,
) (Owner, bool) {
	seen := make(map[int]bool)
	for p.PPID > 1 && !seen[p.PPID] {
		seen[p.PPID] = true
		if owner, ok := leaders[p.PPID]; ok {
			return owner, true
		}
		parent, ok := rows[p.PPID]
		if !ok || parent.StartedAt.After(p.StartedAt) {
			break
		}
		p = parent
	}
	return Owner{}, false
}

func observationChanges(previous, next map[Identity]Observation) ([]Observation, []Identity) {
	updates, deletes := []Observation{}, []Identity{}
	for id, observation := range next {
		if old, ok := previous[id]; !ok || persistedObservation(old) != persistedObservation(observation) {
			updates = append(updates, observation)
		}
	}
	for id := range previous {
		if _, ok := next[id]; !ok {
			deletes = append(deletes, id)
		}
	}
	return updates, deletes
}

func persistedObservation(o Observation) Observation {
	// Runtime demand and leader state are not persisted observation fields.
	o.Owner.LeaderPID, o.Owner.LeaderStartedAt = 0, time.Time{}
	o.Owner.LeaderKernelToken = ""
	o.Owner.Live, o.Owner.ToolBusy, o.Owner.Starting = false, false, false
	return o
}
