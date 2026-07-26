package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	transientMemoryControllerAgentName = "memory-controller"
	transientMemoryControllerPrompt    = "AGH memory controller transient runtime."
	transientModelStopTimeout          = 5 * time.Second
)

// TransientModelCall describes one provider-backed model turn that must not
// create a durable AGH session.
type TransientModelCall struct {
	Config          *aghconfig.Config
	Provider        string
	Model           string
	ReasoningEffort string
	CWD             string
	SystemPrompt    string
	Prompt          string
	MaxOutputBytes  int
}

// TransientModelResult reports whether the provider accepted the prompt.
// Once accepted, higher layers must not attempt a route fallback.
type TransientModelResult struct {
	Output   string
	Accepted bool
}

// InvokeTransientModel performs one ACP turn without publishing a session row.
func (m *Manager) InvokeTransientModel(
	ctx context.Context,
	call TransientModelCall,
) (result TransientModelResult, operationErr error) {
	if ctx == nil {
		return result, errors.New("session: transient model context is required")
	}
	if m == nil || m.driver == nil {
		return result, errors.New("session: transient model driver is required")
	}
	if call.Config == nil {
		return result, errors.New("session: transient model config is required")
	}
	if strings.TrimSpace(call.Prompt) == "" {
		return result, errors.New("session: transient model prompt is required")
	}
	if call.MaxOutputBytes <= 0 {
		return result, errors.New("session: transient model output bound must be positive")
	}

	resolved, err := call.Config.ResolveAgent(aghconfig.AgentDef{
		Name:            transientMemoryControllerAgentName,
		Provider:        strings.TrimSpace(call.Provider),
		Model:           strings.TrimSpace(call.Model),
		ReasoningEffort: strings.TrimSpace(call.ReasoningEffort),
		Permissions:     string(aghconfig.PermissionModeDenyAll),
		Prompt:          transientMemoryControllerPrompt,
	})
	if err != nil {
		return result, fmt.Errorf("session: resolve transient model runtime: %w", err)
	}

	startOpts := acp.StartOpts{
		AgentName:       transientMemoryControllerAgentName,
		Command:         resolved.Command,
		Cwd:             strings.TrimSpace(call.CWD),
		Env:             sessionStartEnvForProvider(os.Environ(), nil, resolved.EnvPolicy),
		Permissions:     aghconfig.PermissionModeDenyAll,
		SystemPrompt:    strings.TrimSpace(call.SystemPrompt),
		PreferredModel:  resolved.Model,
		ReasoningEffort: resolved.ReasoningEffort,
	}
	startOpts, cleanup, err := m.prepareTransientModelProvider(ctx, resolved, startOpts)
	if err != nil {
		if cleanup != nil {
			err = errors.Join(err, cleanup())
		}
		return result, err
	}
	defer func() {
		operationErr = errors.Join(operationErr, cleanup())
	}()

	process, err := m.driver.Start(ctx, startOpts)
	if err != nil {
		return result, fmt.Errorf("session: start transient model: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), transientModelStopTimeout)
		defer cancel()
		if stopErr := m.driver.Stop(stopCtx, process); stopErr != nil {
			operationErr = errors.Join(operationErr, fmt.Errorf("session: stop transient model: %w", stopErr))
		}
	}()

	events, err := m.driver.Prompt(ctx, process, acp.PromptRequest{
		TurnID:  newID("turn"),
		Message: call.Prompt,
	})
	if err != nil {
		return result, fmt.Errorf("session: prompt transient model: %w", err)
	}
	result.Accepted = true
	result.Output, err = collectTransientModelOutput(ctx, events, call.MaxOutputBytes)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (m *Manager) prepareTransientModelProvider(
	ctx context.Context,
	resolved aghconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, func() error, error) {
	prepared, bindings, err := m.prepareProviderStartPolicies(ctx, resolved, opts)
	if err != nil {
		return acp.StartOpts{}, nil, err
	}
	opts = prepared
	runtimeDir := ""
	cleanup := func() error {
		runProviderSecretRedactions(bindings.redactionCleanups)
		if runtimeDir == "" {
			return nil
		}
		if err := os.RemoveAll(runtimeDir); err != nil {
			return fmt.Errorf("session: remove transient provider runtime %q: %w", runtimeDir, err)
		}
		return nil
	}
	if resolved.Harness == aghconfig.ProviderHarnessPiACP &&
		resolved.AuthMode == aghconfig.ProviderAuthModeBoundSecret {
		runtimeDir, err = os.MkdirTemp(m.homePaths.HomeDir, ".memory-controller-")
		if err != nil {
			return acp.StartOpts{}, cleanup, fmt.Errorf("session: create transient provider runtime: %w", err)
		}
		piDir := filepath.Join(runtimeDir, "pi")
		if err := materializePiRuntimeAt(piDir, resolved, bindings.injectedTargetEnvs); err != nil {
			return acp.StartOpts{}, cleanup, err
		}
		opts.Env = setSessionStartEnvValue(opts.Env, "PI_CODING_AGENT_DIR", piDir)
	}
	return opts, cleanup, nil
}

func collectTransientModelOutput(
	ctx context.Context,
	events <-chan acp.AgentEvent,
	maxBytes int,
) (string, error) {
	var output strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("session: collect transient model output: %w", ctx.Err())
		case event, ok := <-events:
			if !ok {
				return output.String(), nil
			}
			switch event.Type {
			case acp.EventTypeAgentMessage:
				if output.Len()+len(event.Text) > maxBytes {
					return "", errors.New("session: transient model output exceeds byte limit")
				}
				output.WriteString(event.Text)
			case acp.EventTypeError:
				return "", fmt.Errorf("session: transient model error: %s", strings.TrimSpace(event.Error))
			}
		}
	}
}
