package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/listcursor"
)

const (
	// DefaultListLimit bounds automation list responses when callers omit a limit.
	DefaultListLimit = 50
	// MaxListLimit bounds one automation list response.
	MaxListLimit = 200

	listCursorVersion = 1
	jobListCursorKind = "job"
	triggerCursorKind = "trigger"
)

var (
	// ErrListCursorInvalid reports a malformed cursor or one reused with different filters.
	ErrListCursorInvalid = errors.New("automation: list cursor is invalid")
)

// JobListPage is one stable page of automation jobs.
type JobListPage struct {
	Jobs       []Job  `json:"jobs"`
	Total      int    `json:"total"`
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// TriggerListPage is one stable page of automation triggers.
type TriggerListPage struct {
	Triggers   []Trigger `json:"triggers"`
	Total      int       `json:"total"`
	Limit      int       `json:"limit"`
	HasMore    bool      `json:"has_more"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// ListCursorPosition is the stable source/name/id anchor carried by automation list cursors.
type ListCursorPosition struct {
	Source JobSource `json:"s"`
	Name   string    `json:"n"`
	ID     string    `json:"i"`
}

type jobListFingerprint struct {
	Kind        string    `json:"kind"`
	Scope       Scope     `json:"scope"`
	WorkspaceID string    `json:"workspace_id"`
	Source      JobSource `json:"source"`
	LoopName    string    `json:"loop_name"`
	Enabled     *bool     `json:"enabled"`
	Search      string    `json:"q"`
}

type triggerListFingerprint struct {
	Kind        string    `json:"kind"`
	Scope       Scope     `json:"scope"`
	WorkspaceID string    `json:"workspace_id"`
	Event       string    `json:"event"`
	Source      JobSource `json:"source"`
	LoopName    string    `json:"loop_name"`
	Enabled     *bool     `json:"enabled"`
	Search      string    `json:"q"`
}

// JobListQueryWithDefaultLimit normalizes a public job query and applies the default page size.
func JobListQueryWithDefaultLimit(query JobListQuery) JobListQuery {
	query = normalizeJobListQuery(query)
	if query.Limit == 0 {
		query.Limit = DefaultListLimit
	}
	return query
}

// TriggerListQueryWithDefaultLimit normalizes a public trigger query and applies the default page size.
func TriggerListQueryWithDefaultLimit(query TriggerListQuery) TriggerListQuery {
	query = normalizeTriggerListQuery(query)
	if query.Limit == 0 {
		query.Limit = DefaultListLimit
	}
	return query
}

// JobListCursor decodes the optional cursor after validating its job-query fingerprint.
func JobListCursor(query JobListQuery) (ListCursorPosition, bool, error) {
	query = normalizeJobListQuery(query)
	if err := ValidateJobListQuery(query); err != nil {
		return ListCursorPosition{}, false, err
	}
	if query.Cursor == "" {
		return ListCursorPosition{}, false, nil
	}
	fingerprint, err := jobQueryFingerprint(query)
	if err != nil {
		return ListCursorPosition{}, false, err
	}
	position, err := decodeListCursor(query.Cursor, jobListCursorKind, fingerprint)
	return position, true, err
}

// TriggerListCursor decodes the optional cursor after validating its trigger-query fingerprint.
func TriggerListCursor(query TriggerListQuery) (ListCursorPosition, bool, error) {
	query = normalizeTriggerListQuery(query)
	if err := ValidateTriggerListQuery(query); err != nil {
		return ListCursorPosition{}, false, err
	}
	if query.Cursor == "" {
		return ListCursorPosition{}, false, nil
	}
	fingerprint, err := triggerQueryFingerprint(query)
	if err != nil {
		return ListCursorPosition{}, false, err
	}
	position, err := decodeListCursor(query.Cursor, triggerCursorKind, fingerprint)
	return position, true, err
}

// EncodeJobListCursor binds a stable anchor to the normalized job query.
func EncodeJobListCursor(query JobListQuery, position ListCursorPosition) (string, error) {
	query = normalizeJobListQuery(query)
	fingerprint, err := jobQueryFingerprint(query)
	if err != nil {
		return "", err
	}
	return encodeListCursor(jobListCursorKind, fingerprint, position.Source, position.Name, position.ID)
}

// EncodeTriggerListCursor binds a stable anchor to the normalized trigger query.
func EncodeTriggerListCursor(query TriggerListQuery, position ListCursorPosition) (string, error) {
	query = normalizeTriggerListQuery(query)
	fingerprint, err := triggerQueryFingerprint(query)
	if err != nil {
		return "", err
	}
	return encodeListCursor(triggerCursorKind, fingerprint, position.Source, position.Name, position.ID)
}

// ValidateJobListQuery validates filters and cursor binding for a job query.
func ValidateJobListQuery(query JobListQuery) error {
	query = normalizeJobListQuery(query)
	if err := validateListLimit("job", query.Limit); err != nil {
		return err
	}
	if query.Scope != "" {
		if err := query.Scope.Validate("job_query.scope"); err != nil {
			return err
		}
	}
	if query.Source != "" {
		if err := query.Source.Validate("job_query.source"); err != nil {
			return err
		}
	}
	if query.Scope == AutomationScopeGlobal && query.WorkspaceID != "" {
		return errors.New("automation: job workspace_id filter must be empty when scope is global")
	}
	if query.Cursor == "" {
		return nil
	}
	if query.Limit == 0 {
		return fmt.Errorf("%w: job cursor requires a positive limit", ErrListCursorInvalid)
	}
	fingerprint, err := jobQueryFingerprint(query)
	if err != nil {
		return err
	}
	_, err = decodeListCursor(query.Cursor, jobListCursorKind, fingerprint)
	return err
}

// ValidateTriggerListQuery validates filters and cursor binding for a trigger query.
func ValidateTriggerListQuery(query TriggerListQuery) error {
	query = normalizeTriggerListQuery(query)
	if err := validateListLimit("trigger", query.Limit); err != nil {
		return err
	}
	if query.Scope != "" {
		if err := query.Scope.Validate("trigger_query.scope"); err != nil {
			return err
		}
	}
	if query.Source != "" {
		if err := query.Source.Validate("trigger_query.source"); err != nil {
			return err
		}
	}
	if query.Scope == AutomationScopeGlobal && query.WorkspaceID != "" {
		return errors.New("automation: trigger workspace_id filter must be empty when scope is global")
	}
	if query.Cursor == "" {
		return nil
	}
	if query.Limit == 0 {
		return fmt.Errorf("%w: trigger cursor requires a positive limit", ErrListCursorInvalid)
	}
	fingerprint, err := triggerQueryFingerprint(query)
	if err != nil {
		return err
	}
	_, err = decodeListCursor(query.Cursor, triggerCursorKind, fingerprint)
	return err
}

// BuildJobListPage applies canonical job filters, ordering, and cursor pagination.
func BuildJobListPage(jobs []Job, query JobListQuery) (JobListPage, error) {
	query = normalizeJobListQuery(query)
	if err := ValidateJobListQuery(query); err != nil {
		return JobListPage{}, err
	}

	filtered := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if jobMatchesListQuery(job, query) {
			filtered = append(filtered, job)
		}
	}
	SortJobsForList(filtered)

	fingerprint, err := jobQueryFingerprint(query)
	if err != nil {
		return JobListPage{}, err
	}
	pageStart, err := jobPageStart(filtered, query.Cursor, fingerprint)
	if err != nil {
		return JobListPage{}, err
	}
	pageEnd, hasMore := listPageEnd(pageStart, len(filtered), query.Limit)
	pageJobs := append(make([]Job, 0, pageEnd-pageStart), filtered[pageStart:pageEnd]...)
	page := JobListPage{
		Jobs:    pageJobs,
		Total:   len(filtered),
		Limit:   query.Limit,
		HasMore: hasMore,
	}
	if hasMore {
		last := pageJobs[len(pageJobs)-1]
		page.NextCursor, err = encodeListCursor(
			jobListCursorKind,
			fingerprint,
			last.Source,
			last.Name,
			last.ID,
		)
		if err != nil {
			return JobListPage{}, err
		}
	}
	return page, nil
}

// BuildTriggerListPage applies canonical trigger filters, ordering, and cursor pagination.
func BuildTriggerListPage(triggers []Trigger, query TriggerListQuery) (TriggerListPage, error) {
	query = normalizeTriggerListQuery(query)
	if err := ValidateTriggerListQuery(query); err != nil {
		return TriggerListPage{}, err
	}

	filtered := make([]Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		if triggerMatchesListQuery(trigger, query) {
			filtered = append(filtered, trigger)
		}
	}
	SortTriggersForList(filtered)

	fingerprint, err := triggerQueryFingerprint(query)
	if err != nil {
		return TriggerListPage{}, err
	}
	pageStart, err := triggerPageStart(filtered, query.Cursor, fingerprint)
	if err != nil {
		return TriggerListPage{}, err
	}
	pageEnd, hasMore := listPageEnd(pageStart, len(filtered), query.Limit)
	pageTriggers := append(make([]Trigger, 0, pageEnd-pageStart), filtered[pageStart:pageEnd]...)
	page := TriggerListPage{
		Triggers: pageTriggers,
		Total:    len(filtered),
		Limit:    query.Limit,
		HasMore:  hasMore,
	}
	if hasMore {
		last := pageTriggers[len(pageTriggers)-1]
		page.NextCursor, err = encodeListCursor(triggerCursorKind, fingerprint, last.Source, last.Name, last.ID)
		if err != nil {
			return TriggerListPage{}, err
		}
	}
	return page, nil
}

func validateListLimit(kind string, limit int) error {
	if limit < 0 || limit > MaxListLimit {
		return fmt.Errorf("automation: invalid %s list limit %d; expected 0..%d", kind, limit, MaxListLimit)
	}
	return nil
}

func jobQueryFingerprint(query JobListQuery) (string, error) {
	return listcursor.Fingerprint(jobListFingerprint{
		Kind:        jobListCursorKind,
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		Source:      query.Source,
		LoopName:    query.LoopName,
		Enabled:     query.Enabled,
		Search:      query.Search,
	})
}

func triggerQueryFingerprint(query TriggerListQuery) (string, error) {
	return listcursor.Fingerprint(triggerListFingerprint{
		Kind:        triggerCursorKind,
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		Event:       query.Event,
		Source:      query.Source,
		LoopName:    query.LoopName,
		Enabled:     query.Enabled,
		Search:      query.Search,
	})
}

func jobPageStart(jobs []Job, rawCursor string, fingerprint string) (int, error) {
	if rawCursor == "" {
		return 0, nil
	}
	cursor, err := decodeListCursor(rawCursor, jobListCursorKind, fingerprint)
	if err != nil {
		return 0, err
	}
	return sort.Search(len(jobs), func(index int) bool {
		job := jobs[index]
		return compareListKey(job.Source, job.Name, job.ID, cursor.Source, cursor.Name, cursor.ID) > 0
	}), nil
}

func triggerPageStart(triggers []Trigger, rawCursor string, fingerprint string) (int, error) {
	if rawCursor == "" {
		return 0, nil
	}
	cursor, err := decodeListCursor(rawCursor, triggerCursorKind, fingerprint)
	if err != nil {
		return 0, err
	}
	return sort.Search(len(triggers), func(index int) bool {
		trigger := triggers[index]
		return compareListKey(trigger.Source, trigger.Name, trigger.ID, cursor.Source, cursor.Name, cursor.ID) > 0
	}), nil
}

func listPageEnd(start int, total int, limit int) (int, bool) {
	if limit == 0 || start+limit >= total {
		return total, false
	}
	return start + limit, true
}

func encodeListCursor(kind string, fingerprint string, source JobSource, name string, id string) (string, error) {
	return listcursor.Encode(listCursorVersion, kind, fingerprint, ListCursorPosition{
		Source: source,
		Name:   name,
		ID:     id,
	})
}

func decodeListCursor(raw string, kind string, fingerprint string) (ListCursorPosition, error) {
	cursor, err := listcursor.Decode[ListCursorPosition](
		raw,
		listCursorVersion,
		kind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return ListCursorPosition{}, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	if strings.TrimSpace(cursor.Name) == "" || strings.TrimSpace(cursor.ID) == "" {
		return ListCursorPosition{}, fmt.Errorf("%w: cursor key is incomplete", ErrListCursorInvalid)
	}
	if err := cursor.Source.Validate("cursor.source"); err != nil {
		return ListCursorPosition{}, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	return cursor, nil
}
