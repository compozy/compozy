package globaldb

import (
	"context"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func insertTaskWithExecutor(ctx context.Context, exec taskSQLExecutor, record taskpkg.Task) error {
	return sqlcgen.New(exec).InsertTask(ctx, insertTaskParams(record))
}
