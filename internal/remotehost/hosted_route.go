package remotehost

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	hostedStateStarting = "starting"
	hostedStateRetrying = "retrying"
	hostedRefresh       = time.Minute
	hostedStartTimeout  = 35 * time.Second
	hostedProbeInterval = 250 * time.Millisecond
	hostedEdgeStartup   = 10 * time.Second
	hostedEdgeRetry     = 2 * time.Second
	hostedEdgeInterval  = 15 * time.Second
	hostedEdgeTimeout   = 3 * time.Second
)

type hostedView struct {
	status HostedStatus
	mode   string
}

type hostedEdgeObservation struct {
	next          time.Time
	startingUntil time.Time
	missed        bool
}

type hostedRun struct {
	cancel   context.CancelFunc
	done     chan struct{}
	view     atomic.Pointer[hostedView]
	logger   *slog.Logger
	now      func() time.Time
	fetch    func(context.Context) (remoteproto.HostedTunnel, error)
	install  func(context.Context, string) (string, error)
	home     string
	identity Identity
	edge     *http.Client
	refresh  chan struct{}
	onChange func(context.Context)
}

func (h *Host) replaceHostedRoute(run *hostRun, listener string, initial remoteproto.HostedTunnel) {
	run.stopHostedRoute()
	if listener == "" {
		return
	}
	// #nosec G118 -- the hosted run owns cancel until listeners are reconfigured or shut down.
	ctx, cancel := context.WithCancel(run.ctx)
	hosted := &hostedRun{
		cancel: cancel, done: make(chan struct{}), logger: h.opts.Logger, now: h.opts.Now,
		fetch: func(ctx context.Context) (remoteproto.HostedTunnel, error) {
			return h.provisionHosted(ctx, run, listener)
		},
		install: h.opts.InstallCloudflared, home: h.opts.Home, refresh: make(chan struct{}, 1),
		identity: h.identity, edge: hostedEdgeClient(h.opts.HostedRouteTransport), onChange: run.syncIngress,
	}
	hosted.view.Store(new(hostedView{status: HostedStatus{Enabled: true, State: hostedStateStarting}}))
	run.hosted.Store(hosted)
	run.workers.Go(func() {
		defer close(hosted.done)
		hosted.supervise(ctx, initial)
	})
}

