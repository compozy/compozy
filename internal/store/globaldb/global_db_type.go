package globaldb

import (
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

// GlobalDB owns the global session index and observability database.
type GlobalDB struct {
	*WorkspaceRepo
	*AppMetadataRepo
	*PermissionRepo
	*SessionRepo
	*TaskRepo
	*TaskRunRepo
	*AutomationRepo
	*AccountRepo
	*RemoteHostRepo
	*ComputersRepo
	*IngressRepo
	*AppTokenRepo
	*MigrationNoticeRepo
	*LoopRepo
	*GoalRepo
	*HeartbeatRepo
	*SoulRepo
	*ModelCatalogRepo
	*MarketplaceRepo
	*SkillExposureRepo
	*ObserveRepo
	*NotificationRepo
	*AttentionRepo
	*ToolRuntimeRepo
	*SessionProcessRepo
	*SessionMonitorRepo
	*VaultRepo
	*ExtensionEnvRepo
	ExtensionInputs *ExtensionInputRepo
	ExtensionMCP    *ExtensionMCPRepo
	*WatchEventsRepo
	*DeadEntityRepo
	*ApprovalGrantRepo
	*ApprovalPendingRepo
	*CmdPaletteRepo
	Worktrees *WorktreeRepo

	db     *sql.DB
	readDB *sql.DB
	// readStatements shadows the promoted repoBase field so GlobalDB owns its close.
	readStatements *readStatementCache
	background     *store.BackgroundWriter
	checkpoint     *store.PassiveCheckpointer
	telemetry      *telemetrydb.Store
	healthBuffer   *sessionHealthBuffer
	summaryRetry   *eventSummaryRetry
	path           string
	now            func() time.Time
	closed         atomic.Int32
}
