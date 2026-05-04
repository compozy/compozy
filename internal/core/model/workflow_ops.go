package model

import "time"

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
	LegacyReviewMetaRemoved int
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
	Force         bool
}

type SyncResult struct {
	Target                 string
	WorkflowsScanned       int
	WorkflowsPruned        int
	MetaCreated            int
	MetaUpdated            int
	SnapshotsUpserted      int
	TaskItemsUpserted      int
	ReviewRoundsUpserted   int
	ReviewIssuesUpserted   int
	CheckpointsUpdated     int
	LegacyArtifactsRemoved int
	SyncedPaths            []string
	PrunedWorkflows        []string
	Warnings               []string
}

type ArchiveResult struct {
	Target               string            `json:"target"`
	ArchiveRoot          string            `json:"archive_root"`
	WorkflowsScanned     int               `json:"workflows_scanned"`
	Archived             int               `json:"archived"`
	Skipped              int               `json:"skipped"`
	Forced               bool              `json:"forced,omitempty"`
	CompletedTasks       int               `json:"completed_tasks,omitempty"`
	ResolvedReviewIssues int               `json:"resolved_review_issues,omitempty"`
	ArchivedAt           *time.Time        `json:"archived_at,omitempty"`
	ArchivedPaths        []string          `json:"archived_paths,omitempty"`
	SkippedPaths         []string          `json:"skipped_paths,omitempty"`
	SkippedReasons       map[string]string `json:"skipped_reasons,omitempty"`
}
