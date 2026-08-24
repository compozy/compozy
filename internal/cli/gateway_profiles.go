package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	gatewayProfileConflictCode        = "gateway_profile_conflict"
	gatewayProfileConfigSectionKey    = "gateway"
	gatewayProfileConnectionsKey      = "connections"
	gatewayProfileActiveConnectionKey = "active_connection"
	gatewayProfileTOMLSchemeKey       = "scheme"
	gatewayProfileTOMLHostKey         = "host"
	gatewayProfileTOMLPortKey         = "port"
	// #nosec G101 -- this is a public config key, not credential material.
	gatewayProfileTOMLCredentialFileKey   = "credential_file"
	gatewayProfileTOMLDefaultWorkspaceKey = "default_workspace"
	gatewayProfileTOMLRemoteHomeKey       = "remote_home"
)

type gatewayProfileSnapshot struct {
	profile    compozyconfig.GatewayConnectionConfig
	credential string
	active     string
	exists     bool
}

func findGatewayProfile(
	connections []compozyconfig.GatewayConnectionConfig,
	name string,
) (compozyconfig.GatewayConnectionConfig, bool) {
	name = strings.TrimSpace(name)
	for index := range connections {
		if connections[index].Name == name {
			return connections[index], true
		}
	}
	return compozyconfig.GatewayConnectionConfig{}, false
}

func captureGatewayProfileSnapshot(
	homePaths compozyconfig.HomePaths,
	cfg compozyconfig.GatewayConfig,
	name string,
	deps commandDeps,
) (gatewayProfileSnapshot, error) {
	profile, exists := findGatewayProfile(cfg.Connections, name)
	snapshot := gatewayProfileSnapshot{profile: profile, active: cfg.ActiveConnection, exists: exists}
	if !exists || profile.Scheme != gatewaySchemeHTTPS {
		return snapshot, nil
	}
	credential, err := deps.readGatewayCredential(homePaths.GatewayCredentialsDir, name)
	if err != nil {
		return gatewayProfileSnapshot{}, fmt.Errorf("cli: preserve existing gateway credential: %w", err)
	}
	snapshot.credential = credential
	return snapshot, nil
}

func restoreGatewayProfileSnapshot(
	homePaths compozyconfig.HomePaths,
	name string,
	snapshot gatewayProfileSnapshot,
	deps commandDeps,
) error {
	var credentialErr error
	if snapshot.exists {
		if snapshot.profile.Scheme == gatewaySchemeHTTPS {
			_, credentialErr = deps.writeGatewayCredential(
				homePaths.GatewayCredentialsDir,
				name,
				snapshot.credential,
			)
		} else {
			credentialErr = deps.removeGatewayCredential(homePaths.GatewayCredentialsDir, name)
		}
	} else {
		credentialErr = deps.removeGatewayCredential(homePaths.GatewayCredentialsDir, name)
	}
	configErr := applyGatewayProfileState(deps, homePaths, name, profilePointer(snapshot), snapshot.active)
	return errors.Join(credentialErr, configErr)
}

func persistPairedGatewayProfile(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	profile compozyconfig.GatewayConnectionConfig,
	credential string,
	overwrite bool,
	use bool,
	deps commandDeps,
) (result error) {
	lock, current, err := acquireRecoveredGatewayProfileState(ctx, homePaths, deps)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, lock.Release())
	}()
	return persistPairedGatewayProfileWithLock(
		homePaths,
		current,
		profile,
		credential,
		overwrite,
		use,
		deps,
		lock,
	)
}

