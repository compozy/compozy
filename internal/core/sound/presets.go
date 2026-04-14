package sound

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// preset names accepted in config.toml. Absolute paths bypass preset lookup.
const (
	PresetGlass  = "glass"
	PresetBasso  = "basso"
	PresetPing   = "ping"
	PresetHero   = "hero"
	PresetFunk   = "funk"
	PresetTink   = "tink"
	PresetSubmar = "submarine"
)

// GOOS string constants used across the package. Centralized so the goconst
// linter does not flag duplicate string literals and so a typo in one place
// is a compile error rather than a silent runtime miss.
const (
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"
)

// Platform sound commands. Same rationale as the GOOS constants above.
const (
	cmdAfplay = "afplay"
	cmdPaplay = "paplay"
)

// KnownPresets lists every preset name the resolver understands. The list is
// exposed so the CLI / docs can surface valid values without re-declaring them.
func KnownPresets() []string {
	return []string{
		PresetGlass,
		PresetBasso,
		PresetPing,
		PresetHero,
		PresetFunk,
		PresetTink,
		PresetSubmar,
	}
}

// ResolvePath maps a preset name or filesystem path to a concrete file that
// the platform player can consume. Absolute paths are returned verbatim.
// Unknown presets produce a descriptive error so users see typos early.
func ResolvePath(sound string) (string, error) {
	trimmed := strings.TrimSpace(sound)
	if trimmed == "" {
		return "", ErrEmptySound
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	key := strings.ToLower(trimmed)
	path, ok := presetPathForOS(key, runtime.GOOS)
	if !ok {
		return "", fmt.Errorf(
			"sound: unknown preset %q (known: %s) — pass an absolute path to use a custom file",
			trimmed,
			strings.Join(KnownPresets(), ", "),
		)
	}
	return path, nil
}

// presetPathForOS is split out so tests can exercise every OS branch from any
// host. It returns (path, true) on a known preset for the given GOOS and
// ("", false) otherwise.
func presetPathForOS(preset, goos string) (string, bool) {
	switch goos {
	case goosDarwin:
		path, ok := darwinPresets[preset]
		return path, ok
	case goosLinux:
		path, ok := linuxPresets[preset]
		return path, ok
	case goosWindows:
		path, ok := windowsPresets[preset]
		return path, ok
	default:
		return "", false
	}
}

var darwinPresets = map[string]string{
	PresetGlass:  "/System/Library/Sounds/Glass.aiff",
	PresetBasso:  "/System/Library/Sounds/Basso.aiff",
	PresetPing:   "/System/Library/Sounds/Ping.aiff",
	PresetHero:   "/System/Library/Sounds/Hero.aiff",
	PresetFunk:   "/System/Library/Sounds/Funk.aiff",
	PresetTink:   "/System/Library/Sounds/Tink.aiff",
	PresetSubmar: "/System/Library/Sounds/Submarine.aiff",
}

// linuxPresets map to freedesktop sound names commonly shipped by
// sound-theme-freedesktop. Distros that omit them will surface the missing
// file as a Play error, which the subscriber logs at debug level.
var linuxPresets = map[string]string{
	PresetGlass:  "/usr/share/sounds/freedesktop/stereo/complete.oga",
	PresetBasso:  "/usr/share/sounds/freedesktop/stereo/dialog-error.oga",
	PresetPing:   "/usr/share/sounds/freedesktop/stereo/message.oga",
	PresetHero:   "/usr/share/sounds/freedesktop/stereo/complete.oga",
	PresetFunk:   "/usr/share/sounds/freedesktop/stereo/bell.oga",
	PresetTink:   "/usr/share/sounds/freedesktop/stereo/message.oga",
	PresetSubmar: "/usr/share/sounds/freedesktop/stereo/bell.oga",
}

// Windows ships a small set of .wav files under %WINDIR%\Media. Users who
// want richer sounds can still pass an absolute path.
var windowsPresets = map[string]string{
	PresetGlass:  `C:\Windows\Media\tada.wav`,
	PresetBasso:  `C:\Windows\Media\chord.wav`,
	PresetPing:   `C:\Windows\Media\ding.wav`,
	PresetHero:   `C:\Windows\Media\tada.wav`,
	PresetFunk:   `C:\Windows\Media\notify.wav`,
	PresetTink:   `C:\Windows\Media\chimes.wav`,
	PresetSubmar: `C:\Windows\Media\Ring01.wav`,
}

// escapePSSingleQuoted escapes a string for use inside a PowerShell
// single-quoted literal. In PowerShell, single-quoted strings take every
// character verbatim except the single quote itself, which is escaped by
// doubling it (”). NTFS filenames can contain apostrophes — e.g.
// C:\Users\O'Neil\alert.wav — so without this escape the generated command
// would be a syntax error. Defined here (no build tag) so it can be
// unit-tested on any host platform.
func escapePSSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
