package telemetry

import (
	"sort"

	"github.com/compozy/compozy/engine/core"
)

// Options configures how run telemetry is recorded.
type Options struct {
	// ProjectRoot is the project directory used to resolve the `.compozy` store.
	ProjectRoot string
	// RunDirName defines the directory under the store where run transcripts are written.
	RunDirName string
	// ToolLogFile defines the filename (relative to the store) used for tool execution logs.
	ToolLogFile string
	// CaptureContent toggles whether prompts/responses are captured verbatim.
	CaptureContent bool
	// RedactContent toggles whether raw content should be redacted (takes precedence over CaptureContent).
	RedactContent bool
	// ContextWarningThresholds defines percentage thresholds (0-1) that trigger warnings
	// when the accumulated token usage exceeds the threshold.
	ContextWarningThresholds []float64
}

// applyDefaults materializes sensible defaults for missing option values.
func (o *Options) applyDefaults() {
	storeDir := core.GetStoreDir(o.ProjectRoot)
	if o.RunDirName == "" {
		o.RunDirName = "llm_runs"
	}
	if o.ToolLogFile == "" {
		o.ToolLogFile = "tools_log.ndjson"
	}
	if o.ContextWarningThresholds == nil {
		o.ContextWarningThresholds = []float64{0.7, 0.85}
	}
	sort.Float64s(o.ContextWarningThresholds)

	// Normalise capture settings.
	if !o.CaptureContent && !o.RedactContent {
		o.CaptureContent = true
	}
	if o.RedactContent {
		o.CaptureContent = false
	}

	// Ensure ProjectRoot resolves to store (even when empty)
	o.ProjectRoot = storeDir
}

// clone makes a shallow copy ensuring defaults are applied without mutating inputs.
func (o *Options) clone() Options {
	if o == nil {
		opts := Options{CaptureContent: true}
		opts.applyDefaults()
		return opts
	}
	opts := *o
	opts.applyDefaults()
	return opts
}
