package daemon

import (
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/controller"
)

func (d *Daemon) configureMemoryController(state *bootState, sessions SessionManager) {
	if d == nil || state == nil || state.memoryStore == nil {
		return
	}
	invoker, ok := sessions.(transientModelInvoker)
	if !ok {
		if state.logger != nil {
			state.logger.Warn(
				"daemon: memory controller tiebreaker skipped; session manager lacks transient model invoker",
			)
		}
		return
	}
	tiebreaker := &daemonMemoryControllerTiebreaker{
		invoker: invoker,
		roles:   roleResolverForState(state),
		configSnapshot: func() aghconfig.Config {
			d.mu.Lock()
			defer d.mu.Unlock()
			return state.cfg
		},
		workspaceResolver: state.workspaceResolver,
		globalCWD:         d.homePaths.HomeDir,
	}
	state.memoryStore.SetDecisionControllerFactory(func(index controller.TargetIndex) memcontract.Controller {
		d.mu.Lock()
		cfg := state.cfg
		d.mu.Unlock()
		options := make([]controller.Option, 0, 2)
		if !strings.EqualFold(strings.TrimSpace(cfg.Memory.Controller.Mode), "rules") {
			options = append(options, controller.WithTiebreaker(tiebreaker))
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Memory.Controller.DefaultOpOnFail), "reject") {
			options = append(options, controller.WithDefaultOpOnFail(memcontract.OpReject))
		}
		return controller.New(index, options...)
	})
}
