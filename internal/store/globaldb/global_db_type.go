package globaldb

import (
	"database/sql"
	"sync/atomic"
	"time"
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
	*BridgeRepo
	*NetworkRepo
	*GatewayRepo
	*LoopRepo
	*GoalRepo
	*HeartbeatRepo
	*SoulRepo
	*ModelCatalogRepo
	*MarketplaceRepo
	*ObserveRepo
	*NotificationRepo
	*ToolRuntimeRepo
	*VaultRepo
	*ExtensionEnvRepo
	*WatchEventsRepo
	*DeadEntityRepo
	*ApprovalGrantRepo
	*ApprovalPendingRepo
	*CmdPaletteRepo
	Worktrees *WorktreeRepo

	db     *sql.DB
	path   string
	now    func() time.Time
	closed atomic.Int32
}
