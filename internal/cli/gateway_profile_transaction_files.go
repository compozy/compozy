package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/gofrs/flock"
)

const (
	gatewayProfileTransactionJournalSuffix  = ".gateway-transaction.json"
	gatewayProfileTransactionLockSuffix     = ".gateway-transaction.lock"
	gatewayProfileTransactionLockRetryDelay = 10 * time.Millisecond
)

type gatewayProfileTransactionLock struct {
	lock *flock.Flock
}

func acquireGatewayProfileTransactionLock(
	ctx context.Context,
	credentialsDir string,
	profile string,
) (*gatewayProfileTransactionLock, error) {
	if ctx == nil {
		return nil, errors.New("cli: gateway profile transaction context is required")
	}
	if err := ensureGatewayProfileTransactionDirectory(credentialsDir); err != nil {
		return nil, err
	}
	path, err := gatewayProfileTransactionLockPath(credentialsDir, profile)
	if err != nil {
		return nil, err
	}
	if err := ensureGatewayProfileTransactionLockFile(path); err != nil {
		return nil, err
	}

	fileLock := flock.New(path, flock.SetPermissions(0o600))
	locked, err := fileLock.TryLockContext(ctx, gatewayProfileTransactionLockRetryDelay)
	if err != nil {
		return nil, fmt.Errorf("cli: acquire gateway profile transaction lock: %w", err)
	}
	if !locked {
		return nil, errors.New("cli: gateway profile transaction lock was not acquired")
	}
	return &gatewayProfileTransactionLock{lock: fileLock}, nil
}

func tryAcquireGatewayProfileTransactionLock(
	credentialsDir string,
	profile string,
) (*gatewayProfileTransactionLock, error) {
	if err := ensureGatewayProfileTransactionDirectory(credentialsDir); err != nil {
		return nil, err
	}
	path, err := gatewayProfileTransactionLockPath(credentialsDir, profile)
	if err != nil {
		return nil, err
	}
	if err := ensureGatewayProfileTransactionLockFile(path); err != nil {
		return nil, err
	}
	fileLock := flock.New(path, flock.SetPermissions(0o600))
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("cli: acquire gateway profile transaction lock: %w", err)
	}
	if !locked {
		return nil, &gatewayClientError{
			code:       "gateway_profile_busy",
			statusCode: http.StatusConflict,
			message:    "another gateway profile change is already in progress",
		}
	}
	return &gatewayProfileTransactionLock{lock: fileLock}, nil
}

func (l *gatewayProfileTransactionLock) Release() error {
	if l == nil || l.lock == nil {
		return nil
	}
	if err := l.lock.Unlock(); err != nil {
		return fmt.Errorf("cli: release gateway profile transaction lock: %w", err)
	}
	l.lock = nil
	return nil
}

func writeGatewayProfileTransactionJournal(
	credentialsDir string,
	journal gatewayProfileTransactionJournal,
) error {
	if err := validateGatewayProfileTransactionJournal(journal); err != nil {
		return err
	}
	if err := ensureGatewayProfileTransactionDirectory(credentialsDir); err != nil {
		return err
	}
	path, err := gatewayProfileTransactionJournalPath(credentialsDir, journal.Profile)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("cli: encode gateway profile transaction journal: %w", err)
	}
	if len(encoded) > gatewayProfileTransactionJournalMaxSize {
		return errors.New("cli: gateway profile transaction journal exceeds maximum size")
	}
	if err := fileutil.AtomicWriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("cli: write gateway profile transaction journal: %w", err)
	}
	return nil
}

func readGatewayProfileTransactionJournal(
	credentialsDir string,
	profile string,
) (gatewayProfileTransactionJournal, error) {
	path, err := gatewayProfileTransactionJournalPath(credentialsDir, profile)
	if err != nil {
		return gatewayProfileTransactionJournal{}, err
	}
	contents, info, err := fileutil.ReadRegularFile(path)
	if err != nil {
		return gatewayProfileTransactionJournal{}, fmt.Errorf("cli: read gateway profile transaction journal: %w", err)
	}
	if info.Mode().Perm() != 0o600 {
		return gatewayProfileTransactionJournal{}, errors.New(
			"cli: gateway profile transaction journal permissions must be 0600",
		)
	}
	return decodeGatewayProfileTransactionJournalForProfile(contents, profile)
}

func removeGatewayProfileTransactionJournal(credentialsDir, profile string) error {
	path, err := gatewayProfileTransactionJournalPath(credentialsDir, profile)
	if err != nil {
		return err
	}
	if err := fileutil.AtomicRemoveFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: remove gateway profile transaction journal: %w", err)
	}
	return nil
}

