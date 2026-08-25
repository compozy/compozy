package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type skillExposureClient interface {
	GetSkill(context.Context, string, SkillQuery) (SkillRecord, error)
	ExposeSkill(
		context.Context,
		string,
		contract.SkillExposureRequest,
		SkillQuery,
	) (contract.SkillExposeResponse, error)
	UnexposeSkill(
		context.Context,
		string,
		contract.SkillExposureRequest,
		SkillQuery,
	) (contract.SkillUnexposeResponse, error)
}

type skillExposureSuccess struct {
	Name        string
	Action      string
	Results     []contract.SkillExposureTargetResultPayload
	Preexisting map[string]bool
	JSONValue   any
}

type skillCreatedExposureError struct{ cause error }

func (e *skillCreatedExposureError) Error() string {
	if e == nil || e.cause == nil {
		return "skill exposure failed after creation"
	}
	return e.cause.Error()
}

func (e *skillCreatedExposureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func newSkillExposeCommand(deps commandDeps) *cobra.Command {
	return newSkillExposureCommand(deps, true)
}

func newSkillUnexposeCommand(deps commandDeps) *cobra.Command {
	return newSkillExposureCommand(deps, false)
}

func newSkillExposureCommand(deps commandDeps, expose bool) *cobra.Command {
	verb := skillUnexposeAction
	short := "Remove owned provider-root links for one skill"
	if expose {
		verb = skillExposeAction
		short = "Expose one skill to provider roots"
	}
	var workspaceRef string
	var agentName string
	var targetsValue string
	cmd := &cobra.Command{
		Use:   verb + " <name>",
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := parseSkillExposureTargets(targetsValue)
			if err != nil {
				return err
			}
			scope, err := resolveSkillCommandScope(cmd.Context(), cmd, deps)
			if err != nil {
				return err
			}
			if !scope.useDaemon {
				return errors.New("cli: skill exposure requires the running daemon")
			}
			client, err := skillExposureClientFromDeps(deps)
			if err != nil {
				return err
			}
			return runSkillExposureCommand(cmd, client, strings.TrimSpace(args[0]), targets, scope, expose)
		},
	}
	cmd.Flags().StringVar(&targetsValue, "to", "", "Comma-separated provider targets")
	cmd.Flags().StringVar(&workspaceRef, workspaceSkillSource, "", "Override workspace context (ID, name, or path)")
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	if err := cmd.MarkFlagRequired("to"); err != nil {
		panic(fmt.Sprintf("cli: mark skill %s --to required: %v", verb, err))
	}
	return cmd
}

func skillExposureClientFromDeps(deps commandDeps) (skillExposureClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	exposureClient, ok := client.(skillExposureClient)
	if !ok {
		return nil, errors.New("cli: skill exposure client is unavailable")
	}
	return exposureClient, nil
}

func runSkillExposureCommand(
	cmd *cobra.Command,
	client skillExposureClient,
	name string,
	targets []string,
	scope skillCommandScope,
	expose bool,
) error {
	request := contract.SkillExposureRequest{Targets: targets, WorkspaceID: scope.query.Workspace}
	preexisting := map[string]bool{}
	if expose {
		record, err := client.GetSkill(cmd.Context(), name, scope.query)
		if err != nil {
			return err
		}
		if record.Exposures != nil {
			for _, exposure := range *record.Exposures {
				if exposure.Status == skillExposureHealthyStatus {
					preexisting[exposure.Target] = true
				}
			}
		}
		response, err := client.ExposeSkill(cmd.Context(), name, request, scope.query)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, skillExposureSuccessBundle(skillExposureSuccess{
			Name: response.Name, Action: skillExposeAction, Results: response.Results,
			Preexisting: preexisting, JSONValue: response,
		}))
	}
	response, err := client.UnexposeSkill(cmd.Context(), name, request, scope.query)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, skillExposureSuccessBundle(skillExposureSuccess{
		Name: response.Name, Action: skillUnexposeAction, Results: response.Results, JSONValue: response,
	}))
}

func parseSkillExposureTargets(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	targets := make([]string, 0, len(parts))
	for _, part := range parts {
		target := strings.TrimSpace(part)
		if target == "" {
			return nil, errors.New("skill exposure target is required")
		}
		targets = append(targets, target)
	}
	return targets, nil
}
