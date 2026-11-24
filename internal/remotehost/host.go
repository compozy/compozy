package remotehost

import (
	"cmp"
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/version"
)

type HostAccount interface {
	KeyStore
	Status() account.Installation
	StartLogin(context.Context, string) (account.Login, error)
	SubscribeSignIn() (<-chan account.SignInEvent, func())
	UseDevice(context.Context, func(context.Context, cloudclient.Credentials) error) error
	Devices(context.Context) ([]remoteproto.AccountDevice, error)
	PatchDevice(context.Context, string, remoteproto.AccountDevicePatch) (remoteproto.AccountDevice, error)
	RevokeDevice(context.Context, string) error
}

type HostCloud interface {
	controlCloud
	JWKS(context.Context) (remoteproto.JWKS, error)
	LinkComputer(
		context.Context,
		cloudclient.ComputerLinkRequest,
		cloudclient.Credentials,
		ed25519.PrivateKey,
	) (cloudclient.ComputerLink, error)
	UpdateRemoteAccess(context.Context, remoteproto.ComputerID, ed25519.PrivateKey, bool, []string) error
	UnlinkComputer(context.Context, remoteproto.ComputerID, cloudclient.Credentials) error
}

type hostedCloud interface {
	ProvisionHostedTunnel(
		context.Context, remoteproto.ComputerID, ed25519.PrivateKey, string,
	) (remoteproto.HostedTunnel, error)
	DeleteHostedTunnel(context.Context, remoteproto.ComputerID, ed25519.PrivateKey) error
}

type HostOptions struct {
	Now                func() time.Time
	Logger             *slog.Logger
	Home               string
	Operator           http.Handler
	Persist            func(context.Context, Patch, func(config.RemoteConfig) error) error
	InstallCloudflared func(context.Context, string) (string, error)
	SyncIngress        func(context.Context, ingress.RemoteEndpoint) error
}

type Patch struct {
	Enabled               *bool `json:"enabled,omitempty"`
	Direct                *bool `json:"direct,omitempty"`
	CloudRoute            *bool `json:"cloud_route,omitempty"`
	RequireDeviceApproval *bool `json:"require_device_approval,omitempty"`
}

type Status struct {
	ComputerID            remoteproto.ComputerID `json:"computer_id"`
	Enabled               bool                   `json:"enabled"`
	Listener              string                 `json:"listener"`
	Direct                DirectStatus           `json:"direct"`
	CloudRoute            HostedStatus           `json:"cloud_route"`
	Entitlement           *EntitlementStatus     `json:"entitlement"`
	ControlLink           string                 `json:"control_link"`
	RequireDeviceApproval bool                   `json:"require_device_approval"`
}

type DirectStatus struct {
	Enabled   bool     `json:"enabled"`
	Addresses []string `json:"addresses,omitempty"`
}

type HostedStatus struct {
	Enabled   bool    `json:"enabled"`
	State     string  `json:"state"`
	Hostname  *string `json:"hostname"`
	Diagnosis string  `json:"diagnosis,omitempty"`
}

type EntitlementStatus struct {
	ValidUntil time.Time `json:"valid_until"`
	Plan       string    `json:"plan"`
}

type Host struct {
	ctx      context.Context
	cancel   context.CancelFunc
	store    Store
	cloud    HostCloud
	account  HostAccount
	opts     HostOptions
	name     string
	platform string
	version  string
	webhook  atomic.Pointer[http.Handler]

	linkMu   sync.Mutex // Serializes link and unlink through the config commit.
	mu       sync.Mutex
	config   config.RemoteConfig
	identity Identity
	run      *hostRun
	workers  sync.WaitGroup
	closed   bool
}

type hostRun struct {
	ctx       context.Context
	cancel    context.CancelFunc
	workers   sync.WaitGroup
	listeners listenerSet
	control   *controlLink
	sessions  *Sessions
	authority *certificateAuthority
	handler   func(Listener) http.Handler
	unlinked  atomic.Bool
	hosted    atomic.Pointer[hostedRun]

	ingressConfig    atomic.Pointer[config.RemoteConfig]
	syncEndpoint     func(context.Context, ingress.RemoteEndpoint) error
	ingressMu        sync.Mutex
	ingressSelection ingress.RemoteEndpoint
	ingressSynced    bool
}

