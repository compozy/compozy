package extensionpkg

type managedInstallBoundary string

const (
	managedInstallBoundaryStaged     managedInstallBoundary = "staged"
	managedInstallBoundaryFinalMoved managedInstallBoundary = "final_moved"
)

func withManagedInstallBoundaryObserver(observer func(managedInstallBoundary) error) InstallOption {
	return func(cfg *installConfig) {
		cfg.boundaryObserver = observer
	}
}
