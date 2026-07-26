package storeschema

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlcheck"
	"ariga.io/atlas/sql/sqlclient"
	"ariga.io/atlas/sql/sqlite"
	_ "ariga.io/atlas/sql/sqlite/sqlitecheck" // Register SQLite analyzers with sqlcheck.
	"ariga.io/atlas/sql/sqltool"
	_ "modernc.org/sqlite" // Register the modernc SQLite driver with database/sql.
)

var atlasDatabaseSequence atomic.Uint64

type atlasPlan struct {
	migration *migrate.Plan
	changes   schema.Changes
}

// GenerateAtlas writes a sequential migration for each changed declarative schema and refreshes checksums.
func GenerateAtlas(ctx context.Context, root string) error {
	for _, descriptor := range streams(root) {
		if err := generateAtlasStream(ctx, descriptor); err != nil {
			return err
		}
	}
	return nil
}

// CheckAtlas validates checksums and proves every migration stream matches its declarative schema.
func CheckAtlas(ctx context.Context, root string) error {
	for _, descriptor := range streams(root) {
		if err := checkAtlasStream(ctx, descriptor); err != nil {
			return err
		}
	}
	return nil
}

func generateAtlasStream(ctx context.Context, descriptor stream) (err error) {
	dir, err := sqltool.NewGooseDir(descriptor.migrationsDir)
	if err != nil {
		return fmt.Errorf("codegen: open %s migration directory: %w", descriptor.name, err)
	}
	plan, dev, closeDev, err := planAtlasStream(ctx, descriptor, dir)
	if err != nil {
		if errors.Is(err, migrate.ErrNoPlan) {
			return writeAtlasSum(dir, descriptor.name)
		}
		return err
	}
	defer func() {
		err = errors.Join(err, closeAtlasDatabase(closeDev, descriptor.name))
	}()
	if err := lintAtlasPlan(ctx, descriptor.name, plan, dev); err != nil {
		return err
	}
	nextVersion, err := nextMigrationVersion(dir)
	if err != nil {
		return fmt.Errorf("codegen: determine %s migration version: %w", descriptor.name, err)
	}
	plan.migration.Version = fmt.Sprintf("%05d", nextVersion)
	planner := migrate.NewPlanner(
		dev.Driver,
		dir,
		migrate.PlanFormat(sequentialGooseFormatter{}),
	)
	if err := planner.WritePlan(plan.migration); err != nil {
		return fmt.Errorf("codegen: write %s migration plan: %w", descriptor.name, err)
	}
	return nil
}

func checkAtlasStream(ctx context.Context, descriptor stream) (err error) {
	dir, err := sqltool.NewGooseDir(descriptor.migrationsDir)
	if err != nil {
		return fmt.Errorf("codegen: open %s migration directory: %w", descriptor.name, err)
	}
	if err := migrate.Validate(dir); err != nil {
		return fmt.Errorf("codegen: validate %s atlas.sum: %w", descriptor.name, err)
	}
	plan, dev, closeDev, err := planAtlasStream(ctx, descriptor, dir)
	if err != nil {
		if errors.Is(err, migrate.ErrNoPlan) {
			return nil
		}
		return err
	}
	defer func() {
		err = errors.Join(err, closeAtlasDatabase(closeDev, descriptor.name))
	}()
	if err := lintAtlasPlan(ctx, descriptor.name, plan, dev); err != nil {
		return err
	}
	return fmt.Errorf(
		"codegen: %s schema drift: declarative schema requires a migration (%s); run make codegen",
		descriptor.name,
		summarizePlan(plan.migration),
	)
}

func summarizePlan(plan *migrate.Plan) string {
	const maxStatements = 3
	statements := make([]string, 0, maxStatements)
	for _, change := range plan.Changes {
		statement := strings.Join(strings.Fields(change.Cmd), " ")
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
		if len(statements) == maxStatements {
			break
		}
	}
	return strings.Join(statements, "; ")
}

