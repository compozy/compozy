package speccycle

import (
	"errors"
	"fmt"
	"time"
)

const (
	reviewIssueFieldTitle    = "title"
	reviewIssueFieldBody     = "body"
	reviewIssueFieldSeverity = "severity"

	reviewStatusPending    = "pending"
	reviewStatusValid      = "valid"
	reviewStatusInvalid    = "invalid"
	reviewStatusResolved   = "resolved"
	reviewStatusUnresolved = "unresolved"
	reviewStatusBlocked    = "blocked"
	unknownReviewFile      = "unknown"
)

var (
	errReviewArtifactInput = errors.New("spec-cycle: review artifact input invalid")
	errReviewRoundNotFound = errors.New("spec-cycle: review round not found")
)

// ReviewIssue is the source-agnostic artifact input emitted by a reviewer agent.
type ReviewIssue struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Author   string `json:"author,omitempty"`
}

type writeReviewArtifactsInput struct {
	TaskName string        `json:"task_name,omitempty"`
	Issues   []ReviewIssue `json:"issues"`
}

type reviewArtifactFile struct {
	Path string `json:"path"`
	File string `json:"file"`
}

type reviewArtifactBatch struct {
	File       string   `json:"file"`
	IssueFiles []string `json:"issue_files"`
}

type writeReviewArtifactsOutput struct {
	TaskName string                `json:"task_name"`
	Round    int                   `json:"round"`
	RoundDir string                `json:"round_dir,omitempty"`
	Files    []reviewArtifactFile  `json:"files"`
	Batches  []reviewArtifactBatch `json:"batches"`
	Issues   []ReviewIssue         `json:"issues"`
	Count    int                   `json:"count"`
}

type finalizeReviewRoundInput struct {
	TaskName string `json:"task_name,omitempty"`
	Round    int    `json:"round"`
}

type finalizeReviewRoundOutput struct {
	TaskName string `json:"task_name"`
	Round    int    `json:"round"`
	Resolved int    `json:"resolved"`
	Invalid  int    `json:"invalid"`
	Pending  int    `json:"pending"`
}

type reviewFileMeta struct {
	Round          int       `yaml:"round,omitempty"`
	RoundCreatedAt time.Time `yaml:"round_created_at,omitempty"`

	Status   string `yaml:"status"`
	File     string `yaml:"file"`
	Line     int    `yaml:"line,omitempty"`
	Severity string `yaml:"severity"`
	Author   string `yaml:"author"`
}

func reviewArtifactInputError(field string, message string) error {
	return fmt.Errorf("%w: %s: %s", errReviewArtifactInput, field, message)
}

func reviewRoundNotFoundError(taskName string, round int) error {
	return fmt.Errorf("%w: task %q round %03d", errReviewRoundNotFound, taskName, round)
}
