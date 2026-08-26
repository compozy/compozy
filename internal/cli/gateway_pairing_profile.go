package cli

import (
	"errors"
	"fmt"
	"net"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

type preparedGatewayPairingProfile struct {
	transaction   *gatewayProfileTransaction
	snapshot      gatewayProfileSnapshot
	profile       compozyconfig.GatewayConnectionConfig
	desiredActive string
	homePaths     compozyconfig.HomePaths
	deps          commandDeps
}

func prepareGatewayPairingProfileWithLock(
	homePaths compozyconfig.HomePaths,
	current compozyconfig.GatewayConfig,
	profile compozyconfig.GatewayConnectionConfig,
	credential string,
	use bool,
	deps commandDeps,
	lock *gatewayProfileTransactionLock,
) (*preparedGatewayPairingProfile, error) {
	if err := profile.Validate(); err != nil {
		return nil, fmt.Errorf("cli: validate gateway profile: %w", err)
	}
	snapshot, err := captureGatewayProfileSnapshot(homePaths, current, profile.Name, deps)
	if err != nil {
		return nil, err
	}
	desiredActive := current.ActiveConnection
	if use {
		desiredActive = profile.Name
	}
	transaction, err := beginGatewayProfileTransactionWithLock(
		homePaths.GatewayCredentialsDir,
		gatewayProfileTransactionPlan{
			operation: gatewayProfileTransactionPair, profile: profile,
			desiredActive: desiredActive, credential: credential,
			previousProfile: snapshot.profile, previousActive: snapshot.active,
			previousCredential: snapshot.credential, previousExists: snapshot.exists,
			pairingStartedAt: gatewayProfileNow(deps),
		},
		lock,
	)
	if err != nil {
		return nil, err
	}
	prepared := &preparedGatewayPairingProfile{
		transaction: transaction, snapshot: snapshot, profile: profile,
		desiredActive: desiredActive, homePaths: homePaths, deps: deps,
	}
	if err := transaction.BackupPreviousCredential(deps, snapshot.credential); err != nil {
		return nil, prepared.Rollback(err)
	}
	if _, err := deps.writeGatewayCredential(
		homePaths.GatewayCredentialsDir,
		profile.Name,
		credential,
	); err != nil {
		return nil, prepared.Rollback(err)
	}
	return prepared, nil
}

func (p *preparedGatewayPairingProfile) Complete() error {
	if p == nil || p.transaction == nil {
		return errors.New("cli: prepared gateway pairing profile is required")
	}
	if err := applyGatewayProfileState(
		p.deps,
		p.homePaths,
		p.profile.Name,
		&p.profile,
		p.desiredActive,
	); err != nil {
		return err
	}
	return p.transaction.Complete(p.deps)
}

func (p *preparedGatewayPairingProfile) Rollback(cause error) error {
	if p == nil || p.transaction == nil {
		return cause
	}
	return rollbackGatewayProfileTransaction(
		p.homePaths,
		p.profile.Name,
		p.snapshot,
		p.transaction,
		p.deps,
		cause,
	)
}

func (p *preparedGatewayPairingProfile) Release() error {
	if p == nil || p.transaction == nil {
		return nil
	}
	return p.transaction.Release()
}

func isDefinitiveGatewayPairingRejection(err error) bool {
	var payloadError interface{ errorPayload() contract.ErrorPayload }
	if !errors.As(err, &payloadError) {
		return false
	}
	switch payloadError.errorPayload().Code {
	case "gateway_invalid_request",
		"gateway_pairing_invalid",
		"gateway_pairing_expired",
		"gateway_pairing_spent",
		"gateway_pairing_forbidden":
		return true
	default:
		return false
	}
}

func completeGatewayPairingRedemption(
	cmd *cobra.Command,
	client gatewayClientAPI,
	artifact string,
	credential string,
	profile compozyconfig.GatewayConnectionConfig,
	use bool,
	prepared *preparedGatewayPairingProfile,
) (returnErr error) {
	defer func() {
		returnErr = errors.Join(returnErr, prepared.Release())
	}()
	issued, err := client.RedeemGatewayPairing(cmd.Context(), contract.GatewayPairingRedeemRequest{
		Artifact: artifact, Name: profile.Name, ActorKind: "cli_profile", Credential: credential,
	})
	if err != nil {
		if isDefinitiveGatewayPairingRejection(err) || gatewayPairingRequestNotSent(err) {
			return prepared.Rollback(err)
		}
		return err
	}
	if issued.Credential != credential {
		return errors.New("cli: gateway returned a different device credential")
	}
	if err := prepared.Complete(); err != nil {
		return err
	}
	active := ""
	if use {
		active = profile.Name
	}
	record := gatewayProfileMutationRecord{
		Profile: gatewayProfileFromConfig(profile, active),
		Device:  &issued.Device, Status: "paired",
	}
	return writeCommandOutput(cmd, gatewayProfileOutput(record))
}

func gatewayPairingRequestNotSent(err error) bool {
	if requestError, ok := errors.AsType[*gatewayPairingRequestError](err); ok {
		return requestError.requestNotSent
	}
	var operationError *net.OpError
	return errors.As(err, &operationError) && operationError.Op == "dial"
}
