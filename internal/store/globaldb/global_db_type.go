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
	*BundleRepo
	*PermissionRepo
	*SessionRepo
	*TaskRepo
	*TaskRunRepo
	*AutomationRepo
	*BridgeRepo
	*NetworkRepo
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
	*WatchEventsRepo
	*DeadEntityRepo
	*ApprovalGrantRepo

	db     *sql.DB
	path   string
	now    func() time.Time
	closed atomic.Int32
}