func hostedEdgeClient(transport http.RoundTripper) *http.Client {
	if transport == nil {
		transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, ForceAttemptHTTP2: true,
		}
	}
	return &http.Client{Transport: transport, Timeout: hostedEdgeTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (r *hostRun) stopHostedRoute() {
	if hosted := r.hosted.Swap(nil); hosted != nil {
		hosted.cancel()
		<-hosted.done
	}
}

func (h *Host) provisionHosted(
	ctx context.Context, run *hostRun, listener string,
) (remoteproto.HostedTunnel, error) {
	cloud, ok := h.cloud.(hostedCloud)
	if !ok || h.opts.Home == "" {
		return remoteproto.HostedTunnel{}, ErrHostedUnavailable
	}
	tunnel, err := cloud.ProvisionHostedTunnel(ctx, run.control.id, run.control.key, listener)
	if err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	if !validHostedHostname(tunnel.Hostname) || tunnel.ConnectorToken == "" ||
		(tunnel.Mode != remoteproto.HostedTunnelActive && tunnel.Mode != remoteproto.HostedTunnelEntitlementOnly) {
		return remoteproto.HostedTunnel{}, ErrHostedUnavailable
	}
	return tunnel, nil
}

func (h *hostedRun) supervise(ctx context.Context, credentials remoteproto.HostedTunnel) {
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 500 * time.Millisecond}
	backoff := time.Second
	for ctx.Err() == nil {
		var ready bool
		var diagnosis string
		var err error
		if credentials.ConnectorToken == "" {
			credentials, err = h.fetch(ctx)
		}
		if err == nil {
			ready, diagnosis, err = h.connect(ctx, client, credentials)
		}
		if ctx.Err() != nil {
			return
		}
		if ready {
			backoff = time.Second
		}
		state, mode := hostedStateRetrying, credentials.Mode
		if cloudclient.FailureCode(err) == planRequiredCode {
			state, mode = remoteproto.HostedTunnelEntitlementOnly, remoteproto.HostedTunnelEntitlementOnly
			credentials.Hostname = ""
		}
		if diagnosis == "" {
			diagnosis = h.view.Load().status.Diagnosis
		}
		h.publish(ctx, credentials.Hostname, mode, state, diagnosis)
		timer := time.NewTimer(max(backoff, cloudclient.RetryDelay(h.now(), err)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		credentials = remoteproto.HostedTunnel{}
		backoff = min(backoff*2, time.Minute)
	}
}

func (h *hostedRun) connect(
	ctx context.Context, client *http.Client, credentials remoteproto.HostedTunnel,
) (bool, string, error) {
	h.publish(ctx, credentials.Hostname, credentials.Mode, hostedStateStarting, h.view.Load().status.Diagnosis)
	binary, err := h.install(ctx, h.home)
	if err != nil {
		h.logger.WarnContext(ctx, "Could not prepare the pinned hosted connector", "error", err)
		return false, "", err
	}
	process, err := startCloudflared(ctx, h.home, binary, credentials.ConnectorToken)
	if err != nil {
		h.logger.WarnContext(ctx, "Could not start the hosted connector", "error", err)
		return false, "", err
	}
	return h.observeProcess(ctx, client, process, credentials)
}

func (h *hostedRun) observeProcess(
	ctx context.Context, client *http.Client, process *cloudflaredProcess, credentials remoteproto.HostedTunnel,
) (bool, string, error) {
	probe := time.NewTicker(hostedProbeInterval)
	defer probe.Stop()
	refresh := time.NewTicker(hostedRefresh)
	defer refresh.Stop()
	startup := time.NewTimer(hostedStartTimeout)
	defer startup.Stop()
	readyDeadline := startup.C
	everReady := false
	var edge hostedEdgeObservation
	for {
		select {
		case <-ctx.Done():
			process.stop(h.logger)
			return everReady, "", ctx.Err()
		case err := <-process.done:
			return everReady, process.diagnostics.diagnosis(), err
		case <-readyDeadline:
			process.stop(h.logger)
			return everReady, process.diagnostics.diagnosis(), errors.New("hosted connector did not become ready")
		case <-probe.C:
			ready, err := process.ready(ctx, client)
			if err == nil && ready {
				everReady = true
				readyDeadline = nil
				h.observeEdge(ctx, credentials, &edge)
			} else {
				edge = hostedEdgeObservation{}
				diagnosis := process.diagnostics.diagnosis()
				if everReady || diagnosis != "" {
					h.publish(ctx, credentials.Hostname, credentials.Mode, hostedStateRetrying, diagnosis)
				}
			}
			continue
		case <-h.refresh:
		case <-refresh.C:
		}
		next, err := h.fetch(ctx)
		if err != nil {
			switch cloudclient.FailureCode(err) {
			case planRequiredCode, "computer_not_owned", "account_suspended":
				process.stop(h.logger)
				return everReady, "", err
			}
			continue
		}
		if next.Hostname != credentials.Hostname || next.ConnectorToken != credentials.ConnectorToken {
			process.stop(h.logger)
			return everReady, "", errors.New("hosted connector credentials changed")
		}
		if next.Mode != credentials.Mode {
			edge = hostedEdgeObservation{}
		}
		credentials = next
	}
}

func (h *hostedRun) observeEdge(
	ctx context.Context,
	credentials remoteproto.HostedTunnel,
	observation *hostedEdgeObservation,
) {
	hostname, mode := credentials.Hostname, credentials.Mode
	if mode == remoteproto.HostedTunnelEntitlementOnly {
		h.publish(ctx, hostname, mode, remoteproto.HostedTunnelEntitlementOnly, "")
		return
	}
	if time.Now().Before(observation.next) {
		return
	}
	if observation.startingUntil.IsZero() {
		observation.startingUntil = time.Now().Add(hostedEdgeStartup)
	}
	target := &url.URL{Scheme: "https", Host: hostname}
	descriptor, err := remoteproto.FetchDescriptor(ctx, target, h.edge)
	if ctx.Err() != nil {
		return
	}
	now := time.Now()
	observation.next = now.Add(hostedEdgeInterval)
	switch {
	case err == nil && descriptor.ComputerID == h.identity.ComputerID &&
		descriptor.TLSRootSHA256 == h.identity.TLSRootSHA256 && descriptor.BrowserSignIn:
		observation.missed = false
		h.publish(ctx, hostname, mode, remoteproto.HostedRouteReady, "")
	case errors.Is(err, remoteproto.ErrDescriptorPlanRequired):
		observation.missed = false
		h.publish(ctx, hostname, mode, remoteproto.HostedTunnelEntitlementOnly, "")
	case errors.Is(err, remoteproto.ErrDescriptorTLS):
		observation.next = now.Add(hostedEdgeRetry)
		h.publish(ctx, hostname, mode, hostedStateRetrying, remoteproto.HostedTLSFailureMessage)
	default:
		observation.next = now.Add(hostedEdgeRetry)
		state := h.view.Load().status.State
		// Give DNS and the edge time to publish the route, and confirm an outage before withdrawing ingress.
		if state == hostedStateStarting && now.Before(observation.startingUntil) {
			return
		}
		if state == remoteproto.HostedRouteReady && !observation.missed {
			observation.missed = true
			return
		}
		h.publish(
			ctx,
			hostname,
			mode,
			hostedStateRetrying,
			"Hosted route did not return this computer's descriptor over HTTPS.",
		)
	}
}

func (h *hostedRun) publish(ctx context.Context, hostname, mode, state, diagnosis string) {
	before := h.view.Load()
	if before.mode == mode && before.status.State == state && before.status.Diagnosis == diagnosis &&
		hostnameValue(before) == hostname {
		return
	}
	status := HostedStatus{Enabled: true, State: state, Diagnosis: diagnosis}
	if hostname != "" {
		status.Hostname = &hostname
	}
	h.view.Store(new(hostedView{status: status, mode: mode}))
	h.onChange(ctx)
}

func hostnameValue(view *hostedView) string {
	if view != nil && view.status.Hostname != nil {
		return *view.status.Hostname
	}
	return ""
}

func validHostedHostname(hostname string) bool {
	if len(hostname) > 253 || !strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return false
			}
		}
	}
	return true
}
