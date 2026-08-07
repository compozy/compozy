package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const gatewayPairingRecoveryWindow = 5 * time.Minute

func acquireRecoveredGatewayProfileState(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	deps commandDeps,
) (*gatewayProfileTransactionLock, compozyconfig.GatewayConfig, error) {
	lock, err := acquireGatewayProfileRecoveryLock(ctx, homePaths, deps)
	if err != nil {
		return nil, compozyconfig.GatewayConfig{}, err
	}
	cfg, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		return nil, compozyconfig.GatewayConfig{}, errors.Join(err, lock.Release())
	}
	return lock, cfg.Gateway, nil
}

func acquireGatewayProfileRecoveryLock(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	deps commandDeps,
) (*gatewayProfileTransactionLock, error) {
	lock, err := acquireGatewayProfileTransactionLock(
		ctx,
		homePaths.GatewayCredentialsDir,
		gatewayProfileTransactionStateLockName,
	)
	if err != nil {
		return nil, err
	}
	if err := recoverGatewayProfileStateWithLock(ctx, homePaths, deps, lock); err != nil {
		return nil, errors.Join(err, lock.Release())
	}
	return lock, nil
}

func tryAcquireRecoveredGatewayProfileState(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	deps commandDeps,
) (*gatewayProfileTransactionLock, compozyconfig.GatewayConfig, error) {
	lock, err := tryAcquireGatewayProfileTransactionLock(
		homePaths.GatewayCredentialsDir,
		gatewayProfileTransactionStateLockName,
	)
	if err != nil {
		return nil, compozyconfig.GatewayConfig{}, err
	}
	if err := recoverGatewayProfileStateWithLock(ctx, homePaths, deps, lock); err != nil {
		return nil, compozyconfig.GatewayConfig{}, errors.Join(err, lock.Release())
	}
	cfg, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		return nil, compozyconfig.GatewayConfig{}, errors.Join(err, lock.Release())
	}
	return lock, cfg.Gateway, nil
}

func recoverGatewayProfileState(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	deps commandDeps,
) error {
	lock, _, err := acquireRecoveredGatewayProfileState(ctx, homePaths, deps)
	if err != nil {
		return err
	}
	return lock.Release()
}

func recoverGatewayProfileStateWithLock(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	deps commandDeps,
	lock *gatewayProfileTransactionLock,
) error {
	return recoverGatewayProfileTransactionsWithLock(
		ctx,
		homePaths.GatewayCredentialsDir,
		func(recoveryCtx context.Context, journal gatewayProfileTransactionJournal) error {
			return recoverGatewayProfileJournal(recoveryCtx, homePaths, journal, deps)
		},
		lock,
	)
}

func recoverGatewayProfileJournal(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	if deps.readGatewayCredential == nil || deps.removeGatewayCredential == nil {
		return errors.New("cli: gateway credential dependencies are required for profile recovery")
	}
	var recoveryErr error
	switch journal.Operation {
	case gatewayProfileTransactionUpsert:
		recoveryErr = recoverGatewayProfileUpsert(homePaths, journal, deps)
	case gatewayProfileTransactionPair:
		recoveryErr = recoverGatewayProfilePairing(ctx, homePaths, journal, deps)
	case gatewayProfileTransactionRemove:
		if err := applyGatewayProfileState(
			deps,
			homePaths,
			journal.Profile,
			nil,
			journal.DesiredActive,
		); err != nil {
			recoveryErr = err
			break
		}
		if err := deps.removeGatewayCredential(homePaths.GatewayCredentialsDir, journal.Profile); err != nil {
			recoveryErr = fmt.Errorf("cli: finish gateway profile credential removal: %w", err)
		}
	default:
		recoveryErr = errors.New("cli: gateway profile transaction operation is invalid")
	}
	if recoveryErr != nil {
		return recoveryErr
	}
	return removeGatewayProfileTransactionBackup(homePaths.GatewayCredentialsDir, journal, deps)
}

func recoverGatewayProfileUpsert(
	homePaths compozyconfig.HomePaths,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	credential, err := deps.readGatewayCredential(homePaths.GatewayCredentialsDir, journal.Profile)
	if err == nil {
		digest := gatewayProfileCredentialDigest(credential)
		switch {
		case digest == journal.CredentialSHA256:
			return applyGatewayProfileState(
				deps,
				homePaths,
				journal.Profile,
				journal.DesiredProfile,
				journal.DesiredActive,
			)
		case journal.PreviousProfile != nil && digest == journal.PreviousCredentialSHA256:
			return restoreGatewayProfileJournalMetadata(homePaths, journal, deps)
		default:
			return errors.New("cli: gateway profile transaction credential does not match recoverable state")
		}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: read gateway profile credential during recovery: %w", err)
	}
	if journal.PreviousProfile != nil && journal.PreviousProfile.Scheme == gatewaySchemeHTTPS {
		if err := restoreGatewayProfileCredentialBackup(homePaths, journal, deps); err != nil {
			return err
		}
		return restoreGatewayProfileJournalMetadata(homePaths, journal, deps)
	}
	if err := deps.removeGatewayCredential(homePaths.GatewayCredentialsDir, journal.Profile); err != nil {
		return fmt.Errorf("cli: clean absent gateway profile credential during recovery: %w", err)
	}
	return restoreGatewayProfileJournalMetadata(homePaths, journal, deps)
}

