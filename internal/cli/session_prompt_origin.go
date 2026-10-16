package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/spf13/cobra"
)

// fromSessionNone opts a prompt sent from inside a managed session out of the session origin.
const fromSessionNone = "none"

type sessionPromptAsClient interface {
	SendSessionPromptAs(
		context.Context,
		string,
		SessionPromptRequest,
		agentidentity.Credentials,
	) (SessionPromptRecord, error)
}

type sessionPromptStreamAsClient interface {
	StreamPromptSessionAs(
		context.Context,
		string,
		SessionPromptRequest,
		agentidentity.Credentials,
		SSEHandler,
	) error
}

var errPromptReplyRequiresSession = errors.New(
	"cli: --in-reply-to requires a calling agent session (" + agentidentity.EnvSessionID + " or --from-session)",
)

func bindSessionPromptOriginFlags(cmd *cobra.Command, flags *sessionPromptCommandFlags) {
	cmd.Flags().StringVar(
		&flags.fromSession,
		"from-session",
		"",
		"Send as this agent session so the receiver sees who wrote it and can reply "+
			"(defaults to "+agentidentity.EnvSessionID+" inside a managed session; \"none\" sends as operator input)",
	)
	cmd.Flags().StringVar(
		&flags.inReplyTo,
		"in-reply-to",
		"",
		"Message id this prompt answers; requires a calling agent session",
	)
	cmd.Flags().BoolVar(
		&flags.supersede,
		"supersede",
		false,
		"Replace your own older queued prompt to the same session",
	)
}

// originCredentials resolves which agent session, if any, the prompt is sent as. Inside a managed session
// the daemon-issued environment is used; --from-session names a session explicitly and is validated by the
// daemon like every agent identity. Outside a session nothing is stamped unless the flag is given.
func (flags *sessionPromptCommandFlags) originCredentials(
	ctx context.Context,
	deps commandDeps,
	client any,
) (agentidentity.Credentials, error) {
	fromSession := strings.TrimSpace(flags.fromSession)
	if fromSession == fromSessionNone {
		return agentidentity.Credentials{}, flags.requireNoReply()
	}
	env := agentCredentialsFromEnv(deps)
	if fromSession == "" || fromSession == env.SessionID {
		if env.SessionID == "" {
			return agentidentity.Credentials{}, flags.requireNoReply()
		}
		return env, nil
	}
	lookup, ok := client.(agentSessionClient)
	if !ok {
		return agentidentity.Credentials{}, errors.New("cli: --from-session requires a daemon session lookup")
	}
	record, err := lookup.GetSession(ctx, fromSession)
	if err != nil {
		return agentidentity.Credentials{}, fmt.Errorf("cli: resolve --from-session %q: %w", fromSession, err)
	}
	return agentidentity.Credentials{SessionID: fromSession, AgentName: strings.TrimSpace(record.AgentName)}, nil
}

func (flags *sessionPromptCommandFlags) requireNoReply() error {
	if strings.TrimSpace(flags.inReplyTo) != "" {
		return errPromptReplyRequiresSession
	}
	return nil
}

func (flags *sessionPromptCommandFlags) validateOriginFlags(cmd *cobra.Command) error {
	if cmd.Flags().Changed("from-session") && strings.TrimSpace(flags.fromSession) == "" {
		return errors.New("cli: --from-session must not be blank")
	}
	if cmd.Flags().Changed("in-reply-to") && strings.TrimSpace(flags.inReplyTo) == "" {
		return errors.New("cli: --in-reply-to must not be blank")
	}
	if flags.steer && (cmd.Flags().Changed("from-session") || cmd.Flags().Changed("in-reply-to") || flags.supersede) {
		return errors.New("cli: --from-session, --in-reply-to, and --supersede apply only to submitted prompts")
	}
	return nil
}

func hasOriginCredentials(credentials agentidentity.Credentials) bool {
	return strings.TrimSpace(credentials.SessionID) != ""
}

func sendSessionPromptWithOrigin(
	ctx context.Context,
	client sessionPromptClient,
	id string,
	request SessionPromptRequest,
	credentials agentidentity.Credentials,
) (SessionPromptRecord, error) {
	if asClient, ok := client.(sessionPromptAsClient); ok && hasOriginCredentials(credentials) {
		return asClient.SendSessionPromptAs(ctx, id, request, credentials)
	}
	return client.SendSessionPrompt(ctx, id, request)
}

func streamSessionPromptWithOrigin(
	ctx context.Context,
	client sessionPromptStreamClient,
	id string,
	request SessionPromptRequest,
	credentials agentidentity.Credentials,
	handler SSEHandler,
) error {
	if asClient, ok := client.(sessionPromptStreamAsClient); ok && hasOriginCredentials(credentials) {
		return asClient.StreamPromptSessionAs(ctx, id, request, credentials, handler)
	}
	return client.StreamPromptSession(ctx, id, request, handler)
}