func planAtlasStream(
	ctx context.Context,
	descriptor stream,
	dir migrate.Dir,
) (_ *atlasPlan, _ *sqlclient.Client, closeDev func() error, err error) {
	dev, err := openAtlasClient(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("codegen: open %s Atlas database: %w", descriptor.name, err)
	}
	closeDev = dev.DB.Close
	desired, err := readDesiredRealm(ctx, descriptor.schemaSource)
	if err != nil {
		return nil, nil, nil, errors.Join(
			fmt.Errorf("codegen: read %s declarative schema: %w", descriptor.name, err),
			closeAtlasDatabase(closeDev, descriptor.name),
		)
	}
	executor, err := migrate.NewExecutor(dev.Driver, dir, migrate.NopRevisionReadWriter{})
	if err != nil {
		return nil, nil, nil, errors.Join(
			fmt.Errorf("codegen: create %s migration replay: %w", descriptor.name, err),
			closeAtlasDatabase(closeDev, descriptor.name),
		)
	}
	current, err := executor.Replay(ctx, migrate.RealmConn(dev.Driver, nil))
	if err != nil {
		return nil, nil, nil, errors.Join(
			fmt.Errorf("codegen: replay %s migrations: %w", descriptor.name, err),
			closeAtlasDatabase(closeDev, descriptor.name),
		)
	}
	normalizeGeneratedSQLiteIndexes(current)
	normalizeGeneratedSQLiteIndexes(desired)
	changes, err := dev.RealmDiff(current, desired, schema.DiffNormalized())
	if err != nil {
		return nil, nil, nil, errors.Join(
			fmt.Errorf("codegen: diff %s schema: %w", descriptor.name, err),
			closeAtlasDatabase(closeDev, descriptor.name),
		)
	}
	if len(changes) == 0 {
		if closeErr := closeAtlasDatabase(closeDev, descriptor.name); closeErr != nil {
			return nil, nil, nil, closeErr
		}
		return nil, nil, nil, migrate.ErrNoPlan
	}
	plan, err := dev.PlanChanges(ctx, "schema", changes, func(options *migrate.PlanOptions) {
		options.Mode = migrate.PlanModeDeferred
	})
	if err != nil {
		return nil, nil, nil, errors.Join(
			fmt.Errorf("codegen: plan %s schema migration: %w", descriptor.name, err),
			closeAtlasDatabase(closeDev, descriptor.name),
		)
	}
	return &atlasPlan{migration: plan, changes: changes}, dev, closeDev, nil
}

func closeAtlasDatabase(closeDev func() error, streamName string) error {
	if closeDev == nil {
		return nil
	}
	if err := closeDev(); err != nil {
		return fmt.Errorf("codegen: close %s Atlas database: %w", streamName, err)
	}
	return nil
}

func normalizeGeneratedSQLiteIndexes(realm *schema.Realm) {
	if realm == nil {
		return
	}
	for _, schemaObject := range realm.Schemas {
		for _, table := range schemaObject.Tables {
			for _, index := range table.Indexes {
				if !strings.HasPrefix(index.Name, "sqlite_autoindex_") || sqliteIndexOrigin(index) != "u" {
					continue
				}
				parts := make([]string, 0, len(index.Parts)+1)
				parts = append(parts, table.Name)
				for _, part := range index.Parts {
					if part.C != nil {
						parts = append(parts, part.C.Name)
					}
				}
				if len(parts) > 1 {
					index.Name = strings.Join(parts, "_")
				}
			}
		}
	}
}

func sqliteIndexOrigin(index *schema.Index) string {
	for _, attribute := range index.Attrs {
		if origin, ok := attribute.(*sqlite.IndexOrigin); ok {
			return origin.O
		}
	}
	return ""
}

func openAtlasClient(ctx context.Context) (*sqlclient.Client, error) {
	sequence := atlasDatabaseSequence.Add(1)
	dsn := fmt.Sprintf("file:agh-atlas-codegen-%d?mode=memory&cache=shared", sequence)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(
			fmt.Errorf("ping SQLite database: %w", err),
			closeAtlasDatabase(db.Close, "temporary"),
		)
	}
	driver, err := sqlite.Open(db)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("open Atlas SQLite driver: %w", err),
			closeAtlasDatabase(db.Close, "temporary"),
		)
	}
	return &sqlclient.Client{
		Name:      sqlite.DriverName,
		DB:        db,
		Driver:    driver,
		Ephemeral: true,
	}, nil
}

