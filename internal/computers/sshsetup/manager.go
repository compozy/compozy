package sshsetup

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/update"
)

type Options struct {
	Home, OperatorHome, SSHConfig, Version string
	ReleaseBase                            string
	ReleaseClient                          *http.Client
	Verifier                               update.BundleVerifier
	Logger                                 *slog.Logger
}

type Manager struct {
	ctx         context.Context
	stop        context.CancelFunc
	accounts    Accounts
	cloud       Cloud
	store       store.ComputerSSHSetupStore
	registry    Registry
	ssh         sshRunner
	releases    releaseSource
	forwards    *forwardPool
	pins        string
	sshDir      string
	configPath  string
	version     string
	logger      *slog.Logger
	mu          sync.Mutex
	pinMu       sync.Mutex
	runs        map[string]*setupRun
	activeHosts map[string]bool
	workers     sync.WaitGroup
}

type setupRun struct {
	ctx                context.Context
	cancel             context.CancelCauseFunc
	done               chan struct{}
	changed            chan struct{}
	linking            chan struct{}
	identity           context.Context
	account            account.Installation
	row                store.ComputerSSHSetup
	status             Status
	answer             func(Answer) error
	reservationExpires time.Time
}

func New(
	ctx context.Context,
	accounts Accounts,
	cloud Cloud,
	db store.ComputerSSHSetupStore,
	registry Registry,
	opts Options,
) *Manager {
	// #nosec G118 -- the daemon's boot cleanup owns the manager lifetime.
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	opener := openSSH{configPath: opts.SSHConfig}
	m := &Manager{
		ctx: lifetime, stop: cancel, accounts: accounts, cloud: cloud, store: db, registry: registry,
		ssh: opener, pins: filepath.Join(opts.Home, "ssh", "known_hosts"),
		sshDir: filepath.Join(opts.OperatorHome, ".ssh"), configPath: opts.SSHConfig,
		version: opts.Version, logger: cmp.Or(opts.Logger, slog.Default()),
		runs: make(map[string]*setupRun), activeHosts: make(map[string]bool),
		releases: releaseSource{
			base: cmp.Or(opts.ReleaseBase, releaseOrigin),
			http: cmp.Or(opts.ReleaseClient, &http.Client{Timeout: time.Minute}),
			verifier: cmp.Or(
				opts.Verifier,
				update.NewSigstoreBundleVerifier(filepath.Join(opts.Home, "cache", "sigstore")),
			),
		},
	}
	m.forwards = &forwardPool{ssh: opener, pins: m.pins, ask: m.forwardPrompt,
		processes: make(map[forwardKey]*forwardProcess)}
	return m
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.stop()
	m.mu.Unlock()
	m.workers.Wait()
	m.forwards.close()
}

func (m *Manager) identity() (context.Context, account.Installation, error) {
	ctx, installation := m.accounts.IdentityLifetime()
	if ctx.Err() != nil || m.ctx.Err() != nil || installation.State != account.SignedIn ||
		installation.AccountID == "" {
		return nil, account.Installation{}, account.ErrSignInRequired
	}
	return ctx, installation, nil
}

func (m *Manager) Start(ctx context.Context, req Request) (string, error) {
	identity, installation, err := m.identity()
	if err != nil {
		return "", err
	}
	t, err := m.ssh.resolve(ctx, req)
	if err != nil {
		return "", err
	}
	name := cmp.Or(strings.TrimSpace(req.Name), t.Alias)
	if name == "local" {
		return "", invalidSSHRequest()
	}
	return m.start(ctx, identity, installation, store.ComputerSSHSetup{
		HostKey: t.address(), TargetAlias: t.Alias, Name: name, Kind: KindAdd, Step: StepAdmit,
	}, req.TrustFingerprint)
}

func (m *Manager) StartUpdate(ctx context.Context, id remoteproto.ComputerID) (string, error) {
	identity, installation, err := m.identity()
	if err != nil {
		return "", err
	}
	route, err := m.registry.SSHUpdateTarget(ctx, installation.AccountID, id)
	if err != nil {
		return "", err
	}
	return m.start(ctx, identity, installation, store.ComputerSSHSetup{
		HostKey: route.Address, TargetAlias: route.SSH.Alias, Kind: KindUpdate, Step: StepVerify,
		ComputerID: string(id),
	}, "")
}

