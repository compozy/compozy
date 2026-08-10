package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

const (
	sshInstallRequiredCode = "gateway_ssh_install_required"
	sshVersionMismatchCode = "gateway_ssh_version_mismatch"
	sshHostKeyChangedCode  = "gateway_ssh_host_key_changed"
	sshUnreachableCode     = "gateway_ssh_unreachable"
	sshTunnelLostCode      = "gateway_ssh_tunnel_lost"
	sshStrictHostKeyOption = "StrictHostKeyChecking=yes"
)

type connectSSHOptions struct {
	name        string
	user        string
	port        int
	remoteHome  string
	overwrite   bool
	remoteStart bool
	ownerToken  string
}

type sshConnectionRecord struct {
	Profile    gatewayProfileRecord `json:"profile"`
	LocalURL   string               `json:"local_url"`
	RemoteHTTP string               `json:"remote_http"`
	Daemon     string               `json:"daemon"`
}

func newConnectSSHCommand(deps commandDeps) *cobra.Command {
	options := connectSSHOptions{port: 22, remoteStart: true}
	cmd := &cobra.Command{
		Use:   "ssh <host>",
		Short: "Reach a remote daemon through a loopback-only SSH forward",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnectSSH(cmd, deps, strings.TrimSpace(args[0]), options)
		},
	}
	cmd.Flags().StringVar(&options.name, "name", "", "Stored SSH profile name")
	cmd.Flags().StringVar(&options.user, "user", "", "SSH user; defaults to SSH configuration")
	cmd.Flags().IntVar(&options.port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&options.remoteHome, "remote-home", "", "Remote COMPOZY_HOME")
	cmd.Flags().BoolVar(&options.overwrite, "overwrite", false, "Replace an existing SSH profile")
	cmd.Flags().BoolVar(&options.remoteStart, "start", true, "Start the remote daemon when it is not running")
	return cmd
}

func runConnectSSH(cmd *cobra.Command, deps commandDeps, host string, options connectSSHOptions) error {
	target, profile, runtime, err := prepareSSHConnection(cmd.Context(), deps, host, options)
	if err != nil {
		return err
	}
	executor := deps.newSSHExecutor()
	if executor == nil {
		return errors.New("cli: SSH executor is required")
	}
	if err := verifyRemoteCompozy(cmd.Context(), executor, target, options.remoteHome); err != nil {
		return err
	}
	resolution, err := resolveRemoteDaemon(cmd.Context(), executor, target, options, deps)
	if err != nil {
		return err
	}
	return runResolvedSSHConnection(cmd, deps, target, profile, runtime.HomePaths, executor, resolution, options)
}

func runResolvedSSHConnection(
	cmd *cobra.Command,
	deps commandDeps,
	target sshTarget,
	profile compozyconfig.GatewayConnectionConfig,
	homePaths compozyconfig.HomePaths,
	executor sshExecutor,
	resolution remoteDaemonResolution,
	options connectSSHOptions,
) (returnErr error) {
	status := resolution.status
	tunnel, err := startResolvedSSHTunnel(cmd.Context(), deps, executor, target, options.remoteHome, resolution)
	if err != nil {
		return err
	}
	stopRemote := resolution.started()
	defer closeSSHTunnelAndOwnedDaemon(
		cmd.Context(), &returnErr, tunnel, deps, executor, target, options.remoteHome, resolution, status, &stopRemote,
	)

	localTarget, err := SSHForwardClientTarget("127.0.0.1", tunnel.LocalPort())
	if err != nil {
		return err
	}
	if err := waitForForwardedDaemon(cmd.Context(), deps, localTarget); err != nil {
		return err
	}
	if err := persistSSHProfile(cmd.Context(), deps, homePaths, profile, options.overwrite); err != nil {
		return err
	}
	daemonState := "reused"
	if resolution.started() {
		daemonState = "started"
	}
	record := sshConnectionRecord{
		Profile:    gatewayProfileFromConfig(profile, ""),
		LocalURL:   localTarget.requestBaseURL(),
		RemoteHTTP: clientNetworkURL(mcpServeTransportHTTP, status.HTTPHost, status.HTTPPort),
		Daemon:     daemonState,
	}
	if err := writeCommandOutput(cmd, sshConnectionOutput(record)); err != nil {
		return err
	}
	return waitForSSHTunnel(cmd.Context(), resolution, tunnel, &stopRemote)
}

