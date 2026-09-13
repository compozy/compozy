package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
)

func (s *daemonExtensionService) checkProfileEnablementInputs(
	ctx context.Context,
	name, profileID string,
	enabled bool,
) error {
	if !enabled {
		return nil
	}
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
	state, err := s.inputReader().load(ctx, extensioninput.Instance{
		Extension: name, ProfileID: profileID,
	}, ext.Manifest)
	if err != nil {
		return err
	}
	return extensionpkg.InputReadiness(ext.Manifest, state, s.getenv).RequiredError(ext.Manifest)
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
		payload.MissingInputs = readiness.MissingInputs
		payload.MissingEnv = readiness.MissingEnv
	}
	return nil
}
