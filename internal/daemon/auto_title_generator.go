package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
)

const (
	autoTitleSessionName        = "Session title"
	autoTitleSyntheticTaskID    = "auto-title"
	autoTitleStopTimeout        = 10 * time.Second
	autoTitleMaxOutputBytes     = 4096
	autoTitleMaxPromptPartBytes = 16 << 10
)

type autoTitleSpawnSessions interface {
	Spawn(context.Context, session.SpawnOpts) (*session.Session, error)
	PromptSynthetic(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	StopWithCause(context.Context, string, session.StopCause, string) error
}

type autoTitleGenerator interface {
	Generate(context.Context, autoTitleRequest) (string, error)
}

type autoTitleRequest struct {
	SessionID      string
	ProfileID      string
	AgentName      string
	WorkspaceID    string
	UserMessage    string
	AssistantReply string
}

type forkedAutoTitleGenerator struct {
	sessions autoTitleSpawnSessions
	roles    RoleResolver
	deadline time.Duration
	logger   *slog.Logger
}

func newForkedAutoTitleGenerator(
	sessions autoTitleSpawnSessions,
	roles RoleResolver,
	deadline time.Duration,
	logger *slog.Logger,
) *forkedAutoTitleGenerator {
	if logger == nil {
		logger = slog.Default()
	}
	return &forkedAutoTitleGenerator{sessions: sessions, roles: roles, deadline: deadline, logger: logger}
}

var errAutoTitleRoleDisabled = errors.New("daemon: automatic title role is disabled")

func (g *forkedAutoTitleGenerator) Generate(
	ctx context.Context,
	request autoTitleRequest,
) (title string, err error) {
	if g == nil || g.sessions == nil {
		return "", errors.New("daemon: automatic title sessions are not configured")
	}
	if ctx == nil {
		return "", errors.New("daemon: automatic title context is required")
	}
	if g.roles == nil {
		return "", errors.New("daemon: automatic title role resolver is not configured")
	}
	correlation := roleInvocationCorrelation{
		ProfileID:   strings.TrimSpace(request.ProfileID),
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		SessionID:   strings.TrimSpace(request.SessionID),
		AgentName:   strings.TrimSpace(request.AgentName),
	}
	roleCtx := withRoleInvocationCorrelation(ctx, correlation)
	role, err := g.roles.Resolve(roleCtx, request.WorkspaceID, compozyconfig.RoleAutoTitle)
	if err != nil {
		return "", fmt.Errorf("daemon: resolve automatic title role: %w", err)
	}
	if !role.Enabled {
		return "", errAutoTitleRoleDisabled
	}
	if role.Inherit {
		role.AgentName = strings.TrimSpace(request.AgentName)
	}
	runCtx, cancelRun := autoTitleRunContext(ctx, g.deadline)
	defer cancelRun()
	prompt := renderAutoTitlePrompt(request)
	var child *session.Session
	output, err := invokeRoleWithFallback(runCtx, role, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (string, bool, error) {
		spawned, spawnErr := g.sessions.Spawn(attemptCtx, session.SpawnOpts{
			ParentSessionID:     strings.TrimSpace(request.SessionID),
			AgentName:           route.AgentName,
			Provider:            route.Provider,
			Model:               route.Model,
			ReasoningEffort:     route.ReasoningEffort,
			Speed:               route.Speed,
			ACPOptions:          session.ACPOptionSelectionsFromConfig(route.ACPOptions),
			Name:                autoTitleSessionName,
			PromptOverlay:       autoTitlePromptOverlay(),
			SpawnRole:           session.SpawnRoleAutoTitle,
			TTL:                 g.childTTL(),
			AutoStopOnParent:    true,
			DiscardStartFailure: true,
		})
		if spawnErr != nil {
			// Spawn can return a live session with a hook-dispatch error; keep the
			// pre-existing behavior of failing the attempt instead of prompting.
			child = spawned
			return "", spawned != nil, fmt.Errorf("daemon: spawn automatic title session: %w", spawnErr)
		}
		if spawned == nil {
			return "", false, errors.New("daemon: automatic title spawn returned no session")
		}
		collected, attemptErr := g.runAutoTitleTurn(attemptCtx, spawned.ID, prompt)
		if errors.Is(attemptErr, errProviderRefusedTurn) {
			return "", false, errors.Join(attemptErr, g.discardAutoTitleAttempt(ctx, spawned.ID))
		}
		child = spawned
		return collected, true, attemptErr
	})
	if child != nil {
		defer g.stopAutoTitleSession(ctx, child.ID, &title, &err)
	}
	if err != nil {
		return "", err
	}
	title = parseAutoTitleOutput(output)
	if title == "" {
		return "", errors.New("daemon: automatic title output is empty")
	}
	return title, nil
}

// runAutoTitleTurn prompts one spawned attempt and collects its output. A
// provider refusal is reported through errProviderRefusedTurn so the caller can
// advance to the next fallback route.
func (g *forkedAutoTitleGenerator) runAutoTitleTurn(
	ctx context.Context,
	sessionID string,
	prompt string,
) (string, error) {
	events, err := g.sessions.PromptSynthetic(ctx, sessionID, session.SyntheticPromptOpts{
		Message: prompt,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:  autoTitleSyntheticTaskID,
			Reason:  string(compozyconfig.RoleAutoTitle),
			Summary: "generate a concise session title",
		},
	})
	if err != nil {
		return "", fmt.Errorf("daemon: prompt automatic title session: %w", err)
	}
	return collectAutoTitleOutput(ctx, events)
}

