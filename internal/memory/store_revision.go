package memory

import "sync/atomic"

type storeMutationRevision struct {
	value atomic.Uint64
}

func (s *Store) recordCommittedMutation() {
	if s == nil || s.mutationRevision == nil {
		return
	}
	s.mutationRevision.value.Add(1)
}

func (s *Store) committedMutationRevision() uint64 {
	if s == nil || s.mutationRevision == nil {
		return 0
	}
	return s.mutationRevision.value.Load()
}

func (s *Store) lockControllerDecisions() func() {
	if s == nil || s.decisionMu == nil {
		return func() {}
	}
	s.decisionMu.Lock()
	return s.decisionMu.Unlock
}
