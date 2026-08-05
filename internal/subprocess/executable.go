package subprocess

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const defaultWindowsPathExt = ".COM;.EXE;.BAT;.CMD"

var errExecutableCandidateMissing = errors.New("subprocess: executable candidate is unavailable")

// ResolveExecutable resolves command against the exact launch environment and
// working directory that will be used by the child process.
func ResolveExecutable(command string, env []string, dir string) (string, error) {
	if command == "" {
		return "", executableNotFound(command)
	}
	base, err := executableSearchBase(dir)
	if err != nil {
		return "", err
	}
	environment := effectiveEnvironment(env)
	pathExt, _ := LookupEnv(environment, "PATHEXT")
	if executableHasPathSeparator(command) || filepath.IsAbs(command) {
		resolved, resolveErr := resolveExecutableCandidate(command, pathExt, base)
		if errors.Is(resolveErr, errExecutableCandidateMissing) {
			return "", executableNotFound(command)
		}
		return resolved, resolveErr
	}

	pathValue, ok := LookupEnv(environment, "PATH")
	if !ok {
		return "", executableNotFound(command)
	}
	for _, pathEntry := range filepath.SplitList(pathValue) {
		if runtime.GOOS == subprocessWindowsKey {
			pathEntry = strings.Trim(pathEntry, `"`)
		}
		if pathEntry == "" {
			pathEntry = "."
		}
		implicitRelativePath := !filepath.IsAbs(pathEntry)
		resolved, resolveErr := resolveExecutableCandidate(filepath.Join(pathEntry, command), pathExt, base)
		if resolveErr == nil {
			if implicitRelativePath {
				return "", fmt.Errorf("%w: %s", exec.ErrDot, resolved)
			}
			return resolved, nil
		}
		if !errors.Is(resolveErr, errExecutableCandidateMissing) {
			return "", resolveErr
		}
	}
	return "", executableNotFound(command)
}

func executableSearchBase(dir string) (string, error) {
	base := dir
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("subprocess: resolve launch working directory: %w", err)
		}
	} else if !filepath.IsAbs(base) {
		absoluteBase, err := filepath.Abs(base)
		if err != nil {
			return "", fmt.Errorf("subprocess: resolve launch directory: %w", err)
		}
		base = absoluteBase
	}
	info, err := os.Stat(base)
	if err != nil {
		return "", fmt.Errorf("subprocess: inspect launch directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("subprocess: launch working directory is not a directory")
	}
	return filepath.Clean(base), nil
}

func effectiveEnvironment(env []string) []string {
	if len(env) == 0 {
		return os.Environ()
	}
	return append([]string(nil), env...)
}

func resolveExecutableCandidate(candidate string, pathExt string, base string) (string, error) {
	absoluteCandidate := absoluteExecutableCandidate(candidate, base)
	for _, executable := range executableCandidates(absoluteCandidate, pathExt) {
		info, statErr := os.Stat(executable)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return "", fmt.Errorf("subprocess: inspect executable candidate: %w", statErr)
		}
		if info.IsDir() {
			continue
		}
		if runtime.GOOS != subprocessWindowsKey && info.Mode().Perm()&0o111 == 0 {
			continue
		}
		return filepath.Clean(executable), nil
	}
	return "", errExecutableCandidateMissing
}

func absoluteExecutableCandidate(candidate string, base string) string {
	if filepath.IsAbs(candidate) {
		return filepath.Clean(candidate)
	}
	return filepath.Clean(filepath.Join(base, candidate))
}

func executableCandidates(candidate string, pathExt string) []string {
	if runtime.GOOS != subprocessWindowsKey || filepath.Ext(candidate) != "" {
		return []string{candidate}
	}
	if strings.TrimSpace(pathExt) == "" {
		pathExt = defaultWindowsPathExt
	}
	exts := filepath.SplitList(pathExt)
	candidates := make([]string, 0, len(exts))
	for _, extension := range exts {
		extension = strings.TrimSpace(extension)
		if extension == "" {
			continue
		}
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		candidates = append(candidates, candidate+extension)
	}
	return candidates
}

func executableHasPathSeparator(command string) bool {
	if strings.ContainsRune(command, os.PathSeparator) {
		return true
	}
	return runtime.GOOS == subprocessWindowsKey && strings.ContainsRune(command, '/')
}

func executableNotFound(command string) error {
	return fmt.Errorf("%w: %s", exec.ErrNotFound, command)
}
