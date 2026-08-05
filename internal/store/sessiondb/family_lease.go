package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

type sessionDBFamilyLeaseEntry struct {
	token chan struct{}
	refs  int
}

var sessionDBFamilyLeaseRegistry = struct {
	sync.Mutex
	entries map[string]*sessionDBFamilyLeaseEntry
}{entries: make(map[string]*sessionDBFamilyLeaseEntry)}

// FamilyLease serializes physical opens and whole-family replacement
// for one canonical events.db path inside this process.
type FamilyLease struct {
	path    string
	entry   *sessionDBFamilyLeaseEntry
	active  atomic.Bool
	release func()
}

// AcquireFamilyLease acquires the mutation/open lease for one
// canonical events.db family. Release must be called when the protected
// operation is complete.
func AcquireFamilyLease(ctx context.Context, path string) (*FamilyLease, error) {
	if ctx == nil {
		return nil, errors.New("store: session database family lease context is required")
	}
	key, err := normalizeSessionDBFamilyLeasePath(path)
	if err != nil {
		return nil, err
	}

	entry := retainSessionDBFamilyLeaseEntry(key)
	select {
	case <-ctx.Done():
		releaseSessionDBFamilyLeaseEntry(key, entry, false)
		return nil, fmt.Errorf("store: acquire session database family lease %q: %w", key, ctx.Err())
	case <-entry.token:
	}

	lease := &FamilyLease{path: key, entry: entry}
	lease.active.Store(true)
	lease.release = sync.OnceFunc(func() {
		lease.active.Store(false)
		releaseSessionDBFamilyLeaseEntry(key, entry, true)
	})
	return lease, nil
}

// Release releases the family lease. It is safe to call more than once.
func (l *FamilyLease) Release() {
	if l == nil || l.release == nil {
		return
	}
	l.release()
}

// VerifyOwner verifies a database file in the leased family directory against
// the exact immutable owner while retaining no-follow identity across the
// read-only SQLite probe.
func (l *FamilyLease) VerifyOwner(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) error {
	pinned, err := l.pinOwnedFile(ctx, owner, path)
	if err != nil {
		return err
	}
	if closeErr := pinned.Close(); closeErr != nil {
		return closeErr
	}
	return nil
}

func (l *FamilyLease) pinOwnedFile(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (*pinnedSessionDBFile, error) {
	if ctx == nil {
		return nil, errors.New("store: verify session database family context is required")
	}
	if l == nil || !l.active.Load() {
		return nil, errors.New("store: active session database family lease is required")
	}
	normalizedOwner, err := owner.Normalize()
	if err != nil {
		return nil, err
	}
	cleanPath, err := l.validateOwnedCandidatePath(path)
	if err != nil {
		return nil, err
	}
	pinned, err := pinSessionDBFile(cleanPath)
	if err != nil {
		return nil, err
	}
	if err := verifySessionDBOwnerReadOnly(ctx, normalizedOwner, cleanPath); err != nil {
		return nil, pinned.closeWithError(err)
	}
	if err := pinned.RequireCurrent(cleanPath); err != nil {
		return nil, pinned.closeWithError(err)
	}
	return pinned, nil
}

func (l *FamilyLease) pinFile(path string) (*pinnedSessionDBFile, string, error) {
	if l == nil || !l.active.Load() {
		return nil, "", errors.New("store: active session database family lease is required")
	}
	cleanPath, err := l.validateCandidatePath(path)
	if err != nil {
		return nil, "", err
	}
	pinned, err := pinSessionDBFile(cleanPath)
	if err != nil {
		return nil, "", err
	}
	return pinned, cleanPath, nil
}

func (l *FamilyLease) validateCandidatePath(path string) (string, error) {
	absolutePath, err := normalizeSessionDBFamilyCandidatePath(path)
	if err != nil {
		return "", err
	}
	if filepath.Dir(absolutePath) != filepath.Dir(l.path) {
		return "", fmt.Errorf(
			"store: session database family candidate %q is outside leased directory %q",
			absolutePath,
			filepath.Dir(l.path),
		)
	}
	return absolutePath, nil
}

func (l *FamilyLease) validateOwnedCandidatePath(path string) (string, error) {
	absolutePath, err := normalizeSessionDBFamilyCandidatePath(path)
	if err != nil {
		return "", err
	}
	leaseDirectory := filepath.Dir(l.path)
	candidateDirectory := filepath.Dir(absolutePath)
	if candidateDirectory != leaseDirectory &&
		filepath.Dir(candidateDirectory) != filepath.Dir(leaseDirectory) {
		return "", fmt.Errorf(
			"store: owned session database candidate %q is outside leased family root %q",
			absolutePath,
			filepath.Dir(leaseDirectory),
		)
	}
	return absolutePath, nil
}

func normalizeSessionDBFamilyCandidatePath(path string) (string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", errors.New("store: session database family candidate path is required")
	}
	absolutePath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("store: resolve session database family candidate %q: %w", cleanPath, err)
	}
	absolutePath = filepath.Clean(absolutePath)
	return absolutePath, nil
}

func normalizeSessionDBFamilyLeasePath(path string) (string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", errors.New("store: session database family path is required")
	}
	absolutePath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("store: resolve session database family path %q: %w", cleanPath, err)
	}
	return filepath.Clean(absolutePath), nil
}

