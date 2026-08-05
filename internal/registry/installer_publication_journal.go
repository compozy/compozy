package registry

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	installPublicationJournalVersion = 1
	installPublicationJournalMaxSize = 4096

	installPublicationPhasePrepared  = "prepared"
	installPublicationPhaseIncoming  = "incoming_ready"
	installPublicationPhaseBackup    = "backup_staged"
	installPublicationPhasePublished = "published"

	installIncomingPrefix = ".compozy-incoming-"
	installBackupPrefix   = ".compozy-backup-"
	installJournalPrefix  = ".compozy-publication-"
)

type installPublicationJournal struct {
	Version      int    `json:"version"`
	TargetName   string `json:"target_name"`
	IncomingName string `json:"incoming_name"`
	BackupName   string `json:"backup_name"`
	Phase        string `json:"phase"`
}

func rollbackIncomingInstall(
	targetParent *fileutil.Directory,
	journal installPublicationJournal,
	sourceParent *fileutil.Directory,
	sourceName string,
	journalName string,
) error {
	_, restoreErr := moveNamedInstalledDirectory(
		targetParent,
		journal.IncomingName,
		sourceParent,
		sourceName,
	)
	if restoreErr != nil {
		return publicationRecoveryError("restore incoming source", restoreErr)
	}
	if err := removeInstallPublicationJournal(targetParent, journalName); err != nil {
		return publicationRecoveryError("remove rolled-back publication journal", err)
	}
	return nil
}

func rollbackInterruptedReplacement(
	targetParent *fileutil.Directory,
	journal installPublicationJournal,
	sourceParent *fileutil.Directory,
	sourceName string,
	journalName string,
) error {
	_, restoreTargetErr := moveNamedInstalledDirectory(
		targetParent,
		journal.BackupName,
		targetParent,
		journal.TargetName,
	)
	_, restoreSourceErr := moveNamedInstalledDirectory(
		targetParent,
		journal.IncomingName,
		sourceParent,
		sourceName,
	)
	if restoreTargetErr != nil || restoreSourceErr != nil {
		return errors.Join(
			publicationRecoveryStepError("restore canonical target", restoreTargetErr),
			publicationRecoveryStepError("restore incoming source", restoreSourceErr),
		)
	}
	if err := removeInstallPublicationJournal(targetParent, journalName); err != nil {
		return publicationRecoveryError("remove rolled-back publication journal", err)
	}
	return nil
}

func appendJournalPhaseDiagnostic(
	parent *fileutil.Directory,
	journalName string,
	journal *installPublicationJournal,
	phase string,
	diagnostics *[]CleanupDiagnostic,
) {
	journal.Phase = phase
	if err := writeInstallPublicationJournal(parent, journalName, *journal, true); err != nil {
		*diagnostics, _ = appendCleanupDiagnostic(*diagnostics, CleanupOperationPublication)
	}
}

func finalizeCommittedInstallPublication(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
) ([]CleanupDiagnostic, error) {
	diagnostics := make([]CleanupDiagnostic, 0, 1)
	if err := parent.RemoveAll(journal.BackupName); err != nil && !errors.Is(err, os.ErrNotExist) {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationBackup)
		return diagnostics, recordInstallCleanupFailure(CleanupOperationBackup, err)
	}
	if err := removeInstallPublicationJournal(parent, journalName); err != nil {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationPublication)
		return diagnostics, recordInstallCleanupFailure(CleanupOperationPublication, err)
	}
	return diagnostics, nil
}

func newInstallPublicationJournal(targetName string) (installPublicationJournal, error) {
	incomingName, err := newInstallPublicationTransactionName(installIncomingPrefix)
	if err != nil {
		return installPublicationJournal{}, err
	}
	backupName, err := newInstallPublicationTransactionName(installBackupPrefix)
	if err != nil {
		return installPublicationJournal{}, err
	}
	return installPublicationJournal{
		Version:      installPublicationJournalVersion,
		TargetName:   targetName,
		IncomingName: incomingName,
		BackupName:   backupName,
		Phase:        installPublicationPhasePrepared,
	}, nil
}

