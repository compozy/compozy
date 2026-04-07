package model

type FetchResult struct {
	Name       string
	Provider   string
	PR         string
	Round      int
	ReviewsDir string
	Total      int
}

type MigrationConfig struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
	ReviewsDir    string
	DryRun        bool
}

type MigrationResult struct {
	Target                  string
	DryRun                  bool
	FilesScanned            int
	FilesMigrated           int
	V1ToV2Migrated          int
	FilesAlreadyFrontmatter int
	FilesSkipped            int
	FilesInvalid            int
	MigratedPaths           []string
	UnmappedTypeFiles       []string
	InvalidPaths            []string
}

type SyncConfig struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
}

type ArchiveConfig struct {
	WorkspaceRoot string
	RootDir       string
	Name          string
	TasksDir      string
}

type SyncResult struct {
	Target           string
	WorkflowsScanned int
	MetaCreated      int
	MetaUpdated      int
	SyncedPaths      []string
}

type ArchiveResult struct {
	Target           string
	ArchiveRoot      string
	WorkflowsScanned int
	Archived         int
	Skipped          int
	ArchivedPaths    []string
	SkippedReasons   map[string]string
}