func retainSessionDBFamilyLeaseEntry(key string) *sessionDBFamilyLeaseEntry {
	sessionDBFamilyLeaseRegistry.Lock()
	defer sessionDBFamilyLeaseRegistry.Unlock()

	entry := sessionDBFamilyLeaseRegistry.entries[key]
	if entry == nil {
		entry = &sessionDBFamilyLeaseEntry{token: make(chan struct{}, 1)}
		entry.token <- struct{}{}
		sessionDBFamilyLeaseRegistry.entries[key] = entry
	}
	entry.refs++
	return entry
}

func releaseSessionDBFamilyLeaseEntry(key string, entry *sessionDBFamilyLeaseEntry, acquired bool) {
	if acquired {
		entry.token <- struct{}{}
	}
	sessionDBFamilyLeaseRegistry.Lock()
	defer sessionDBFamilyLeaseRegistry.Unlock()

	entry.refs--
	if entry.refs == 0 && sessionDBFamilyLeaseRegistry.entries[key] == entry {
		delete(sessionDBFamilyLeaseRegistry.entries, key)
	}
}

type pinnedSessionDBFile struct {
	file      *os.File
	info      os.FileInfo
	directory *fileutil.Directory
}

func pinSessionDBFile(path string) (*pinnedSessionDBFile, error) {
	directory, name, err := fileutil.OpenParentDirectory(path)
	if err != nil {
		return nil, fmt.Errorf("store: open session database parent directory %q: %w", path, err)
	}
	file, err := directory.OpenRegularFile(name)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("store: pin session database file %q: %w", path, err),
			closePinnedSessionDBDirectory(directory),
		)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(closePinnedSessionDBFileWithError(
			file,
			fmt.Errorf("store: stat pinned session database file %q: %w", path, err),
		), closePinnedSessionDBDirectory(directory))
	}
	return &pinnedSessionDBFile{file: file, info: info, directory: directory}, nil
}

type sessionDBFamilyMutationStamp struct {
	info        os.FileInfo
	changeToken string
}

func (p *pinnedSessionDBFile) FamilyMutationStamp() (sessionDBFamilyMutationStamp, error) {
	if p == nil || p.file == nil {
		return sessionDBFamilyMutationStamp{}, errors.New("store: pinned session database file is required")
	}
	info, err := p.file.Stat()
	if err != nil {
		return sessionDBFamilyMutationStamp{}, fmt.Errorf("store: stat pinned session database file: %w", err)
	}
	return sessionDBFamilyMutationStamp{info: info, changeToken: sessionDBFileChangeToken(info)}, nil
}

func (p *pinnedSessionDBFile) RequireFamilyUnchanged(want sessionDBFamilyMutationStamp) error {
	got, err := p.FamilyMutationStamp()
	if err != nil {
		return err
	}
	if want.info == nil || got.info == nil ||
		!os.SameFile(want.info, got.info) ||
		want.info.Size() != got.info.Size() ||
		want.info.Mode() != got.info.Mode() ||
		!want.info.ModTime().Equal(got.info.ModTime()) ||
		want.changeToken != got.changeToken {
		return fmt.Errorf("%w: pinned database identity mutated", ErrSessionDBFamilyChanged)
	}
	return nil
}

func (p *pinnedSessionDBFile) RequireCurrent(path string) (retErr error) {
	if p == nil || p.file == nil || p.info == nil {
		return errors.New("store: pinned session database file is required")
	}
	current, err := fileutil.OpenRegularFile(path)
	if err != nil {
		return fmt.Errorf("store: reopen pinned session database file %q: %w", path, err)
	}
	defer func() {
		if closeErr := current.Close(); closeErr != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("store: close current session database file %q: %w", path, closeErr),
			)
		}
	}()
	currentInfo, err := current.Stat()
	if err != nil {
		return fmt.Errorf("store: stat current session database file %q: %w", path, err)
	}
	if !os.SameFile(p.info, currentInfo) {
		return fmt.Errorf("%w: %q", ErrSessionDBFamilyChanged, path)
	}
	return nil
}

func (p *pinnedSessionDBFile) Close() error {
	if p == nil {
		return nil
	}
	file := p.file
	p.file = nil
	directory := p.directory
	p.directory = nil
	var fileErr error
	if file != nil {
		if err := file.Close(); err != nil {
			fileErr = fmt.Errorf("store: close pinned session database file: %w", err)
		}
	}
	return errors.Join(fileErr, closePinnedSessionDBDirectory(directory))
}

func (p *pinnedSessionDBFile) closeWithError(primaryErr error) error {
	if closeErr := p.Close(); closeErr != nil {
		return errors.Join(primaryErr, closeErr)
	}
	return primaryErr
}

func closePinnedSessionDBFileWithError(file *os.File, primaryErr error) error {
	if file == nil {
		return primaryErr
	}
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(primaryErr, fmt.Errorf("store: close rejected pinned session database file: %w", closeErr))
	}
	return primaryErr
}

func closePinnedSessionDBDirectory(directory *fileutil.Directory) error {
	if directory == nil {
		return nil
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("store: close pinned session database directory: %w", err)
	}
	return nil
}
