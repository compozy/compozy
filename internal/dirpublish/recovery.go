package dirpublish

import (
	"errors"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

// Layout records which of a journal's directories exist on disk.
type Layout uint8

// Layout flags.
const (
	HasTarget Layout = 1 << iota
	HasIncoming
	HasBackup
)

// RecoveryPolicy decides how an interrupted publication found on disk is
// resolved. Each caller owns its policy (for example roll-forward for registry
// packages); the policy runs under the caller's publication lock.
type RecoveryPolicy interface {
	RecoverLayout(parent *fileutil.Directory, journal Journal, layout Layout) error
}

// Recover reads the journal of targetName and, when one exists, inspects its
// layout and delegates the decision to policy. No journal means nothing to
// recover. The caller must hold the target's Lock.
func Recover(parent *fileutil.Directory, targetName string, policy RecoveryPolicy) error {
	journal, exists, err := ReadJournal(parent, targetName)
	if err != nil || !exists {
		return err
	}
	layout, err := InspectLayout(parent, journal)
	if err != nil {
		return err
	}
	return policy.RecoverLayout(parent, journal, layout)
}

// InspectLayout reports which of the journal's target, incoming, and backup
// directories exist.
func InspectLayout(parent *fileutil.Directory, journal Journal) (Layout, error) {
	checks := []struct {
		name      string
		operation string
		flag      Layout
	}{
		{name: journal.TargetName, operation: "inspect canonical target", flag: HasTarget},
		{name: journal.IncomingName, operation: "inspect incoming target", flag: HasIncoming},
		{name: journal.BackupName, operation: "inspect backup target", flag: HasBackup},
	}
	var layout Layout
	for _, check := range checks {
		exists, err := DirectoryExists(parent, check.name)
		if err != nil {
			return 0, RecoveryError(check.operation, err)
		}
		if exists {
			layout |= check.flag
		}
	}
	return layout, nil
}

// DirectoryExists reports whether name is a directory inside parent.
func DirectoryExists(parent *fileutil.Directory, name string) (bool, error) {
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
