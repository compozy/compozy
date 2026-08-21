package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/spf13/cobra"
)

const (
	loopLoopKey       = "loop"
	loopWorkspaceKey  = "workspace"
	loopNameKey       = "name"
	loopFileKey       = "file"
	loopRunIDKey      = "run-id"
	loopGateIDKey     = "gate-id"
	loopDecisionKey   = "decision"
	loopDryRunKey     = "dry-run"
	loopInputKey      = "input"
	loopConfigFileKey = "config-file"
	loopRuntimeKey    = "runtime"
	loopPayloadKey    = "payload"
	loopListKey       = "list"
	loopInspectKey    = "inspect"
	loopCreateKey     = "create"
	loopConfigureKey  = "configure"
	loopRunKey        = "run"
	loopStatusKey     = "status"
	loopStateKey      = "state"
	loopGenerationKey = "generation"
	loopTurnsKey      = "turns"
	loopCancelKey     = "cancel"
	loopKillKey       = "kill"
	loopNodeKey       = "node"
	loopNodesKey      = "nodes"
	loopRequeueKey    = "requeue"
	loopPauseKey      = "pause"
	loopResumeKey     = "resume"
	loopApproveKey    = "approve"
	loopRequestsKey   = "requests"
	loopRequestKey    = "request"
	loopRespondKey    = "respond"
	loopAmendKey      = "amend"
	loopDiffKey       = "diff"
	loopRerunKey      = "rerun"
	loopForkKey       = "fork"
	loopEditKey       = "edit"
	loopDeleteKey     = "delete"
	loopWhyKey        = "why"
	loopEventsKey     = "events"
)

func newLoopCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   loopLoopKey,
		Short: "Manage Loop definitions and runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newLoopListCommand(deps))
	cmd.AddCommand(newLoopInspectCommand(deps))
	cmd.AddCommand(newLoopCreateCommand(deps))
	cmd.AddCommand(newLoopValidateCommand(deps))
	cmd.AddCommand(newLoopRunCommand(deps))
	cmd.AddCommand(newLoopStatusCommand(deps))
	cmd.AddCommand(newLoopRunsCommand(deps))
	cmd.AddCommand(newLoopTurnsCommand(deps))
	cmd.AddCommand(newLoopRunActionCommand(deps, loopCancelKey, "Cancel one Loop run"))
	cmd.AddCommand(newLoopRunActionCommand(deps, loopKillKey, "Kill one Loop run"))
	cmd.AddCommand(newLoopRunActionCommand(deps, loopPauseKey, "Pause one Loop run"))
	cmd.AddCommand(newLoopRunActionCommand(deps, loopResumeKey, "Resume one Loop run"))
	cmd.AddCommand(newLoopNodeCommand(deps))
	cmd.AddCommand(newLoopNodesCommand(deps))
	cmd.AddCommand(newLoopWhyCommand(deps))
	cmd.AddCommand(newLoopEventsCommand(deps))
	cmd.AddCommand(newLoopConfigureCommand(deps))
	cmd.AddCommand(newLoopApproveCommand(deps))
	cmd.AddCommand(newLoopRequestsCommand(deps))
	cmd.AddCommand(newLoopRequestCommand(deps))
	cmd.AddCommand(newLoopRespondCommand(deps))
	cmd.AddCommand(newLoopDiffCommand(deps))
	cmd.AddCommand(newLoopRerunCommand(deps))
	cmd.AddCommand(newLoopForkCommand(deps))
	cmd.AddCommand(newLoopEditCommand(deps))
	cmd.AddCommand(newLoopDeleteCommand(deps))
	return cmd
}

func newLoopInspectCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, name string
	cmd := &cobra.Command{
		Use:   loopInspectKey,
		Short: "Inspect one Loop definition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			loopName, err := requiredLoopFlag(loopNameKey, name)
			if err != nil {
				return err
			}
			response, err := client.GetLoop(cmd.Context(), workspaceID, loopName)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopInspectOutputBundle(&response))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	mustMarkFlagRequired(cmd, loopNameKey)
	return cmd
}

func newLoopCreateCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, filePath, forkFrom string
	var expectedVersion int
	cmd := &cobra.Command{
		Use:   loopCreateKey,
		Short: "Create, fork, or publish one Loop definition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			if strings.TrimSpace(filePath) == "" && strings.TrimSpace(forkFrom) == "" {
				return errors.New("cli: --file or --fork-from-name is required")
			}
			if strings.TrimSpace(filePath) != "" && strings.TrimSpace(forkFrom) != "" {
				return errors.New("cli: --file and --fork-from-name cannot be combined")
			}
			if cmd.Flags().Changed("expected-version") {
				if strings.TrimSpace(filePath) == "" {
					return errors.New("cli: --expected-version requires --file")
				}
				definition, err := readLoopDefinitionFile(cmd.Context(), filePath)
				if err != nil {
					return err
				}
				document, err := loopDefinitionDocumentFromDefinition(definition)
				if err != nil {
					return err
				}
				credentials := agentCredentialsFromEnv(deps)
				response, err := client.PatchLoop(
					cmd.Context(),
					workspaceID,
					definition.Meta.Name,
					contract.PatchLoopRequest{
						ExpectedVersion: &expectedVersion,
						Definition:      document,
					},
					credentials,
				)
				if err != nil {
					return err
				}
				return writeLoopDefinition(cmd, &response)
			}
			request := contract.CreateLoopRequest{ForkFromName: strings.TrimSpace(forkFrom)}
			if strings.TrimSpace(filePath) != "" {
				definition, err := readLoopDefinitionFile(cmd.Context(), filePath)
				if err != nil {
					return err
				}
				document, err := loopDefinitionDocumentFromDefinition(definition)
				if err != nil {
					return err
				}
				request.Definition = &document
			}
			response, err := client.CreateLoop(cmd.Context(), workspaceID, request, agentCredentialsFromEnv(deps))
			if err != nil {
				return err
			}
			return writeLoopDefinition(cmd, &response)
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&filePath, loopFileKey, "", "Loop definition YAML or JSON file")
	cmd.Flags().StringVar(&forkFrom, "fork-from-name", "", "Read-only Loop name to fork")
	cmd.Flags().IntVar(&expectedVersion, "expected-version", 0, "Expected published version for CAS publish")
	return cmd
}

func newLoopValidateCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, filePath, name string
	cmd := &cobra.Command{
		Use:   cliValidateVerb,
		Short: "Validate one Loop definition without saving",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			definition, err := readLoopDefinitionFile(cmd.Context(), filePath)
			if err != nil {
				return err
			}
			loopName := strings.TrimSpace(name)
			if loopName == "" {
				loopName = definition.Meta.Name
			}
			document, err := loopDefinitionDocumentFromDefinition(definition)
			if err != nil {
				return err
			}
			response, err := client.ValidateLoop(cmd.Context(), workspaceID, loopName, contract.ValidateLoopRequest{
				Definition: document,
			})
			if err != nil {
				return err
			}
			summary := "Loop validation failed"
			if response.Valid {
				summary = "Loop validation passed"
			}
			return writeCommandOutput(cmd, loopOutputBundle(response, summary))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&filePath, loopFileKey, "", "Loop definition YAML or JSON file")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Override the route Loop name")
	mustMarkFlagRequired(cmd, loopFileKey)
	return cmd
}

type loopRunOptions struct {
	workspaceRef string
	name         string
	parentRunID  string
	configPath   string
	inputs       []string
	runtimeFlags []string
	dry          bool
	noPrompt     bool
	networkFlags networkParticipationFlags
}

func newLoopRunCommand(deps commandDeps) *cobra.Command {
	options := loopRunOptions{}
	cmd := &cobra.Command{
		Use:   loopRunKey,
		Short: "Start or dry-run one Loop",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeLoopRun(cmd, deps, options)
		},
	}
	cmd.Flags().StringVar(&options.workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&options.name, loopNameKey, "", "Loop name")
	cmd.Flags().StringArrayVar(
		&options.inputs,
		loopInputKey,
		nil,
		"Input key=value (repeatable; JSON values supported)",
	)
	cmd.Flags().StringArrayVar(
		&options.runtimeFlags,
		loopRuntimeKey,
		nil,
		"Runtime worker|judge|id|type|complexity selector (repeatable)",
	)
	cmd.Flags().StringVar(&options.parentRunID, "parent-loop-run-id", "", "Parent Loop run ID")
	cmd.Flags().StringVar(&options.configPath, loopConfigFileKey, "", "Per-run config override YAML or JSON file")
	cmd.Flags().BoolVar(&options.dry, loopDryRunKey, false, "Preview the plan without creating a run")
	cmd.Flags().BoolVar(
		&options.noPrompt,
		"no-prompt",
		false,
		"Disable interactive prompts for missing required inputs",
	)
	bindNetworkParticipationFlags(cmd, &options.networkFlags)
	mustMarkFlagRequired(cmd, loopNameKey)
	return cmd
}