func newInstallPublicationTransactionName(prefix string) (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", fmt.Errorf("registry: generate publication transaction name: %w", err)
	}
	return prefix + hex.EncodeToString(randomBytes[:]), nil
}

func installPublicationJournalName(targetName string) string {
	digest := sha256.Sum256([]byte(targetName))
	return installJournalPrefix + hex.EncodeToString(digest[:]) + ".json"
}

func writeInstallPublicationJournal(
	parent *fileutil.Directory,
	name string,
	journal installPublicationJournal,
	replace bool,
) error {
	contents, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("registry: encode install publication journal: %w", err)
	}
	if err := parent.AtomicWriteFile(name, contents, 0o600, replace); err != nil {
		return fmt.Errorf("registry: write install publication journal: %w", err)
	}
	return nil
}

func readInstallPublicationJournal(
	parent *fileutil.Directory,
	name string,
	targetName string,
) (_ installPublicationJournal, exists bool, err error) {
	file, err := parent.OpenRegularFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return installPublicationJournal{}, false, nil
	}
	if err != nil {
		return installPublicationJournal{}, false, fmt.Errorf("registry: open install publication journal: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close install publication journal: %w", closeErr))
		}
	}()

	contents, err := io.ReadAll(io.LimitReader(file, installPublicationJournalMaxSize+1))
	if err != nil {
		return installPublicationJournal{}, false, fmt.Errorf("registry: read install publication journal: %w", err)
	}
	if len(contents) > installPublicationJournalMaxSize {
		return installPublicationJournal{}, false, publicationRecoveryError("journal exceeds size limit", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var journal installPublicationJournal
	if err := decoder.Decode(&journal); err != nil {
		return installPublicationJournal{}, false, publicationRecoveryError("decode journal", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return installPublicationJournal{}, false, publicationRecoveryError("decode journal", err)
	}
	if err := validateInstallPublicationJournal(journal, targetName); err != nil {
		return installPublicationJournal{}, false, err
	}
	return journal, true, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func validateInstallPublicationJournal(journal installPublicationJournal, targetName string) error {
	if journal.Version != installPublicationJournalVersion || journal.TargetName != targetName {
		return publicationRecoveryError("journal identity mismatch", nil)
	}
	switch journal.Phase {
	case installPublicationPhasePrepared,
		installPublicationPhaseIncoming,
		installPublicationPhaseBackup,
		installPublicationPhasePublished:
	default:
		return publicationRecoveryError("journal phase is invalid", nil)
	}
	if !validInstallPublicationTransactionName(journal.IncomingName, installIncomingPrefix) ||
		!validInstallPublicationTransactionName(journal.BackupName, installBackupPrefix) ||
		journal.IncomingName == journal.BackupName ||
		journal.IncomingName == targetName ||
		journal.BackupName == targetName {
		return publicationRecoveryError("journal transaction names are invalid", nil)
	}
	return nil
}

func validInstallPublicationTransactionName(name string, prefix string) bool {
	encoded := strings.TrimPrefix(name, prefix)
	if encoded == name || len(encoded) != 32 {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

func installedDirectoryExists(parent *fileutil.Directory, name string) (bool, error) {
	directory, err := parent.OpenDirectory(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := directory.Close(); err != nil {
		return false, err
	}
	return true, nil
}

func removeInstallPublicationJournal(parent *fileutil.Directory, name string) error {
	if err := parent.RemoveRegularFile(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("registry: remove install publication journal: %w", err)
	}
	return nil
}

func publicationRecoveryError(operation string, cause error) error {
	detail := fmt.Errorf("registry: %s", operation)
	if cause != nil {
		detail = fmt.Errorf("registry: %s: %w", operation, cause)
	}
	return errors.Join(ErrInstallPublicationRecoveryRequired, detail)
}

func publicationRecoveryStepError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return publicationRecoveryError(operation, cause)
}

func reportCleanupDiagnostics(diagnostics []CleanupDiagnostic) {
	for _, diagnostic := range diagnostics {
		reportInstallCleanup(diagnostic.Operation)
	}
}
