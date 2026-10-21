package providerusage

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

type credentialReadFence struct {
	identity CredentialIdentity
	files    []os.FileInfo
}

// Only local file evidence is probed here; collection and Keychain reads stay off the read path.
func (s *Manager) fenceReadLocked(ctx context.Context, profile string) {
	p := s.profiles[profile]
	if !s.enabled || s.stopped || p == nil || ctx.Err() != nil {
		return
	}
	for _, id := range p.order {
		state := p.targets[id]
		if !state.hasIdentity || state.target.Status != "" {
			continue
		}
		paths := credentialEvidencePaths(state.target)
		if len(paths) == 0 {
			continue
		}
		files, cacheable := credentialFileStats(paths)
		if cacheable && state.readFence != nil && state.readFence.identity == state.identity &&
			sameCredentialFiles(state.readFence.files, files) {
			continue
		}
		mismatch, err := validatePublication(ctx, s.options.Collectors[id], state.target, state.identity)
		if !mismatch {
			if cacheable && err == nil {
				state.readFence = &credentialReadFence{identity: state.identity, files: files}
			}
			continue
		}
		state.epoch++
		state.snapshot = initialSnapshot(state.target, s.options.Collectors[id])
		state.hasIdentity = false
		state.readFence = nil
		state.lastSuccess = time.Time{}
		state.cooldown = nil
		s.changedLocked(p)
		s.admitFlightsLocked(p, []string{id}, RefreshRequest{ProfileID: profile, Intent: IntentBackground})
	}
}

func credentialFileStats(paths []string) ([]os.FileInfo, bool) {
	files := make([]os.FileInfo, len(paths))
	cacheable := true
	for i, path := range paths {
		info, err := os.Stat(path)
		files[i] = info
		if err != nil && !os.IsNotExist(err) {
			cacheable = false
		}
	}
	return files, cacheable
}

func sameCredentialFiles(a, b []os.FileInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i, previous := range a {
		current := b[i]
		if previous == nil || current == nil {
			if previous != nil || current != nil {
				return false
			}
			continue
		}
		if !os.SameFile(previous, current) || previous.Size() != current.Size() ||
			!previous.ModTime().Equal(current.ModTime()) || previous.Mode() != current.Mode() {
			return false
		}
	}
	return true
}

func credentialEvidencePaths(target Target) []string {
	switch target.ProviderID {
	case "codex":
		home := codexHome(target)
		if home == "" {
			return nil
		}
		return []string{filepath.Join(home, "auth.json"), filepath.Join(home, "config.toml")}
	case "claude":
		legacy, modern := claudeAccountPaths(target)
		return []string{claudeCredentialPath(target), legacy, modern}
	case "cursor":
		home, err := collectorHomePath(target, "XDG_CONFIG_HOME", ".config")
		if err != nil {
			return nil
		}
		return []string{filepath.Join(home, "cursor", "auth.json")}
	case "gemini":
		home, err := collectorHomePath(target, "", ".gemini")
		if err != nil {
			return nil
		}
		return []string{filepath.Join(home, "oauth_creds.json")}
	case "kimi-cli":
		home, err := collectorHomePath(target, "KIMI_CODE_HOME", ".kimi-code")
		if err != nil {
			return nil
		}
		return []string{filepath.Join(home, "credentials", "kimi-code.json")}
	default:
		return nil
	}
}
