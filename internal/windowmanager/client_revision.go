package windowmanager

import "fmt"

// MaxWireRevision is the largest revision exactly representable by JSON clients.
const MaxWireRevision uint64 = 1<<53 - 1

// NextTopologyRevision returns the next JSON-safe durable topology revision.
func NextTopologyRevision(current Revision) (Revision, error) {
	if current >= Revision(MaxWireRevision) {
		return 0, fmt.Errorf(
			"topology revision %d cannot advance: %w",
			current,
			ErrTopologyRevisionExhausted,
		)
	}
	return current + 1, nil
}

func nextPresentationRevision(current uint64) (uint64, error) {
	if current >= MaxWireRevision {
		return 0, fmt.Errorf(
			"presentation revision %d cannot advance: %w",
			current,
			ErrPresentationRevisionExhausted,
		)
	}
	return current + 1, nil
}
