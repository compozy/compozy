package workspace

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/compozy/agh/internal/fileutil"
	"github.com/oklog/ulid"
)

const (
	workspaceIdentityFileName = "workspace.toml"
	workspaceIdentityFilePerm = 0o644
	workspaceIdentityDirPerm  = 0o755
)

var workspaceIDPattern = regexp.MustCompile(`^[0-9A-HJ-KMNP-TV-Z]{26}$`)

// Identity is the stable workspace identity stored in <workspace>/.agh/workspace.toml.
type Identity struct {
	WorkspaceID        string
	CreatedAt          time.Time
	RealpathAtCreation string
	Path               string
}

type identityFile struct {
	WorkspaceID        string `toml:"workspace_id"`
	CreatedAt          string `toml:"created_at"`
	RealpathAtCreation string `toml:"realpath_at_creation"`
}

// NewWorkspaceID returns a ULID formatted for durable workspace identity.
func NewWorkspaceID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader).String()
}

// IsWorkspaceID reports whether value is a canonical workspace ULID.
func IsWorkspaceID(value string) bool {
	return workspaceIDPattern.MatchString(strings.TrimSpace(value))
}

// EnsureIdentity loads or creates <workspace>/.agh/workspace.toml.
func EnsureIdentity(ctx context.Context, rootDir string) (Identity, error) {
	return ensureIdentity(ctx, rootDir, time.Now, NewWorkspaceID)
}

func ensureIdentity(
	ctx context.Context,
	rootDir string,
	now func() time.Time,
	idGenerator func() string,
) (Identity, error) {
	if err := checkContext(ctx); err != nil {
		return Identity{}, err
	}
	if now == nil {
		now = time.Now
	}
	if idGenerator == nil {
		idGenerator = NewWorkspaceID
	}

	root, err := canonicalRoot(rootDir)
	if err != nil {
		return Identity{}, err
	}
	path := identityPath(root)
	identity, err := loadIdentityFile(path)
	switch {
	case err == nil:
		identity.Path = path
		return identity, nil
	case errors.Is(err, os.ErrNotExist):
		return createIdentityFile(ctx, root, path, now, idGenerator)
	case errors.Is(err, ErrWorkspaceIdentityInvalid), errors.Is(err, ErrWorkspaceIdentityPermissionDenied):
		return Identity{}, err
	default:
		return Identity{}, fmt.Errorf("workspace: load identity %q: %w", path, err)
	}
}

func identityPath(rootDir string) string {
	return filepath.Join(rootDir, ".agh", workspaceIdentityFileName)
}

func loadIdentityFile(path string) (Identity, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsPermission(err) {
			return Identity{}, fmt.Errorf(
				"workspace: read identity %q: %w",
				path,
				ErrWorkspaceIdentityPermissionDenied,
			)
		}
		return Identity{}, err
	}

	var parsed identityFile
	if _, err := toml.Decode(string(content), &parsed); err != nil {
		return Identity{}, fmt.Errorf(
			"workspace: parse identity %q: %w: %v",
			path,
			ErrWorkspaceIdentityInvalid,
			err,
		)
	}
	workspaceID := strings.TrimSpace(parsed.WorkspaceID)
	if !IsWorkspaceID(workspaceID) {
		return Identity{}, fmt.Errorf(
			"workspace: identity %q has invalid workspace_id %q: %w",
			path,
			workspaceID,
			ErrWorkspaceIdentityInvalid,
		)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(parsed.CreatedAt))
	if err != nil {
		return Identity{}, fmt.Errorf(
			"workspace: identity %q has invalid created_at %q: %w",
			path,
			parsed.CreatedAt,
			ErrWorkspaceIdentityInvalid,
		)
	}
	realpath := strings.TrimSpace(parsed.RealpathAtCreation)
	if realpath == "" {
		return Identity{}, fmt.Errorf(
			"workspace: identity %q missing realpath_at_creation: %w",
			path,
			ErrWorkspaceIdentityInvalid,
		)
	}
	return Identity{
		WorkspaceID:        workspaceID,
		CreatedAt:          createdAt.UTC(),
		RealpathAtCreation: realpath,
		Path:               path,
	}, nil
}

func createIdentityFile(
	ctx context.Context,
	rootDir string,
	path string,
	now func() time.Time,
	idGenerator func() string,
) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	createdAt := now().UTC()
	workspaceID := strings.TrimSpace(idGenerator())
	if !IsWorkspaceID(workspaceID) {
		return Identity{}, fmt.Errorf(
			"workspace: generated invalid workspace_id %q: %w",
			workspaceID,
			ErrWorkspaceIdentityInvalid,
		)
	}
	if err := os.MkdirAll(filepath.Dir(path), workspaceIdentityDirPerm); err != nil {
		if os.IsPermission(err) {
			return Identity{}, fmt.Errorf(
				"workspace: create identity directory %q: %w",
				filepath.Dir(path),
				ErrWorkspaceIdentityPermissionDenied,
			)
		}
		return Identity{}, fmt.Errorf("workspace: create identity directory %q: %w", filepath.Dir(path), err)
	}
	content := fmt.Appendf(
		nil,
		"workspace_id = %q\ncreated_at = %q\nrealpath_at_creation = %q\n",
		workspaceID,
		createdAt.Format(time.RFC3339Nano),
		rootDir,
	)
	if err := fileutil.AtomicWriteFile(path, content, workspaceIdentityFilePerm); err != nil {
		if os.IsPermission(err) {
			return Identity{}, fmt.Errorf(
				"workspace: write identity %q: %w",
				path,
				ErrWorkspaceIdentityPermissionDenied,
			)
		}
		return Identity{}, fmt.Errorf("workspace: write identity %q: %w", path, err)
	}
	return Identity{
		WorkspaceID:        workspaceID,
		CreatedAt:          createdAt,
		RealpathAtCreation: rootDir,
		Path:               path,
	}, nil
}
