package dirpublish

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

// Journal phases, in publication order. A journal at PhasePublished is committed.
const (
	PhasePrepared  = "prepared"
	PhaseIncoming  = "incoming_ready"
	PhaseBackup    = "backup_staged"
	PhasePublished = "published"
)

// MaxPayloadSize bounds the caller's opaque journal payload.
const MaxPayloadSize = 2048

const (
	journalVersion = 1
	// journalBaseMaxSize bounds a journal without a payload; the read bound
	// grows only by the largest payload field (base64 of MaxPayloadSize bytes).
	journalBaseMaxSize = 4096
	journalMaxSize     = journalBaseMaxSize + len(`,"payload":""`) + (MaxPayloadSize+2)/3*4

	incomingPrefix = ".compozy-incoming-"
	backupPrefix   = ".compozy-backup-"
	journalPrefix  = ".compozy-publication-"
)

// ErrRecoveryRequired reports an interrupted publication whose filesystem state
// cannot be resolved without operator inspection.
var ErrRecoveryRequired = errors.New("dirpublish: publication recovery required")

// ErrPayloadTooLarge reports a journal payload over MaxPayloadSize.
var ErrPayloadTooLarge = errors.New("dirpublish: journal payload exceeds size limit")

// Journal is the versioned on-disk record of one publication, written next to
// the target before anything moves. IncomingName and BackupName are the hidden
// sibling directories the publication may use. Payload is opaque caller state.
type Journal struct {
	Version      int    `json:"version"`
	TargetName   string `json:"target_name"`
	IncomingName string `json:"incoming_name"`
	BackupName   string `json:"backup_name"`
	Phase        string `json:"phase"`
	Payload      []byte `json:"payload,omitempty"`
}

// NewJournal returns a PhasePrepared journal for targetName with fresh random
// incoming and backup names. It does not write anything.
func NewJournal(targetName string, payload []byte) (Journal, error) {
	if len(payload) > MaxPayloadSize {
		return Journal{}, ErrPayloadTooLarge
	}
	incomingName, err := newTransactionName(incomingPrefix)
	if err != nil {
		return Journal{}, err
	}
	backupName, err := newTransactionName(backupPrefix)
	if err != nil {
		return Journal{}, err
	}
	return Journal{
		Version:      journalVersion,
		TargetName:   targetName,
		IncomingName: incomingName,
		BackupName:   backupName,
		Phase:        PhasePrepared,
		Payload:      payload,
	}, nil
}

func newTransactionName(prefix string) (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", fmt.Errorf("dirpublish: generate publication transaction name: %w", err)
	}
	return prefix + hex.EncodeToString(randomBytes[:]), nil
}

// JournalName returns the journal file name for targetName.
func JournalName(targetName string) string {
	digest := sha256.Sum256([]byte(targetName))
	return journalPrefix + hex.EncodeToString(digest[:]) + ".json"
}

// WriteJournal atomically writes journal next to its target. Without replace,
// an existing journal fails the write.
func WriteJournal(parent *fileutil.Directory, journal Journal, replace bool) error {
	if len(journal.Payload) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}
	contents, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("dirpublish: encode publication journal: %w", err)
	}
	if err := parent.AtomicWriteFile(JournalName(journal.TargetName), contents, 0o600, replace); err != nil {
		return fmt.Errorf("dirpublish: write publication journal: %w", err)
	}
	return nil
}

// AdvancePhase durably moves journal to phase. A failed write leaves the
// in-memory phase unchanged and returns the error, so the caller decides
// whether it is fatal or a cleanup diagnostic.
func AdvancePhase(parent *fileutil.Directory, journal *Journal, phase string) error {
	previous := journal.Phase
	journal.Phase = phase
	if err := WriteJournal(parent, *journal, true); err != nil {
		journal.Phase = previous
		return fmt.Errorf("dirpublish: advance publication journal to %s: %w", phase, err)
	}
	return nil
}

// ReadJournal reads and validates the journal of targetName. A missing journal
// reports exists=false; an unreadable or invalid one wraps ErrRecoveryRequired.
func ReadJournal(parent *fileutil.Directory, targetName string) (_ Journal, exists bool, err error) {
	file, err := parent.OpenRegularFile(JournalName(targetName))
	if errors.Is(err, os.ErrNotExist) {
		return Journal{}, false, nil
	}
	if err != nil {
		return Journal{}, false, fmt.Errorf("dirpublish: open publication journal: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("dirpublish: close publication journal: %w", closeErr))
		}
	}()

	contents, err := io.ReadAll(io.LimitReader(file, journalMaxSize+1))
	if err != nil {
		return Journal{}, false, fmt.Errorf("dirpublish: read publication journal: %w", err)
	}
	if len(contents) > journalMaxSize {
		return Journal{}, false, RecoveryError("journal exceeds size limit", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var journal Journal
	if err := decoder.Decode(&journal); err != nil {
		return Journal{}, false, RecoveryError("decode journal", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Journal{}, false, RecoveryError("decode journal", err)
	}
	if err := validateJournal(journal, targetName); err != nil {
		return Journal{}, false, err
	}
	return journal, true, nil
}

// RemoveJournal removes the journal of targetName; a missing journal is not an error.
func RemoveJournal(parent *fileutil.Directory, targetName string) error {
	if err := parent.RemoveRegularFile(JournalName(targetName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("dirpublish: remove publication journal: %w", err)
	}
	return nil
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

func validateJournal(journal Journal, targetName string) error {
	if journal.Version != journalVersion || journal.TargetName != targetName {
		return RecoveryError("journal identity mismatch", nil)
	}
	switch journal.Phase {
	case PhasePrepared, PhaseIncoming, PhaseBackup, PhasePublished:
	default:
		return RecoveryError("journal phase is invalid", nil)
	}
	if !validTransactionName(journal.IncomingName, incomingPrefix) ||
		!validTransactionName(journal.BackupName, backupPrefix) ||
		journal.IncomingName == journal.BackupName ||
		journal.IncomingName == targetName ||
		journal.BackupName == targetName {
		return RecoveryError("journal transaction names are invalid", nil)
	}
	if len(journal.Payload) > MaxPayloadSize {
		return RecoveryError("journal payload exceeds size limit", nil)
	}
	return nil
}

func validTransactionName(name string, prefix string) bool {
	encoded := strings.TrimPrefix(name, prefix)
	if encoded == name || len(encoded) != 32 {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

// RecoveryError wraps ErrRecoveryRequired with the failed operation and cause.
func RecoveryError(operation string, cause error) error {
	detail := fmt.Errorf("dirpublish: %s", operation)
	if cause != nil {
		detail = fmt.Errorf("dirpublish: %s: %w", operation, cause)
	}
	return errors.Join(ErrRecoveryRequired, detail)
}

// RecoveryStepError is RecoveryError for an optional cause: nil stays nil.
func RecoveryStepError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return RecoveryError(operation, cause)
}
