package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
)

func (s *daemonExtensionService) checkProfileEnablementInputs(
	ctx context.Context,
	name, profileID string, operator bool,
) error {
	ext, err := s.runtime.InspectPackageResources(ctx, name)
	if err != nil {
		return err
	}
	if ext == nil || ext.Manifest == nil {
		return errors.New("daemon: inspected extension manifest is required for enablement")
	}
	if len(ext.Manifest.Inputs) == 0 {
		return nil
	}
	workspaces, err := s.extensionEnablementWorkspaces(ctx, name, profileID)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaces {
		if workspaceID != "" {
			_, err := s.registry.GetDevLink(name, workspaceID)
			if err == nil {
				continue
			}
			if !errors.Is(err, extensionpkg.ErrExtensionNotDevLinked) {
				return err
			}
		}
		state, err := s.inputReader().load(ctx, extensioninput.Instance{
			Extension: ext.Info.Name, ProfileID: profileID, WorkspaceID: workspaceID,
		}, ext.Manifest)
		if err != nil {
			return err
		}
		if err := extensionpkg.InputReadiness(ext.Manifest, state, s.getenv).RequiredError(ext.Manifest); err != nil {
			return extensionEnablementInputError(err, workspaceID, operator)
		}
	}
	return nil
}

func extensionEnablementInputError(err error, workspaceID string, operator bool) error {
	if operator && workspaceID != "" {
		return fmt.Errorf("workspace %q: %w", workspaceID, err)
	}
	return err
}

func (s *daemonExtensionService) extensionEnablementWorkspaces(
	ctx context.Context, name, profileID string,
) ([]string, error) {
	installations, err := s.registry.Installations(ctx, name)
	if err != nil {
		return nil, err
	}
	var workspaces []string
	for _, installation := range installations {
		scope := installation.Scope
		if scope.ProfileID == "" || scope.ProfileID == profileID {
			workspaces = append(workspaces, scope.WorkspaceID)
		}
	}
	// The registry orders attachments by workspace, so profile-specific and shared rows are adjacent.
	return slices.Compact(workspaces), nil
}

func (s *daemonExtensionService) populateExtensionInputStatus(
	ctx context.Context, ext *extensionpkg.Extension, profileID string, payload *contract.ExtensionPayload,
) error {
	state, err := s.inputReader().load(ctx, extensioninput.Instance{
		Extension: ext.Info.Name, ProfileID: profileID, WorkspaceID: ext.Status.WorkspaceID,
	}, ext.Manifest)
	if err != nil {
		return err
	}
	payload.Inputs = extensionpkg.DescribeInputState(ext.Manifest, state, s.getenv)
	if ext.Manifest != nil && len(ext.Manifest.Inputs) > 0 {
		readiness := extensionpkg.InputReadiness(ext.Manifest, state, s.getenv)
		payload.MissingInputs = statusMissingInputs(ext.Manifest, readiness)
		payload.MissingEnv = readiness.MissingEnv
	}
	return nil
}

// statusMissingInputs adds required subprocess inputs that readiness tracks by env name, so status
// reports them by id like RequiredError does; MCP-only env inputs keep their missing_env shape.
func statusMissingInputs(manifest *extensionpkg.Manifest, readiness extensionpkg.Readiness) []string {
	missing := slices.Clone(readiness.MissingInputs)
	for _, input := range manifest.SubprocessInputs() {
		if input.Required && slices.Contains(readiness.MissingEnv, input.Binding.Name) {
			missing = append(missing, input.ID)
		}
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}