func (m *Manager) start(
	ctx, identity context.Context, installation account.Installation, input store.ComputerSSHSetup, fingerprint string,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := cmp.Or(ctx.Err(), identity.Err(), m.ctx.Err()); err != nil {
		return "", err
	}
	if m.activeHosts[input.HostKey] {
		return "", ErrInProgress
	}
	owner := installation.AccountID
	row, err := m.store.FindPendingComputerSSHSetup(ctx, owner, input.HostKey)
	if err != nil && !errors.Is(err, store.ErrComputerSSHSetupNotFound) {
		return "", err
	}
	if row.ID != "" && (input.Kind != row.Kind || (input.Kind == KindUpdate && row.ComputerID != input.ComputerID)) {
		return "", ErrInProgress
	}
	if input.Kind == KindAdd && row.ComputerID == "" {
		added, err := m.registry.HasSSHRoute(ctx, owner, input.HostKey)
		if err != nil {
			return "", err
		}
		if added {
			return "", failure(
				http.StatusConflict,
				"computer_already_added",
				input.TargetAlias+" is already in your computers.",
			)
		}
	}
	if row.ID == "" {
		row = input
		row.ID, row.AccountID, row.UpdatedAt = "setup_"+rand.Text(), owner, setupTime()
		row.ReleaseVersion = m.version
		if err := m.store.CreateComputerSSHSetup(ctx, row); err != nil {
			return "", err
		}
	}
	return m.launch(identity, installation, row, fingerprint), nil
}

func (m *Manager) launch(
	identity context.Context, installation account.Installation, row store.ComputerSSHSetup, fingerprint string,
) string {
	// #nosec G118 -- the worker owns cancel and is joined by Close or Cancel.
	ctx, cancel := context.WithCancelCause(identity)
	stop := context.AfterFunc(m.ctx, func() { cancel(errInterrupted) })
	job := &setupRun{ctx: ctx, cancel: cancel, done: make(chan struct{}), changed: make(chan struct{}),
		identity: identity, account: installation, row: row}
	m.runs[row.ID], m.activeHosts[row.HostKey] = job, true
	m.publish(job, runningStatus(row))
	m.workers.Go(func() {
		defer stop()
		defer cancel(nil)
		err := m.run(job, fingerprint)
		m.finish(job, err)
	})
	return row.ID
}

func runningStatus(row store.ComputerSSHSetup) Status {
	return Status{ID: row.ID, Step: row.Step, State: Running, ComputerID: remoteproto.ComputerID(row.ComputerID)}
}

func storedStatus(row store.ComputerSSHSetup) Status {
	out := runningStatus(row)
	switch {
	case row.Step == StepDone || row.Step == Canceled:
		out.State = row.Step
	case row.ErrorCode != "":
		out.State = Failed
		out.Error = &Failure{Code: row.ErrorCode, Message: row.ErrorDetail}
		var detail Failure
		if err := json.Unmarshal([]byte(row.ErrorDetail), &detail); err == nil && detail.Code == row.ErrorCode {
			out.Error = &detail
			out.Step = cmp.Or(detail.Step, out.Step)
		}
	default:
		out.State, out.Error = Failed, errInterrupted
	}
	return out
}

func (m *Manager) load(ctx context.Context, owner, id string) (store.ComputerSSHSetup, error) {
	row, err := m.store.GetComputerSSHSetup(ctx, owner, id)
	if errors.Is(err, store.ErrComputerSSHSetupNotFound) {
		err = ErrNotFound
	}
	return row, err
}

func (m *Manager) publish(job *setupRun, status Status) {
	job.status = status
	close(job.changed)
	job.changed = make(chan struct{})
	m.registry.PublishSSHSetup(job.account.AccountID, job.account.AuthGeneration, status)
}

func (m *Manager) persist(ctx context.Context, row *store.ComputerSSHSetup) error {
	row.UpdatedAt = setupTime()
	return m.store.WriteComputerSSHSetup(ctx, *row)
}

func setupTime() string { return store.FormatTimestamp(time.Now()) }

func (m *Manager) ensurePins() error {
	return os.MkdirAll(filepath.Dir(m.pins), 0o700)
}