func listGatewayProfileTransactionJournals(
	credentialsDir string,
) (journals []gatewayProfileTransactionJournal, err error) {
	directory, err := fileutil.OpenDirectory(credentialsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []gatewayProfileTransactionJournal{}, nil
		}
		return nil, fmt.Errorf("cli: open gateway profile transaction directory: %w", err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close gateway profile transaction directory: %w", closeErr))
		}
	}()

	names, err := directory.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("cli: list gateway profile transaction journals: %w", err)
	}
	sort.Strings(names)
	journals = make([]gatewayProfileTransactionJournal, 0, len(names))
	for _, name := range names {
		profile, ok := gatewayProfileTransactionProfileFromJournalName(name)
		if !ok {
			continue
		}
		journal, readErr := readGatewayProfileTransactionJournalFromDirectory(directory, profile)
		if readErr != nil {
			return nil, readErr
		}
		journals = append(journals, journal)
	}
	return journals, nil
}

func ensureGatewayProfileTransactionDirectory(credentialsDir string) error {
	if strings.TrimSpace(credentialsDir) == "" || strings.ContainsRune(credentialsDir, 0) {
		return errors.New("cli: gateway credentials directory is invalid")
	}
	if err := os.MkdirAll(credentialsDir, 0o700); err != nil {
		return fmt.Errorf("cli: create gateway credentials directory: %w", err)
	}
	directory, err := fileutil.OpenDirectoryForMutation(credentialsDir)
	if err != nil {
		return fmt.Errorf("cli: secure gateway credentials directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("cli: close gateway credentials directory: %w", err)
	}
	return nil
}

func ensureGatewayProfileTransactionLockFile(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cli: gateway profile transaction lock: %w", fileutil.ErrSymlink)
		}
		if !info.Mode().IsRegular() {
			return errors.New("cli: gateway profile transaction lock is not a regular file")
		}
		if info.Mode().Perm() != 0o600 {
			return errors.New("cli: gateway profile transaction lock permissions must be 0600")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: inspect gateway profile transaction lock: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return ensureGatewayProfileTransactionLockFile(path)
	}
	if err != nil {
		return fmt.Errorf("cli: create gateway profile transaction lock: %w", err)
	}
	if chmodErr := file.Chmod(0o600); chmodErr != nil {
		closeErr := file.Close()
		return errors.Join(
			fmt.Errorf("cli: secure gateway profile transaction lock: %w", chmodErr),
			closeGatewayProfileTransactionLockFile(closeErr),
		)
	}
	if closeErr := file.Close(); closeErr != nil {
		return closeGatewayProfileTransactionLockFile(closeErr)
	}
	return nil
}

func closeGatewayProfileTransactionLockFile(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: close gateway profile transaction lock: %w", err)
}

func gatewayProfileTransactionJournalPath(credentialsDir, profile string) (string, error) {
	if !validGatewayProfileName(profile) {
		return "", errors.New("cli: gateway profile transaction profile is invalid")
	}
	return filepath.Join(credentialsDir, profile+gatewayProfileTransactionJournalSuffix), nil
}

func gatewayProfileTransactionLockPath(credentialsDir, profile string) (string, error) {
	if !validGatewayProfileName(profile) {
		return "", errors.New("cli: gateway profile transaction profile is invalid")
	}
	return filepath.Join(credentialsDir, profile+gatewayProfileTransactionLockSuffix), nil
}

func gatewayProfileTransactionProfileFromJournalName(name string) (string, bool) {
	if !strings.HasSuffix(name, gatewayProfileTransactionJournalSuffix) {
		return "", false
	}
	profile := strings.TrimSuffix(name, gatewayProfileTransactionJournalSuffix)
	if !validGatewayProfileName(profile) {
		return "", false
	}
	return profile, true
}

func readGatewayProfileTransactionJournalFromDirectory(
	directory *fileutil.Directory,
	profile string,
) (gatewayProfileTransactionJournal, error) {
	if !validGatewayProfileName(profile) {
		return gatewayProfileTransactionJournal{}, errors.New("cli: gateway profile transaction profile is invalid")
	}
	contents, info, err := directory.ReadRegularFile(profile + gatewayProfileTransactionJournalSuffix)
	if err != nil {
		return gatewayProfileTransactionJournal{}, fmt.Errorf("cli: read gateway profile transaction journal: %w", err)
	}
	if info.Mode().Perm() != 0o600 {
		return gatewayProfileTransactionJournal{}, errors.New(
			"cli: gateway profile transaction journal permissions must be 0600",
		)
	}
	return decodeGatewayProfileTransactionJournalForProfile(contents, profile)
}

func decodeGatewayProfileTransactionJournalForProfile(
	contents []byte,
	profile string,
) (gatewayProfileTransactionJournal, error) {
	journal, err := decodeGatewayProfileTransactionJournal(contents)
	if err != nil {
		return gatewayProfileTransactionJournal{}, err
	}
	if journal.Profile != profile {
		return gatewayProfileTransactionJournal{}, errors.New(
			"cli: gateway profile transaction journal profile does not match its path",
		)
	}
	return journal, nil
}
