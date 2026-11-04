package heartbeat

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/agentinstance"
)

var ErrTriageOutputInvalid = errors.New("triage_output_invalid")

const (
	// TriageTitleMaxRunes bounds a proposal title.
	TriageTitleMaxRunes = 120
	// TriageBodyMaxRunes bounds a proposal body.
	TriageBodyMaxRunes = 500
)

type TriageProposal struct {
	Sources  []AutomationEventSource `json:"sources,omitempty"`
	Title    string                  `json:"title"`
	Body     string                  `json:"body"`
	ItemKeys []string                `json:"item_keys"`
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
		Proposals *[]jsontext.Value `json:"proposals"`
	}
	if err := json.Unmarshal([]byte(reply), &envelope, json.RejectUnknownMembers(true)); err != nil {
		return nil, ErrTriageOutputInvalid
	}
	if envelope.Proposals == nil || len(*envelope.Proposals) == 0 {
		return nil, ErrTriageOutputInvalid
	}
	allowed := make(map[string]bool, len(digest.Items))
	for _, item := range digest.Items {
		allowed[item.Key] = true
	}
	proposals := make([]TriageProposal, 0, min(limit, len(*envelope.Proposals)))
	values := *envelope.Proposals
	hasSources := slices.ContainsFunc(
		digest.Items,
		func(item DigestItem) bool { return item.Source != nil && item.Source.Kind == itemSourceKind },
	)
	if !hasSources {
		values = values[:min(limit, len(values))]
	}
	for _, value := range values {
		raw, err := parseTriageProposal(value)
		if err != nil {
			return nil, err
		}
		keys := []string{}
		for _, key := range raw.ItemKeys {
			if allowed[key] {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		proposals = append(proposals, TriageProposal{Title: raw.Title, Body: raw.Body, ItemKeys: slices.Compact(keys),
			Sources: triageProposalSources(digest, keys)})
	}
	if hasSources && len(proposals) > limit {
		proposals = foldSourceProposals(proposals, limit, digest)
	}
	return proposals, nil
}

func parseTriageProposal(value jsontext.Value) (TriageProposal, error) {
	var raw struct {
		Title    *string   `json:"title"`
		Body     *string   `json:"body"`
		ItemKeys *[]string `json:"item_keys"`
	}
	if err := json.Unmarshal(value, &raw, json.RejectUnknownMembers(true)); err != nil {
		return TriageProposal{}, ErrTriageOutputInvalid
	}
	if raw.Title == nil || raw.Body == nil || raw.ItemKeys == nil {
		return TriageProposal{}, ErrTriageOutputInvalid
	}
	if strings.TrimSpace(*raw.Title) == "" || strings.TrimSpace(*raw.Body) == "" ||
		utf8.RuneCountInString(*raw.Title) > TriageTitleMaxRunes ||
		utf8.RuneCountInString(*raw.Body) > TriageBodyMaxRunes {
		return TriageProposal{}, ErrTriageOutputInvalid
	}
	return TriageProposal{Title: *raw.Title, Body: *raw.Body, ItemKeys: *raw.ItemKeys}, nil
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
	var source *AutomationEventSource
	for _, item := range digest.Items {
		if item.Source != nil {
			if item.Source.Kind == itemSourceKind &&
				!slices.ContainsFunc(
					proposals,
					func(p TriageProposal) bool { return slices.Contains(p.ItemKeys, item.Key) },
				) {
				continue
			}
			first := *item.Source
			// The operator's readable line comes from the admitted, fenced event, never the reply.
			first.Summary = readableFencedSummary(item.Summary)
			source = &first
			break
		}
	}
	payload, err := json.Marshal(struct {
		Source    *AutomationEventSource `json:"source,omitempty"`
		Proposals []TriageProposal       `json:"proposals"`
	}{Proposals: proposals, Source: source})
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

// Source attribution comes from the admitted digest, never from provider-authored IDs.
func triageEventSources(digest CheckDigest) []AutomationEventSource {
	var sources []AutomationEventSource
	for _, item := range digest.Items {
		if item.Source != nil {
			sources = append(sources, *item.Source)
		}
	}
	return sources
}

func triageProposalSources(digest CheckDigest, keys []string) []AutomationEventSource {
	out := triageEventSources(digest)
	return slices.DeleteFunc(out, func(s AutomationEventSource) bool {
		return s.Kind == itemSourceKind && !slices.Contains(keys, "source:"+s.SourceID+":"+s.ItemKey)
	})
}
func foldSourceProposals(proposals []TriageProposal, limit int, digest CheckDigest) []TriageProposal {
	last := &proposals[limit-1]
	titles := []string{}
	for _, p := range proposals[limit:] {
		last.ItemKeys = append(last.ItemKeys, p.ItemKeys...)
		titles = append(titles, p.Title)
	}
	slices.Sort(last.ItemKeys)
	last.ItemKeys = slices.Compact(last.ItemKeys)
	body := last.Body
	for i, title := range titles {
		candidate := body + "\n" + title
		suffix := ""
		if left := len(titles) - i - 1; left > 0 {
			suffix = fmt.Sprintf(" …and %d more", left)
		}
		if utf8.RuneCountInString(candidate+suffix) > TriageBodyMaxRunes {
			suffix = fmt.Sprintf(" …and %d more", len(titles)-i)
			runes := []rune(body)
			last.Body = string(runes[:min(len(runes), TriageBodyMaxRunes-utf8.RuneCountInString(suffix))]) + suffix
			last.Sources = triageProposalSources(digest, last.ItemKeys)
			return proposals[:limit]
		}
		body = candidate
	}
	last.Body = body
	last.Sources = triageProposalSources(digest, last.ItemKeys)
	return proposals[:limit]
}
