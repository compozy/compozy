package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
)

var localDatabaseMigrationLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type localDatabaseOpenError struct {
	Surface string
	Path    string
	Err     error
}

func (e *localDatabaseOpenError) Error() string {
	if e == nil {
		return "cli: local database open failed"
	}
	return fmt.Sprintf("cli: open %s database %q: %v", e.Surface, e.Path, e.Err)
}

func (e *localDatabaseOpenError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *localDatabaseOpenError) cliExitCode() int {
	return agentidentity.ExitConfigInvalid
}

func (e *localDatabaseOpenError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	code, title := localDatabaseMigrationDiagnostic(e.Err)
	category, ok := diagnosticcontract.DiagnosticCodeCategory(code)
	if !ok {
		category = diagnosticcontract.CategoryDaemon
	}
	message := "The local database could not be opened."
	if e.Err != nil {
		message = e.Err.Error()
	}
	item := diagnostics.NewItem(
		"migrations."+code,
		code,
		category,
		title,
		message,
		diagnosticcontract.SeverityError,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"surface":        e.Surface,
			"offending_path": e.Path,
			"recovery_scope": "complete AGH_HOME or workspace .agh directory",
		}),
	)
	return contract.ErrorPayload{Error: diagnostics.Redact(e.Error()), Diagnostic: &item}
}

func localDatabaseMigrationDiagnostic(err error) (code string, title string) {
	switch {
	case errors.Is(err, store.ErrLegacyDatabase):
		return diagnosticcontract.CodeLegacyDatabase, "Legacy database refused"
	case errors.Is(err, store.ErrSchemaAhead):
		return diagnosticcontract.CodeSchemaAhead, "Database schema is ahead"
	default:
		return diagnosticcontract.CodeDaemonUnavailable, "Local database unavailable"
	}
}

func openLocalGlobalDatabase(
	ctx context.Context,
	path string,
	surface string,
) (*globaldb.GlobalDB, error) {
	db, err := globaldb.OpenGlobalDB(store.WithMigrationLogger(ctx, localDatabaseMigrationLogger), path)
	if err != nil {
		reportedPath := strings.TrimSpace(path)
		if canonicalPath, pathErr := filepath.EvalSymlinks(reportedPath); pathErr == nil {
			reportedPath = canonicalPath
		}
		return nil, &localDatabaseOpenError{
			Surface: strings.TrimSpace(surface),
			Path:    reportedPath,
			Err:     err,
		}
	}
	return db, nil
}
