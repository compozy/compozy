package soul

import (
	"context"

	"errors"
	"fmt"
	"os"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	soulCapabilitiesKey     = "capabilities"
	soulConfigKey           = "config"
	soulForbiddenFieldKey   = "forbidden_field"
	soulInvalidFieldTypeKey = "invalid_field_type"
	soulOversizedBodyKey    = "oversized_body"
	soulPathEscapeKey       = "path_escape"
	soulReservedSectionKey  = "reserved_section"
	soulRoleKey             = "role"
	soulTaskRunsKey         = "task_runs"
	soulToneKey             = "tone"
	soulVersionKey          = "version"
)

const (
	// FileName is the canonical authored persona filename.
	FileName = "SOUL.md"

	digestPrefix = "agh.soul.v1\n"
)

var (
	// ErrInvalid reports a SOUL.md file that exists but cannot be accepted.
	ErrInvalid = errors.New("soul: invalid SOUL.md")
	// ErrPathEscape reports a SOUL.md path outside its configured workspace root.
	ErrPathEscape = errors.New("soul: path escapes workspace root")
)

// ResolveRequest describes a SOUL.md file located beside an AGENT.md file.
type ResolveRequest struct {
	AgentPath     string
	WorkspaceRoot string
	Config        aghconfig.SoulConfig
}

// ParseRequest describes in-memory SOUL.md content to validate and project.
type ParseRequest struct {
	SourcePath    string
	WorkspaceRoot string
	Content       []byte
	Config        aghconfig.SoulConfig
}

// ResolvedSoul is the normalized result that later runtime surfaces can consume.
type ResolvedSoul struct {
	Enabled     bool
	Present     bool
	Active      bool
	Valid       bool
	SourcePath  string
	Digest      string
	Profile     Profile
	Compact     CompactProjection
	ReadModel   ReadModel
	Diagnostics []Diagnostic
}

// Profile is the normalized authored persona profile.
type Profile struct {
	SourcePath    string
	Digest        string
	Version       string
	Role          string
	Tone          []string
	Principles    []string
	Constraints   []string
	Collaboration []string
	MemoryPolicy  []string
	Tags          []string
	Body          string
	Truncated     bool
}

// CompactProjection is the bounded context-safe soul projection.
type CompactProjection struct {
	Enabled      bool
	Present      bool
	Active       bool
	Digest       string
	SourcePath   string
	Role         string
	Tone         []string
	Principles   []string
	Truncated    bool
	MaxBytes     int64
	MaxBodyBytes int64
}

// ReadModel is the full resolved soul view for dedicated inspect surfaces.
type ReadModel struct {
	Enabled                bool
	Present                bool
	Active                 bool
	Valid                  bool
	SourcePath             string
	Digest                 string
	Frontmatter            Frontmatter
	Body                   string
	Truncated              bool
	MaxBodyBytes           int64
	ContextProjectionBytes int64
	Diagnostics            []Diagnostic
}

// Frontmatter is the allowlisted strict SOUL.md metadata.
type Frontmatter struct {
	Version       string
	Role          string
	Tone          []string
	Principles    []string
	Constraints   []string
	Collaboration []string
	MemoryPolicy  []string
	Tags          []string
}

// Diagnostic describes a closed, redacted SOUL.md validation problem.
type Diagnostic struct {
	Code       string
	Field      string
	Section    string
	Message    string
	SourcePath string
	Line       int
	Column     int
}

// DiagnosticError carries structured diagnostics for invalid authored content.
type DiagnosticError struct {
	Diagnostics []Diagnostic
	cause       error
}

func (e *DiagnosticError) Error() string {
	if e == nil || len(e.Diagnostics) == 0 {
		return ErrInvalid.Error()
	}
	first := e.Diagnostics[0]
	return fmt.Sprintf("%s: %s", first.Code, first.Message)
}

// Unwrap exposes the validation sentinel for errors.Is callers.
func (e *DiagnosticError) Unwrap() error {
	if e == nil || e.cause == nil {
		return ErrInvalid
	}
	return e.cause
}

