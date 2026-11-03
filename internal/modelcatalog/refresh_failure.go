package modelcatalog

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/compozy/compozy/internal/providerexec"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/vault"
)

// permanentSourceError marks an I/O-boundary failure that retrying cannot fix.
type permanentSourceError struct {
	err error
}

func (e *permanentSourceError) Error() string {
	if e == nil || e.err == nil {
		return "model catalog: source unavailable"
	}
	return e.err.Error()
}

func (e *permanentSourceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// permanentSourceFailure marks err as permanent without changing its diagnostic text.
func permanentSourceFailure(err error) error {
	if err == nil {
		return nil
	}
	return &permanentSourceError{err: err}
}

// IsPermanentSourceFailure identifies failures requiring a config, credential, binary, or login change.
func IsPermanentSourceFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if _, ok := errors.AsType[*permanentSourceError](err); ok {
		return true
	}
	if errors.Is(err, vault.ErrMissingSecret) || errors.Is(err, vault.ErrSecretNotFound) ||
		errors.Is(err, exec.ErrNotFound) || isMissingExecutableStart(err) {
		return true
	}
	return requiresLoginText(err.Error())
}

// requiresLoginText parks explicit login failures; generic permission denials remain transient.
func requiresLoginText(text string) bool {
	classification, ok := authproviders.ClassifyErrorText(text)
	return ok && classification.State == authproviders.ProviderAuthStateNeedsLogin
}

func isMissingExecutableStart(err error) bool {
	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok || pathErr.Op != "fork/exec" || !errors.Is(pathErr.Err, fs.ErrNotExist) {
		return false
	}
	// An existing script can fail ENOENT when its interpreter is absent; preflight cannot observe that.
	_, statErr := os.Stat(pathErr.Path)
	return errors.Is(statErr, fs.ErrNotExist)
}

// PermanentFailureHeld identifies failed TTL sources parked without a scheduled refresh.
func (s SourceStatus) PermanentFailureHeld() bool {
	return s.RefreshState == RefreshStateFailed &&
		!s.LastRefresh.IsZero() &&
		s.NextRefresh.IsZero() &&
		strings.TrimSpace(s.LastError) != ""
}

// HeldCauseUnchanged compares cold-start preflight with the persisted reason; login needs an explicit refresh.
func (s SourceStatus) HeldCauseUnchanged(preflightErr error) bool {
	if preflightErr == nil {
		return requiresLoginText(s.LastError)
	}
	return strings.Contains(s.LastError, RedactString(preflightErr.Error()))
}

// DiscoveryPreflight checks the effective discovery prerequisites without starting it or writing state.
func (s *LiveProviderSource) DiscoveryPreflight(ctx context.Context) error {
	if s == nil {
		return errors.New("model catalog: live provider source is required")
	}
	if ctx == nil {
		return errors.New("model catalog: live provider context is required")
	}
	provider := s.providerSnapshot()
	target, err := s.discoveryTarget(provider)
	if err != nil {
		return err
	}
	env, err := s.discoveryEnv(ctx, provider)
	if err != nil {
		return err
	}
	switch target.kind {
	case liveDiscoveryHTTP:
		return s.requireRequestCredential(env)
	case liveDiscoveryCommand:
		if isAutomaticOpenCodeDiscovery(s.providerID, provider) {
			_, err := providerexec.ResolveOpenCodeCommand(provider, env, s.workingDir)
			return err
		}
		return s.resolveDiscoveryExecutable(target.command, env)
	case liveDiscoveryACP:
		provider.Command = target.command
		env, err := nativeCLIACPEnv(provider, env, strings.TrimSpace(s.workingDir))
		if err != nil {
			return err
		}
		// Native Codex discovery runs the resolved CLI directly and never starts the ACP launcher.
		if s.usesNativeCodexDiscovery(provider) {
			return nil
		}
		return s.resolveDiscoveryExecutable(target.command, env)
	}
	return nil
}

func (s *LiveProviderSource) resolveDiscoveryExecutable(command string, env []string) error {
	bin, _, err := parseDiscoveryCommand(command)
	if err != nil {
		return err
	}
	_, err = subprocess.ResolveExecutable(bin, env, s.workingDir)
	return err
}
