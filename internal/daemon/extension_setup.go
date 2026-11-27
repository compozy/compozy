package daemon

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// extensionInputsTarget selects one installed instance like install does. An empty Profile uses the
// caller's read-scope profile, or the default profile.
type extensionInputsTarget struct {
	Name        string
	Scope       string // global|workspace
	WorkspaceID string
	Profile     string
}

// extensionInputsInstance is the resolved installation that setup reads and writes.
type extensionInputsInstance struct {
	key     extensionpkg.InstanceKey // the installation row's scope
	profile extensionpkg.ProfileLens
}

func (i extensionInputsInstance) lockKeys() []extensionpkg.InstanceKey {
	return []extensionpkg.InstanceKey{extensionpkg.GlobalInstanceKey(i.key.Name), i.key}
}

func (i extensionInputsInstance) inputInstance() extensioninput.Instance {
	return extensioninput.Instance{Extension: i.key.Name, ProfileID: i.profile.ID, WorkspaceID: i.key.WorkspaceID}
}

// ExtensionInputs implements core.ExtensionInputsService; the workspace comes from the actor.
func (s *daemonExtensionService) ExtensionInputs(
	ctx context.Context, name, scope string, actor taskpkg.ActorContext,
) (contract.ExtensionInputsPayload, error) {
	return s.InputsState(
		ctx,
		extensionInputsTarget{Name: name, Scope: scope, WorkspaceID: actor.Scope.WorkspaceID},
		actor,
	)
}

// SetExtensionInputs implements core.ExtensionInputsService; the workspace comes from the actor.
func (s *daemonExtensionService) SetExtensionInputs(
	ctx context.Context, name, scope string, req contract.SetExtensionInputsRequest, actor taskpkg.ActorContext,
) (contract.ExtensionInputsPayload, error) {
	return s.SetInputs(
		ctx, extensionInputsTarget{Name: name, Scope: scope, WorkspaceID: actor.Scope.WorkspaceID}, req.Inputs, actor,
	)
}

// InputsState returns declared inputs and their state for one instance, never values or Vault references.
func (s *daemonExtensionService) InputsState(
	ctx context.Context, target extensionInputsTarget, actor taskpkg.ActorContext,
) (contract.ExtensionInputsPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	if err := actor.Validate(); err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	if !actor.Authority.Read {
		return contract.ExtensionInputsPayload{}, taskpkg.ErrPermissionDenied
	}
	instance, err := s.resolveInputsInstance(ctx, target, actor, false)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	ext, err := s.inputsInstanceExtension(ctx, instance)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	return s.inputsPayload(ctx, instance, ext)
}

// SetInputs merges values into one instance under its lifecycle lock and reloads it when it is enabled
// and a value changed. Agents may write only missing, invalid or daemon_env inputs (D7).
func (s *daemonExtensionService) SetInputs(
	ctx context.Context, target extensionInputsTarget, values map[string]extensioninput.Value,
	actor taskpkg.ActorContext,
) (contract.ExtensionInputsPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	instance, err := s.resolveInputsInstance(ctx, target, actor, true)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	var payload contract.ExtensionInputsPayload
	err = s.lifecycle.withInstances(ctx, instance.lockKeys(), func() error {
		var setErr error
		payload, setErr = s.setInputsLocked(ctx, instance, values, actor)
		return setErr
	})
	return payload, err
}

