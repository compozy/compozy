package agentinstance

import (
	"context"
	"errors"
	"testing"
)

type reportStoreStub struct {
	rows   []ReportPublication
	marked []string
}

var _ ReportStore = (*reportStoreStub)(nil)

func (s *reportStoreStub) AppendReport(context.Context, string, ReportInput) (Report, error) {
	return Report{}, nil
}
func (s *reportStoreStub) ListReports(context.Context, ReportQuery) ([]Report, error) {
	return nil, nil
}
func (s *reportStoreStub) UnpublishedReports(_ context.Context, after string, limit int) ([]ReportPublication, error) {
	var rows []ReportPublication
	for _, r := range s.rows {
		if r.Report.ID > after && len(rows) < limit {
			rows = append(rows, r)
		}
	}
	return rows, nil
}
func (s *reportStoreStub) MarkReportPublished(_ context.Context, id, session string, seq int64) error {
	s.marked = append(s.marked, id)
	return nil
}

type reportWriterStub struct {
	fail      string
	published []string
}

var _ ReportEventWriter = (*reportWriterStub)(nil)

func (w *reportWriterStub) Publish(_ context.Context, session string, r Report) (int64, error) {
	if r.ID == w.fail {
		return 0, errors.New("event store unavailable")
	}
	w.published = append(w.published, session+":"+r.ID)
	return 1, nil
}
func TestReportPublisher(t *testing.T) {
	t.Run("Should continue beyond failed publications and retry them on the next sweep", func(t *testing.T) {
		t.Parallel()
		store := &reportStoreStub{rows: []ReportPublication{{Report: Report{ID: "a"}, MainSessionID: "main"}, {Report: Report{ID: "b"}, MainSessionID: "successor"}}}
		writer := &reportWriterStub{fail: "a"}
		publisher := ReportPublisher{Store: store, Writer: writer, BatchSize: 1}
		if err := publisher.Sweep(t.Context()); err == nil {
			t.Fatal("expected publication failure")
		}
		if len(store.marked) != 1 || store.marked[0] != "b" {
			t.Fatalf("marked: %v", store.marked)
		}
		if len(writer.published) != 1 || writer.published[0] != "successor:b" {
			t.Fatalf("published: %v", writer.published)
		}
		writer.fail = ""
		if err := publisher.Sweep(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(store.marked) != 3 || store.marked[1] != "a" {
			t.Fatalf("retry marked: %v", store.marked)
		}
	})
}
func TestReportInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input ReportInput
		valid bool
	}{
		{"Should accept a bounded failure", ReportInput{SourceKey: "occ:1", Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"provider exited"}`)}, true},
		{"Should reject an unknown source", ReportInput{SourceKey: "random:1", Kind: "failure", Payload: []byte(`{}`)}, false},
		{"Should reject missing payload fields", ReportInput{SourceKey: "occ:1", Kind: "failure", Payload: []byte(`{}`)}, false},
		{"Should reject an unknown expiry source", ReportInput{SourceKey: "interaction:1", Kind: "approval_expired", Payload: []byte(`{"session_id":"s","tool":"read","expired_by":"other"}`)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.input.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, valid = %v", err, tc.valid)
			}
		})
	}
}
