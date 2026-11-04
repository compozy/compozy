package heartbeat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"html"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/heartbeat/source"
)

type PolledSource struct {
	Polled        bool
	Ref           source.Ref
	RefDigest     string
	Generation    int64
	Result        source.Result
	Err           *source.Error
	FailureStreak int64
	DurationMS    int64
}
type PollBatch struct {
	Token   string
	Sources []PolledSource
}
type SourceOutcome struct {
	ID         string        `json:"id"`
	Result     string        `json:"result"`
	Items      int           `json:"items"`
	More       bool          `json:"more"`
	DurationMS int64         `json:"duration_ms"`
	Error      *source.Error `json:"error,omitempty"`
}
type SourceStatus struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind,omitempty"`
	Script        string        `json:"script,omitempty"`
	Status        string        `json:"status"`
	LastPolledAt  string        `json:"last_polled_at,omitempty"`
	LastSuccessAt string        `json:"last_success_at,omitempty"`
	LastItemCount int64         `json:"last_item_count"`
	FailureStreak int64         `json:"failure_streak"`
	Error         *source.Error `json:"error,omitempty"`
	NextPollAt    string        `json:"next_poll_at,omitempty"`
}
type SourceState struct {
	ID, RefDigest               string
	Generation                  int64
	State                       jsontext.Value
	FailureStreak               int64
	NextPollAt                  time.Time
	Error                       *source.Error
	LastPolledAt, LastSuccessAt string
	LastItemCount               int64
}

func SourceResultStatus(err *source.Error) string {
	if err == nil {
		return "ok"
	}
	switch err.Code {
	case "source_kind_unavailable":
		return "unavailable"
	case "source_scripts_disabled":
		return "disabled"
	case "source_script_unapproved":
		return "unapproved"
	default:
		return "failing"
	}
}
func (b PollBatch) Outcomes() []SourceOutcome {
	out := make([]SourceOutcome, 0, len(b.Sources))
	for _, p := range b.Sources {
		out = append(
			out,
			SourceOutcome{
				ID:         p.Ref.ID,
				Result:     SourceResultStatus(p.Err),
				Items:      len(p.Result.Items),
				More:       p.Result.More,
				DurationMS: p.DurationMS,
				Error:      p.Err,
			},
		)
	}
	return out
}
func (b PollBatch) DigestItems(threshold int) []DigestItem {
	var out []DigestItem
	for _, p := range b.Sources {
		prefix := "source:" + p.Ref.ID + ":"
		if p.Err != nil {
			if SourceResultStatus(p.Err) == "failing" && p.FailureStreak >= int64(threshold) {
				out = append(
					out,
					DigestItem{
						Key:     prefix + "failing:" + string(p.Err.Code),
						Summary: "Source " + p.Ref.ID + " failed: " + string(p.Err.Code),
					},
				)
			}
			continue
		}
		ref := p.Ref.Kind
		if ref == "" {
			ref = "script:" + p.Ref.Script
		}
		budget := source.SummaryBytes
		for i, item := range p.Result.Items {
			itemBudget := budget / (len(p.Result.Items) - i)
			text := source.SanitizeText(
				item.Title+"\n"+item.Summary+"\n"+item.URL+"\n"+string(item.Attributes),
				itemBudget,
			)
			text = source.SanitizeText(html.EscapeString(text), itemBudget)
			budget -= len(text)
			out = append(
				out,
				DigestItem{
					Key: prefix + source.SanitizeText(item.Key, 200),
					Summary: fmt.Sprintf(
						"<untrusted_source_item source=%q>\n%s\n</untrusted_source_item>",
						p.Ref.ID,
						text,
					),
					Source: &AutomationEventSource{
						Kind:      itemSourceKind,
						SourceID:  p.Ref.ID,
						SourceRef: ref,
						ItemKey:   source.SanitizeText(item.Key, 200),
						URL:       source.SanitizeText(item.URL, 2048),
					},
				},
			)
		}
		if p.Result.More {
			out = append(out, DigestItem{Key: prefix + "more", Summary: "More items remain in this source"})
		}
	}
	return out
}
func AppendSourceDigest(digest *CheckDigest, batch PollBatch, threshold int) error {
	digest.SourceMore = batch.HasMore()
	digest.Items = append(digest.Items, batch.DigestItems(threshold)...)
	slices.SortFunc(digest.Items, func(a, b DigestItem) int { return strings.Compare(a.Key, b.Key) })
	data, err := json.Marshal(struct {
		Fingerprint string
		Items       []DigestItem
	}{digest.Fingerprint, digest.Items})
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	digest.Fingerprint = hex.EncodeToString(sum[:])
	return nil
}
func WarnUntrustedScripts(policy *ResolvedPolicy) {
	for _, ref := range policy.Preferences.Context.Sources {
		if ref.Script != "" {
			warning := Diagnostic{
				Code:       "heartbeat_source_untrusted_script",
				Severity:   diagnosticWarning,
				Message:    "Extension-owned source scripts cannot execute",
				SourcePath: policy.SourcePath,
			}
			policy.Diagnostics = append(policy.Diagnostics, warning)
			policy.Status.Diagnostics = append(policy.Status.Diagnostics, warning)
			return
		}
	}
}

const (
	checkHookCancelled      = "hook_cancelled" //nolint:misspell // Persisted wake reason.
	checkTimerSource        = "timer"
	checkManualSource       = "instance_check"
	checkNoSlot             = "no_slot"
	checkSourcePollInFlight = "source_poll_in_flight"
	itemSourceKind          = "source"
)

func (b PollBatch) HasMore() bool {
	return slices.ContainsFunc(b.Sources, func(p PolledSource) bool { return p.Err == nil && p.Result.More })
}