func startResolvedSSHTunnel(
	ctx context.Context,
	deps commandDeps,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
	resolution remoteDaemonResolution,
) (sshTunnel, error) {
	tunnel, err := executor.StartTunnel(ctx, target, resolution.status.HTTPHost, resolution.status.HTTPPort)
	if err == nil {
		return tunnel, nil
	}
	if !resolution.started() {
		return nil, classifySSHFailure(err)
	}
	return nil, cleanupOwnedRemoteDaemon(
		ctx, deps, executor, target, remoteHome, resolution.ownership, resolution.lease,
		resolution.status, classifySSHFailure(err),
	)
}

func closeSSHTunnelAndOwnedDaemon(
	ctx context.Context,
	returnErr *error,
	tunnel sshTunnel,
	deps commandDeps,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
	resolution remoteDaemonResolution,
	status DaemonStatus,
	stopRemote *bool,
) {
	closeErr := tunnel.Close()
	var lifecycleErr error
	if stopRemote != nil && *stopRemote {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deps.stopTimeout)
		defer cancel()
		lifecycleErr = stopSSHOwnedRemoteDaemon(
			stopCtx, executor, target, remoteHome, status, resolution.ownership,
		)
	}
	lifecycleErr = errors.Join(lifecycleErr, closeSSHOwnershipLease(resolution.lease))
	if returnErr != nil {
		*returnErr = errors.Join(*returnErr, closeErr, lifecycleErr)
	}
}

func waitForSSHTunnel(
	ctx context.Context,
	resolution remoteDaemonResolution,
	tunnel sshTunnel,
	stopRemote *bool,
) error {
	waitResult := make(chan error, 1)
	go func() { waitResult <- tunnel.Wait() }()
	leaseResult := make(<-chan error)
	if resolution.lease != nil {
		result := make(chan error, 1)
		go func() { result <- resolution.lease.Wait() }()
		leaseResult = result
	}
	select {
	case <-ctx.Done():
		return nil
	case waitErr := <-waitResult:
		if waitErr == nil {
			return nil
		}
		if stopRemote != nil {
			*stopRemote = false
		}
		if classified := classifySSHFailure(waitErr); classified != waitErr {
			return classified
		}
		return &gatewayClientError{
			code: sshTunnelLostCode, statusCode: http.StatusServiceUnavailable,
			message: "SSH tunnel was interrupted; remote work continues and can be observed after reconnect",
			cause:   waitErr,
		}
	case leaseErr := <-leaseResult:
		if stopRemote != nil {
			*stopRemote = false
		}
		if leaseErr == nil {
			leaseErr = errors.New("SSH ownership lease ended unexpectedly")
		}
		return &gatewayClientError{
			code: sshTunnelLostCode, statusCode: http.StatusServiceUnavailable,
			message: "SSH ownership lease was interrupted; the remote daemon was left running",
			cause:   leaseErr,
		}
	}
}

func closeSSHOwnershipLease(lease sshOwnershipLease) error {
	if lease == nil {
		return nil
	}
	if err := lease.Close(); err != nil {
		return fmt.Errorf("cli: close SSH ownership lease: %w", err)
	}
	return nil
}

