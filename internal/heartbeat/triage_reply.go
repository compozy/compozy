package heartbeat

import (
	"encoding/json/v2"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
)

var ErrTriageOutputInvalid = errors.New("triage_output_invalid")

type TriageProposal struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	ItemKeys []string `json:"item_keys"`
}

// ParseTriageReply consumes only the final assistant message selected by the dispatcher.
func ParseTriageReply(reply string, digest CheckDigest, limit int) ([]TriageProposal, error) {
	reply = strings.TrimSpace(reply)
	if strings.HasPrefix(reply, "```json\n") && strings.HasSuffix(reply, "\n```") {
		reply = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(reply, "```json\n"), "\n```"))
	}
	if reply == "NOTHING_TO_REPORT" {
		return nil, nil
	}
	if limit < 1 || !strings.HasPrefix(reply, "{") {
		return nil, ErrTriageOutputInvalid
	}
	var envelope struct {
		Proposals *[]struct {
			Title    *string   `json:"title"`
			Body     *string   `json:"body"`
			ItemKeys *[]string `json:"item_keys"`
		} `json:"proposals"`
	}
	if err := json.Unmarshal([]byte(reply), &envelope, json.RejectUnknownMembers(true)); err != nil || envelope.Proposals == nil {
		return nil, ErrTriageOutputInvalid
	}
	allowed := make(map[string]bool, len(digest.Items))
	for _, item := range digest.Items {
		allowed[item.Key] = true
	}
	proposals := make([]TriageProposal, 0, min(limit, len(*envelope.Proposals)))
	for _, raw := range *envelope.Proposals {
		if raw.Title == nil || raw.Body == nil || raw.ItemKeys == nil || strings.TrimSpace(*raw.Title) == "" || strings.TrimSpace(*raw.Body) == "" {
			return nil, ErrTriageOutputInvalid
		}
		if len(proposals) >= limit {
			continue
		}
		keys := []string{}
		for _, key := range *raw.ItemKeys {
			if allowed[key] {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		proposals = append(proposals, TriageProposal{Title: *raw.Title, Body: *raw.Body, ItemKeys: slices.Compact(keys)})
	}
	return proposals, nil
}

// TriageReplyOutcome converts provider output to a finalization that satisfies the report contract.
func TriageReplyOutcome(reply string, digest CheckDigest, limit int) TriageOutcome {
	proposals, err := ParseTriageReply(reply, digest, limit)
	if err != nil {
		return failedTriage("triage_output_invalid")
	}
	if len(proposals) == 0 {
		return TriageOutcome{Result: "quiet", Reason: "quiet"}
	}
	payload, err := json.Marshal(struct {
		Proposals []TriageProposal `json:"proposals"`
	}{Proposals: proposals})
	if err != nil {
		return failedTriage("triage_output_invalid")
	}
	report := &agentinstance.ReportInput{Kind: "proposal", Payload: payload}
	// Source keys are assigned transactionally by Finish.
	validation := *report
	validation.SourceKey = "occ:validation"
	if err := validation.Validate(); err != nil {
		return failedTriage("triage_output_invalid")
	}
	return TriageOutcome{Result: "reported", Reason: "reported", Report: report}
}
