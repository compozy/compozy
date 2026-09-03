package terminal

import "sync/atomic"

// auditGate is the task_03 seam consulted by the single input choke point.
type auditGate struct {
	blocked atomic.Bool
}

func (g *auditGate) Blocked() bool { return g != nil && g.blocked.Load() }

func (g *auditGate) SetBlocked(blocked bool) {
	if g != nil {
		g.blocked.Store(blocked)
	}
}
