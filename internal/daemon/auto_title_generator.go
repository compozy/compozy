package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
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
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		SessionID:   strings.TrimSpace(request.SessionID),
		AgentName:   strings.TrimSpace(request.AgentName),
	}
	roleCtx := withRoleInvocationCorrelation(ctx, correlation)
	role, err := g.roles.Resolve(roleCtx, request.WorkspaceID, aghconfig.RoleAutoTitle)
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
	child, err := invokeRoleWithFallback(runCtx, role, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (*session.Session, bool, error) {
		spawned, spawnErr := g.sessions.Spawn(attemptCtx, session.SpawnOpts{
			ParentSessionID:  strings.TrimSpace(request.SessionID),
			AgentName:        route.AgentName,
			Provider:         route.Provider,
			Model:            route.Model,
			ReasoningEffort:  route.ReasoningEffort,
			Name:             autoTitleSessionName,
			PromptOverlay:    autoTitlePromptOverlay(),
			SpawnRole:        session.SpawnRoleAutoTitle,
			TTL:              g.childTTL(),
			AutoStopOnParent: true,
		})
		return spawned, spawned != nil, spawnErr
	})
	if child != nil {
		defer g.stopAutoTitleSession(ctx, child.ID, &title, &err)
	}
	if err != nil {
		return "", fmt.Errorf("daemon: spawn automatic title session: %w", err)
	}

	events, err := g.sessions.PromptSynthetic(runCtx, child.ID, session.SyntheticPromptOpts{
		Message: prompt,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:  autoTitleSyntheticTaskID,
			Reason:  string(aghconfig.RoleAutoTitle),
			Summary: "generate a concise session title",
		},
	})
	if err != nil {
		return "", fmt.Errorf("daemon: prompt automatic title session: %w", err)
	}
	output, err := collectAutoTitleOutput(runCtx, events)
	if err != nil {
		return "", err
	}
	title = parseAutoTitleOutput(output)
	if title == "" {
		return "", errors.New("daemon: automatic title output is empty")
	}
	return title, nil
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
You are an AGH internal session-title generator.
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
				return "", fmt.Errorf("daemon: automatic title agent error: %s", strings.TrimSpace(event.Error))
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
