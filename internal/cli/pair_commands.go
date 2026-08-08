package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/spf13/cobra"
)

type pairRedeemOptions struct {
	name             string
	address          string
	defaultWorkspace string
	overwrite        bool
	use              bool
}

func newPairCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "pair", Short: "Pair remote clients with a gateway"}
	cmd.AddCommand(newPairMintCommand(deps))
	cmd.AddCommand(newPairRedeemCommand(deps))
	return cmd
}

func newPairMintCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "mint",
		Short: "Mint a short-lived pairing artifact into a private handoff file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			homePaths, err := deps.resolveHome()
			if err != nil {
				return err
			}
			if err := deps.ensureHome(homePaths); err != nil {
				return err
			}
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			artifact, err := client.MintGatewayPairing(cmd.Context())
			if err != nil {
				return err
			}
			reference, err := persistGatewayPairingArtifact(homePaths, artifact)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, pairingArtifactOutput(reference))
		},
	}
}

func newPairRedeemCommand(deps commandDeps) *cobra.Command {
	var options pairRedeemOptions
	cmd := &cobra.Command{
		Use:   "redeem <artifact>",
		Short: "Redeem a pairing artifact into a named remote profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return redeemGatewayProfile(cmd, deps, strings.TrimSpace(args[0]), options)
		},
	}
	bindPairRedeemFlags(cmd, &options)
	return cmd
}

func bindPairRedeemFlags(cmd *cobra.Command, options *pairRedeemOptions) {
	cmd.Flags().StringVar(&options.name, automationNameKey, "", "Connection profile name")
	cmd.Flags().StringVar(&options.address, "address", "", "Gateway HTTPS address")
	cmd.Flags().StringVar(&options.defaultWorkspace, "default-workspace", "", "Default remote workspace")
	cmd.Flags().BoolVar(&options.overwrite, "overwrite", false, "Replace an existing profile with this name")
	cmd.Flags().BoolVar(&options.use, "use", false, "Make the new profile active")
	mustMarkFlagRequired(cmd, automationNameKey)
	mustMarkFlagRequired(cmd, "address")
}

func redeemGatewayProfile(
	cmd *cobra.Command,
	deps commandDeps,
	artifact string,
	options pairRedeemOptions,
) (returnErr error) {
	if strings.TrimSpace(artifact) == "" {
		return errors.New("cli: pairing artifact is required")
	}
	name := strings.TrimSpace(options.name)
	host, port, err := parseGatewayHTTPSAddress(options.address)
	if err != nil {
		return err
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return err
	}
	lock, gatewayConfig, err := tryAcquireRecoveredGatewayProfileState(cmd.Context(), homePaths, deps)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, lock.Release())
	}()
	if _, exists := findGatewayProfile(gatewayConfig.Connections, name); exists && !options.overwrite {
		return &gatewayClientError{
			code: gatewayProfileConflictCode, statusCode: 409,
			message: fmt.Sprintf(
				"gateway profile %q already exists; choose another name or pass --overwrite",
				name,
			),
		}
	}
	unauthenticatedTarget, err := UnauthenticatedGatewayClientTarget(name, host, port)
	if err != nil {
		return err
	}
	baseClient, err := deps.newClient(unauthenticatedTarget)
	if err != nil {
		return err
	}
	client, ok := baseClient.(gatewayClientAPI)
	if !ok {
		return errors.New("cli: daemon client does not support gateway pairing")
	}
	credential, err := gateway.GenerateDeviceCredential()
	if err != nil {
		return fmt.Errorf("cli: generate gateway device credential: %w", err)
	}
	profile := compozyconfig.GatewayConnectionConfig{
		Name: name, Scheme: gatewaySchemeHTTPS, Host: host, Port: port,
		CredentialFile: name + ".cred", DefaultWorkspace: strings.TrimSpace(options.defaultWorkspace),
	}
	prepared, err := prepareGatewayPairingProfileWithLock(
		homePaths,
		gatewayConfig,
		profile,
		credential,
		options.use,
		deps,
		lock,
	)
	if err != nil {
		return err
	}
	return completeGatewayPairingRedemption(
		cmd, client, artifact, credential, profile, options.use, prepared,
	)
}

func parseGatewayHTTPSAddress(raw string) (string, int, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, fmt.Errorf("cli: parse gateway address: %w", err)
	}
	if parsed.Scheme != gatewaySchemeHTTPS || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", 0, errors.New("cli: gateway address must be an HTTPS origin without a path")
	}
	port := 443
	if rawPort := parsed.Port(); rawPort != "" {
		port, err = strconv.Atoi(rawPort)
		if err != nil || port <= 0 || port > 65535 {
			return "", 0, errors.New("cli: gateway address port must be between 1 and 65535")
		}
	}
	return parsed.Hostname(), port, nil
}