func persistPairedGatewayProfileWithLock(
	homePaths compozyconfig.HomePaths,
	current compozyconfig.GatewayConfig,
	profile compozyconfig.GatewayConnectionConfig,
	credential string,
	overwrite bool,
	use bool,
	deps commandDeps,
	lock *gatewayProfileTransactionLock,
) (result error) {
	_, exists := findGatewayProfile(current.Connections, profile.Name)
	if exists && !overwrite {
		return &gatewayClientError{
			code: gatewayProfileConflictCode, statusCode: http.StatusConflict,
			message: fmt.Sprintf(
				"gateway profile %q already exists; choose another name or pass --overwrite",
				profile.Name,
			),
		}
	}
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("cli: validate gateway profile: %w", err)
	}
	snapshot, err := captureGatewayProfileSnapshot(homePaths, current, profile.Name, deps)
	if err != nil {
		return err
	}
	desiredActive := current.ActiveConnection
	if use {
		desiredActive = profile.Name
	}
	transaction, err := beginGatewayProfileTransactionWithLock(
		homePaths.GatewayCredentialsDir,
		gatewayProfileTransactionPlan{
			operation:          gatewayProfileTransactionUpsert,
			profile:            profile,
			desiredActive:      desiredActive,
			credential:         credential,
			previousProfile:    snapshot.profile,
			previousActive:     snapshot.active,
			previousCredential: snapshot.credential,
			previousExists:     snapshot.exists,
		},
		lock,
	)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, transaction.Release())
	}()
	if err := transaction.BackupPreviousCredential(deps, snapshot.credential); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, profile.Name, snapshot, transaction, deps, err)
	}
	if _, err := deps.writeGatewayCredential(
		homePaths.GatewayCredentialsDir,
		profile.Name,
		credential,
	); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, profile.Name, snapshot, transaction, deps, err)
	}

	if err := applyGatewayProfileState(deps, homePaths, profile.Name, &profile, desiredActive); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, profile.Name, snapshot, transaction, deps, err)
	}
	if err := transaction.Complete(deps); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, profile.Name, snapshot, transaction, deps, err)
	}
	return nil
}

func rollbackGatewayProfileTransaction(
	homePaths compozyconfig.HomePaths,
	name string,
	snapshot gatewayProfileSnapshot,
	transaction *gatewayProfileTransaction,
	deps commandDeps,
	cause error,
) error {
	rollbackErr := restoreGatewayProfileSnapshot(homePaths, name, snapshot, deps)
	if rollbackErr != nil {
		return errors.Join(cause, rollbackErr, transaction.Release())
	}
	return errors.Join(cause, transaction.Complete(deps))
}

func upsertGatewayProfileMetadata(
	homePaths compozyconfig.HomePaths,
	profile compozyconfig.GatewayConnectionConfig,
) error {
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return err
	}
	_, err = compozyconfig.EditConfigOverlay(homePaths, "", target, func(editor *compozyconfig.OverlayEditor) error {
		return editor.UpsertArrayTableItem(
			[]string{gatewayProfileConfigSectionKey, gatewayProfileConnectionsKey},
			"name",
			profile.Name,
			gatewayProfileTOMLValues(profile),
		)
	})
	if err != nil {
		return fmt.Errorf("cli: persist gateway profile %q: %w", profile.Name, err)
	}
	return nil
}

func applyGatewayProfileConfigState(
	homePaths compozyconfig.HomePaths,
	name string,
	profile *compozyconfig.GatewayConnectionConfig,
	active string,
) error {
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return err
	}
	_, err = compozyconfig.EditConfigOverlay(homePaths, "", target, func(editor *compozyconfig.OverlayEditor) error {
		if profile == nil {
			if _, deleteErr := editor.DeleteArrayTableItem(
				[]string{gatewayProfileConfigSectionKey, gatewayProfileConnectionsKey},
				"name",
				name,
			); deleteErr != nil {
				return deleteErr
			}
		} else if upsertErr := editor.UpsertArrayTableItem(
			[]string{gatewayProfileConfigSectionKey, gatewayProfileConnectionsKey},
			"name",
			profile.Name,
			gatewayProfileTOMLValues(*profile),
		); upsertErr != nil {
			return upsertErr
		}
		if strings.TrimSpace(active) == "" {
			return editor.Delete([]string{gatewayProfileConfigSectionKey, gatewayProfileActiveConnectionKey})
		}
		return editor.SetValue([]string{gatewayProfileConfigSectionKey, gatewayProfileActiveConnectionKey}, active)
	})
	if err != nil {
		return fmt.Errorf("cli: persist gateway profile state %q: %w", name, err)
	}
	return nil
}

