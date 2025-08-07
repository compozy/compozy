package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/gofrs/flock"
	"github.com/spf13/afero"
)

const (
	// StateSchemaVersion defines the current schema version for state files
	StateSchemaVersion = "1.0.0"
	// StateFilePermissions defines the permissions for state files
	StateFilePermissions = 0600
	// StateDirPermissions defines the permissions for state directory
	StateDirPermissions = 0700
	// LockTimeout defines the maximum time to wait for a lock
	LockTimeout = 30 * time.Second
	// LockRetryInterval defines the interval between lock retry attempts
	LockRetryInterval = 100 * time.Millisecond
)

// StateRepository defines the interface for managing rollback state
type StateRepository interface {
	Save(ctx context.Context, state *domain.RollbackState) error
	Load(ctx context.Context, sessionID string) (*domain.RollbackState, error)
	LoadLatest(ctx context.Context) (*domain.RollbackState, error)
	Delete(ctx context.Context, sessionID string) error
	Exists(ctx context.Context, sessionID string) (bool, error)
}

// StateMetadata contains metadata about the state file
type StateMetadata struct {
	SchemaVersion string    `json:"schema_version"`
	Checksum      string    `json:"checksum"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StateWrapper wraps the state with metadata
type StateWrapper struct {
	Metadata StateMetadata         `json:"metadata"`
	State    *domain.RollbackState `json:"state"`
}

// JSONStateRepository implements StateRepository using JSON file storage
type JSONStateRepository struct {
	fs       afero.Fs
	stateDir string
	mu       sync.RWMutex
}

// NewJSONStateRepository creates a new JSON-based state repository
func NewJSONStateRepository(fs afero.Fs, stateDir string) StateRepository {
	if stateDir == "" {
		stateDir = ".release-state"
	}
	return &JSONStateRepository{
		fs:       fs,
		stateDir: stateDir,
	}
}

// Save persists the rollback state to a JSON file with proper locking
func (r *JSONStateRepository) Save(ctx context.Context, state *domain.RollbackState) error {
	if err := r.ensureStateDir(); err != nil {
		return fmt.Errorf("failed to ensure state directory: %w", err)
	}
	filename := r.getStateFilename(state.SessionID)
	lockFile := r.getLockFilename(state.SessionID)
	// Acquire file lock with timeout
	lock := flock.New(lockFile)
	lockCtx, cancel := context.WithTimeout(ctx, LockTimeout)
	defer cancel()
	locked, err := r.acquireLockWithContext(lockCtx, lock)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire lock within timeout")
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			// Log error but don't fail the operation
			fmt.Fprintf(os.Stderr, "warning: failed to unlock file: %v\n", unlockErr)
		}
	}()
	// Create state wrapper with metadata
	wrapper := StateWrapper{
		Metadata: StateMetadata{
			SchemaVersion: StateSchemaVersion,
			CreatedAt:     state.StartedAt,
			UpdatedAt:     time.Now(),
		},
		State: state,
	}
	// Calculate checksum before saving
	stateData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state for checksum: %w", err)
	}
	wrapper.Metadata.Checksum = r.calculateChecksum(stateData)
	// Marshal wrapper
	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state wrapper: %w", err)
	}
	// Write atomically using temp file
	tempFile := filename + ".tmp"
	if err := afero.WriteFile(r.fs, tempFile, data, StateFilePermissions); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	// Rename atomically
	if err := r.fs.Rename(tempFile, filename); err != nil {
		// Clean up temp file on error
		if removeErr := r.fs.Remove(tempFile); removeErr != nil {
			// Log but don't fail - temp file cleanup is best effort
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp file: %v\n", removeErr)
		}
		return fmt.Errorf("failed to rename state file: %w", err)
	}
	// Update latest link
	if err := r.updateLatestLink(filename); err != nil {
		return fmt.Errorf("failed to update latest link: %w", err)
	}
	return nil
}

// Load retrieves a specific rollback state by session ID with validation
func (r *JSONStateRepository) Load(ctx context.Context, sessionID string) (*domain.RollbackState, error) {
	filename := r.getStateFilename(sessionID)
	lockFile := r.getLockFilename(sessionID)
	// Acquire shared lock for reading
	lock := flock.New(lockFile)
	lockCtx, cancel := context.WithTimeout(ctx, LockTimeout)
	defer cancel()
	locked, err := r.acquireSharedLockWithContext(lockCtx, lock)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire shared lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("could not acquire shared lock within timeout")
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			// Log error but don't fail the operation
			fmt.Fprintf(os.Stderr, "warning: failed to unlock file: %v\n", unlockErr)
		}
	}()
	// Read state file
	data, err := afero.ReadFile(r.fs, filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("state not found for session %s", sessionID)
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	// Unmarshal and validate
	var wrapper StateWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state wrapper: %w", err)
	}
	// Validate schema version
	if wrapper.Metadata.SchemaVersion != StateSchemaVersion {
		return nil, fmt.Errorf("incompatible schema version: expected %s, got %s",
			StateSchemaVersion, wrapper.Metadata.SchemaVersion)
	}
	// Validate checksum
	stateData, err := json.Marshal(wrapper.State)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state for checksum validation: %w", err)
	}
	expectedChecksum := r.calculateChecksum(stateData)
	if wrapper.Metadata.Checksum != expectedChecksum {
		return nil, fmt.Errorf("state checksum mismatch: data may be corrupted")
	}
	return wrapper.State, nil
}

// LoadLatest retrieves the most recent rollback state with validation
func (r *JSONStateRepository) LoadLatest(ctx context.Context) (*domain.RollbackState, error) {
	latestLink := r.getLatestLink()
	// Read the latest link with lock
	r.mu.RLock()
	defer r.mu.RUnlock()
	data, err := afero.ReadFile(r.fs, latestLink)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no latest state found")
		}
		return nil, fmt.Errorf("failed to read latest link: %w", err)
	}
	// Extract session ID from filename
	targetFile := string(data)
	sessionID := r.extractSessionID(targetFile)
	if sessionID == "" {
		return nil, fmt.Errorf("invalid latest link target: %s", targetFile)
	}
	// Load the state using the session ID
	return r.Load(ctx, sessionID)
}

// Delete removes a rollback state
func (r *JSONStateRepository) Delete(ctx context.Context, sessionID string) error {
	filename := r.getStateFilename(sessionID)
	lockFile := r.getLockFilename(sessionID)
	// Acquire exclusive lock for deletion
	lock := flock.New(lockFile)
	lockCtx, cancel := context.WithTimeout(ctx, LockTimeout)
	defer cancel()
	locked, err := r.acquireLockWithContext(lockCtx, lock)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for deletion: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire lock within timeout")
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			// Log error but don't fail the operation
			fmt.Fprintf(os.Stderr, "warning: failed to unlock file: %v\n", unlockErr)
		}
	}()
	// Remove state file
	if err := r.fs.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	// Clean up lock file
	if removeErr := r.fs.Remove(lockFile); removeErr != nil && !os.IsNotExist(removeErr) {
		// Log but don't fail - lock file cleanup is best effort
		fmt.Fprintf(os.Stderr, "warning: failed to remove lock file: %v\n", removeErr)
	}
	return nil
}

// Exists checks if a rollback state exists
func (r *JSONStateRepository) Exists(_ context.Context, sessionID string) (bool, error) {
	filename := r.getStateFilename(sessionID)
	_, err := r.fs.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check state file: %w", err)
	}
	return true, nil
}

// acquireLockWithContext attempts to acquire an exclusive lock with context support
func (r *JSONStateRepository) acquireLockWithContext(ctx context.Context, lock *flock.Flock) (bool, error) {
	ticker := time.NewTicker(LockRetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			locked, err := lock.TryLock()
			if err != nil {
				return false, err
			}
			if locked {
				return true, nil
			}
		}
	}
}

// acquireSharedLockWithContext attempts to acquire a shared lock with context support
func (r *JSONStateRepository) acquireSharedLockWithContext(ctx context.Context, lock *flock.Flock) (bool, error) {
	ticker := time.NewTicker(LockRetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			locked, err := lock.TryRLock()
			if err != nil {
				return false, err
			}
			if locked {
				return true, nil
			}
		}
	}
}

// calculateChecksum calculates SHA-256 checksum of data
func (r *JSONStateRepository) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ensureStateDir creates the state directory if it doesn't exist
func (r *JSONStateRepository) ensureStateDir() error {
	return r.fs.MkdirAll(r.stateDir, StateDirPermissions)
}

// getStateFilename returns the filename for a given session ID
func (r *JSONStateRepository) getStateFilename(sessionID string) string {
	return filepath.Join(r.stateDir, fmt.Sprintf("state-%s.json", sessionID))
}

// getLockFilename returns the lock filename for a given session ID
func (r *JSONStateRepository) getLockFilename(sessionID string) string {
	return filepath.Join(r.stateDir, fmt.Sprintf(".state-%s.lock", sessionID))
}

// getLatestLink returns the path to the latest state link
func (r *JSONStateRepository) getLatestLink() string {
	return filepath.Join(r.stateDir, "latest.txt")
}

// updateLatestLink updates the link pointing to the latest state
func (r *JSONStateRepository) updateLatestLink(target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	link := r.getLatestLink()
	tempLink := link + ".tmp"
	if err := afero.WriteFile(r.fs, tempLink, []byte(target), StateFilePermissions); err != nil {
		return fmt.Errorf("failed to write temp latest link: %w", err)
	}
	if err := r.fs.Rename(tempLink, link); err != nil {
		if removeErr := r.fs.Remove(tempLink); removeErr != nil {
			// Log but don't fail - temp link cleanup is best effort
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp link: %v\n", removeErr)
		}
		return fmt.Errorf("failed to update latest link: %w", err)
	}
	return nil
}

// extractSessionID extracts session ID from state filename
func (r *JSONStateRepository) extractSessionID(filename string) string {
	base := filepath.Base(filename)
	if len(base) > 11 && base[:6] == "state-" && base[len(base)-5:] == ".json" {
		return base[6 : len(base)-5]
	}
	return ""
}
