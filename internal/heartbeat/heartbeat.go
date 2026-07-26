package heartbeat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	heartbeatEnabledKey                    = "enabled"
	heartbeatHeartbeatForbiddenFieldKey    = "heartbeat_forbidden_field"
	heartbeatHeartbeatInvalidFieldTypeKey  = "heartbeat_invalid_field_type"
	heartbeatHeartbeatPathEscapeKey        = "heartbeat_path_escape"
	heartbeatHeartbeatReservedBodyFieldKey = "heartbeat_reserved_body_field"
	heartbeatVersionKey                    = "version"
)

const (
	// FileName is the canonical authored wake-policy filename.
	FileName = "HEARTBEAT.md"

	schemaVersion       = 1
	digestPrefix        = "agh.heartbeat.v1\n"
	contentDigestPrefix = "agh.heartbeat.content.v1\n"
	absentDigestPrefix  = "agh.heartbeat.absent.v1\n"
	configDigestPrefix  = "agh.heartbeat.config.v1\n"
	diagnosticError     = "error"
	diagnosticWarning   = "warning"
)

var (
	// ErrInvalid reports a HEARTBEAT.md file that exists but cannot be accepted.
	ErrInvalid = errors.New("heartbeat: invalid HEARTBEAT.md")
	// ErrPathEscape reports a HEARTBEAT.md path outside its configured workspace root.
	ErrPathEscape = errors.New("heartbeat: path escapes workspace root")
)

// ResolveRequest describes a HEARTBEAT.md file located beside an AGENT.md file.
type ResolveRequest struct {
	AgentPath     string
	WorkspaceRoot string
	Config        aghconfig.HeartbeatConfig
}

// ParseRequest describes in-memory HEARTBEAT.md content to validate and project.
type ParseRequest struct {
	SourcePath    string
	WorkspaceRoot string
	Content       []byte
	Config        aghconfig.HeartbeatConfig
}

// ResolvedPolicy is the normalized wake policy later runtime surfaces consume.
type ResolvedPolicy struct {
	Enabled          bool
	Present          bool
	Active           bool
	Valid            bool
	SourcePath       string
	Digest           string
	ConfigDigest     string
	SchemaVersion    int
	Summary          string
	GuidanceMarkdown string
	Frontmatter      Frontmatter
	Preferences      Preferences
	ConfigProvenance ConfigProvenance
	Prompt           PromptContribution
	Status           StatusData
	Diagnostics      []Diagnostic
	contentDigest    string
}

// Frontmatter is the allowlisted strict HEARTBEAT.md metadata.
type Frontmatter struct {
	Version     int                    `json:"version"`
	Enabled     bool                   `json:"enabled"`
	Summary     string                 `json:"summary,omitempty"`
	Preferences FrontmatterPreferences `json:"preferences"`
	Context     ContextProjection      `json:"context"`
}

// FrontmatterPreferences captures authored preference hints before config bounds.
type FrontmatterPreferences struct {
	MinInterval  string       `json:"min_interval,omitempty"`
	ActiveHours  []TimeWindow `json:"active_hours,omitempty"`
	QuietWindows []TimeWindow `json:"quiet_windows,omitempty"`
}

// Preferences captures config-bound resolved wake-policy hints.
type Preferences struct {
	MinInterval  time.Duration     `json:"min_interval"`
	ActiveHours  []TimeWindow      `json:"active_hours,omitempty"`
	QuietWindows []TimeWindow      `json:"quiet_windows,omitempty"`
	Context      ContextProjection `json:"context"`
}

// ContextProjection captures authored context projection hints.
type ContextProjection struct {
	Include []string `json:"include,omitempty"`
}

