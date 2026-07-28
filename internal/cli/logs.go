package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	logsTypeKey      = "type"
	logsAgentValue   = "Agent"
	logsAgentNameKey = "agent_name"
	logsLogsKey      = "logs"
)

type logsCommandOptions struct {
	session       string
	agent         string
	typ           string
	since         string
	last          int
	follow        bool
	workspaceRef  string
	runID         string
	actor         string
	provider      string
	outcome       string
	errorOnly     bool
	afterSequence int64
	component     string
}

func newLogsCommand(deps commandDeps) *cobra.Command {
	var opts logsCommandOptions

	cmd := &cobra.Command{
		Use:   logsLogsKey,
		Short: "Read cross-session runtime logs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveCLIWorkspaceRouteRef(cmd.Context(), deps, client, opts.workspaceRef)
			if err != nil {
				return err
			}

			sinceTime, err := parseSinceFlag(opts.since, deps.now)
			if err != nil {
				return err
			}
			actorKind, actorID, err := parseLogsActor(opts.actor)
			if err != nil {
				return err
			}
			query := LogsListQuery{
				WorkspaceRef:  workspace,
				SessionID:     opts.session,
				AgentName:     opts.agent,
				Type:          opts.typ,
				RunID:         opts.runID,
				ActorKind:     actorKind,
				ActorID:       actorID,
				Provider:      opts.provider,
				Outcome:       opts.outcome,
				Component:     opts.component,
				ErrorOnly:     opts.errorOnly,
				AfterSequence: opts.afterSequence,
				Since:         sinceTime,
				Last:          opts.last,
			}

			if opts.follow {
				return streamLogs(cmd, client, query)
			}

			events, err := client.ListLogs(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeLogsOutput(cmd, events)
		},
	}
	registerLogsFlags(cmd, &opts)
	return cmd
}

func registerLogsFlags(cmd *cobra.Command, opts *logsCommandOptions) {
	cmd.Flags().StringVar(&opts.session, "session", "", "Filter by session id")
	cmd.Flags().StringVar(&opts.agent, "agent", "", "Filter by agent name")
	cmd.Flags().StringVar(&opts.typ, logsTypeKey, "", "Filter by event type")
	cmd.Flags().StringVar(&opts.workspaceRef, "workspace", "", "Workspace root, name, or ID for scoped logs")
	cmd.Flags().StringVar(&opts.since, "since", "", "Show logs since an RFC3339 timestamp or relative duration")
	cmd.Flags().IntVar(&opts.last, "last", 0, "Show only the most recent N logs")
	cmd.Flags().BoolVar(&opts.follow, "follow", false, "Stream new logs over SSE")
	cmd.Flags().StringVar(&opts.runID, "run", "", "Filter by task run id")
	cmd.Flags().StringVar(&opts.actor, "actor", "", "Filter by actor as kind:id")
	cmd.Flags().StringVar(&opts.provider, cliProviderKey, "", "Filter by provider id")
	cmd.Flags().StringVar(&opts.outcome, cliOutcomeKey, "", "Filter by registry outcome")
	cmd.Flags().BoolVar(&opts.errorOnly, "error-only", false, "Show failure and warning outcomes only")
	cmd.Flags().Int64Var(&opts.afterSequence, "after-seq", 0, "Replay logs after the supplied event summary sequence")
	cmd.Flags().StringVar(&opts.component, "component", "", "Filter by registry component")
}

func parseLogsActor(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", nil
	}
	kind, id, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(kind) == "" || strings.TrimSpace(id) == "" {
		return "", "", fmt.Errorf("cli: actor must use kind:id format")
	}
	return strings.TrimSpace(kind), strings.TrimSpace(id), nil
}

func writeLogsOutput(cmd *cobra.Command, events []LogEventRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		return writeJSONLines(cmd, events)
	}
	return writeCommandOutput(cmd, logsBundle(events))
}

func streamLogs(cmd *cobra.Command, client DaemonClient, query LogsListQuery) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)

	return client.StreamLogs(cmd.Context(), query, "", func(event SSEEvent) error {
		var payload LogEventRecord
		if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return fmt.Errorf("cli: decode logs stream event: %w", err)
			}
		}
		if payload.Type == "" {
			payload.Type = event.Event
		}

		switch mode {
		case OutputJSON, OutputJSONL:
			if err := encoder.Encode(payload); err != nil {
				return err
			}
		case OutputToon:
			if err := writeRawCommandOutput(cmd, renderToonObject("log_event", []string{
				"id",
				automationSessionIDKey,
				logsTypeKey,
				logsAgentNameKey,
				cliProviderKey,
				"component",
				cliOutcomeKey,
				memorySummaryKey,
				networkTimestampKey,
			}, []string{
				payload.ID,
				payload.SessionID,
				payload.Type,
				payload.AgentName,
				payload.Provider,
				payload.Component,
				payload.Outcome,
				payload.Summary,
				formatTime(payload.Timestamp),
			})); err != nil {
				return err
			}
		default:
			if err := writeRawCommandOutput(cmd, strings.Join([]string{
				stringOrDash(formatTime(payload.Timestamp)),
				stringOrDash(payload.Type),
				stringOrDash(payload.SessionID),
				stringOrDash(payload.AgentName),
				stringOrDash(payload.Provider),
				stringOrDash(payload.Component),
				stringOrDash(payload.Outcome),
				stringOrDash(payload.Summary),
			}, "  ")); err != nil {
				return err
			}
		}
		return nil
	})
}

func logsBundle(events []LogEventRecord) outputBundle {
	return listBundle(
		events,
		events,
		"Logs",
		[]string{
			"ID",
			agentKernelSessionValue,
			sessionTypeValue,
			logsAgentValue,
			installProviderValue,
			"Component",
			taskOutcomeValue,
			authoredContextSummaryValue,
			"Timestamp",
		},
		"logs",
		[]string{
			"id",
			automationSessionIDKey,
			logsTypeKey,
			logsAgentNameKey,
			cliProviderKey,
			"component",
			cliOutcomeKey,
			memorySummaryKey,
			networkTimestampKey,
		},
		func(event LogEventRecord) []string {
			return []string{
				stringOrDash(event.ID),
				stringOrDash(event.SessionID),
				stringOrDash(event.Type),
				stringOrDash(event.AgentName),
				stringOrDash(event.Provider),
				stringOrDash(event.Component),
				stringOrDash(event.Outcome),
				stringOrDash(event.Summary),
				stringOrDash(formatTime(event.Timestamp)),
			}
		},
		func(event LogEventRecord) []string {
			return []string{
				event.ID,
				event.SessionID,
				event.Type,
				event.AgentName,
				event.Provider,
				event.Component,
				event.Outcome,
				event.Summary,
				formatTime(event.Timestamp),
			}
		},
	)
}
