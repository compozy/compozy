package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/task"
)

const (
	maxQuarantineEpisodes   = 32
	maxQuarantineRequeues   = 32
	maxQuarantineEntryBytes = 8 * 1024
)

// QuarantineEntry is the self-contained, sanitized repair record retained across requeues.
type QuarantineEntry struct {
	NodeID    NodeID                 `json:"node_id"`
	InputRef  string                 `json:"input_ref"`
	Target    string                 `json:"target,omitempty"`
	Episodes  []QuarantineEpisode    `json:"episodes"`
	Requeues  []QuarantineProvenance `json:"requeues,omitempty"`
	Truncated bool                   `json:"truncated,omitempty"`
}

// QuarantineEpisode captures the classified attempt chain that caused one quarantine.
type QuarantineEpisode struct {
	Generation    int           `json:"generation"`
	QuarantinedAt time.Time     `json:"quarantined_at"`
	Attempts      []NodeAttempt `json:"attempts"`
}

// QuarantineProvenance attributes one operator or agent requeue request.
type QuarantineProvenance struct {
	ActorKind   string    `json:"actor_kind"`
	ActorID     string    `json:"actor_id"`
	Reason      string    `json:"reason,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
	Generation  int       `json:"generation"`
}

func decodeQuarantineEntry(raw json.RawMessage) (QuarantineEntry, error) {
	if len(raw) == 0 {
		return QuarantineEntry{}, nil
	}
	var entry QuarantineEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return QuarantineEntry{}, fmt.Errorf("%w: decode quarantine entry: %w", ErrValidation, err)
	}
	return entry, nil
}

func encodeQuarantineEntry(entry QuarantineEntry) (json.RawMessage, error) {
	normalizeQuarantineEntry(&entry)
	if len(entry.Episodes) > maxQuarantineEpisodes {
		entry.Episodes = append([]QuarantineEpisode(nil), entry.Episodes[len(entry.Episodes)-maxQuarantineEpisodes:]...)
		entry.Truncated = true
	}
	if len(entry.Requeues) > maxQuarantineRequeues {
		entry.Requeues = append(
			[]QuarantineProvenance(nil),
			entry.Requeues[len(entry.Requeues)-maxQuarantineRequeues:]...,
		)
		entry.Truncated = true
	}
	for {
		redacted, err := marshalSanitizedQuarantineEntry(entry)
		if err != nil {
			return nil, err
		}
		if len(redacted) <= maxQuarantineEntryBytes {
			return json.RawMessage(redacted), nil
		}
		entry.Truncated = true
		if !trimOldestQuarantineEvidence(&entry) {
			return nil, fmt.Errorf(
				"%w: quarantine entry exceeds %d bytes after bounded normalization",
				ErrValidation,
				maxQuarantineEntryBytes,
			)
		}
	}
}

func normalizeQuarantineEntry(entry *QuarantineEntry) {
	if entry == nil {
		return
	}
	entry.NodeID = NodeID(diagnostics.RedactAndBound(strings.TrimSpace(string(entry.NodeID)), actionFailureCodeLimit))
	entry.InputRef = diagnostics.RedactAndBound(strings.TrimSpace(entry.InputRef), 1024)
	entry.Target = diagnostics.RedactAndBound(strings.TrimSpace(entry.Target), 1024)
	for episodeIndex := range entry.Episodes {
		for attemptIndex := range entry.Episodes[episodeIndex].Attempts {
			attempt := &entry.Episodes[episodeIndex].Attempts[attemptIndex]
			attempt.NodeID = NodeID(diagnostics.RedactAndBound(
				strings.TrimSpace(string(attempt.NodeID)), actionFailureCodeLimit,
			))
			attempt.FailureCode = diagnostics.RedactAndBound(attempt.FailureCode, actionFailureCodeLimit)
			attempt.Cause = diagnostics.RedactAndBound(attempt.Cause, actionFailureTextLimit)
			attempt.Hint = diagnostics.RedactAndBound(attempt.Hint, actionFailureTextLimit)
			attempt.Target = diagnostics.RedactAndBound(attempt.Target, 1024)
		}
	}
	for index := range entry.Requeues {
		entry.Requeues[index].ActorKind = diagnostics.RedactAndBound(entry.Requeues[index].ActorKind, 256)
		entry.Requeues[index].ActorID = diagnostics.RedactAndBound(entry.Requeues[index].ActorID, 1024)
		entry.Requeues[index].Reason = diagnostics.RedactAndBound(entry.Requeues[index].Reason, 1024)
	}
}

func marshalSanitizedQuarantineEntry(entry QuarantineEntry) ([]byte, error) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal quarantine entry: %w", err)
	}
	redacted, err := diagnostics.RedactJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("loop: sanitize quarantine entry: %w", err)
	}
	return redacted, nil
}

func trimOldestQuarantineEvidence(entry *QuarantineEntry) bool {
	if entry == nil {
		return false
	}
	if len(entry.Episodes) > 1 {
		entry.Episodes = append([]QuarantineEpisode(nil), entry.Episodes[1:]...)
		return true
	}
	if len(entry.Episodes) == 1 && len(entry.Episodes[0].Attempts) > 1 {
		entry.Episodes[0].Attempts = append([]NodeAttempt(nil), entry.Episodes[0].Attempts[1:]...)
		return true
	}
	if len(entry.Requeues) > 1 {
		entry.Requeues = append([]QuarantineProvenance(nil), entry.Requeues[1:]...)
		return true
	}
	return false
}

func quarantineInputRef(run Run, nodeID NodeID) string {
	return fmt.Sprintf("loop-run:%s:node:%s:input", run.ID, nodeID)
}

func requeueProvenance(actor task.ActorContext, reason string, at time.Time, generation int) QuarantineProvenance {
	return QuarantineProvenance{
		ActorKind:   strings.TrimSpace(string(actor.Actor.Kind)),
		ActorID:     strings.TrimSpace(actor.Actor.Ref),
		Reason:      diagnostics.RedactAndBound(reason, 1024),
		RequestedAt: at.UTC(),
		Generation:  generation,
	}
}
