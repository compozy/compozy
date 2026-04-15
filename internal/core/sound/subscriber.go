package sound

import (
	"context"
	"log/slog"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

// playbackTimeout bounds how long a single Notify call can block on the
// underlying player. Three seconds is comfortably above the ~500ms that
// macOS system sounds take while still preventing a hung player from
// delaying run shutdown indefinitely.
const playbackTimeout = 3 * time.Second

// Config binds a Player to per-event-kind preset names. Both presets and
// absolute filesystem paths are accepted; an empty string for a given kind
// disables playback for that kind.
type Config struct {
	Player      Player
	OnCompleted string
	OnFailed    string
}

// Notify plays the configured sound for the given lifecycle event kind and
// blocks until the underlying player returns. Notify is the only API the run
// pipelines use: it runs on the caller's goroutine so the sound is guaranteed
// to finish before run cleanup tears down state.
//
// The parent context is detached via context.WithoutCancel before playback
// starts. This is deliberate: on run.failed and run.cancelled the caller
// passes a ctx that is already done, and a canceled exec.CommandContext
// would immediately kill afplay/paplay so the failure sound would never be
// audible. A bounded playbackTimeout prevents a hung player from delaying
// run shutdown indefinitely.
//
// Unrecognized kinds and empty preset configurations are silent no-ops.
// Playback errors are logged at debug and never bubble back to the caller — a
// missing system sound must not break a successful run.
func Notify(
	ctx context.Context,
	cfg Config,
	kind events.EventKind,
	logger *slog.Logger,
) {
	if cfg.Player == nil {
		return
	}
	name := pickSound(kind, cfg)
	if name == "" {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	playCtx, cancel := playbackContext(ctx)
	defer cancel()
	if err := cfg.Player.Play(playCtx, name); err != nil {
		logger.DebugContext(
			playCtx,
			"sound playback failed",
			slog.String("kind", string(kind)),
			slog.String("sound", name),
			slog.String("err", err.Error()),
		)
	}
}

// playbackContext returns a bounded, uncancelable derivative of the caller's
// context so terminal events (which often arrive with ctx already done) can
// still reach the audio command. The returned context preserves values from
// the parent for logging/tracing but is isolated from parent cancellation.
func playbackContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	detached := context.WithoutCancel(parent)
	return context.WithTimeout(detached, playbackTimeout)
}

// pickSound returns the configured sound name for a lifecycle event kind, or
// an empty string when the kind should be silent.
func pickSound(kind events.EventKind, cfg Config) string {
	switch kind {
	case events.EventKindRunCompleted:
		return cfg.OnCompleted
	case events.EventKindRunFailed, events.EventKindRunCancelled:
		return cfg.OnFailed
	default:
		return ""
	}
}
