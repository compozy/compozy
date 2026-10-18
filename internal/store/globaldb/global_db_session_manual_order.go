package globaldb

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const sessionCatalogSortManual = "manual"

// Manual order groups folder members first (by folder position), then the
// workspace root. Inside each container unplaced sessions lead, newest first,
// followed by placed sessions in their pinned order.
const sessionCatalogManualFolderRankExpression = `COALESCE((SELECT f.position FROM session_placements AS p
 JOIN session_folders AS f ON f.id = p.folder_id WHERE p.session_id = sessions.id), ` +
	sessionCatalogManualRootRankLiteral + `)`

const sessionCatalogManualPositionExpression = `COALESCE((SELECT p.position FROM session_placements AS p
 WHERE p.session_id = sessions.id), -1)`

// sessionCatalogManualRootRankLiteral mirrors store.SessionManualRootRank.
const sessionCatalogManualRootRankLiteral = "1000000000"

const sessionCatalogManualOrderClause = " ORDER BY " + sessionCatalogManualFolderRankExpression +
	" ASC, " + sessionCatalogManualPositionExpression + " ASC, created_at DESC, id DESC"

func sessionCatalogManualCursorClause(position store.SessionCatalogPosition, created string) (string, []any) {
	rank := *position.ManualFolderRank
	manual := *position.ManualPosition
	clause := `(` + sessionCatalogManualFolderRankExpression + ` > ? OR (` +
		sessionCatalogManualFolderRankExpression + ` = ? AND (` +
		sessionCatalogManualPositionExpression + ` > ? OR (` +
		sessionCatalogManualPositionExpression + ` = ? AND
 (created_at < ? OR (created_at = ? AND id < ?))))))`
	return clause, []any{rank, rank, manual, manual, created, created, strings.TrimSpace(position.ID)}
}
