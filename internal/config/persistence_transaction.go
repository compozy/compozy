package config

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
)

var configOverlayWriteMu sync.Mutex

// EditConfigOverlayAndApply restores rejected writes; apply must not re-enter overlay mutation.
func EditConfigOverlayAndApply(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
	mutate func(*OverlayEditor) error,
	apply func(Config) error,
) (cfg Config, err error) {
	if !target.isConfigTarget() {
		return Config{}, fmt.Errorf("config: write target %q is not a config overlay", target.Kind())
	}
	if mutate == nil {
		return Config{}, errors.New("config: config overlay mutation is required")
	}
	configOverlayWriteMu.Lock()
	defer configOverlayWriteMu.Unlock()
	original, existed, err := readOptionalRegularFile(target.path, "config overlay")
	if err != nil {
		return Config{}, err
	}
	contents, err := archiveRetiredSkillMarketplace(original, target.path)
	if err != nil {
		return Config{}, err
	}
	editor, err := newOverlayEditor(target.path, contents)
	if err != nil {
		return Config{}, err
	}
	if err := mutate(editor); err != nil {
		return Config{}, err
	}
	rendered, err := editor.Bytes()
	if err != nil {
		return Config{}, err
	}
	finalCfg, err := validateEffectiveConfigWrite(homePaths, workspaceRoot, target, rendered)
	if err != nil {
		return Config{}, err
	}
	directory, name, err := fileutil.OpenOrCreateParentDirectory(target.path, privateDirMode)
	if err != nil {
		return Config{}, err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	if err := requireOverlayContents(directory, name, target.path, original, existed); err != nil {
		return Config{}, err
	}
	rollback := func() error {
		return restoreConfigOverlay(directory, name, target.path, original, rendered, existed)
	}
	if err := writePersistedFileInDirectory(directory, name, target.path, rendered, true); err != nil {
		return Config{}, errors.Join(err, rollback())
	}
	if apply != nil {
		if applyErr := apply(finalCfg); applyErr != nil {
			return Config{}, errors.Join(applyErr, rollback())
		}
	}
	return finalCfg, nil
}

func requireOverlayContents(directory *fileutil.Directory, name, path string, expected []byte, existed bool) error {
	current, exists, err := readOptionalRegularFileFromDirectory(directory, name, path, "config overlay")
	if err != nil {
		return err
	}
	if exists != existed || !bytes.Equal(current, expected) {
		return errors.New("config: overlay changed during mutation; retry against the current file")
	}
	return nil
}

func restoreConfigOverlay(
	directory *fileutil.Directory,
	name, path string,
	original, rendered []byte,
	existed bool,
) error {
	current, exists, err := readOptionalRegularFileFromDirectory(directory, name, path, "config overlay")
	if err != nil {
		return fmt.Errorf("config: inspect rejected overlay: %w", err)
	}
	if exists == existed && bytes.Equal(current, original) {
		return nil
	}
	if !exists || !bytes.Equal(current, rendered) {
		return errors.New("config: preserve concurrent overlay edit during rollback; retry against the current file")
	}
	if existed {
		err = writePersistedFileInDirectory(directory, name, path, original, true)
	} else {
		err = directory.RemoveRegularFile(name)
	}
	if err != nil {
		return fmt.Errorf("config: restore rejected overlay: %w", err)
	}
	return nil
}
