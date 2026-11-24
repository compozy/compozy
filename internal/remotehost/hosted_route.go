package remotehost

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	hostedStateStarting = "starting"
	hostedStateRetrying = "retrying"
	hostedStateReady    = "ready"
	hostedRefresh       = time.Minute
	hostedStartTimeout  = 35 * time.Second
	hostedProbeInterval = 250 * time.Millisecond
)

type hostedView struct {
	status HostedStatus
	mode   string
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
		onChange: run.syncIngress,
	}
	hosted.view.Store(new(hostedView{status: HostedStatus{Enabled: true, State: hostedStateStarting}}))
	run.hosted.Store(hosted)
	run.workers.Go(func() {
		defer close(hosted.done)
		hosted.supervise(ctx, initial)
	})
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
		if cloudclient.FailureCode(err) == "plan_required" {
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
			everReady = everReady || ready
			if err == nil && ready {
				readyDeadline = nil
				state := hostedStateReady
				if credentials.Mode == remoteproto.HostedTunnelEntitlementOnly {
					state = remoteproto.HostedTunnelEntitlementOnly
				}
				h.publish(ctx, credentials.Hostname, credentials.Mode, state, "")
			} else {
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
			if cloudclient.FailureCode(err) == "plan_required" || cloudclient.FailureCode(err) == "computer_not_owned" ||
				cloudclient.FailureCode(err) == "account_suspended" {
				process.stop(h.logger)
				return everReady, "", err
			}
			continue
		}
		if next.Hostname != credentials.Hostname || next.ConnectorToken != credentials.ConnectorToken {
			process.stop(h.logger)
			return everReady, "", errors.New("hosted connector credentials changed")
		}
		credentials = next
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
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
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
