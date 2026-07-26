package globaldb

import (
	"context"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func insertTaskWithExecutor(ctx context.Context, exec taskSQLExecutor, record taskpkg.Task) error {
	return sqlcgen.New(exec).InsertTask(ctx, insertTaskParams(record))
}
