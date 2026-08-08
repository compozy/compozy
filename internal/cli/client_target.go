package cli

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

const (
	clientTargetLocal      = "local"
	clientTargetGateway    = "gateway"
	clientTargetSSHForward = "ssh-forward"
	clientSSHProtocol      = "ssh"
	clientHTTPSProtocol    = "https"
	clientUnixNetwork      = "unix"

	gatewayReachabilityCode          = "gateway_reachability_failed"
	gatewayStreamInterruptedCode     = "gateway_stream_interrupted"
	gatewayDeviceUnauthenticatedCode = "gateway_device_unauthenticated"
)

// ClientTarget describes one resolved daemon transport without exposing credentials in config.
type ClientTarget struct {
	kind       string
	name       string
	socketPath string
	baseURL    string
	credential string
}

// LocalClientTarget resolves the unchanged HTTP-over-UDS transport.
func LocalClientTarget(socketPath string) ClientTarget {
	return ClientTarget{kind: clientTargetLocal, name: clientTargetLocal, socketPath: strings.TrimSpace(socketPath)}
}

// GatewayClientTarget resolves a directly reachable HTTPS gateway profile.
func GatewayClientTarget(name, host string, port int, credential string) (ClientTarget, error) {
	profile := strings.TrimSpace(name)
	if !validGatewayProfileName(profile) {
		return ClientTarget{}, errors.New("cli: safe gateway profile name is required")
	}
	base, err := clientNetworkBaseURL(clientHTTPSProtocol, host, port, false)
	if err != nil {
		return ClientTarget{}, err
	}
	credential = strings.TrimSpace(credential)
	if !validGatewayDeviceCredential(credential) {
		return ClientTarget{}, errors.New("cli: gateway profile credential is invalid")
	}
	return ClientTarget{
		kind: clientTargetGateway, name: profile, baseURL: base, credential: credential,
	}, nil
}

// UnauthenticatedGatewayClientTarget is restricted to pairing redemption before a credential exists.
func UnauthenticatedGatewayClientTarget(name, host string, port int) (ClientTarget, error) {
	profile := strings.TrimSpace(name)
	if !validGatewayProfileName(profile) {
		return ClientTarget{}, errors.New("cli: safe gateway profile name is required")
	}
	base, err := clientNetworkBaseURL(clientHTTPSProtocol, host, port, false)
	if err != nil {
		return ClientTarget{}, err
	}
	return ClientTarget{kind: clientTargetGateway, name: profile, baseURL: base}, nil
}

// SSHForwardClientTarget resolves an authenticated SSH tunnel as a local-equivalent loopback client.
func SSHForwardClientTarget(host string, port int) (ClientTarget, error) {
	base, err := clientNetworkBaseURL(mcpServeTransportHTTP, host, port, true)
	if err != nil {
		return ClientTarget{}, err
	}
	return ClientTarget{kind: clientTargetSSHForward, name: clientSSHProtocol, baseURL: base}, nil
}

func clientNetworkBaseURL(scheme, host string, port int, requireLoopback bool) (string, error) {
	host = strings.TrimSpace(host)
	normalizedHost := strings.Trim(host, "[]")
	if port <= 0 || port > 65535 {
		return "", errors.New("cli: connection target port must be between 1 and 65535")
	}
	ip := net.ParseIP(normalizedHost)
	if normalizedHost == "" || strings.ContainsAny(host, "/?#@\r\n\t") {
		return "", errors.New("cli: connection target host is invalid")
	}
	if ip == nil && strings.Contains(host, ":") {
		return "", errors.New("cli: IPv6 connection targets must be valid addresses")
	}
	if requireLoopback && (ip == nil || !ip.IsLoopback()) {
		return "", errors.New("cli: SSH forward must bind to a loopback address")
	}
	return clientNetworkURL(scheme, normalizedHost, port), nil
}

func clientNetworkURL(scheme, host string, port int) string {
	return (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(strings.Trim(strings.TrimSpace(host), "[]"), strconv.Itoa(port)),
	}).String()
}

func (t ClientTarget) validate() error {
	switch t.kind {
	case clientTargetLocal:
		if strings.TrimSpace(t.socketPath) == "" {
			return errors.New("cli: daemon socket path is required")
		}
	case clientTargetGateway:
		if strings.TrimSpace(t.name) == "" || strings.TrimSpace(t.baseURL) == "" {
			return errors.New("cli: remote gateway target is incomplete")
		}
	case clientTargetSSHForward:
		if strings.TrimSpace(t.baseURL) == "" {
			return errors.New("cli: SSH forward target is incomplete")
		}
	default:
		return errors.New("cli: connection target kind is invalid")
	}
	return nil
}

func (t ClientTarget) isRemoteGateway() bool {
	return t.kind == clientTargetGateway
}

func (t ClientTarget) requestBaseURL() string {
	if t.kind == clientTargetLocal {
		return baseURL
	}
	return t.baseURL
}

func (t ClientTarget) displayName() string {
	if t.kind == clientTargetLocal {
		return t.socketPath
	}
	return t.name + " (" + t.baseURL + ")"
}

func (t ClientTarget) websocketBaseURL() (string, error) {
	parsed, err := url.Parse(t.requestBaseURL())
	if err != nil {
		return "", fmt.Errorf("cli: parse connection target: %w", err)
	}
	switch parsed.Scheme {
	case mcpServeTransportHTTP:
		parsed.Scheme = "ws"
	case clientHTTPSProtocol:
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("cli: unsupported connection target scheme %q", parsed.Scheme)
	}
	return strings.TrimSuffix(parsed.String(), "/"), nil
}

type gatewayClientError struct {
	code       string
	message    string
	statusCode int
	cause      error
}

func (e *gatewayClientError) Error() string {
	if e == nil {
		return "gateway client error"
	}
	return e.message
}

func (e *gatewayClientError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *gatewayClientError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	return contract.ErrorPayload{Error: e.message, Code: e.code}
}

func (e *gatewayClientError) cliExitCode() int {
	if e == nil {
		return 1
	}
	return apiStatusExitCode(e.statusCode)
}

func newGatewayReachabilityError(target ClientTarget, cause error) error {
	return &gatewayClientError{
		code: gatewayReachabilityCode, statusCode: http.StatusServiceUnavailable, cause: cause,
		message: fmt.Sprintf("gateway profile %q is unreachable", target.name),
	}
}

func newGatewayStreamInterruptedError(target ClientTarget, cause error) error {
	return &gatewayClientError{
		code: gatewayStreamInterruptedCode, statusCode: http.StatusServiceUnavailable, cause: cause,
		message: fmt.Sprintf("gateway stream for profile %q was interrupted", target.name),
	}
}