func readDesiredRealm(ctx context.Context, source schemaSource) (_ *schema.Realm, err error) {
	files, err := source.files()
	if err != nil {
		return nil, err
	}
	client, err := openAtlasClient(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, closeAtlasDatabase(client.DB.Close, filepath.Base(source.path)))
	}()
	for _, schemaPath := range files {
		contents, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", schemaPath, err)
		}
		if _, err := client.DB.ExecContext(ctx, string(contents)); err != nil {
			return nil, fmt.Errorf("execute %q: %w", schemaPath, err)
		}
	}
	realm, err := client.InspectRealm(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("inspect schema source %q: %w", source.path, err)
	}
	return realm, nil
}

func writeAtlasSum(dir migrate.Dir, streamName string) error {
	sum, err := dir.Checksum()
	if err != nil {
		return fmt.Errorf("codegen: calculate %s atlas.sum: %w", streamName, err)
	}
	if err := migrate.WriteSumFile(dir, sum); err != nil {
		return fmt.Errorf("codegen: write %s atlas.sum: %w", streamName, err)
	}
	return nil
}

func nextMigrationVersion(dir migrate.Dir) (int64, error) {
	files, err := dir.Files()
	if err != nil {
		return 0, err
	}
	var highest int64
	for _, file := range files {
		version, err := strconv.ParseInt(file.Version(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse version %q from %q: %w", file.Version(), file.Name(), err)
		}
		if version > highest {
			highest = version
		}
	}
	return highest + 1, nil
}

type sequentialGooseFormatter struct{}

func (sequentialGooseFormatter) Format(plan *migrate.Plan) ([]migrate.File, error) {
	files, err := sqltool.GooseFormatter.Format(plan)
	if err != nil {
		return nil, fmt.Errorf("format goose migration: %w", err)
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("format goose migration: got %d files, want 1", len(files))
	}
	contents := files[0].Bytes()
	if downIndex := bytes.Index(contents, []byte("\n-- +goose Down")); downIndex >= 0 {
		contents = append(bytes.TrimRight(contents[:downIndex], "\r\n"), '\n')
	}
	name := strings.Trim(strings.ToLower(plan.Name), " _-")
	name = strings.NewReplacer(" ", "_", "-", "_").Replace(name)
	if name == "" {
		name = "schema"
	}
	return []migrate.File{
		migrate.NewLocalFile(fmt.Sprintf("%s_%s.sql", plan.Version, name), contents),
	}, nil
}

func lintAtlasPlan(ctx context.Context, streamName string, plan *atlasPlan, dev *sqlclient.Client) error {
	changes := make([]*sqlcheck.Change, 0, len(plan.changes))
	statements := make([]string, 0, len(plan.migration.Changes))
	for _, change := range plan.migration.Changes {
		statements = append(statements, change.Cmd+";")
	}
	for index, change := range plan.changes {
		statement := strings.Join(statements, "\n")
		if index < len(plan.migration.Changes) {
			statement = plan.migration.Changes[index].Cmd
		}
		changes = append(changes, &sqlcheck.Change{
			Changes: schema.Changes{change},
			Stmt: &migrate.Stmt{
				Pos:  index + 1,
				Text: statement,
			},
		})
	}
	if len(changes) == 0 {
		return nil
	}
	reports := make([]sqlcheck.Report, 0)
	pass := &sqlcheck.Pass{
		File: &sqlcheck.File{
			File:    migrate.NewLocalFile("pending.sql", []byte(strings.Join(statements, "\n"))),
			Changes: changes,
			Sum:     plan.changes,
		},
		Dev: dev,
		Reporter: sqlcheck.ReportWriterFunc(func(report sqlcheck.Report) {
			reports = append(reports, report)
		}),
	}
	analyzers, err := sqlcheck.AnalyzerFor(sqlite.DriverName, nil)
	if err != nil {
		return fmt.Errorf("codegen: configure %s SQLite sqlcheck: %w", streamName, err)
	}
	if err := sqlcheck.Analyzers(analyzers).Analyze(ctx, pass); err != nil {
		return fmt.Errorf("codegen: lint %s migration (%s): %w", streamName, formatReports(reports), err)
	}
	return nil
}

func formatReports(reports []sqlcheck.Report) string {
	parts := make([]string, 0, len(reports))
	for _, report := range reports {
		parts = append(parts, report.Text)
	}
	if len(parts) == 0 {
		return "no report"
	}
	return strings.Join(parts, "; ")
}
