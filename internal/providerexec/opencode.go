package providerexec

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/subprocess"
)

const openCodeNativeCommand = "opencode"

var openCodeNPMFallbackPrefix = []string{"-y", "opencode-ai@latest"}

// ResolveOpenCodeCommand returns the existing LaunchCommand with an absolute
// executable, preserved args including the acp operation, and private leading
// environment assignments. It performs no version probe, installation, or login.
func ResolveOpenCodeCommand(
	provider compozyconfig.ProviderConfig,
	env []string,
	dir string,
) (compozyconfig.LaunchCommand, error) {
	raw := strings.TrimSpace(provider.Command)
	if raw == "" {
		return compozyconfig.LaunchCommand{}, errors.New("providerexec: OpenCode launch command is required")
	}
	parsed, err := compozyconfig.ParseLaunchCommand(raw)
	if err != nil {
		return compozyconfig.LaunchCommand{}, fmt.Errorf("providerexec: resolve OpenCode command: %w", err)
	}
	effectiveEnv := ApplyOpenCodeAssignments(env, parsed.Environment)
	if !provider.CommandIsBuiltin {
		resolved, err := subprocess.ResolveExecutable(parsed.Executable, effectiveEnv, dir)
		if err != nil {
			return compozyconfig.LaunchCommand{}, err
		}
		return compozyconfig.LaunchCommand{
			Executable:  resolved,
			Args:        append([]string(nil), parsed.Args...),
			Environment: append([]string(nil), parsed.Environment...),
		}, nil
	}
	resolved, err := subprocess.ResolveExecutable(parsed.Executable, effectiveEnv, dir)
	if err == nil {
		return compozyconfig.LaunchCommand{
			Executable:  resolved,
			Args:        append([]string(nil), parsed.Args...),
			Environment: append([]string(nil), parsed.Environment...),
		}, nil
	}
	if !errors.Is(err, exec.ErrNotFound) {
		return compozyconfig.LaunchCommand{}, err
	}
	npxResolved, npxErr := subprocess.ResolveExecutable("npx", effectiveEnv, dir)
	if npxErr != nil {
		return compozyconfig.LaunchCommand{}, npxErr
	}
	fallbackArgs := append(append([]string(nil), openCodeNPMFallbackPrefix...), parsed.Args...)
	return compozyconfig.LaunchCommand{
		Executable:  npxResolved,
		Args:        fallbackArgs,
		Environment: append([]string(nil), parsed.Environment...),
	}, nil
}

// DeriveOpenCodeDiscoveryBase strips a safely derivable terminal acp operation,
// preserving the installation selector and private assignments for Task 02 to
// substitute version/list operations. Opaque wrappers fail actionably.
func DeriveOpenCodeDiscoveryBase(cmd compozyconfig.LaunchCommand) (compozyconfig.LaunchCommand, error) {
	if len(cmd.Args) == 0 || cmd.Args[len(cmd.Args)-1] != "acp" {
		return compozyconfig.LaunchCommand{}, errors.New(
			"model catalog: OpenCode launch command requires an explicit models.discovery.command or models.discovery.endpoint",
		)
	}
	base := strings.ToLower(strings.TrimSpace(filepath.Base(cmd.Executable)))
	// Mirror subprocess PATHEXT defaults so .cmd/.bat npm shims derive like their Unix names.
	for _, suffix := range []string{".com", ".exe", ".bat", ".cmd"} {
		base = strings.TrimSuffix(base, suffix)
	}
	switch base {
	case openCodeNativeCommand:
		if len(cmd.Args) != 1 {
			return compozyconfig.LaunchCommand{}, errors.New(
				"model catalog: OpenCode launch command requires an explicit models.discovery.command or models.discovery.endpoint",
			)
		}
	case "npx":
		if !openCodeNPMPrefixHasPackage(cmd.Args[:len(cmd.Args)-1]) {
			return compozyconfig.LaunchCommand{}, errors.New(
				"model catalog: OpenCode launch command requires an explicit models.discovery.command or models.discovery.endpoint",
			)
		}
	default:
		return compozyconfig.LaunchCommand{}, errors.New(
			"model catalog: OpenCode launch command requires an explicit models.discovery.command or models.discovery.endpoint",
		)
	}
	return compozyconfig.LaunchCommand{
		Executable:  cmd.Executable,
		Args:        append([]string(nil), cmd.Args[:len(cmd.Args)-1]...),
		Environment: append([]string(nil), cmd.Environment...),
	}, nil
}

// OpenCodeInvocationFingerprint is the pure selector/origin identity consumed by
// the catalog. It hashes the prepared absolute identity and private assignments
// without resolving secrets, probing versions, or installing packages.
func OpenCodeInvocationFingerprint(cmd compozyconfig.LaunchCommand, builtin bool) string {
	origin := "authored"
	if builtin {
		origin = "builtin"
	}
	parts := []string{
		strings.TrimSpace(cmd.Executable),
		strings.Join(cmd.Args, "\x1f"),
		origin,
		strings.Join(cmd.Environment, "\x1f"),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", digest[:])
}

func openCodeNPMPrefixHasPackage(prefix []string) bool {
	for _, arg := range prefix {
		if strings.Contains(arg, "opencode-ai") {
			return true
		}
	}
	return false
}

// ApplyOpenCodeAssignments overlays private leading NAME=value assignments on an environment.
func ApplyOpenCodeAssignments(env []string, assignments []string) []string {
	effective := append([]string(nil), env...)
	for _, assignment := range assignments {
		key, value, ok := strings.Cut(assignment, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		entry := key + "=" + value
		replaced := false
		for i, current := range effective {
			if currentKey, _, ok := strings.Cut(current, "="); ok && currentKey == key {
				effective[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			effective = append(effective, entry)
		}
	}
	return effective
}