func NewHost(
	ctx context.Context,
	storage Store,
	cloud HostCloud,
	account HostAccount,
	cfg config.RemoteConfig,
	opts HostOptions,
) (*Host, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.InstallCloudflared == nil {
		opts.InstallCloudflared = installCloudflared
	}
	if opts.Home != "" {
		home, err := filepath.EvalSymlinks(opts.Home)
		if err != nil {
			return nil, err
		}
		opts.Home = home
	}
	name, err := os.Hostname()
	if err != nil {
		opts.Logger.DebugContext(ctx, "Could not read the computer name", "error", err)
	}
	identity, err := storage.RemoteIdentity(ctx)
	if err != nil && !errors.Is(err, ErrMissing) {
		return nil, err
	}
	if cfg.Listener == "" && cfg.HostedListener == "" {
		cfg = config.DefaultRemoteConfig()
	}
	// #nosec G118 -- the host owns cancel until Close.
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	host := &Host{
		ctx: lifetime, cancel: cancel, store: storage, cloud: cloud, account: account, opts: opts,
		name: cmp.Or(name, "CompozyOS"), platform: runtimePlatform(), version: version.Current().Version,
		config: cfg.Clone(), identity: identity,
	}
	if host.config.Enabled && identity.ComputerID == "" {
		host.config.Enabled = false
		opts.Logger.WarnContext(ctx, ErrNotLinked.Message, "code", ErrNotLinked.Code)
	}
	if host.config.Enabled {
		err = host.start(ctx, cfg, false)
	} else {
		err = host.clearIngress(ctx)
	}
	if err != nil {
		cancel()
		return nil, err
	}
	return host, nil
}

func runtimePlatform() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	return runtime.GOOS + "_" + arch
}

func (h *Host) Close() {
	h.mu.Lock()
	h.closed = true
	h.cancel()
	h.stopLocked()
	h.mu.Unlock()
	h.workers.Wait()
}

func (h *Host) Status() Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	status := Status{
		ComputerID:  h.identity.ComputerID,
		Listener:    h.config.Listener,
		ControlLink: "disconnected",
		Direct: DirectStatus{
			Enabled: h.config.Direct.Enabled,
		},
		CloudRoute:            HostedStatus{State: remoteproto.HostedRouteOff},
		RequireDeviceApproval: h.config.RequireDeviceApproval,
	}
	run := h.run
	if run == nil {
		return status
	}
	if run.unlinked.Load() {
		status.ComputerID = ""
		return status
	}
	status.Enabled = true
	if addrs := run.listeners.addresses(ListenerTunnel); len(addrs) > 0 {
		status.Listener = addrs[0]
	}
	status.Direct.Addresses = run.listeners.addresses(ListenerDirect)
	if hosted := run.hostedView(); hosted != nil {
		status.CloudRoute = hosted.status
		if hosted.status.Hostname != nil {
			status.CloudRoute.Hostname = new(*hosted.status.Hostname)
		}
		if run.hostedEntitlement(hosted) == remoteproto.HostedTunnelEntitlementOnly {
			status.CloudRoute.State = remoteproto.HostedTunnelEntitlementOnly
		}
	}
	if run.control.connected.Load() {
		status.ControlLink = "connected"
	}
	if lease, ok := run.sessions.Entitlement(); ok {
		status.Entitlement = &EntitlementStatus{Plan: lease.Plan, ValidUntil: time.Unix(lease.ValidUntil, 0).UTC()}
	}
	return status
}

func (h *Host) Patch(ctx context.Context, patch Patch) (Status, error) {
	apply := func(cfg config.RemoteConfig) error { return h.ApplyConfig(ctx, cfg) }
	if h.opts.Persist != nil {
		if err := h.opts.Persist(ctx, patch, apply); err != nil {
			return Status{}, err
		}
		return h.Status(), nil
	}
	h.mu.Lock()
	next := h.config.Clone()
	h.mu.Unlock()
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.Direct != nil {
		next.Direct.Enabled = *patch.Direct
	}
	if patch.CloudRoute != nil {
		next.CloudRoute.Enabled = *patch.CloudRoute
	}
	if patch.RequireDeviceApproval != nil {
		next.RequireDeviceApproval = *patch.RequireDeviceApproval
	}
	if err := apply(next); err != nil {
		return Status{}, err
	}
	return h.Status(), nil
}

func (h *Host) ApplyConfig(ctx context.Context, cfg config.RemoteConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return context.Canceled
	}
	h.forgetUnlinkedLocked()
	if err := h.applyConfigLocked(ctx, cfg); err != nil {
		return err
	}
	h.config = cfg.Clone()
	return nil
}

func (h *Host) applyConfigLocked(ctx context.Context, cfg config.RemoteConfig) error {
	enablingHosted := cfg.CloudRoute.Enabled && !h.config.CloudRoute.Enabled
	if (cfg.Enabled || enablingHosted) && h.identity.ComputerID == "" {
		return ErrNotLinked
	}
	if enablingHosted && !cfg.Enabled {
		return conflict("remote_access_disabled", "Turn on remote access to enable the hosted route.")
	}
	switch {
	case !cfg.Enabled:
		return h.disableListeners(ctx, cfg)
	case h.run == nil:
		return h.start(ctx, cfg, true)
	default:
		return h.reconfigure(ctx, cfg, enablingHosted)
	}
}

func (h *Host) clearIngress(ctx context.Context) error {
	if h.opts.SyncIngress == nil {
		return nil
	}
	return h.opts.SyncIngress(ctx, ingress.RemoteEndpoint{})
}

func (h *Host) stopLocked() {
	if h.run != nil {
		h.run.close()
		h.run = nil
	}
}

func (h *Host) forgetUnlinkedLocked() {
	if h.run != nil && h.run.unlinked.Load() {
		h.stopLocked()
		h.identity = Identity{}
		h.config.Enabled = false
	}
}
