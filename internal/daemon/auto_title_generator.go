package daemon

import (
	"context"
	"encoding/json"
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
	output, err := role.invokeRoleWithFallback(runCtx, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (string, bool, error) {
		spawned, spawnErr := g.sessions.Spawn(attemptCtx, g.autoTitleSpawnOpts(request, route))
		if spawnErr != nil {
			// Spawn errors can retain an accepted child that still requires cleanup.
			child = spawned
			return "", session.StartAccepted(spawnErr) || spawned != nil,
				fmt.Errorf("daemon: spawn automatic title session: %w", spawnErr)
		}
		if spawned == nil {
			return "", false, errors.New("daemon: automatic title spawn returned no session")
		}
		collected, attemptErr := g.runAutoTitleTurn(attemptCtx, spawned.ID, prompt)
		if errors.Is(attemptErr, errProviderRefusedTurn) {
			discardErr := g.discardAutoTitleAttempt(ctx, spawned.ID)
			if discardErr == nil {
				return "", false, attemptErr
			}
			// A failed stop keeps the child owned and prevents another route from starting.
			child = spawned
			return "", true, errors.Join(attemptErr, discardErr)
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
	title = session.CleanAutomaticSessionTitle(output)
	if title == "" {
		return "", errors.New("daemon: automatic title output has no usable title")
	}
	return title, nil
}

func (g *forkedAutoTitleGenerator) autoTitleSpawnOpts(
	request autoTitleRequest,
	route roleAttemptRoute,
) session.SpawnOpts {
	return session.SpawnOpts{
		ParentSessionID:     strings.TrimSpace(request.SessionID),
		AgentName:           route.AgentName,
		Provider:            route.Provider,
		Model:               route.Model,
		ReasoningEffort:     route.ReasoningEffort,
		Speed:               route.Speed,
		ACPOptions:          session.ACPOptionSelectionsFromConfig(route.ACPOptions),
		Command:             route.Command,
		SessionEnv:          route.SessionEnv,
		ChainOwner:          session.ChainOwnerCaller,
		Name:                autoTitleSessionName,
		PromptOverlay:       autoTitlePromptOverlay(),
		SpawnRole:           session.SpawnRoleAutoTitle,
		TTL:                 g.childTTL(),
		AutoStopOnParent:    true,
		DiscardStartFailure: true,
	}
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
	// Providers can send several messages in one turn. A separate message starts
	// a new line so its text cannot glue onto the title.
	separate := false
	messageID := ""
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
				nextID := autoTitleMessageIdentity(event.Raw)
				if separate || nextID != messageID {
					event.Text = "\n" + event.Text
				}
				separate, messageID = false, nextID
				if output.Len()+len(event.Text) > autoTitleMaxOutputBytes {
					return "", errors.New("daemon: automatic title output exceeds byte limit")
				}
				output.WriteString(event.Text)
			case acp.EventTypeThought, acp.EventTypeToolCall, acp.EventTypeToolResult:
				separate = output.Len() > 0
			case acp.EventTypeError:
				agentErr := fmt.Errorf(
					"daemon: automatic title agent error: %s", strings.TrimSpace(event.Error),
				)
				if output.Len() > 0 {
					// The turn already produced work; refusing it now is not a
					// pre-output refusal and must not advance to another route.
					return "", agentErr
				}
				return "", providerRefusedTurnError(event, agentErr)
			}
		}
	}
}

// autoTitleMessageIdentity reads the optional ACP message id of a text chunk.
func autoTitleMessageIdentity(raw json.RawMessage) string {
	var chunk struct {
		MessageID string `json:"messageId"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &chunk) != nil {
		return ""
	}
	return chunk.MessageID
}
