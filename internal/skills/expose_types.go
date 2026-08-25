package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

// ExposureStatus reports filesystem health relative to a durable ownership record.
type ExposureStatus string

const (
	ExposureHealthy         ExposureStatus = "healthy"
	ExposureMissing         ExposureStatus = "missing"
	ExposureBroken          ExposureStatus = "broken"
	ExposureForeignConflict ExposureStatus = "foreign_conflict"
)

const (
	ExposureCodeSkillNotExposable        = "skill_not_exposable"
	ExposureCodeProfileSkillNotExposable = "profile_skill_not_exposable"
	ExposureCodeTargetDisabled           = "expose_target_disabled"
	ExposureCodeTargetInvalid            = "expose_target_invalid"
	ExposureCodeNameConflict             = "expose_name_conflict"
	ExposureCodeLinkUnsupported          = "expose_link_unsupported"
	ExposureCodeForeignLink              = "expose_foreign_link"
	ExposureCodeUnsafeSkillName          = "unsafe_skill_name"
	ExposureCodeRolledBack               = "rolled_back"
	ExposureCodeCleanupFailed            = "exposure_cleanup_failed"
	ExposureCodeSkillRemoveBlocked       = "skill_remove_blocked"
)

// ExposureRecord is the durable ownership proof for one link.
type ExposureRecord = store.SkillExposureRecord

// ExposureState combines durable ownership with live filesystem health.
type ExposureState struct {
	Record ExposureRecord
	Status ExposureStatus
}

// TargetResult reports one target's independent operation result.
type TargetResult struct {
	Target     string
	OK         bool
	Exposure   *ExposureState
	Err        error
	RolledBack bool
	CleanupErr error
}

// ExposureError carries one deterministic public error code.
type ExposureError struct {
	Code      string
	Target    string
	Path      string
	Message   string
	Retryable bool
	Cause     error
}

var _ error = (*ExposureError)(nil)

func (e *ExposureError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Message
	}
	if e.Message == "" {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *ExposureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ExposureBatchError reports whether an expose operation compensated completed targets.
type ExposureBatchError struct {
	RolledBack bool
	Cause      error
}

var _ error = (*ExposureBatchError)(nil)

func (e *ExposureBatchError) Error() string {
	if e == nil || e.Cause == nil {
		return "skill exposure failed"
	}
	return e.Cause.Error()
}

func (e *ExposureBatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type exposureErrorParams struct {
	code    string
	target  string
	path    string
	message string
	cause   error
}

func newExposureError(params exposureErrorParams) error {
	return &ExposureError{
		Code: params.code, Target: params.target, Path: params.path,
		Message: params.message, Cause: params.cause,
	}
}

type exposureFS interface {
	Lstat(string) (fs.FileInfo, error)
	Stat(string) (fs.FileInfo, error)
	EvalSymlinks(string) (string, error)
	Readlink(string) (string, error)
	Symlink(string, string) error
	Mkdir(string, fs.FileMode) error
	Remove(string) error
	ReadDir(string) ([]os.DirEntry, error)
}

type osExposureFS struct{}

func (osExposureFS) Lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }
func (osExposureFS) Stat(path string) (fs.FileInfo, error)  { return os.Stat(path) }
func (osExposureFS) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
func (osExposureFS) Readlink(path string) (string, error) { return os.Readlink(path) }
func (osExposureFS) Symlink(target string, path string) error {
	return os.Symlink(target, path)
}
func (osExposureFS) Mkdir(path string, mode fs.FileMode) error { return os.Mkdir(path, mode) }
func (osExposureFS) Remove(path string) error                  { return os.Remove(path) }
func (osExposureFS) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func exposureErrorCode(err error) string {
	if err == nil {
		return ""
	}
	typed := &ExposureError{}
	if errors.As(err, &typed) {
		return typed.Code
	}
	return "internal"
}
