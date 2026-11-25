package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/tools"
)

// WithAccountOptions configures the installation's account service.
func WithAccountOptions(options ...account.Option) Option {
	return func(d *Daemon) { d.accountOptions = append(d.accountOptions, options...) }
}

func (d *Daemon) bootAccount(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	store, ok := state.registry.(account.Store)
	if !ok {
		return nil
	}
	cloud, err := cloudclient.New(cloudclient.WithClock(d.now))
	if err != nil {
		return fmt.Errorf("daemon: configure Compozy Cloud: %w", err)
	}
	var linkedAccount string
	if storage, ok := state.registry.(remotehost.Store); ok {
		identity, err := storage.RemoteIdentity(ctx)
		if err != nil && !errors.Is(err, remotehost.ErrMissing) {
			return fmt.Errorf("daemon: load linked installation: %w", err)
		}
		linkedAccount = identity.AccountID
	}
	options := append([]account.Option{
		account.WithLogger(
			state.logger,
		),
		account.WithLinkedAccount(linkedAccount),
		account.WithAccountToken(state.accountToken),
	}, d.accountOptions...)
	state.accountToken = ""
	service, err := account.New(ctx, store, cloud, d.homePaths.HomeDir, options...)
	if err != nil {
		return fmt.Errorf("daemon: open installation account: %w", err)
	}
	state.runtimeWorkers.account = service
	state.runtimeWorkers.cloud = cloud
	d.admission.SetSignInCheck(service.AdmitNewWork)
	cleanup.add(func(context.Context) error {
		d.admission.SetSignInCheck(nil)
		service.Close()
		return nil
	})
	return nil
}

func accountRuntimeService(state *bootState) core.AccountService {
	if state.runtimeWorkers.account == nil {
		return nil
	}
	return state.runtimeWorkers.account
}

func (d *Daemon) signInToolAdmission(scope tools.Scope, id tools.ToolID) error {
	if !scope.Operator && scope.SessionID != "" {
		switch id {
		case tools.ToolIDSessionCreate, tools.ToolIDSessionSpawn, tools.ToolIDTaskRunClaimNext:
		default:
			return nil
		}
	}
	if err := d.admission.CheckSignIn(); err != nil {
		failure := tools.NewToolError(tools.ErrorCode(account.ErrSignInRequired.Code), id,
			account.ErrSignInRequired.Message, err)
		failure.HTTPStatus = http.StatusUnauthorized
		return failure
	}
	return nil
}

func (s *bootState) admitBootWork(ctx context.Context, work func(context.Context) error) error {
	if s.runtimeWorkers.account.AdmitNewWork() != nil {
		s.signInDeferred = append(s.signInDeferred, work)
		return nil
	}
	return work(ctx)
}

func (s *bootState) startDeferredSignInWork(ctx context.Context, cleanup *bootCleanup) {
	if len(s.signInDeferred) == 0 {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	group := newOwnedWorkerGroup(cancel)
	s.runtimeWorkers.signInRecovery = group
	cleanup.add(func(ctx context.Context) error { return stopOwnedWorker(ctx, "sign-in recovery", group) })
	finish, _ := group.Begin()
	go func() {
		defer finish()
		for _, work := range s.signInDeferred {
			if s.runtimeWorkers.account.WaitForWorkAdmission(workerCtx) != nil {
				return
			}
			if err := work(workerCtx); err != nil && workerCtx.Err() == nil {
				s.logger.ErrorContext(workerCtx, "Could not resume deferred sign-in work", "error", err)
			}
		}
	}()
}