// TimeWindow is one local wall-clock active or quiet window.
type TimeWindow struct {
	Timezone string `json:"timezone"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

// ConfigSubset is the canonical config authority subset included in digests.
type ConfigSubset struct {
	Enabled                      bool   `json:"enabled"`
	MaxBodyBytes                 int64  `json:"max_body_bytes"`
	ContextProjectionBytes       int64  `json:"context_projection_bytes"`
	MinInterval                  string `json:"min_interval"`
	DefaultInterval              string `json:"default_interval"`
	WakeCooldown                 string `json:"wake_cooldown"`
	MaxWakesPerCycle             int    `json:"max_wakes_per_cycle"`
	ActiveSessionOnly            bool   `json:"active_session_only"`
	AllowActiveHoursPreferences  bool   `json:"allow_active_hours_preferences"`
	WakeEventRetention           string `json:"wake_event_retention"`
	SessionHealthStaleAfter      string `json:"session_health_stale_after"`
	SessionHealthHookMinInterval string `json:"session_health_hook_min_interval"`
}

// ConfigProvenance records the resolved config subset that shaped a policy.
type ConfigProvenance struct {
	Digest string       `json:"digest"`
	Subset ConfigSubset `json:"subset"`
}

// PromptContribution is the bounded synthetic-wake prompt contribution.
type PromptContribution struct {
	Active           bool              `json:"active"`
	Digest           string            `json:"digest,omitempty"`
	ConfigDigest     string            `json:"config_digest,omitempty"`
	SourcePath       string            `json:"source_path,omitempty"`
	Summary          string            `json:"summary,omitempty"`
	GuidanceMarkdown string            `json:"guidance_markdown,omitempty"`
	Preferences      Preferences       `json:"preferences"`
	Truncated        bool              `json:"truncated"`
	MaxBytes         int64             `json:"max_bytes"`
	MaxBodyBytes     int64             `json:"max_body_bytes"`
	Diagnostics      []Diagnostic      `json:"diagnostics,omitempty"`
	Context          ContextProjection `json:"context"`
}

// StatusData is the compact inspect/status read model for policy diagnostics.
type StatusData struct {
	Enabled                      bool               `json:"enabled"`
	Present                      bool               `json:"present"`
	Active                       bool               `json:"active"`
	Valid                        bool               `json:"valid"`
	SourcePath                   string             `json:"source_path,omitempty"`
	Digest                       string             `json:"digest,omitempty"`
	ConfigDigest                 string             `json:"config_digest,omitempty"`
	SchemaVersion                int                `json:"schema_version"`
	Summary                      string             `json:"summary,omitempty"`
	Preferences                  Preferences        `json:"preferences"`
	Diagnostics                  []Diagnostic       `json:"diagnostics,omitempty"`
	MaxBodyBytes                 int64              `json:"max_body_bytes"`
	ContextProjectionBytes       int64              `json:"context_projection_bytes"`
	ActiveSessionOnly            bool               `json:"active_session_only"`
	AllowActiveHoursPreferences  bool               `json:"allow_active_hours_preferences"`
	WakeCooldown                 time.Duration      `json:"wake_cooldown"`
	MaxWakesPerCycle             int                `json:"max_wakes_per_cycle"`
	WakeEventRetention           time.Duration      `json:"wake_event_retention"`
	SessionHealthStaleAfter      time.Duration      `json:"session_health_stale_after"`
	SessionHealthHookMinInterval time.Duration      `json:"session_health_hook_min_interval"`
	ConfigProvenance             ConfigProvenance   `json:"config_provenance"`
	Prompt                       PromptContribution `json:"prompt"`
}

// Diagnostic describes a closed, redacted HEARTBEAT.md validation problem.
type Diagnostic struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
	Section    string `json:"section,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
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

// Resolve reads and resolves HEARTBEAT.md beside the provided agent path.
func Resolve(ctx context.Context, req ResolveRequest) (ResolvedPolicy, error) {
	if err := req.Config.Validate(); err != nil {
		return ResolvedPolicy{}, err
	}
	if err := checkContext(ctx); err != nil {
		return ResolvedPolicy{}, err
	}
	provenance, err := ConfigProvenanceFor(req.Config)
	if err != nil {
		return ResolvedPolicy{}, err
	}

	heartbeatPath, pathErr := heartbeatPathForAgent(req.AgentPath)
	safePath, diagnostic := safeSourcePath(heartbeatPath, req.WorkspaceRoot)
	result := emptyResult(req.Config, provenance, safePath)
	if pathErr != nil {
		diag := diagnosticForError("heartbeat_invalid_source_path", safePath, pathErr, ErrInvalid)
		diag.Message = "HEARTBEAT.md source path is required"
		return resultWithDiagnostics(&result, []Diagnostic{diag})
	}
	if diagnostic != nil {
		return resultWithDiagnostics(&result, []Diagnostic{*diagnostic})
	}

	content, err := os.ReadFile(heartbeatPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		diag := diagnosticForError("heartbeat_parser_io", safePath, err, err)
		diag.Message = "HEARTBEAT.md could not be read"
		return resultWithDiagnostics(&result, []Diagnostic{diag})
	}
	if err := checkContext(ctx); err != nil {
		return ResolvedPolicy{}, err
	}

	return Parse(ctx, ParseRequest{
		SourcePath:    heartbeatPath,
		WorkspaceRoot: req.WorkspaceRoot,
		Content:       content,
		Config:        req.Config,
	})
}

// Parse validates and normalizes in-memory HEARTBEAT.md content.
func Parse(ctx context.Context, req ParseRequest) (ResolvedPolicy, error) {
	if err := req.Config.Validate(); err != nil {
		return ResolvedPolicy{}, err
	}
	if err := checkContext(ctx); err != nil {
		return ResolvedPolicy{}, err
	}
	provenance, err := ConfigProvenanceFor(req.Config)
	if err != nil {
		return ResolvedPolicy{}, err
	}

	safePath, diagnostic := safeSourcePath(req.SourcePath, req.WorkspaceRoot)
	result := emptyResult(req.Config, provenance, safePath)
	if diagnostic != nil {
		return resultWithDiagnostics(&result, []Diagnostic{*diagnostic})
	}
	result.Present = true
	result.Status.Present = true
	result.contentDigest = digestHeartbeatContent(req.Content)

	front, body, bodyLineOffset, parseDiagnostics := parseDocument(req.Content, req.Config, safePath)
	if len(parseDiagnostics) > 0 {
		return resultWithDiagnostics(&result, parseDiagnostics)
	}

	preferences, preferenceDiagnostics := resolvePreferences(front, req.Config, safePath)
	if hasErrorDiagnostics(preferenceDiagnostics) {
		return resultWithDiagnostics(&result, preferenceDiagnostics)
	}
	bodyDiagnostics := validateBodyAuthorityClaims(body, safePath, bodyLineOffset)
	if len(bodyDiagnostics) > 0 {
		return resultWithDiagnostics(&result, append(preferenceDiagnostics, bodyDiagnostics...))
	}

	digest, err := digestPolicy(front, body, provenance.Subset)
	if err != nil {
		diag := diagnosticForError("heartbeat_digest_failed", safePath, err, ErrInvalid)
		diag.Message = "HEARTBEAT.md digest could not be computed"
		return resultWithDiagnostics(&result, []Diagnostic{diag})
	}

	active := req.Config.Enabled && front.Enabled
	result.Enabled = req.Config.Enabled
	result.Present = true
	result.Active = active
	result.Valid = true
	result.SourcePath = safePath
	result.Digest = digest
	result.ConfigDigest = provenance.Digest
	result.SchemaVersion = schemaVersion
	result.Summary = front.Summary
	result.GuidanceMarkdown = body
	result.Frontmatter = front
	result.Preferences = preferences
	result.Diagnostics = sanitizeDiagnostics(preferenceDiagnostics)
	result.Prompt = promptContribution(req.Config, &result)
	result.Status = statusData(req.Config, &result)
	return result, nil
}

// Empty returns a valid absent HEARTBEAT.md resolution for non-filesystem agent sources.
func Empty(config aghconfig.HeartbeatConfig, sourcePath string) (ResolvedPolicy, error) {
	if err := config.Validate(); err != nil {
		return ResolvedPolicy{}, err
	}
	provenance, err := ConfigProvenanceFor(config)
	if err != nil {
		return ResolvedPolicy{}, err
	}
	safePath, diagnostic := safeSourcePath(sourcePath, "")
	result := emptyResult(config, provenance, safePath)
	if diagnostic != nil {
		return resultWithDiagnostics(&result, []Diagnostic{*diagnostic})
	}
	return result, nil
}

// ConfigProvenanceFor returns the canonical config subset and digest used by Heartbeat.
func ConfigProvenanceFor(cfg aghconfig.HeartbeatConfig) (ConfigProvenance, error) {
	if err := cfg.Validate(); err != nil {
		return ConfigProvenance{}, err
	}
	subset := ConfigSubset{
		Enabled:                      cfg.Enabled,
		MaxBodyBytes:                 cfg.MaxBodyBytes,
		ContextProjectionBytes:       cfg.ContextProjectionBytes,
		MinInterval:                  cfg.MinInterval.String(),
		DefaultInterval:              cfg.DefaultInterval.String(),
		WakeCooldown:                 cfg.WakeCooldown.String(),
		MaxWakesPerCycle:             cfg.MaxWakesPerCycle,
		ActiveSessionOnly:            cfg.ActiveSessionOnly,
		AllowActiveHoursPreferences:  cfg.AllowActiveHoursPreferences,
		WakeEventRetention:           cfg.WakeEventRetention.String(),
		SessionHealthStaleAfter:      cfg.SessionHealthStaleAfter.String(),
		SessionHealthHookMinInterval: cfg.SessionHealthHookMinInterval.String(),
	}
	encoded, err := json.Marshal(subset)
	if err != nil {
		return ConfigProvenance{}, fmt.Errorf("marshal heartbeat config subset: %w", err)
	}
	sum := sha256.Sum256([]byte(configDigestPrefix + string(encoded)))
	return ConfigProvenance{
		Digest: "sha256:" + hex.EncodeToString(sum[:]),
		Subset: subset,
	}, nil
}

// AllowsAt evaluates active-hours allow windows minus quiet-window deny windows.
func (p Preferences) AllowsAt(now time.Time) (bool, error) {
	activeAllowed := len(p.ActiveHours) == 0
	for _, window := range p.ActiveHours {
		contains, err := window.Contains(now)
		if err != nil {
			return false, err
		}
		if contains {
			activeAllowed = true
			break
		}
	}
	if !activeAllowed {
		return false, nil
	}
	for _, window := range p.QuietWindows {
		contains, err := window.Contains(now)
		if err != nil {
			return false, err
		}
		if contains {
			return false, nil
		}
	}
	return true, nil
}

// Contains reports whether now falls inside this local wall-clock window.
func (w TimeWindow) Contains(now time.Time) (bool, error) {
	location, err := time.LoadLocation(strings.TrimSpace(w.Timezone))
	if err != nil {
		return false, fmt.Errorf("load heartbeat time window timezone %q: %w", w.Timezone, err)
	}
	start, err := parseClock(w.Start)
	if err != nil {
		return false, err
	}
	end, err := parseClock(w.End)
	if err != nil {
		return false, err
	}
	local := now.In(location)
	current := local.Hour()*60 + local.Minute()
	startMinutes := start.Hour()*60 + start.Minute()
	endMinutes := end.Hour()*60 + end.Minute()
	if endMinutes <= startMinutes {
		return current >= startMinutes || current < endMinutes, nil
	}
	return current >= startMinutes && current < endMinutes, nil
}
