package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type extensionDataRemovalOps struct {
	removeAll func(string) error
	rename    func(string, string) error
	now       func() time.Time
}

type extensionDataCleanup struct {
	quarantined    bool
	dataPath       string
	quarantinePath string
}

func defaultExtensionDataRemovalOps() extensionDataRemovalOps {
	return extensionDataRemovalOps{removeAll: os.RemoveAll, rename: os.Rename, now: time.Now}
}

// stagedExtensionData keeps plugin data recoverable until durable removal commits.
type stagedExtensionData struct {
	dataPath, stagedPath string
	ops                  extensionDataRemovalOps
}

func stageAgentPluginDataForRemoval(
	info ExtensionInfo, homePaths compozyconfig.HomePaths, ops extensionDataRemovalOps,
) (*stagedExtensionData, error) {
	data := &stagedExtensionData{ops: ops}
	if normalizeExtensionFormat(info.Format) != FormatAgentPlugin {
		return data, nil
	}
	path, err := homePaths.ExtensionDataPath(info.Name, "", "")
	if err != nil {
		return nil, err
	}
	return stageExtensionDataPath(path, ops)
}

func stageExtensionDataPath(path string, ops extensionDataRemovalOps) (*stagedExtensionData, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("extension: data path is required")
	}
	data := &stagedExtensionData{dataPath: path, ops: ops}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return data, nil
	} else if err != nil {
		return nil, err
	}
	if ops.removeAll == nil || ops.rename == nil || ops.now == nil {
		return nil, errors.New("extension: data removal operations are required")
	}
	staged := fmt.Sprintf("%s.compozy-quarantine-%d", path, ops.now().UTC().UnixNano())
	if err := ops.rename(path, staged); err != nil {
		return nil, fmt.Errorf("extension: stage data removal: %w", err)
	}
	data.stagedPath = staged
	return data, nil
}

func (data *stagedExtensionData) rollback() error {
	if data.stagedPath == "" {
		return nil
	}
	if err := data.ops.rename(data.stagedPath, data.dataPath); err != nil {
		return fmt.Errorf("extension: restore staged plugin data: %w", err)
	}
	return nil
}

func (data *stagedExtensionData) commit() (extensionDataCleanup, error) {
	cleanup := extensionDataCleanup{dataPath: data.dataPath}
	if data.stagedPath == "" {
		return cleanup, nil
	}
	if err := data.ops.removeAll(data.stagedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanup.quarantined, cleanup.quarantinePath = true, data.stagedPath
		return cleanup, fmt.Errorf("extension: plugin data residue remains quarantined: %w", err)
	}
	return cleanup, nil
}
