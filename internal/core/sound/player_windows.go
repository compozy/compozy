//go:build windows

package sound

import "fmt"

// newOSPlayer returns the windows player, which shells out to powershell and
// uses System.Media.SoundPlayer to play the resolved file synchronously.
func newOSPlayer() Player {
	return &osPlayer{runner: execRunner{}, resolve: resolveWindowsCommand}
}

// resolveWindowsCommand maps a preset or absolute path to a powershell
// invocation. The caller guarantees a non-empty input.
func resolveWindowsCommand(sound string) (string, []string, error) {
	path, err := ResolvePath(sound)
	if err != nil {
		return "", nil, err
	}
	script := fmt.Sprintf("(New-Object Media.SoundPlayer '%s').PlaySync()", escapePSSingleQuoted(path))
	return "powershell", []string{"-NoProfile", "-Command", script}, nil
}
