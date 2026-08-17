package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/spf13/cobra"
)

const (
	sessionWaitExitInvalid     = 65
	sessionWaitExitUnavailable = 69
	sessionWaitExitTimeout     = 75
)

func newSessionWaitCommand(deps commandDeps) *cobra.Command {
	var (
		until     []string
		timeout   time.Duration
		unbounded bool
	)
	cmd := &cobra.Command{
		Use:   "wait <id>",
		Short: "Wait for a session state",
		Example: `  # Wait until a session needs input or becomes idle
  compozy session wait sess_1234 --until waiting-for-input,idle --timeout 120s

  # Keep waiting through bounded, gapless server requests
  compozy session wait sess_1234 --unbounded`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionWait(cmd, deps, args[0], until, timeout, unbounded)
		},
	}
	cmd.Flags().StringSliceVar(&until, "until", nil, "Session states to wait for")
	cmd.Flags().DurationVar(&timeout, cliTimeoutKey, session.WaitTimeoutDefault, "Maximum wait duration")
	cmd.Flags().BoolVar(&unbounded, "unbounded", false, "Keep waiting through bounded server requests")
	cmd.MarkFlagsMutuallyExclusive(cliTimeoutKey, "unbounded")
	return cmd
}

func runSessionWait(
	cmd *cobra.Command,
	deps commandDeps,
	sessionID string,
	until []string,
	timeout time.Duration,
	unbounded bool,
) error {
	normalized, err := normalizeSessionWaitUntil(until)
	if err != nil {
		return withCommandExitCode(sessionWaitExitInvalid, err)
	}
	if timeout <= 0 || timeout > session.WaitTimeoutMax {
		return withCommandExitCode(sessionWaitExitInvalid, fmt.Errorf(
			"cli: --timeout must be greater than zero and at most %s",
			session.WaitTimeoutMax,
		))
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	info, err := client.GetSession(cmd.Context(), sessionID)
	if err != nil {
		return withCommandExitCode(sessionWaitExitUnavailable, err)
	}
	requestUntil := make([]string, 0, len(normalized))
	for _, badge := range normalized {
		requestUntil = append(requestUntil, string(badge))
	}
	request := SessionWaitRequest{
		Until: requestUntil, TimeoutMS: timeout.Milliseconds(), Epoch: info.TranscriptEpoch,
	}
	var waitedMS int64
	for {
		outcome, waitErr := client.WaitSession(
			cmd.Context(),
			info.WorkspaceID,
			sessionID,
			request,
		)
		if waitErr != nil {
			if unbounded && sessionWaitAPIErrorCode(waitErr) == "wait_expired" {
				request.ResumeID = ""
				continue
			}
			return withCommandExitCode(sessionWaitExitUnavailable, waitErr)
		}
		waitedMS += outcome.WaitedMS
		outcome.WaitedMS = waitedMS
		switch outcome.Outcome {
		case string(session.WaitResultStateReached):
			return writeCommandOutput(cmd, sessionWaitBundle(outcome))
		case string(session.WaitResultTimeout):
			if unbounded {
				if strings.TrimSpace(outcome.ResumeID) == "" {
					return withCommandExitCode(
						sessionWaitExitUnavailable,
						errors.New("cli: session wait timeout did not return resume_id"),
					)
				}
				request.ResumeID = outcome.ResumeID
				continue
			}
			if err := writeCommandOutput(cmd, sessionWaitBundle(outcome)); err != nil {
				return err
			}
			return withCommandExitCode(sessionWaitExitTimeout, errors.New("session wait timed out"))
		default:
			if err := writeCommandOutput(cmd, sessionWaitBundle(outcome)); err != nil {
				return err
			}
			return withCommandExitCode(
				sessionWaitExitUnavailable,
				fmt.Errorf("session wait ended with outcome %q", outcome.Outcome),
			)
		}
	}
}

func normalizeSessionWaitUntil(until []string) ([]session.Badge, error) {
	badges := make([]session.Badge, 0, len(until))
	for _, value := range until {
		badges = append(badges, session.Badge(value))
	}
	return session.NormalizeWaitBadges(badges)
}

func sessionWaitAPIErrorCode(err error) string {
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	if !ok || apiErr == nil {
		return ""
	}
	return strings.TrimSpace(apiErr.payload.Code)
}