func (s *daemonExtensionService) setInputsLocked(
	ctx context.Context, instance extensionInputsInstance, values map[string]extensioninput.Value,
	actor taskpkg.ActorContext,
) (contract.ExtensionInputsPayload, error) {
	ext, err := s.inputsInstanceExtension(ctx, instance)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	prior, err := s.inputsPayload(ctx, instance, ext)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	if !actor.Scope.Operator {
		if denied := overwrittenInputs(prior, values); len(denied) > 0 {
			return contract.ExtensionInputsPayload{}, &extensionpkg.InputOverwriteDeniedError{InputIDs: denied}
		}
	}
	binder := extensionInputBinder{service: s, partial: true}
	plan, err := binder.Prepare(ctx, instance.key, instance.profile, ext.Manifest, values)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	changed := plannedInputChanges(plan)
	mutated := len(plan.rows) > 0 || len(plan.writes) > 0
	if mutated {
		if _, err := binder.Commit(ctx, plan); err != nil {
			return contract.ExtensionInputsPayload{}, err
		}
	}
	payload, err := s.inputsPayload(ctx, instance, ext)
	if err != nil {
		// Keep the stored values in step with the runtime that loaded the prior ones.
		if mutated {
			err = errors.Join(err, binder.Rollback(ctx, plan))
		}
		return contract.ExtensionInputsPayload{}, err
	}
	// An instance still missing required inputs cannot start (rules 4-5), so a partial write keeps
	// its progress instead of failing the reload and rolling back. A save that only mirrors an
	// unchanged binding into its typed row changes no launched value, so it does not restart.
	valueChanged := len(changed) > 0 || len(plan.writes) > 0
	reloaded := valueChanged && ext.Status.Enabled && !inputsBlockStart(payload)
	if reloaded {
		restarted, err := s.reloadInputsInstance(ctx, instance)
		if err != nil {
			rollbackErr := binder.Rollback(ctx, plan)
			if rollbackErr == nil {
				rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
				defer cancel()
				_, rollbackErr = s.reloadInputsInstance(rollbackCtx, instance)
			}
			return contract.ExtensionInputsPayload{}, errors.Join(err, rollbackErr)
		}
		reloaded = restarted
	}
	payload.Changed, payload.Reloaded = changed, reloaded
	s.logger.Info("extension inputs updated",
		"extension", instance.key.Name, "profile", instance.profile.Name,
		"workspace_id", instance.key.WorkspaceID, "changed", changed, "reloaded", reloaded)
	return payload, nil
}

// resolveInputsInstance applies install's selector rules, then binds to the visible installation row.
// A write must address the installation's own cell; reads of a visible global installation are allowed.
func (s *daemonExtensionService) resolveInputsInstance(
	ctx context.Context, target extensionInputsTarget, actor taskpkg.ActorContext, write bool,
) (extensionInputsInstance, error) {
	name := strings.TrimSpace(target.Name)
	if err := extensionpkg.GlobalInstanceKey(name).Validate(); err != nil {
		return extensionInputsInstance{}, err
	}
	resolved, err := s.resolveExtensionScope(ctx, target.Scope, target.WorkspaceID, target.Profile, actor)
	if err != nil {
		return extensionInputsInstance{}, err
	}
	profile := resolved.profile
	if strings.TrimSpace(target.Profile) == "" && actor.Scope.Operator &&
		strings.TrimSpace(actor.ReadScope.ProfileID) != "" {
		if profile, err = s.extensionReadProfile(ctx, actor); err != nil {
			return extensionInputsInstance{}, err
		}
	}
	installation, err := s.registry.ResolveInstallation(ctx, name, extensionpkg.InstallationScope{
		ProfileID: profile.ID, WorkspaceID: resolved.scope.WorkspaceID,
	})
	if err != nil {
		return extensionInputsInstance{}, err
	}
	if write && installation.Scope.WorkspaceID != resolved.scope.WorkspaceID {
		// A global installation serves every workspace from one runtime and input cell, so a
		// workspace-scoped write would change the other workspaces (Safety Invariant 3).
		return extensionInputsInstance{}, extensionScopeError(
			"scope", "extension is installed globally; set up its inputs with scope global (--scope global)",
		)
	}
	return extensionInputsInstance{
		key: extensionpkg.InstanceKey{
			Name: name, ProfileID: installation.Scope.ProfileID, WorkspaceID: installation.Scope.WorkspaceID,
		},
		profile: profile,
	}, nil
}

// inputsInstanceExtension loads the installed package and its enablement for the instance's profile.
func (s *daemonExtensionService) inputsInstanceExtension(
	ctx context.Context, instance extensionInputsInstance,
) (*extensionpkg.Extension, error) {
	var ext *extensionpkg.Extension
	if runtime, ok := s.runtime.(extensionDevRuntime); ok {
		projected, err := s.projectExtensionReadProfile(ctx, runtime, instance.key, instance.profile)
		if err != nil {
			return nil, err
		}
		ext = projected
	} else {
		info, err := s.registry.Get(instance.key.Name)
		if err != nil {
			return nil, err
		}
		enabled, err := s.registry.IsEnabledForProfile(instance.key.Name, instance.profile.ID)
		if err != nil {
			return nil, err
		}
		ext = &extensionpkg.Extension{Info: *info, Status: extensionpkg.ExtensionStatus{
			Name: info.Name, Version: info.Version, Source: info.Source, Enabled: enabled,
			WorkspaceID: instance.key.WorkspaceID,
		}}
	}
	populateExtensionManifest(s.logger, ext)
	if ext.Manifest == nil {
		return nil, errors.New("daemon: installed extension manifest is required for input setup")
	}
	return ext, nil
}

