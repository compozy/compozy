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

type credentialReadCheck struct {
	state         *targetState
	target        Target
	identity      CredentialIdentity
	epoch         uint64
	validationSeq uint64
	fence         *credentialReadFence
}

// Filesystem and credential I/O must not serialize unrelated profile reads or lifecycle changes.
// Only local evidence is probed; collection and Keychain reads stay off the read path.
func (s *Manager) fenceRead(ctx context.Context, profile string) {
	for _, check := range s.credentialReadChecks(profile) {
		s.fenceReadCheck(ctx, profile, check)
	}
}

func (s *Manager) fenceReadCheck(ctx context.Context, profile string, check credentialReadCheck) {
	paths := credentialEvidencePaths(check.target)
	if len(paths) == 0 {
		return
	}
	for ctx.Err() == nil {
		files, cacheable := credentialFileStats(paths)
		if cacheable && check.fence != nil && check.fence.identity == check.identity &&
			sameCredentialFiles(check.fence.files, files) {
			s.applyCredentialReadCheck(profile, &check, check.fence, false, nil)
			return
		}
		mismatch, err := validatePublication(
			ctx, s.options.Collectors[check.target.Collector()], check.target, check.identity,
		)
		fence := check.fence
		if cacheable && err == nil && !mismatch {
			fence = &credentialReadFence{identity: check.identity, files: files}
		}
		if !s.applyCredentialReadCheck(profile, &check, fence, mismatch, err) {
			return
		}
	}
}

func (s *Manager) credentialReadChecks(profile string) []credentialReadCheck {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.profiles[profile]
	if !s.enabled || s.stopped || p == nil {
		return nil
	}
	checks := make([]credentialReadCheck, 0, len(p.order))
	for _, id := range p.order {
		state := p.targets[id]
		if state.hasIdentity && state.target.Status == "" {
			checks = append(checks, credentialReadCheck{
				state: state, target: state.target, identity: state.identity, epoch: state.epoch,
				validationSeq: state.validationSeq, fence: state.readFence,
			})
		}
	}
	return checks
}

func (s *Manager) applyCredentialReadCheck(
	profile string, check *credentialReadCheck, fence *credentialReadFence, mismatch bool, validationErr error,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := check.target.ProviderID
	state := check.state
	if !s.currentLocked(profile, id, state) || !state.hasIdentity ||
		state.epoch != check.epoch || state.identity != check.identity {
		return false
	}
	if state.validationSeq != check.validationSeq {
		if mismatch {
			// Apply order alone cannot establish which credential observation was newer.
			check.validationSeq = state.validationSeq
			check.fence = state.readFence
		}
		return mismatch
	}
	if !mismatch {
		if validationErr == nil {
			// Successful reads fence older observations just like verified collection does.
			state.validationSeq++
			state.readFence = fence
		}
		return false
	}
	state.epoch++
	state.snapshot = initialSnapshot(state.target, s.options.Collectors[state.target.Collector()])
	state.hasIdentity = false
	state.readFence = nil
	state.lastSuccess = time.Time{}
	state.cooldown = nil
	p := s.profiles[profile]
	s.changedLocked(p)
	s.admitFlightsLocked(p, []string{id}, RefreshRequest{ProfileID: profile, Intent: IntentBackground})
	return false
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
	switch target.Collector() {
	case codexProviderID:
		home := codexHome(target)
		if home == "" {
			return nil
		}
		return []string{filepath.Join(home, "auth.json"), filepath.Join(home, "config.toml")}
	case claudeProviderID:
		legacy, modern := claudeAccountPaths(target)
		return []string{claudeCredentialPath(target), legacy, modern}
	case "cursor":
		home, err := collectorHomePath(target, cursorConfigHomeKey, ".config")
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
		home, err := collectorHomePath(target, kimiCodeHomeKey, ".kimi-code")
		if err != nil {
			return nil
		}
		return []string{filepath.Join(home, "credentials", "kimi-code.json")}
	default:
		return nil
	}
}
