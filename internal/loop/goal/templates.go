package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

const (
	promptKindWork         = "work"
	promptKindContinuation = "continuation"
	promptKindCompact      = "compact"
)

type promptIdentitySeed struct {
	RunID      string `json:"run_id"`
	Generation int    `json:"generation"`
	NodeID     string `json:"node_id"`
	ItemIndex  int    `json:"item_index"`
	Turn       int    `json:"turn"`
	Kind       string `json:"kind"`
	Attempt    int    `json:"attempt"`
}

type judgeIdentitySeed struct {
	RunID       string `json:"run_id"`
	Generation  int    `json:"generation"`
	NodeID      string `json:"node_id"`
	ItemIndex   int    `json:"item_index"`
	Turn        int    `json:"turn"`
	JudgeDigest string `json:"judge_digest"`
}

func deterministicPromptID(key TurnKey, turn int, kind string, attempt int) (string, error) {
	digest, err := stableDigest(promptIdentitySeed{
		RunID:      string(key.LoopRunID),
		Generation: key.Generation,
		NodeID:     string(key.NodeID),
		ItemIndex:  key.ItemIndex,
		Turn:       turn,
		Kind:       kind,
		Attempt:    attempt,
	})
	if err != nil {
		return "", err
	}
	return "goal-prompt:" + digest, nil
}

func deterministicJudgeIdentity(
	key TurnKey,
	turn int,
	criteria []dsl.GateCriterion,
) (string, string, error) {
	digest, err := stableDigest(criteria)
	if err != nil {
		return "", "", err
	}
	attemptDigest, err := stableDigest(judgeIdentitySeed{
		RunID:       string(key.LoopRunID),
		Generation:  key.Generation,
		NodeID:      string(key.NodeID),
		ItemIndex:   key.ItemIndex,
		Turn:        turn,
		JudgeDigest: digest,
	})
	if err != nil {
		return "", "", err
	}
	return "goal-judge:" + attemptDigest, digest, nil
}

func stableDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal deterministic Goal identity: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func renderWorkPrompt(segment *segmentState, turn int, kind string) string {
	var prompt strings.Builder
	prompt.WriteString("You are advancing a durable Goal.\n\nObjective:\n")
	prompt.WriteString(strings.TrimSpace(segment.params.Objective))
	prompt.WriteString("\n\nThis is work turn ")
	fmt.Fprintf(&prompt, "%d of %d", turn, segment.checkpoint.TurnLimit)
	prompt.WriteString(
		". Make concrete progress, preserve existing correct work, " +
			"and stop at one provider turn boundary.",
	)
	if kind == promptKindContinuation {
		prompt.WriteString(
			" This is a continuation: inspect the current durable/session state " +
				"and do not replay an earlier prompt blindly.",
		)
	}
	if segment.lastVerdict != "" {
		prompt.WriteString("\n\nLast authoritative judge outcome: ")
		prompt.WriteString(segment.lastVerdict)
		prompt.WriteString(".")
	}
	if len(segment.lastBlockingIssues) > 0 {
		prompt.WriteString("\n\nPrevious blocking issues:")
		for _, issue := range segment.lastBlockingIssues {
			id := strings.TrimSpace(issue.ID)
			note := strings.TrimSpace(issue.Note)
			if id == "" && note == "" {
				continue
			}
			prompt.WriteString("\n- ")
			if id != "" {
				prompt.WriteString("[")
				prompt.WriteString(id)
				prompt.WriteString("] ")
			}
			prompt.WriteString(note)
		}
	}
	prompt.WriteString(
		"\n\nThe Goal engine will evaluate the pinned criteria after this turn. " +
			"Use the Goal report tool only for an explicit complete or evidenced blocked boundary; " +
			"reporting does not end the current prompt.",
	)
	return prompt.String()
}

func renderCompactionPrompt(segment *segmentState) string {
	return fmt.Sprintf(
		"Compact the active session context for this durable Goal without changing its objective or claiming completion. "+
			"Preserve decisions, evidence, current work, and the pinned criteria.\n\nObjective:\n%s",
		strings.TrimSpace(segment.params.Objective),
	)
}