// discardAutoTitleAttempt stops the child session of a refused route so the next
// attempt does not leave it running.
func (g *forkedAutoTitleGenerator) discardAutoTitleAttempt(ctx context.Context, sessionID string) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), autoTitleStopTimeout)
	defer cancel()
	if err := g.sessions.StopWithCause(
		stopCtx, sessionID, session.CauseFailed, "automatic title route refused by provider",
	); err != nil {
		return fmt.Errorf("daemon: stop refused automatic title session: %w", err)
	}
	return nil
}

func autoTitleRunContext(ctx context.Context, deadline time.Duration) (context.Context, context.CancelFunc) {
	if deadline > 0 {
		return context.WithTimeout(ctx, deadline)
	}
	return ctx, func() {}
}

func (g *forkedAutoTitleGenerator) stopAutoTitleSession(
	ctx context.Context,
	sessionID string,
	title *string,
	operationErr *error,
) {
	cause := session.CauseCompleted
	detail := "automatic title generation completed"
	if *operationErr != nil {
		cause = session.CauseFailed
		detail = "automatic title generation failed"
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), autoTitleStopTimeout)
	defer cancel()
	if err := g.sessions.StopWithCause(stopCtx, sessionID, cause, detail); err != nil {
		*title = ""
		*operationErr = errors.Join(*operationErr, fmt.Errorf("daemon: stop automatic title session: %w", err))
	}
}

func (g *forkedAutoTitleGenerator) childTTL() time.Duration {
	if g != nil && g.deadline > 0 {
		return g.deadline + autoTitleStopTimeout
	}
	return autoTitleStopTimeout
}

func renderAutoTitlePrompt(request autoTitleRequest) string {
	const promptTemplate = "Create a concise title for this session. " +
		"Return only the title, with no quotes or commentary.\n\n" +
		"User request:\n%s\n\nAssistant response:\n%s"
	return fmt.Sprintf(
		promptTemplate,
		boundAutoTitlePromptPart(request.UserMessage),
		boundAutoTitlePromptPart(request.AssistantReply),
	)
}

func boundAutoTitlePromptPart(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= autoTitleMaxPromptPartBytes {
		return trimmed
	}
	return strings.TrimSpace(truncateUTF8Bytes(trimmed, autoTitleMaxPromptPartBytes-len("…"))) + "…"
}

func autoTitlePromptOverlay() string {
	return strings.TrimSpace(`
You are an Compozy internal session-title generator.
Return only one concise title of at most eight words.
Do not modify files, run commands, or include markdown or commentary.
`)
}

func collectAutoTitleOutput(ctx context.Context, events <-chan acp.AgentEvent) (string, error) {
	var output strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("daemon: collect automatic title output: %w", ctx.Err())
		case event, ok := <-events:
			if !ok {
				return output.String(), nil
			}
			switch event.Type {
			case acp.EventTypeAgentMessage:
				if output.Len()+len(event.Text) > autoTitleMaxOutputBytes {
					return "", errors.New("daemon: automatic title output exceeds byte limit")
				}
				output.WriteString(event.Text)
			case acp.EventTypeError:
				return "", providerRefusedTurnError(event, fmt.Errorf(
					"daemon: automatic title agent error: %s", strings.TrimSpace(event.Error),
				))
			}
		}
	}
}

func parseAutoTitleOutput(output string) string {
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		candidate := strings.TrimSpace(line)
		candidate = strings.TrimPrefix(candidate, "Title:")
		candidate = strings.Trim(strings.TrimSpace(candidate), "`\"' ")
		if candidate != "" {
			return candidate
		}
	}
	return ""
}
