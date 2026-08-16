package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	quarantinePath string
}

func defaultExtensionDataRemovalOps() extensionDataRemovalOps {
	return extensionDataRemovalOps{removeAll: os.RemoveAll, rename: os.Rename, now: time.Now}
}

func removeAgentPluginDataForInstall(
	info ExtensionInfo,
	installDir string,
	ops extensionDataRemovalOps,
) (extensionDataCleanup, error) {
	if normalizeExtensionFormat(info.Format) != FormatAgentPlugin {
		return extensionDataCleanup{}, nil
	}
	homeDir := filepath.Dir(filepath.Dir(filepath.Clean(installDir)))
	homePaths, err := compozyconfig.ResolveHomePathsFrom(homeDir)
	if err != nil {
		return extensionDataCleanup{}, fmt.Errorf("extension: resolve data home for %q: %w", info.Name, err)
	}
	dataPath, err := homePaths.ExtensionDataPath(info.Name, "")
	if err != nil {
		return extensionDataCleanup{}, fmt.Errorf("extension: resolve data path for %q: %w", info.Name, err)
	}
	return removeExtensionDataPath(dataPath, ops)
}

func removeExtensionDataPath(path string, ops extensionDataRemovalOps) (extensionDataCleanup, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return extensionDataCleanup{}, errors.New("extension: data path is required")
	}
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return extensionDataCleanup{}, nil
	} else if err != nil {
		return extensionDataCleanup{}, fmt.Errorf("extension: inspect data path %q: %w", target, err)
	}
	if ops.removeAll == nil || ops.rename == nil || ops.now == nil {
		return extensionDataCleanup{}, errors.New("extension: data removal operations are required")
	}
	removeErr := ops.removeAll(target)
	if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
		return extensionDataCleanup{}, nil
	}
	quarantine := fmt.Sprintf("%s.compozy-quarantine-%d", target, ops.now().UTC().UnixNano())
	if renameErr := ops.rename(target, quarantine); renameErr != nil {
		return extensionDataCleanup{}, errors.Join(
			fmt.Errorf("extension: remove data path %q: %w", target, removeErr),
			fmt.Errorf("extension: quarantine data path %q: %w", target, renameErr),
		)
	}
	return extensionDataCleanup{quarantined: true, quarantinePath: quarantine},
		fmt.Errorf("extension: remove data path %q; residue quarantined at %q: %w", target, quarantine, removeErr)
}
