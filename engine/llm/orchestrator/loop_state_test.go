package orchestrator

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
)

func TestLoopState_RecordCompactionFailureRespectsCooldown(t *testing.T) {
	state := newLoopState(&settings{compactionCooldown: 2}, &MemoryContext{}, &agent.ActionConfig{ID: "a"})
	state.markCompaction(0.8, 0.86)
	require.True(t, state.compactionPending(1, 2))
	failures := state.recordCompactionFailure(1)
	require.Equal(t, 1, failures)
	require.False(t, state.compactionPending(2, 2))
	require.True(t, state.compactionPending(3, 2))
	state.completeCompaction(3)
	require.Equal(t, 0, state.Memory.CompactionFailures)
}

func TestLoopState_ResetCompactionClearsFlags(t *testing.T) {
	state := &loopState{
		Memory: memoryState{CompactionSuggested: true, LastCompactionIteration: 5, CompactionFailures: 2},
	}
	state.Memory.References = []core.MemoryReference{{ID: "mem"}}
	state.resetCompaction()
	require.False(t, state.Memory.CompactionSuggested)
	require.Equal(t, 0, state.Memory.LastCompactionIteration)
	require.Equal(t, 0, state.Memory.CompactionFailures)
	require.Len(t, state.Memory.References, 1)
}
