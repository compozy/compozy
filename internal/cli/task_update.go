package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

type taskUpdateInput struct {
	Title              string
	Description        string
	PriorityRaw        string
	MetadataRaw        string
	OwnerKindRaw       string
	OwnerRef           string
	ClearOwner         bool
	AutoEnqueueOnReady bool
	AutoEnqueueSet     bool
	NetworkFlags       networkParticipationFlags
}

func newTaskUpdateCommand(deps commandDeps) *cobra.Command {
	var (
		title        string
		description  string
		metadataRaw  string
		priorityRaw  string
		ownerKindRaw string
		ownerRef     string
		clearOwner   bool
		autoEnqueue  bool
		networkFlags networkParticipationFlags
	)

	cmd := &cobra.Command{
		Use:   taskUpdateIDValue,
		Short: "Update mutable task fields",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request, err := buildTaskUpdateRequest(cmd, taskUpdateInput{
				Title:              title,
				Description:        description,
				PriorityRaw:        priorityRaw,
				MetadataRaw:        metadataRaw,
				OwnerKindRaw:       ownerKindRaw,
				OwnerRef:           ownerRef,
				ClearOwner:         clearOwner,
				AutoEnqueueOnReady: autoEnqueue,
				AutoEnqueueSet:     cmd.Flags().Changed(taskAutoEnqueueOnReadyFlag),
				NetworkFlags:       networkFlags,
			})
			if err != nil {
				return err
			}
			if !request.HasChanges() {
				return errors.New("cli: task update requires at least one change flag")
			}

			updated, err := client.UpdateTask(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&updated))
		},
	}
	cmd.Flags().StringVar(&title, taskTitleKey, "", "Update the task title")
	cmd.Flags().StringVar(&description, taskDescriptionKey, "", "Update the task description")
	cmd.Flags().
		StringVar(&priorityRaw, "priority", "", "Update the task priority: low, medium, high, or urgent")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Update metadata JSON")
	cmd.Flags().StringVar(&ownerKindRaw, "owner-kind", "", "Update the owner kind")
	cmd.Flags().StringVar(&ownerRef, "owner-ref", "", "Update the owner reference")
	cmd.Flags().BoolVar(&clearOwner, "clear-owner", false, "Remove the current owner")
	cmd.Flags().
		BoolVar(&autoEnqueue, taskAutoEnqueueOnReadyFlag, false, "Toggle auto-enqueue once blocking dependencies complete")
	bindNetworkParticipationFlags(cmd, &networkFlags)
	return cmd
}

func buildTaskUpdateRequest(cmd *cobra.Command, input taskUpdateInput) (UpdateTaskRequest, error) {
	request := UpdateTaskRequest{}
	if cmd.Flags().Changed(taskTitleKey) {
		trimmed := strings.TrimSpace(input.Title)
		if trimmed == "" {
			return UpdateTaskRequest{}, errors.New("cli: --title cannot be blank")
		}
		request.Title = new(trimmed)
	}
	if cmd.Flags().Changed(taskDescriptionKey) {
		request.Description = new(strings.TrimSpace(input.Description))
	}
	if cmd.Flags().Changed("priority") {
		priority, err := parseOptionalTaskPriority(input.PriorityRaw)
		if err != nil {
			return UpdateTaskRequest{}, err
		}
		request.Priority = &priority
	}
	if cmd.Flags().Changed("metadata") {
		metadata, err := parseJSONFlag("metadata", input.MetadataRaw)
		if err != nil {
			return UpdateTaskRequest{}, err
		}
		request.Metadata = &metadata
	}
	if input.AutoEnqueueSet {
		autoEnqueue := input.AutoEnqueueOnReady
		request.AutoEnqueueOnReady = &autoEnqueue
	}
	ownerChanged := cmd.Flags().Changed("owner-kind") || cmd.Flags().Changed("owner-ref")
	if input.ClearOwner && ownerChanged {
		return UpdateTaskRequest{}, errors.New(
			"cli: --clear-owner cannot be combined with --owner-kind or --owner-ref",
		)
	}
	if ownerChanged {
		owner, err := parseRequiredTaskOwnership(input.OwnerKindRaw, input.OwnerRef)
		if err != nil {
			return UpdateTaskRequest{}, err
		}
		request.Owner = owner
	}
	if input.ClearOwner {
		request.ClearOwner = true
	}
	participationRequest, err := input.NetworkFlags.request()
	if err != nil {
		return UpdateTaskRequest{}, err
	}
	request.NetworkParticipation = participationRequest
	return request, nil
}
