package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/memory/consolidation"

	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/sandbox/daytona"
	"github.com/compozy/agh/internal/sandbox/local"
	"github.com/compozy/agh/internal/session"
	sessionledger "github.com/compozy/agh/internal/sessions/ledger"

	"github.com/compozy/agh/internal/skills"

	"github.com/compozy/agh/internal/store"

	"github.com/compozy/agh/internal/toolruntime"

	"github.com/compozy/agh/internal/vault"
)

func (d *Daemon) bootSessionRepair(ctx context.Context, state *bootState) error {
	if state == nil {
		return errors.New("daemon: boot session repair state is required")
	}
	if state.sessions == nil {
		return errors.New("daemon: boot session repair requires session manager")
	}

	infos, err := state.sessions.ListAll(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("daemon: boot session repair canceled: %w", ctxErr)
		}
		state.logger.Warn("daemon: boot session repair skipped session list", "error", err)
		return nil
	}

	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("daemon: boot session repair canceled: %w", err)
		}
		if !bootShouldRepairSession(info) {
			continue
		}

		result, repairErr := state.sessions.RepairSession(
			ctx,
			session.RepairOpts{SessionID: info.ID},
		)
		if repairErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("daemon: boot session repair canceled: %w", ctxErr)
			}
			state.logger.Warn(
				"daemon: boot session repair failed",
				"session_id", info.ID,
				"error", repairErr,
			)
			continue
		}
		if result == nil {
			continue
		}
		errorIssues := repairIssueCount(result, session.RepairSeverityError)
		if len(result.Actions) == 0 && errorIssues == 0 {
			continue
		}
		state.logger.Info(
			"daemon: boot session repair complete",
			"session_id", result.SessionID,
			"persisted", result.Persisted,
			"actions", len(result.Actions),
			"issues", len(result.Issues),
			"error_issues", errorIssues,
		)
	}
	if recoverer, ok := state.sessions.(sessionHealthRecoverer); ok {
		result, recoveryErr := recoverer.RecoverSessionHealth(ctx)
		if recoveryErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("daemon: session health recovery canceled: %w", ctxErr)
			}
			state.logger.Warn("daemon: session health recovery failed", "error", recoveryErr)
			return nil
		}
		if result.RefreshedActive > 0 || result.Recomputed > 0 || result.MarkedStale > 0 {
			state.logger.Info(
				"daemon: session health recovery complete",
				"refreshed_active", result.RefreshedActive,
				"recomputed", result.Recomputed,
				"marked_stale", result.MarkedStale,
			)
		}
	}
	return nil
}

type sessionHealthRecoverer interface {
	RecoverSessionHealth(ctx context.Context) (session.HealthRecoveryResult, error)
}

func bootShouldRepairSession(info *session.Info) bool {
	if info == nil || strings.TrimSpace(info.ID) == "" {
		return false
	}
	if info.State != session.StateStopped {
		return false
	}
	switch info.StopReason {
	case store.StopAgentCrashed, store.StopError:
		return true
	default:
		return false
	}
}

func repairIssueCount(result *session.RepairResult, severity string) int {
	if result == nil {
		return 0
	}
	count := 0
	for _, issue := range result.Issues {
		if strings.TrimSpace(issue.Severity) == severity {
			count++
		}
	}
	return count
}

func sessionInputQueueStoreDependency(registry Registry) store.SessionInputQueueStore {
	queueStore, ok := registry.(store.SessionInputQueueStore)
	if !ok {
		return nil
	}
	return queueStore
}

func (d *Daemon) newSessionLedgerMaterializer(
	state *bootState,
) (session.LedgerMaterializer, error) {
	if state == nil || !state.cfg.Memory.Enabled {
		return nil, nil
	}
	root := strings.TrimSpace(state.cfg.Memory.Session.LedgerRoot)
	if root == "" {
		root = d.homePaths.SessionsDir
	}
	materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
		RootDir:          root,
		UnboundPartition: state.cfg.Memory.Session.UnboundPartition,
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: create session ledger materializer: %w", err)
	}
	return materializer, nil
}

func (d *Daemon) buildProviderVault(state *bootState) (*vault.Service, error) {
	if state == nil || state.registry == nil {
		return nil, errors.New("daemon: provider vault registry is required")
	}
	vaultStore, ok := state.registry.(vault.Store)
	if !ok {
		if state.logger != nil {
			state.logger.Warn(
				"daemon.provider_vault.disabled",
				"reason",
				"registry_missing_vault_store",
				"registry_type",
				fmt.Sprintf("%T", state.registry),
			)
		}
		return nil, nil
	}
	lookupEnv := func(key string) (string, bool) {
		value := d.getenv(key)
		return value, strings.TrimSpace(value) != ""
	}
	service, err := vault.NewService(
		vaultStore,
		vault.NewFileKeyProvider(d.homePaths.HomeDir, lookupEnv),
		vault.WithLookupEnv(lookupEnv),
		vault.WithNow(d.now),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create provider vault: %w", err)
	}
	return service, nil
}

func sessionProviderVaultDependency(service *vault.Service) session.ProviderSecretResolver {
	if service == nil {
		return nil
	}
	return service
}

func (d *Daemon) bootProcessRegistry(ctx context.Context, state *bootState) error {
	if state == nil {
		return errors.New("daemon: process registry state is required")
	}
	var store toolruntime.Store
	if processStore, ok := state.registry.(toolruntime.Store); ok {
		store = processStore
	}
	state.processRegistry = toolruntime.NewRegistry(
		store,
		toolruntime.WithLogger(state.logger),
		toolruntime.WithDaemonPID(d.pid()),
		toolruntime.WithNow(d.now),
	)
	report, err := state.processRegistry.ReconcileBoot(ctx)
	if err != nil {
		return fmt.Errorf("daemon: reconcile tool process registry: %w", err)
	}
	if report.Checked > 0 && state.logger != nil {
		state.logger.Info(
			"daemon: reconciled tool process registry",
			"checked", report.Checked,
			"recovered", report.Recovered,
			"stale", report.Stale,
		)
	}
	return nil
}

func (d *Daemon) buildSandboxRegistry(state *bootState) (*sandbox.Registry, error) {
	if state == nil {
		return nil, errors.New("daemon: sandbox registry state is required")
	}
	registry, err := local.NewRegistry(
		local.WithLogger(state.logger),
		local.WithProcessRegistry(state.processRegistry),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create sandbox registry: %w", err)
	}
	if err := registry.Register(daytona.NewProvider(
		daytona.WithLogger(state.logger),
		daytona.WithProcessRegistry(state.processRegistry),
	)); err != nil {
		return nil, fmt.Errorf("daemon: register daytona sandbox provider: %w", err)
	}
	return registry, nil
}

func (d *Daemon) sessionNotifier(state *bootState) session.Notifier {
	if state == nil {
		return nil
	}

	notifier := session.Notifier(state.notifier)
	if state.bridges != nil {
		notifier = extensionpkg.NewBridgeDeliveryNotifier(state.bridges.Broker(), state.notifier)
	}
	return notifier
}

func skillRegistryDependency(registry *skills.Registry) session.SkillRegistry {
	if registry == nil {
		return nil
	}
	return registry
}

func mcpResolverDependency(resolver *skills.MCPResolver) session.MCPResolver {
	if resolver == nil {
		return nil
	}
	return resolver
}

func dreamTriggerFromRuntime(runtime *consolidation.Runtime) DreamTrigger {
	if runtime == nil {
		return nil
	}
	return runtime
}