// Resolve reads and resolves SOUL.md beside the provided agent path.
func Resolve(ctx context.Context, req ResolveRequest) (ResolvedSoul, error) {
	if err := req.Config.Validate(); err != nil {
		return ResolvedSoul{}, err
	}
	if err := checkContext(ctx); err != nil {
		return ResolvedSoul{}, err
	}

	soulPath, pathErr := soulPathForAgent(req.AgentPath)
	safePath, diagnostic := safeSourcePath(soulPath, req.WorkspaceRoot)
	result := emptyResult(req.Config, safePath)
	if pathErr != nil {
		diag := diagnosticForError("invalid_source_path", safePath, pathErr, ErrInvalid)
		diag.Message = "SOUL.md source path is required"
		return resultWithDiagnostics(&result, []Diagnostic{diag})
	}
	if diagnostic != nil {
		return resultWithDiagnostics(&result, []Diagnostic{*diagnostic})
	}

	content, err := os.ReadFile(soulPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		diag := diagnosticForError("parser_io", safePath, err, err)
		diag.Message = "SOUL.md could not be read"
		return resultWithDiagnostics(&result, []Diagnostic{diag})
	}
	if err := checkContext(ctx); err != nil {
		return ResolvedSoul{}, err
	}

	return Parse(ctx, ParseRequest{
		SourcePath:    soulPath,
		WorkspaceRoot: req.WorkspaceRoot,
		Content:       content,
		Config:        req.Config,
	})
}

// Parse validates and normalizes in-memory SOUL.md content.
func Parse(ctx context.Context, req ParseRequest) (ResolvedSoul, error) {
	if err := req.Config.Validate(); err != nil {
		return ResolvedSoul{}, err
	}
	if err := checkContext(ctx); err != nil {
		return ResolvedSoul{}, err
	}

	safePath, diagnostic := safeSourcePath(req.SourcePath, req.WorkspaceRoot)
	result := emptyResult(req.Config, safePath)
	if diagnostic != nil {
		return resultWithDiagnostics(&result, []Diagnostic{*diagnostic})
	}
	result.Present = true
	result.Active = req.Config.Enabled
	result.Compact.Present = true
	result.Compact.Active = req.Config.Enabled
	result.ReadModel.Present = true
	result.ReadModel.Active = req.Config.Enabled

	front, body, offset, parseDiagnostics := parseDocument(req.Content, req.Config, safePath)
	if len(parseDiagnostics) > 0 {
		return resultWithDiagnostics(&result, parseDiagnostics)
	}

	validationDiagnostics := validateReservedSections(body, safePath, offset)
	if len(validationDiagnostics) > 0 {
		return resultWithDiagnostics(&result, validationDiagnostics)
	}

	digest, err := digestSoul(front, body)
	if err != nil {
		diag := diagnosticForError("digest_failed", safePath, err, ErrInvalid)
		diag.Message = "SOUL.md digest could not be computed"
		return resultWithDiagnostics(&result, []Diagnostic{diag})
	}

	profile := Profile{
		SourcePath:    safePath,
		Digest:        digest,
		Version:       front.Version,
		Role:          front.Role,
		Tone:          cloneStrings(front.Tone),
		Principles:    cloneStrings(front.Principles),
		Constraints:   cloneStrings(front.Constraints),
		Collaboration: cloneStrings(front.Collaboration),
		MemoryPolicy:  cloneStrings(front.MemoryPolicy),
		Tags:          cloneStrings(front.Tags),
		Body:          body,
	}
	compact := compactProjection(req.Config, profile)
	profile.Truncated = compact.Truncated
	readModel := ReadModel{
		Enabled:                req.Config.Enabled,
		Present:                true,
		Active:                 req.Config.Enabled,
		Valid:                  true,
		SourcePath:             safePath,
		Digest:                 digest,
		Frontmatter:            front,
		Body:                   body,
		Truncated:              compact.Truncated,
		MaxBodyBytes:           req.Config.MaxBodyBytes,
		ContextProjectionBytes: req.Config.ContextProjectionBytes,
	}
	return ResolvedSoul{
		Enabled:    req.Config.Enabled,
		Present:    true,
		Active:     req.Config.Enabled,
		Valid:      true,
		SourcePath: safePath,
		Digest:     digest,
		Profile:    profile,
		Compact:    compact,
		ReadModel:  readModel,
	}, nil
}

// Empty returns a valid absent SOUL.md resolution for non-filesystem agent sources.
func Empty(config aghconfig.SoulConfig, sourcePath string) (ResolvedSoul, error) {
	if err := config.Validate(); err != nil {
		return ResolvedSoul{}, err
	}
	safePath, diagnostic := safeSourcePath(sourcePath, "")
	result := emptyResult(config, safePath)
	if diagnostic != nil {
		return resultWithDiagnostics(&result, []Diagnostic{*diagnostic})
	}
	return result, nil
}