func applyGatewayProfileState(
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	name string,
	profile *compozyconfig.GatewayConnectionConfig,
	active string,
) error {
	if deps.applyGatewayProfileState != nil {
		return deps.applyGatewayProfileState(homePaths, name, profile, active)
	}
	return applyGatewayProfileConfigState(homePaths, name, profile, active)
}

func profilePointer(snapshot gatewayProfileSnapshot) *compozyconfig.GatewayConnectionConfig {
	if !snapshot.exists {
		return nil
	}
	profile := snapshot.profile
	return &profile
}

func gatewayProfileTOMLValues(profile compozyconfig.GatewayConnectionConfig) map[string]any {
	values := map[string]any{
		gatewayProfileTOMLSchemeKey: profile.Scheme,
		gatewayProfileTOMLHostKey:   profile.Host,
		gatewayProfileTOMLPortKey:   profile.Port,
	}
	if profile.CredentialFile != "" {
		values[gatewayProfileTOMLCredentialFileKey] = profile.CredentialFile
	}
	if profile.DefaultWorkspace != "" {
		values[gatewayProfileTOMLDefaultWorkspaceKey] = profile.DefaultWorkspace
	}
	if profile.RemoteHome != "" {
		values[gatewayProfileTOMLRemoteHomeKey] = profile.RemoteHome
	}
	return values
}

func setActiveGatewayProfile(homePaths compozyconfig.HomePaths, name string) error {
	target, err := compozyconfig.ResolveConfigWriteTarget(homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return err
	}
	_, err = compozyconfig.EditConfigOverlay(homePaths, "", target, func(editor *compozyconfig.OverlayEditor) error {
		if strings.TrimSpace(name) == "" {
			return editor.Delete([]string{gatewayProfileConfigSectionKey, gatewayProfileActiveConnectionKey})
		}
		return editor.SetValue([]string{gatewayProfileConfigSectionKey, gatewayProfileActiveConnectionKey}, name)
	})
	if err != nil {
		return fmt.Errorf("cli: persist active gateway profile: %w", err)
	}
	return nil
}

func removeGatewayProfile(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	name string,
	deps commandDeps,
) (result error) {
	lock, cfg, err := acquireRecoveredGatewayProfileState(ctx, homePaths, deps)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, lock.Release())
	}()
	return removeGatewayProfileWithLock(homePaths, cfg, name, deps, lock)
}

func removeGatewayProfileWithLock(
	homePaths compozyconfig.HomePaths,
	cfg compozyconfig.GatewayConfig,
	name string,
	deps commandDeps,
	lock *gatewayProfileTransactionLock,
) (result error) {
	profile, exists := findGatewayProfile(cfg.Connections, name)
	if !exists {
		return fmt.Errorf("cli: gateway profile %q was not found", name)
	}
	snapshot, err := captureGatewayProfileSnapshot(homePaths, cfg, name, deps)
	if err != nil {
		return err
	}
	desiredActive := cfg.ActiveConnection
	if desiredActive == name {
		desiredActive = ""
	}
	transaction, err := beginGatewayProfileTransactionWithLock(
		homePaths.GatewayCredentialsDir,
		gatewayProfileTransactionPlan{
			operation:          gatewayProfileTransactionRemove,
			profile:            profile,
			desiredActive:      desiredActive,
			previousProfile:    snapshot.profile,
			previousActive:     snapshot.active,
			previousCredential: snapshot.credential,
			previousExists:     snapshot.exists,
		},
		lock,
	)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, transaction.Release())
	}()
	if err := transaction.BackupPreviousCredential(deps, snapshot.credential); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, name, snapshot, transaction, deps, err)
	}
	if err := applyGatewayProfileState(deps, homePaths, name, nil, desiredActive); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, name, snapshot, transaction, deps, err)
	}
	if profile.Scheme == gatewaySchemeHTTPS {
		if err := deps.removeGatewayCredential(homePaths.GatewayCredentialsDir, name); err != nil {
			return rollbackGatewayProfileTransaction(homePaths, name, snapshot, transaction, deps, err)
		}
	}
	if err := transaction.Complete(deps); err != nil {
		return rollbackGatewayProfileTransaction(homePaths, name, snapshot, transaction, deps, err)
	}
	return nil
}
