package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/diagnostics"
)

type Ref struct {
	ID     string         `json:"id"`
	Kind   string         `json:"kind,omitempty"`
	Script string         `json:"script,omitempty"`
	With   jsontext.Value `json:"with,omitempty"`
}
type Limits struct {
	MaxItems        int `json:"max_items"`
	MaxSummaryBytes int `json:"max_summary_bytes"`
	MaxStateBytes   int `json:"max_state_bytes"`
}
type WorkspaceRef struct {
	ID      string `json:"id"`
	RootDir string `json:"root_dir"`
}
type Request struct {
	Version   int            `json:"version"`
	SourceID  string         `json:"source_id"`
	Kind      string         `json:"kind,omitempty"`
	Params    jsontext.Value `json:"params"`
	State     jsontext.Value `json:"state"`
	Limits    Limits         `json:"limits"`
	Workspace WorkspaceRef   `json:"workspace"`
	Now       time.Time      `json:"now"`
	DryRun    bool           `json:"dry_run"`
}
type Item struct {
	Key        string         `json:"key"`
	Title      string         `json:"title"`
	Summary    string         `json:"summary,omitempty"`
	URL        string         `json:"url,omitempty"`
	Attributes jsontext.Value `json:"attributes,omitempty"`
}
type Result struct {
	Items []Item         `json:"items"`
	State jsontext.Value `json:"state,omitempty"`
	More  bool           `json:"more,omitzero"`
}
type ErrorCode string
type Error struct {
	Code   ErrorCode `json:"code"`
	Detail string    `json:"detail"`
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Detail }
func Failure(code, detail string) *Error {
	return &Error{Code: ErrorCode(code), Detail: SanitizeText(detail, 500)}
}

type Runner interface {
	Poll(context.Context, Ref, Request) (Result, error)
}
type CatalogScope struct{ ProfileID, WorkspaceID string }
type KindDescriptor struct {
	Kind         string         `json:"kind"`
	Extension    string         `json:"extension"`
	Title        string         `json:"title"`
	Description  string         `json:"description,omitempty"`
	ParamsSchema jsontext.Value `json:"params_schema,omitempty"`
}
type ExtensionPoller interface {
	PollSource(context.Context, CatalogScope, string, Request) (Result, error)
	SourceKinds(context.Context, CatalogScope) ([]KindDescriptor, error)
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
var filePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var kindPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)

func ValidScript(name string) bool { return filePattern.MatchString(name) }
func (r Ref) Validate() error {
	if !idPattern.MatchString(r.ID) {
		return Failure("heartbeat_source_invalid_id", "invalid source id")
	}
	if (r.Kind == "") == (r.Script == "") || r.Script != "" && !ValidScript(r.Script) ||
		r.Kind != "" && !kindPattern.MatchString(r.Kind) {
		return Failure("heartbeat_source_ref_invalid", "set exactly one valid kind or script")
	}
	if len(r.With) > 0 && (r.With[0] != '{' || !r.With.IsValid()) {
		return Failure("heartbeat_source_ref_invalid", "with must be an object")
	}
	if len(r.With) > 4096 {
		return Failure("heartbeat_source_params_too_large", "source parameters exceed 4096 bytes")
	}
	return nil
}
func (r Ref) Digest() (string, error) {
	data, err := json.Marshal(struct {
		Kind, Script string
		With         jsontext.Value
	}{r.Kind, r.Script, r.With})
	if err != nil {
		return "", err
	}
	canonical := jsontext.Value(data)
	if err := canonical.Canonicalize(); err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
func ValidateResult(r Result, l Limits) error {
	if len(r.Items) > l.MaxItems {
		return Failure("source_output_invalid", fmt.Sprintf("returned %d items, limit %d", len(r.Items), l.MaxItems))
	}
	if len(r.State) > l.MaxStateBytes {
		return Failure("source_state_too_large", "source state exceeds limit")
	}
	seen := map[string]bool{}
	for _, item := range r.Items {
		if item.Key == "" || len(item.Key) > 200 || seen[item.Key] || item.Title == "" ||
			utf8.RuneCountInString(item.Title) > 200 ||
			len(item.Summary) > 1024 ||
			len(item.Attributes) > 2048 {
			return Failure("source_output_invalid", "invalid or duplicate item")
		}
		seen[item.Key] = true
	}
	return nil
}

func (e *Error) DiagnosticCode() string { return string(e.Code) }

var sourceSensitiveWords = regexp.MustCompile(`(?i)\b(token|password|api[_-]?key|secret)\s+[^\s]+`)

func SanitizeText(text string, limit int) string {
	return diagnostics.RedactAndBound(sourceSensitiveWords.ReplaceAllString(text, "$1 [REDACTED]"), limit)
}

const SummaryBytes = 8 << 10
