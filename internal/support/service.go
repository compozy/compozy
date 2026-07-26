package support

import (
	"context"

	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strings"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"

	"github.com/google/uuid"
)

const (
	bundlesDirName              = "support-bundles"
	manifestSchemaVersion       = "support-bundle.v1"
	redactionVersion            = "diagnostics-redaction.v1"
	defaultBundleMaxBytes       = int64(25 << 20)
	defaultArtifactMaxBytes     = int64(1 << 20)
	defaultLogTailMaxBytes      = int64(2 << 20)
	defaultEventSummaryMaxBytes = int64(5 << 20)
	defaultOperationRetention   = 24 * time.Hour
	truncatedJSONArtifact       = `{"truncated":true,"reason":"artifact exceeded byte cap"}` + "\n"
)

var (
	ErrOperationNotFound = errors.New("support: bundle operation not found")
	ErrOperationNotReady = errors.New("support: bundle operation is not ready for download")
)

type OperationStatus string

const (
	OperationPending   OperationStatus = "pending"
	OperationRunning   OperationStatus = "running"
	OperationCompleted OperationStatus = "completed"
	OperationFailed    OperationStatus = "failed"
)

type CreateRequest struct {
	IncludeStatus bool
}

type Operation struct {
	OperationID   string
	Status        OperationStatus
	FileName      string
	FilePath      string
	SizeBytes     int64
	Manifest      *Manifest
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   *time.Time
}

type Manifest struct {
	SchemaVersion        string             `json:"schema_version"`
	OperationID          string             `json:"operation_id"`
	CreatedAt            time.Time          `json:"created_at"`
	BundleMaxBytes       int64              `json:"bundle_max_bytes"`
	ArtifactMaxBytes     int64              `json:"artifact_max_bytes"`
	LogTailMaxBytes      int64              `json:"log_tail_max_bytes"`
	EventSummaryMaxBytes int64              `json:"event_summary_max_bytes"`
	RedactionVersion     string             `json:"redaction_version"`
	Artifacts            []ManifestArtifact `json:"artifacts"`
}

type ManifestArtifact struct {
	Path             string `json:"path"`
	Included         bool   `json:"included"`
	OmittedReason    string `json:"omitted_reason,omitempty"`
	Bytes            int64  `json:"bytes"`
	Truncated        bool   `json:"truncated"`
	RedactionVersion string `json:"redaction_version,omitempty"`
}

type SnapshotFunc func(context.Context) (any, error)

type ConfigSnapshotFunc func(context.Context) (aghconfig.Config, error)

type Sources struct {
	Status             SnapshotFunc
	Doctor             SnapshotFunc
	Providers          SnapshotFunc
	ConfigApplyRecords SnapshotFunc
	EventSummaries     SnapshotFunc
	Sessions           SnapshotFunc
}

type Builder struct {
	HomePaths            aghconfig.HomePaths
	Config               aghconfig.Config
	ConfigSnapshot       ConfigSnapshotFunc
	Sources              Sources
	Now                  func() time.Time
	BundleMaxBytes       int64
	ArtifactMaxBytes     int64
	LogTailMaxBytes      int64
	EventSummaryMaxBytes int64
}

type Service struct {
	builder   *Builder
	store     *operationStore
	now       func() time.Time
	retention time.Duration
}

func NewService(builder *Builder) *Service {
	if builder == nil {
		builder = &Builder{}
	}
	now := builder.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	builder.Now = now
	return &Service{
		builder:   builder,
		store:     newOperationStore(now),
		now:       now,
		retention: defaultOperationRetention,
	}
}

func BundlesDir(paths aghconfig.HomePaths) string {
	return filepath.Join(paths.HomeDir, bundlesDirName)
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Operation, error) {
	if s == nil {
		return Operation{}, errors.New("support: service is required")
	}
	s.store.cleanup(s.retention)
	operationID := uuid.NewString()
	op := s.store.create(operationID)
	runCtx, cancel := detachedContext(ctx)
	go func() {
		defer cancel()
		s.run(runCtx, operationID, req)
	}()
	return op, nil
}

func (s *Service) Get(_ context.Context, operationID string) (Operation, error) {
	if s == nil {
		return Operation{}, errors.New("support: service is required")
	}
	return s.store.get(operationID)
}

func (s *Service) DownloadPath(ctx context.Context, operationID string) (Operation, string, error) {
	op, err := s.Get(ctx, operationID)
	if err != nil {
		return Operation{}, "", err
	}
	if op.Status != OperationCompleted {
		return Operation{}, "", ErrOperationNotReady
	}
	if strings.TrimSpace(op.FilePath) == "" {
		return Operation{}, "", ErrOperationNotReady
	}
	if _, err := os.Stat(op.FilePath); err != nil {
		return Operation{}, "", fmt.Errorf("support: stat bundle %q: %w", op.FilePath, err)
	}
	return op, op.FilePath, nil
}

func (s *Service) run(ctx context.Context, operationID string, req CreateRequest) {
	s.store.markRunning(operationID)
	result, err := s.builder.Build(ctx, operationID, req)
	if err != nil {
		s.store.markFailed(operationID, diagnostics.RedactAndBound(err.Error(), 4096))
		return
	}
	s.store.markCompleted(operationID, result)
}
