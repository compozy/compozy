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
	triageQuietReply = "NOTHING_TO_REPORT"
	// TriageTitleMaxRunes bounds a proposal title.
	TriageTitleMaxRunes = 120
	// TriageBodyMaxRunes bounds a proposal body.
	TriageBodyMaxRunes = 500
)

type TriageProposal struct {
	Action   string                  `json:"action,omitempty"`
	Sources  []AutomationEventSource `json:"sources,omitempty"`
	Title    string                  `json:"title"`
	Body     string                  `json:"body"`
	ItemKeys []string                `json:"item_keys"`
}

// ParseTriageReply consumes only the final assistant message selected by the dispatcher.
func ParseTriageReply(reply string, digest CheckDigest, limit int, actions ...ActionDecl) ([]TriageProposal, error) {
	reply = strings.TrimSpace(reply)
	var err error
	reply, err = unwrapTriageFence(reply)
	if err != nil {
		return nil, err
	}
	if reply == triageQuietReply {
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
		if raw.Action != "" && !slices.ContainsFunc(actions, func(a ActionDecl) bool { return a.ID == raw.Action }) {
			raw.Action = ""
		}
		keys := []string{}
		for _, key := range raw.ItemKeys {
			if allowed[key] {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		proposals = append(
			proposals,
			TriageProposal{Action: raw.Action, Title: raw.Title, Body: raw.Body, ItemKeys: slices.Compact(keys),
				Sources: triageProposalSources(digest, keys)},
		)
	}
	if hasSources && len(proposals) > limit {
		proposals = foldSourceProposals(proposals, limit, digest)
	}
	return proposals, nil
}

func parseTriageProposal(value jsontext.Value) (TriageProposal, error) {
	var raw struct {
		Action   string    `json:"action,omitempty"`
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
	return TriageProposal{Action: raw.Action, Title: *raw.Title, Body: *raw.Body, ItemKeys: *raw.ItemKeys}, nil
}

// TriageReplyOutcome converts provider output to a finalization that satisfies the report contract.
func TriageReplyOutcome(reply string, digest CheckDigest, limit int, actions ...ActionDecl) TriageOutcome {
	proposals, err := ParseTriageReply(reply, digest, limit, actions...)
	if err != nil {
		return invalidTriageReply(reply, err)
	}
	if len(proposals) == 0 {
		return TriageOutcome{Result: "quiet", Reason: "quiet"}
	}
	var diagnostics []string
	unwrapped, err := unwrapTriageFence(strings.TrimSpace(reply))
	if err != nil {
		return invalidTriageReply(reply, err)
	}
	var raw struct {
		Proposals []struct {
			Action string `json:"action"`
		} `json:"proposals"`
	}
	if err := json.Unmarshal([]byte(unwrapped), &raw); err == nil {
		for _, p := range raw.Proposals {
			if p.Action != "" && !slices.ContainsFunc(actions, func(a ActionDecl) bool { return a.ID == p.Action }) {
				diagnostics = []string{"check_in_action_unknown"}
				break
			}
		}
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
		Diagnostics []string               `json:"diagnostics,omitempty"`
		Source      *AutomationEventSource `json:"source,omitempty"`
		Proposals   []TriageProposal       `json:"proposals"`
	}{Proposals: proposals, Source: source, Diagnostics: diagnostics})
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
	for _, p := range proposals[limit:] {
		if last.Action != p.Action {
			last.Action = ""
		}
		last.ItemKeys = append(last.ItemKeys, p.ItemKeys...)
	}
	slices.Sort(last.ItemKeys)
	last.ItemKeys = slices.Compact(last.ItemKeys)
	// Share the text budget so a full first body cannot hide later findings.
	folded := proposals[limit-1:]
	sections := make([]string, len(folded))
	budget := TriageBodyMaxRunes
	for i, p := range folded {
		allowance := budget / (len(folded) - i)
		prefix := ""
		if i > 0 {
			prefix = "\n" + boundedTriageText(p.Title, allowance/3) + ": "
		}
		bodyBudget := max(0, allowance-utf8.RuneCountInString(prefix))
		sections[i] = boundedTriageText(prefix+boundedTriageText(p.Body, bodyBudget), allowance)
		budget -= utf8.RuneCountInString(sections[i])
	}
	last.Body = strings.Join(sections, "")
	last.Sources = triageProposalSources(digest, last.ItemKeys)
	return proposals[:limit]
}

func boundedTriageText(text string, budget int) string {
	runes := []rune(text)
	if len(runes) <= budget {
		return text
	}
	if budget == 0 {
		return ""
	}
	return string(runes[:budget-1]) + "…"
}

// invalidTriageReply generates diagnostics without copying potentially secret reply characters.
func invalidTriageReply(reply string, cause error) TriageOutcome {
	outcome := failedTriage("triage_output_invalid")
	reply = strings.TrimSpace(reply)
	format := "unfenced"
	if strings.Contains(reply, "```") {
		format = "fenced"
	}
	failure := "schema"
	if errors.Is(cause, errTriageFraming) {
		failure = "framing"
	} else if errors.Is(cause, errTriageAmbiguous) {
		failure = "ambiguous_reply"
	}
	payload, err := json.Marshal(struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}{Reason: outcome.Reason, Detail: "The proactive check returned an invalid response. " +
		"Output excerpt: [REDACTED]. Format: " + format + ". Validation: " + failure + "."})
	if err != nil {
		return outcome
	}
	outcome.Report.Payload = payload
	return outcome
}

var (
	errTriageFraming   = fmt.Errorf("%w: framing", ErrTriageOutputInvalid)
	errTriageAmbiguous = fmt.Errorf("%w: ambiguous_reply", ErrTriageOutputInvalid)
)

// unwrapTriageFence isolates one value without letting surrounding text supply another candidate.
func unwrapTriageFence(reply string) (string, error) {
	if reply == triageQuietReply {
		return reply, nil
	}
	start := strings.IndexAny(reply, "{[")
	fence := strings.Index(reply, "```")
	if fence >= 0 && (start < 0 || fence < start) {
		start = fence
	}
	if start < 0 {
		return "", errTriageFraming
	}
	prefix, candidate := reply[:start], reply[start:]
	var body, suffix string
	if strings.HasPrefix(candidate, "```") {
		opener, content, ok := strings.Cut(candidate, "\n")
		opener = strings.TrimSpace(opener)
		if !ok || (opener != "```" && !strings.EqualFold(opener, "```json")) {
			return "", errTriageFraming
		}
		offset := 0
		found := false
		for line := range strings.SplitSeq(content, "\n") {
			if strings.TrimSpace(line) == "```" {
				body = content[:offset]
				suffix = content[min(offset+len(line)+1, len(content)):]
				found = true
				break
			}
			offset += len(line) + 1
		}
		if !found {
			return "", errTriageFraming
		}
	} else {
		decoder := jsontext.NewDecoder(strings.NewReader(candidate))
		value, err := decoder.ReadValue()
		if err != nil {
			return "", errTriageFraming
		}
		body, suffix = string(value), candidate[decoder.InputOffset():]
	}
	if ambiguousTriageCommentary(prefix) || ambiguousTriageCommentary(suffix) {
		return "", errTriageAmbiguous
	}
	body = strings.TrimSpace(body)
	if body == triageQuietReply && (strings.TrimSpace(prefix) != "" || strings.TrimSpace(suffix) != "") {
		return "", errTriageAmbiguous
	}
	return body, nil
}

func ambiguousTriageCommentary(text string) bool {
	return strings.ContainsAny(text, "{}[]") || strings.Contains(text, "```") ||
		strings.Contains(text, triageQuietReply)
}
