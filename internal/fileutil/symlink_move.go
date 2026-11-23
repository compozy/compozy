package fileutil

import "fmt"

// MoveSymlinkNoReplace renames the direct child symlink sourceName to targetName without following
// it and without replacing an existing targetName, then syncs the directory. A sourceName that is
// not a symlink is refused.
func (d *Directory) MoveSymlinkNoReplace(sourceName, targetName string) error {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(sourceName); err != nil {
		return err
	}
	if err := validateChildName(targetName); err != nil {
		return err
	}
	if err := moveSymlinkNoReplace(d, sourceName, targetName); err != nil {
		return err
	}
	return wrapDirectoryMovePostCommitError("sync symlink move parent", d.Sync())
}
