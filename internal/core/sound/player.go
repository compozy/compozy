// Package sound provides optional audio notifications for run-lifecycle events.
//
// Playback is opt-in via the [sound] section in .compozy/config.toml and is
// disabled by default. A Noop player is used whenever the feature flag is off
// or the platform is not supported, so callers never need to special-case
// "sound is turned off".
package sound

import (
	"context"
	"errors"
	"strings"
)

// ErrUnsupported indicates the running platform cannot play sounds.
var ErrUnsupported = errors.New("sound: unsupported platform")

// ErrEmptySound indicates the caller passed an empty sound identifier.
var ErrEmptySound = errors.New("sound: empty sound name")

// Player plays a named preset or a filesystem path. Implementations must be
// safe to call from any goroutine and must honor context cancellation.
type Player interface {
	Play(ctx context.Context, sound string) error
}

// Noop is the zero-cost player used when sound is disabled or unsupported.
type Noop struct{}

// Play satisfies Player and always returns nil.
func (Noop) Play(context.Context, string) error { return nil }

var _ Player = Noop{}

// commandRunner is the injection seam that lets tests replace the real
// os/exec call with a recorder. The default implementation shells out via
// exec.CommandContext.
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// osPlayer plays sounds by shelling out to a platform-native command
// (afplay on darwin, paplay on linux, powershell on windows).
type osPlayer struct {
	runner  commandRunner
	resolve func(sound string) (name string, args []string, err error)
}

// Play runs the platform-native sound command. An empty sound name is a
// user error and returns ErrEmptySound; platform errors are wrapped with
// context so the subscriber can log the failing preset.
func (p *osPlayer) Play(ctx context.Context, sound string) error {
	trimmed := strings.TrimSpace(sound)
	if trimmed == "" {
		return ErrEmptySound
	}
	name, args, err := p.resolve(trimmed)
	if err != nil {
		return err
	}
	if err := p.runner.Run(ctx, name, args...); err != nil {
		return err
	}
	return nil
}

// New returns the default Player for the current platform. Callers that
// want to skip playback entirely should use Noop{} directly.
func New() Player {
	return newOSPlayer()
}