func prepareSSHConnection(
	ctx context.Context,
	deps commandDeps,
	host string,
	options connectSSHOptions,
) (sshTarget, compozyconfig.GatewayConnectionConfig, runtimeContext, error) {
	if err := validateSSHTarget(host, options); err != nil {
		return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, err
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, err
	}
	if err := recoverAvailableGatewayProfileState(ctx, homePaths, deps); err != nil {
		return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, err
	}
	cfg, err := deps.loadConfig()
	if err != nil {
		return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, err
	}
	name := strings.TrimSpace(options.name)
	if name == "" {
		name = defaultSSHProfileName(host)
	}
	profile := compozyconfig.GatewayConnectionConfig{
		Name: name, Scheme: clientSSHProtocol, Host: host, Port: options.port,
		RemoteHome: strings.TrimSpace(options.remoteHome),
	}
	if err := profile.Validate(); err != nil {
		return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, err
	}
	if existing, exists := findGatewayProfile(cfg.Gateway.Connections, name); exists {
		if !options.overwrite {
			return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, &gatewayClientError{
				code:       gatewayProfileConflictCode,
				statusCode: http.StatusConflict,
				message: fmt.Sprintf(
					"gateway profile %q already exists; choose another name or pass --overwrite",
					name,
				),
			}
		}
		if existing.Scheme != clientSSHProtocol {
			return sshTarget{}, compozyconfig.GatewayConnectionConfig{}, runtimeContext{}, errors.New(
				"cli: remove the existing HTTPS profile before replacing it with SSH",
			)
		}
	}
	return sshTarget{host: host, user: strings.TrimSpace(options.user), port: options.port},
		profile, runtimeContext{HomePaths: homePaths, Config: cfg}, nil
}

func validateSSHTarget(host string, options connectSSHOptions) error {
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "-") || strings.ContainsAny(host, "\x00\r\n\t /@") {
		return errors.New("cli: SSH host is invalid")
	}
	user := strings.TrimSpace(options.user)
	if strings.HasPrefix(user, "-") || strings.ContainsAny(user, "\x00\r\n\t /@") {
		return errors.New("cli: SSH user is invalid")
	}
	if options.port <= 0 || options.port > 65535 {
		return errors.New("cli: SSH port must be between 1 and 65535")
	}
	if strings.ContainsAny(options.remoteHome, "\x00\r\n") {
		return errors.New("cli: remote home contains control characters")
	}
	return nil
}

func defaultSSHProfileName(host string) string {
	var builder strings.Builder
	builder.WriteString("ssh-")
	for _, character := range strings.ToLower(host) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' {
			builder.WriteRune(character)
			continue
		}
		builder.WriteByte('-')
	}
	return strings.Trim(builder.String(), "-")
}

func verifyRemoteCompozy(ctx context.Context, executor sshExecutor, target sshTarget, remoteHome string) error {
	if _, err := executor.Run(ctx, target, remoteCompozyLookupCommand()); err != nil {
		if classified := classifySSHFailure(err); classified != err {
			return classified
		}
		return &gatewayClientError{
			code: sshInstallRequiredCode, statusCode: http.StatusNotFound,
			message: "CompozyOS is not installed on the remote host",
			cause:   err,
		}
	}
	output, err := executor.Run(ctx, target, remoteCompozyCommand(remoteHome, versionKey, "-o", "json"))
	if err != nil {
		return classifySSHFailure(err)
	}
	remoteVersion, err := parseRemoteVersion(output)
	if err != nil {
		return err
	}
	localVersion := strings.TrimSpace(version.Current().Version)
	if remoteVersion != localVersion {
		return &gatewayClientError{
			code:       sshVersionMismatchCode,
			statusCode: http.StatusConflict,
			message: fmt.Sprintf(
				"remote CompozyOS version %s is incompatible with local version %s",
				remoteVersion,
				localVersion,
			),
		}
	}
	return nil
}

func parseRemoteVersion(output []byte) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		return "", fmt.Errorf("cli: decode remote CompozyOS version: %w", err)
	}
	for _, key := range []string{versionKey, versionValue} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", errors.New("cli: remote CompozyOS version response is incomplete")
}
