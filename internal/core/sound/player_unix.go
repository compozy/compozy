//go:build !windows

package sound

import "runtime"

// newOSPlayer returns the unix-family player. It uses afplay on darwin and
// paplay on linux. Other unix variants fall back to the Noop player because
// they have no universally available command-line sound tool.
func newOSPlayer() Player {
	switch runtime.GOOS {
	case goosDarwin, goosLinux:
		return &osPlayer{runner: execRunner{}, resolve: resolveUnixCommand}
	default:
		return Noop{}
	}
}

// resolveUnixCommand maps a preset name or absolute path to the
// platform-native playback command. The caller guarantees a non-empty input.
func resolveUnixCommand(sound string) (string, []string, error) {
	path, err := ResolvePath(sound)
	if err != nil {
		return "", nil, err
	}
	switch runtime.GOOS {
	case goosDarwin:
		return cmdAfplay, []string{path}, nil
	case goosLinux:
		return cmdPaplay, []string{path}, nil
	default:
		return "", nil, ErrUnsupported
	}
}
