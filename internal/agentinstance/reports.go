package agentinstance

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ReportInput struct {
	SourceKey   string
	Kind        string
	Payload     jsontext.Value
	Fingerprint string
}

type Report struct {
	ID         string
	InstanceID string
	ReportInput
	State             string
	PromptAdmissionID string
	EventSessionID    string
	EventSeq          int64
	CreatedAt         time.Time
	DeliveredAt       *time.Time
}

type ReportPublication struct {
	Report        Report
	MainSessionID string
}

type ReportQuery struct {
	State          string
	InstanceID     string
	AfterCreatedAt time.Time
	AfterID        string
	Limit          int
}

type ReportStore interface {
	AppendReport(context.Context, string, ReportInput) (Report, error)
	ListReports(context.Context, ReportQuery) ([]Report, error)
	UnpublishedReports(context.Context, string, int) ([]ReportPublication, error)
	MarkReportPublished(context.Context, string, string, int64) error
}

// Publish must durably deduplicate by (sessionID, Report.ID), including after restart.
type ReportEventWriter interface {
	Publish(context.Context, string, Report) (int64, error)
}

type ReportPublisher struct {
	Store     ReportStore
	Writer    ReportEventWriter
	BatchSize int
}

func (p ReportPublisher) Sweep(ctx context.Context) error {
	if p.Store == nil || p.Writer == nil || p.BatchSize <= 0 {
		return errors.New("agentinstance: report publisher requires store, writer and positive batch size")
	}
	var failures []error
	after := ""
	for {
		rows, err := p.Store.UnpublishedReports(ctx, after, p.BatchSize)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, row := range rows {
			after = row.Report.ID
			seq, err := p.Writer.Publish(ctx, row.MainSessionID, row.Report)
			if err == nil {
				err = p.Store.MarkReportPublished(ctx, row.Report.ID, row.MainSessionID, seq)
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("publish report %s: %w", row.Report.ID, err))
			}
		}
		if len(rows) < p.BatchSize {
			return errors.Join(failures...)
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
	}
}

func (r ReportInput) Validate() error {
	prefix, suffix, ok := strings.Cut(r.SourceKey, ":")
	if !ok || strings.TrimSpace(suffix) == "" || (prefix != "occ" && prefix != "interaction" && prefix != "turn") {
		return errors.New("agentinstance: invalid report source key")
	}
	if len(r.Payload) == 0 || len(r.Payload) > 65536 || !r.Payload.IsValid() {
		return errors.New("agentinstance: report payload must be valid JSON within 64 KiB")
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(r.Payload, &fields); err != nil || fields == nil {
		return errors.New("agentinstance: report payload must be an object")
	}
	var required []string
	switch r.Kind {
	case "proposal":
		var proposals []struct {
			Title    string   `json:"title"`
			Body     string   `json:"body"`
			ItemKeys []string `json:"item_keys"`
		}
		if err := json.Unmarshal(fields["proposals"], &proposals); err != nil || len(proposals) == 0 || len(proposals) > 20 {
			return errors.New("agentinstance: invalid proposals")
		}
		for _, proposal := range proposals {
			if strings.TrimSpace(proposal.Title) == "" || strings.TrimSpace(proposal.Body) == "" {
				return errors.New("agentinstance: proposal title and body are required")
			}
		}
	case "failure":
		required = []string{"reason", "detail"}
	case "approval_expired":
		required = []string{"session_id", "tool", "expired_by"}
	case "interrupted":
		required = []string{"session_id", "cause"}
	case "rotation_failed", "rotation_deferred":
		required = []string{"reason", "since"}
	default:
		return errors.New("agentinstance: invalid report kind")
	}
	for _, key := range required {
		var value string
		if err := json.Unmarshal(fields[key], &value); err != nil || strings.TrimSpace(value) == "" {
			return fmt.Errorf("agentinstance: report %s is required", key)
		}
		if key == "expired_by" && value != "timeout" && value != "provider" {
			return errors.New("agentinstance: invalid expiry source")
		}
	}
	return nil
}
