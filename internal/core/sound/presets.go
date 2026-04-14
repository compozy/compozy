package sound

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// preset names accepted in config.toml. Absolute paths bypass preset lookup.
const (
	PresetGlass     = "glass"
	PresetBasso     = "basso"
	PresetPing      = "ping"
	PresetHero      = "hero"
	PresetFunk      = "funk"
	PresetTink      = "tink"
	PresetSubmarine = "submarine"
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
		PresetSubmarine,
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
// ("", false) otherwise. Windows lookups are resolved at call time so they
// honor a caller-provided WINDIR (falling back to C:\Windows) rather than
// a hardcoded install path.
func presetPathForOS(preset, goos string) (string, bool) {
	switch goos {
	case goosDarwin:
		path, ok := darwinPresets[preset]
		return path, ok
	case goosLinux:
		path, ok := linuxPresets[preset]
		return path, ok
	case goosWindows:
		return windowsPresetPath(preset, os.Getenv("WINDIR"))
	default:
		return "", false
	}
}

// windowsPresetPath resolves a preset name to an absolute Windows path by
// joining the Media directory of the active Windows install. It accepts
// windir explicitly so tests can exercise both the set and fallback branches
// without touching process environment state. Backslashes are used directly
// instead of filepath.Join because non-windows test hosts would otherwise
// emit forward-slash separators for a path the OS will never see.
func windowsPresetPath(preset, windir string) (string, bool) {
	file, ok := windowsPresetFiles[preset]
	if !ok {
		return "", false
	}
	root := strings.TrimRight(strings.TrimSpace(windir), `\`)
	if root == "" {
		root = defaultWindowsDir
	}
	return root + `\Media\` + file, true
}

const defaultWindowsDir = `C:\Windows`

var darwinPresets = map[string]string{
	PresetGlass:     "/System/Library/Sounds/Glass.aiff",
	PresetBasso:     "/System/Library/Sounds/Basso.aiff",
	PresetPing:      "/System/Library/Sounds/Ping.aiff",
	PresetHero:      "/System/Library/Sounds/Hero.aiff",
	PresetFunk:      "/System/Library/Sounds/Funk.aiff",
	PresetTink:      "/System/Library/Sounds/Tink.aiff",
	PresetSubmarine: "/System/Library/Sounds/Submarine.aiff",
}

// linuxPresets map to freedesktop sound names commonly shipped by
// sound-theme-freedesktop. Distros that omit them will surface the missing
// file as a Play error, which the subscriber logs at debug level.
var linuxPresets = map[string]string{
	PresetGlass:     "/usr/share/sounds/freedesktop/stereo/complete.oga",
	PresetBasso:     "/usr/share/sounds/freedesktop/stereo/dialog-error.oga",
	PresetPing:      "/usr/share/sounds/freedesktop/stereo/message.oga",
	PresetHero:      "/usr/share/sounds/freedesktop/stereo/complete.oga",
	PresetFunk:      "/usr/share/sounds/freedesktop/stereo/bell.oga",
	PresetTink:      "/usr/share/sounds/freedesktop/stereo/message.oga",
	PresetSubmarine: "/usr/share/sounds/freedesktop/stereo/bell.oga",
}

// windowsPresetFiles maps each preset to the basename of its .wav under the
// active Windows install's Media directory. The full path is built at lookup
// time using %WINDIR% so non-standard installs (D:\Windows etc.) resolve
// correctly. Users who want richer sounds can still pass an absolute path.
var windowsPresetFiles = map[string]string{
	PresetGlass:     "tada.wav",
	PresetBasso:     "chord.wav",
	PresetPing:      "ding.wav",
	PresetHero:      "tada.wav",
	PresetFunk:      "notify.wav",
	PresetTink:      "chimes.wav",
	PresetSubmarine: "Ring01.wav",
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
