package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
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
	loopPayloadKey    = "payload"
	loopListKey       = "list"
	loopInspectKey    = "inspect"
	loopCreateKey     = "create"
	loopRunKey        = "run"
	loopStatusKey     = "status"
	loopRunsKey       = "runs"
	loopTurnsKey      = "turns"
	loopStopKey       = "stop"
	loopPauseKey      = "pause"
	loopResumeKey     = "resume"
	loopApproveKey    = "approve"
	loopEditKey       = "edit"
	loopDeleteKey     = "delete"
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
	cmd.AddCommand(newLoopRunActionCommand(deps, loopStopKey, "Stop one Loop run"))
	cmd.AddCommand(newLoopRunActionCommand(deps, loopPauseKey, "Pause one Loop run"))
	cmd.AddCommand(newLoopRunActionCommand(deps, loopResumeKey, "Resume one Loop run"))
	cmd.AddCommand(newLoopConfigureCommand(deps))
	cmd.AddCommand(newLoopApproveCommand(deps))
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
			summary := fmt.Sprintf("Loop %s v%d", response.Loop.Name, response.Loop.Version)
			return writeCommandOutput(cmd, loopOutputBundle(response, summary))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
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
				definition, err := readLoopDefinitionFile(filePath)
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
				definition, err := readLoopDefinitionFile(filePath)
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
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&filePath, loopFileKey, "", "Loop definition YAML or JSON file")
	cmd.Flags().StringVar(&forkFrom, "fork-from-name", "", "Read-only Loop name to fork")
	cmd.Flags().IntVar(&expectedVersion, "expected-version", 0, "Expected published version for CAS publish")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	return cmd
}

func newLoopValidateCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, filePath, name string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate one Loop definition without saving",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			definition, err := readLoopDefinitionFile(filePath)
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
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&filePath, loopFileKey, "", "Loop definition YAML or JSON file")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Override the route Loop name")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	mustMarkFlagRequired(cmd, loopFileKey)
	return cmd
}

func newLoopRunCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, name, parentRunID, configPath string
	var inputs []string
	var dry bool
	var networkFlags networkParticipationFlags
	cmd := &cobra.Command{
		Use:   loopRunKey,
		Short: "Start or dry-run one Loop",
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
			values, err := parseLoopInputFlags(inputs)
			if err != nil {
				return err
			}
			var overrides *looppkg.LoopConfig
			if strings.TrimSpace(configPath) != "" {
				cfg, err := readLoopConfigFile(configPath)
				if err != nil {
					return err
				}
				overrides = &cfg
			}
			configOverrides, err := loopConfigPayloadFromDomain(overrides)
			if err != nil {
				return err
			}
			participationRequest, err := networkFlags.request()
			if err != nil {
				return err
			}
			response, err := client.RunLoop(cmd.Context(), workspaceID, loopName, contract.RunLoopRequest{
				Inputs:               values,
				ParentLoopRunID:      strings.TrimSpace(parentRunID),
				ConfigOverrides:      configOverrides,
				NetworkParticipation: participationRequest,
			}, dry, agentCredentialsFromEnv(deps))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response, fmt.Sprintf("Loop %s run requested", loopName)))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	cmd.Flags().StringArrayVar(&inputs, loopInputKey, nil, "Input key=value (repeatable; JSON values supported)")
	cmd.Flags().StringVar(&parentRunID, "parent-loop-run-id", "", "Parent Loop run ID")
	cmd.Flags().StringVar(&configPath, loopConfigFileKey, "", "Per-run config override YAML or JSON file")
	cmd.Flags().BoolVar(&dry, loopDryRunKey, false, "Preview the plan without creating a run")
	bindNetworkParticipationFlags(cmd, &networkFlags)
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	mustMarkFlagRequired(cmd, loopNameKey)
	return cmd
}

func newLoopConfigureCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, name, filePath string
	var setFlags []string
	cmd := &cobra.Command{
		Use:   "configure",
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
			config, err := loopConfigPayloadFromDomain(&cfg)
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
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	cmd.Flags().StringVar(&filePath, loopFileKey, "", "Loop config YAML or JSON file")
	cmd.Flags().StringArrayVar(&setFlags, "set", nil, "Config field key=value (repeatable; JSON values supported)")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
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
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	cmd.Flags().StringVar(&editor, "editor", "", "Editor command; defaults to $EDITOR")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
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
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&name, loopNameKey, "", "Loop name")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
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
	workspaceID, err := resolveLoopWorkspaceID(cmd.Context(), client, workspaceRef)
	if err != nil {
		return nil, "", err
	}
	return client, workspaceID, nil
}
