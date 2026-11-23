package agentinstance

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	skillInstallLogDir        = ".skill-installs"
	skillInstallLogMaxEntries = 200
	skillInstallLogMaxBytes   = 1 << 20
)

// Install-log actions.
const (
	SkillInstallActionInstall = "install"
	SkillInstallActionRevert  = "revert"
	SkillInstallActionResolve = "resolve"
)

// SkillInstallMarker is a writer record: the `current` and `previous` values of an install-log entry
// in <target root>/.skill-installs/<name>.jsonl, and PrevMarker/NextMarker in the journal payload.
// There is no separate marker file; the install log is the only authority. A skill's current writer
// is the last log entry's `current`; no log means never installed by this feature.
type SkillInstallMarker struct {
	ReviewID        string `json:"review_id"` // "" = operator-authored (conflict kept current)
	InstanceID      string `json:"instance_id"`
	HomeID          string `json:"home_id"`          // SHA-256 of the canonical home directory
	InstalledDigest string `json:"installed_digest"` // tree digest v1
	DecidedVia      string `json:"decided_via"`      // operator | auto_approve
	DecidedBy       string `json:"decided_by"`
	OpID            string `json:"op_id"`
}

// skillInstallLogEntry is one committed install, revert, or conflict resolution.
type skillInstallLogEntry struct {
	OpID            string              `json:"op_id"`
	HomeID          string              `json:"home_id"`
	InstanceID      string              `json:"instance_id"`
	Action          string              `json:"action"`
	ReviewID        string              `json:"review_id"`
	InstalledDigest string              `json:"installed_digest"`
	DecidedVia      string              `json:"decided_via"`
	DecidedBy       string              `json:"decided_by"`
	Current         *SkillInstallMarker `json:"current"`
	Previous        *SkillInstallMarker `json:"previous"`
	// Keep is the operator's choice on a `resolve` entry: current | previous.
	Keep string    `json:"keep,omitempty"`
	At   time.Time `json:"at"`
}

func skillInstallLogPath(root, name string) string {
	return filepath.Join(root, skillInstallLogDir, name+".jsonl")
}

// readSkillInstallLog returns the entries of a skill's install log, oldest first; a missing log is
// empty. A malformed log is an error: the log is the authority and is never guessed around.
func readSkillInstallLog(root, name string) ([]skillInstallLogEntry, error) {
	data, _, err := fileutil.ReadRegularFile(skillInstallLogPath(root, name), skillInstallLogMaxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill install log: %w", err)
	}
	var entries []skillInstallLogEntry
	for line := range bytes.Lines(data) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var entry skillInstallLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("decode skill install log %s: %w", name, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// currentSkillWriter is the last entry's `current`; nil when the skill has no log or the last
// operation removed it.
func currentSkillWriter(entries []skillInstallLogEntry) *SkillInstallMarker {
	if len(entries) == 0 {
		return nil
	}
	return entries[len(entries)-1].Current
}

// appendSkillInstallLog atomically appends entry. Nothing is evicted here: an operation that rolls
// back removes its own entry and must find the committed history it appended to intact.
func appendSkillInstallLog(root, name string, entry skillInstallLogEntry) error {
	entries, err := readSkillInstallLog(root, name)
	if err != nil {
		return err
	}
	return writeSkillInstallLog(root, name, append(entries, entry))
}

// trimSkillInstallLog keeps the newest 200 entries. It runs only once an operation is published, so
// published recovery retries it after a crash.
func trimSkillInstallLog(root, name string) error {
	entries, err := readSkillInstallLog(root, name)
	if err != nil || len(entries) <= skillInstallLogMaxEntries {
		return err
	}
	return writeSkillInstallLog(root, name, entries[len(entries)-skillInstallLogMaxEntries:])
}

// removeSkillInstallLogOp drops the trailing entries of an uncommitted operation. Entries of a
// committed operation are never trailing ones of a later rollback, so only the tail is inspected.
func removeSkillInstallLogOp(root, name, opID string) error {
	entries, err := readSkillInstallLog(root, name)
	if err != nil {
		return err
	}
	kept := len(entries)
	for kept > 0 && entries[kept-1].OpID == opID {
		kept--
	}
	if kept == len(entries) {
		return nil
	}
	return writeSkillInstallLog(root, name, entries[:kept])
}

func writeSkillInstallLog(root, name string, entries []skillInstallLogEntry) error {
	var data bytes.Buffer
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data.Write(line)
		data.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Join(root, skillInstallLogDir), 0o700); err != nil {
		return err
	}
	if err := fileutil.AtomicWriteFile(skillInstallLogPath(root, name), data.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write skill install log: %w", err)
	}
	return nil
}