func (s *daemonExtensionService) inputsPayload(
	ctx context.Context, instance extensionInputsInstance, ext *extensionpkg.Extension,
) (contract.ExtensionInputsPayload, error) {
	state, err := s.inputReader().load(ctx, instance.inputInstance(), ext.Manifest)
	if err != nil {
		return contract.ExtensionInputsPayload{}, err
	}
	payload := extensionpkg.DescribeInputSetup(ext.Manifest, state, s.getenv)
	payload.Extension = instance.key.Name
	payload.WorkspaceID = instance.key.WorkspaceID
	payload.Profile = instance.profile.Name
	payload.Enabled = ext.Status.Enabled
	return payload, nil
}

const extensionInputTargetSubprocess = "subprocess"

type extensionInstanceRestarter interface {
	RestartInstance(context.Context, extensionpkg.InstanceKey) error
}

// reloadInputsInstance restarts only the runtime serving the resolved instance so new values take
// effect; other profiles, workspaces and extensions keep running (Safety Invariant 3). It reports false
// when the runtime keeps its launch values, as a development base does until `extension dev reload`.
func (s *daemonExtensionService) reloadInputsInstance(
	ctx context.Context, instance extensionInputsInstance,
) (bool, error) {
	restarter, ok := s.runtime.(extensionInstanceRestarter)
	if !ok {
		return true, s.reloadExtensions(ctx, instance.key.Name)
	}
	key := extensionpkg.InstanceKey{
		Name: instance.key.Name, ProfileID: instance.profile.ID, WorkspaceID: instance.key.WorkspaceID,
	}
	err := restarter.RestartInstance(ctx, key)
	if errors.Is(err, extensionpkg.ErrDevelopmentRestartDeferred) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, s.syncExtensionConsumers(ctx)
}

// inputsBlockStart mirrors the launch gate, which decides a dual-consumer input with the same readiness
// precedence (an active typed row wins, the daemon env is read only without one): a required subprocess
// input that is invalid or missing refuses start; set and daemon_env stay ready (rule 5a).
func inputsBlockStart(payload contract.ExtensionInputsPayload) bool {
	return slices.ContainsFunc(payload.Inputs, func(input contract.ExtensionInputStatusPayload) bool {
		if !input.Required || !slices.Contains(input.Targets, extensionInputTargetSubprocess) {
			return false
		}
		return input.State == contract.ExtensionInputStateInvalid || input.State == contract.ExtensionInputStateMissing
	})
}

// overwrittenInputs lists supplied inputs whose state is already set; daemon_env holds no stored value.
func overwrittenInputs(prior contract.ExtensionInputsPayload, values map[string]extensioninput.Value) []string {
	var denied []string
	for _, input := range prior.Inputs {
		if _, supplied := values[input.ID]; supplied && input.State == contract.ExtensionInputStateSet {
			denied = append(denied, input.ID)
		}
	}
	slices.Sort(denied)
	return denied
}

// plannedInputChanges lists the declared inputs whose row or binding the plan changes. A subprocess
// input's binding is canonical (D2): a row write that only mirrors it changes no value.
func plannedInputChanges(plan *extensionInputPlan) []string {
	declared := make(map[string]bool, len(plan.manifest.Inputs))
	mirrored := make(map[string]bool, len(plan.manifest.Inputs))
	for _, input := range plan.manifest.Inputs {
		declared[input.ID] = true
		mirrored[input.ID] = plan.manifest.SubprocessInput(input)
	}
	changed := map[string]struct{}{}
	for _, row := range plan.rows {
		if declared[row.InputID] && !mirrored[row.InputID] {
			changed[row.InputID] = struct{}{}
		}
	}
	for _, write := range plan.writes {
		if declared[write.inputID] {
			changed[write.inputID] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(changed))
}