func recoverGatewayProfilePairing(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	credential, err := deps.readGatewayCredential(homePaths.GatewayCredentialsDir, journal.Profile)
	if errors.Is(err, os.ErrNotExist) {
		return restoreGatewayProfilePairingPreviousState(homePaths, journal, deps)
	}
	if err != nil {
		return fmt.Errorf("cli: read pending gateway pairing credential: %w", err)
	}
	digest := gatewayProfileCredentialDigest(credential)
	if digest == journal.PreviousCredentialSHA256 && journal.PreviousProfile != nil {
		return restoreGatewayProfileJournalMetadata(homePaths, journal, deps)
	}
	if digest != journal.CredentialSHA256 {
		return errors.New("cli: pending gateway pairing credential does not match recoverable state")
	}
	paired, probeErr := probePendingGatewayPairing(ctx, journal, credential, deps)
	if probeErr == nil && paired {
		return applyGatewayProfileState(
			deps,
			homePaths,
			journal.Profile,
			journal.DesiredProfile,
			journal.DesiredActive,
		)
	}
	if !gatewayPairingCredentialRejected(probeErr) || gatewayProfileNow(deps).Before(
		journal.PairingStartedAt.Add(gatewayPairingRecoveryWindow),
	) {
		return &gatewayClientError{
			code: "gateway_pairing_recovery_pending", statusCode: http.StatusConflict,
			message: "gateway pairing outcome is still uncertain; retry after the pairing window",
			cause:   probeErr,
		}
	}
	return restoreGatewayProfilePairingPreviousState(homePaths, journal, deps)
}

func probePendingGatewayPairing(
	ctx context.Context,
	journal gatewayProfileTransactionJournal,
	credential string,
	deps commandDeps,
) (bool, error) {
	if journal.DesiredProfile == nil {
		return false, errors.New("cli: pending gateway pairing profile is missing")
	}
	target, err := GatewayClientTarget(
		journal.DesiredProfile.Name,
		journal.DesiredProfile.Host,
		journal.DesiredProfile.Port,
		credential,
	)
	if err != nil {
		return false, err
	}
	baseClient, err := deps.newClient(target)
	if err != nil {
		return false, err
	}
	client, ok := baseClient.(gatewayClientAPI)
	if !ok {
		return false, errors.New("cli: daemon client does not support gateway pairing recovery")
	}
	if _, err := client.ListGatewayDevices(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func restoreGatewayProfilePairingPreviousState(
	homePaths compozyconfig.HomePaths,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	if journal.PreviousProfile != nil && journal.PreviousProfile.Scheme == gatewaySchemeHTTPS {
		if err := restoreGatewayProfileCredentialBackup(homePaths, journal, deps); err != nil {
			return err
		}
	} else if err := deps.removeGatewayCredential(
		homePaths.GatewayCredentialsDir,
		journal.Profile,
	); err != nil {
		return fmt.Errorf("cli: remove unredeemed gateway pairing credential: %w", err)
	}
	return restoreGatewayProfileJournalMetadata(homePaths, journal, deps)
}

func restoreGatewayProfileCredentialBackup(
	homePaths compozyconfig.HomePaths,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	credential, err := deps.readGatewayCredential(
		homePaths.GatewayCredentialsDir,
		journal.PreviousCredentialBackup,
	)
	if err != nil {
		return fmt.Errorf("cli: read previous gateway credential backup: %w", err)
	}
	if gatewayProfileCredentialDigest(credential) != journal.PreviousCredentialSHA256 {
		return errors.New("cli: previous gateway credential backup does not match its digest")
	}
	if _, err := deps.writeGatewayCredential(
		homePaths.GatewayCredentialsDir,
		journal.Profile,
		credential,
	); err != nil {
		return fmt.Errorf("cli: restore previous gateway credential backup: %w", err)
	}
	return nil
}

func gatewayPairingCredentialRejected(err error) bool {
	if err == nil {
		return false
	}
	var payloadError interface{ errorPayload() contract.ErrorPayload }
	return errors.As(err, &payloadError) &&
		payloadError.errorPayload().Code == gatewayDeviceUnauthenticatedCode
}

func gatewayProfileNow(deps commandDeps) time.Time {
	if deps.now != nil {
		return deps.now().UTC()
	}
	return time.Now().UTC()
}

func restoreGatewayProfileJournalMetadata(
	homePaths compozyconfig.HomePaths,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	return applyGatewayProfileState(
		deps,
		homePaths,
		journal.Profile,
		journal.PreviousProfile,
		journal.PreviousActive,
	)
}