func executeLoopRun(cmd *cobra.Command, deps commandDeps, options loopRunOptions) error {
	client, workspaceID, err := loopClientAndWorkspace(cmd, deps, options.workspaceRef)
	if err != nil {
		return err
	}
	loopName, err := requiredLoopFlag(loopNameKey, options.name)
	if err != nil {
		return err
	}
	values, err := prepareLoopRunInputs(cmd, deps, client, workspaceID, loopName, options)
	if err != nil {
		return err
	}
	overrides, err := prepareLoopRunOverrides(cmd, options)
	if err != nil {
		return err
	}
	participationRequest, err := options.networkFlags.request()
	if err != nil {
		return err
	}
	response, err := client.RunLoop(cmd.Context(), workspaceID, loopName, contract.RunLoopRequest{
		Inputs:               values,
		ParentLoopRunID:      strings.TrimSpace(options.parentRunID),
		ConfigOverrides:      overrides,
		NetworkParticipation: participationRequest,
	}, options.dry, agentCredentialsFromEnv(deps))
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, loopRunOutputBundle(response, fmt.Sprintf("Loop %s run requested", loopName)))
}

func prepareLoopRunInputs(
	cmd *cobra.Command,
	deps commandDeps,
	client loopCommandClient,
	workspaceID string,
	loopName string,
	options loopRunOptions,
) (map[string]any, error) {
	values, err := parseLoopInputFlags(options.inputs)
	if err != nil {
		return nil, err
	}
	loopResponse, err := client.GetLoop(cmd.Context(), workspaceID, loopName)
	if err != nil {
		return nil, err
	}
	var definition dsl.Definition
	if err := loopResponse.Loop.Definition.Decode(&definition); err != nil {
		return nil, fmt.Errorf("cli: decode Loop input declarations: %w", err)
	}
	values, err = normalizeLoopRunInputs(definition, values)
	if err != nil {
		return nil, err
	}
	return promptForMissingLoopInputs(cmd, deps, client, workspaceID, definition, values, options.noPrompt)
}

func prepareLoopRunOverrides(cmd *cobra.Command, options loopRunOptions) (*contract.LoopConfig, error) {
	var overrides *looppkg.LoopConfig
	if strings.TrimSpace(options.configPath) != "" {
		cfg, err := readLoopConfigFile(options.configPath)
		if err != nil {
			return nil, err
		}
		overrides = &cfg
	}
	runtimeOverrides, err := parseLoopRuntimeFlags(options.runtimeFlags)
	if err != nil {
		return nil, err
	}
	if runtimeOverrides.RuntimeDefaults != nil || runtimeOverrides.RuntimeRules != nil {
		if overrides == nil {
			overrides = &looppkg.LoopConfig{}
		}
		mergeLoopRuntimeFlags(overrides, runtimeOverrides)
	}
	return loopConfigPayloadFromDomain(cmd.Context(), overrides)
}

func newLoopConfigureCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, name, filePath string
	var setFlags []string
	cmd := &cobra.Command{
		Use:   loopConfigureKey,
		Short: "Write per-loop runtime config overrides",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			loopName, err := requiredLoopFlag(loopNameKey, name)
			if err != nil {
				return err
			}
			cfg, err := loopConfigFromFlags(filePath, setFlags)
			if err != nil {
				return err
			}
			config, err := loopConfigPayloadFromDomain(cmd.Context(), &cfg)
			if err != nil {
				return err
			}
			response, err := client.PutLoopConfig(cmd.Context(), workspaceID, loopName, contract.PutLoopConfigRequest{
				Config: *config,
			}, agentCredentialsFromEnv(deps))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response, fmt.Sprintf("Loop %s configured", loopName)))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	cmd.Flags().StringVar(&filePath, loopFileKey, "", "Loop config YAML or JSON file")
	cmd.Flags().StringArrayVar(&setFlags, "set", nil, "Config field key=value (repeatable; JSON values supported)")
	mustMarkFlagRequired(cmd, loopNameKey)
	return cmd
}

func newLoopEditCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, name, editor string
	cmd := &cobra.Command{
		Use:   loopEditKey,
		Short: "Edit and publish one Loop definition with $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			loopName, err := requiredLoopFlag(loopNameKey, name)
			if err != nil {
				return err
			}
			response, err := editLoopDefinition(cmd, deps, client, workspaceID, loopName, editor)
			if err != nil {
				return err
			}
			return writeLoopDefinition(cmd, &response)
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	cmd.Flags().StringVar(&editor, "editor", "", "Editor command; defaults to $EDITOR")
	mustMarkFlagRequired(cmd, loopNameKey)
	return cmd
}

func newLoopDeleteCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, name string
	cmd := &cobra.Command{
		Use:   loopDeleteKey,
		Short: "Delete one user-authored Loop definition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			loopName, err := requiredLoopFlag(loopNameKey, name)
			if err != nil {
				return err
			}
			if err := client.DeleteLoop(
				cmd.Context(),
				workspaceID,
				loopName,
				agentCredentialsFromEnv(deps),
			); err != nil {
				return err
			}
			return writeLoopMutationOK(cmd, "deleted", loopName)
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	mustMarkFlagRequired(cmd, loopNameKey)
	return cmd
}

func loopClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (loopCommandClient, string, error) {
	client, err := loopClientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return nil, "", err
	}
	return client, resolution.ID, nil
}
