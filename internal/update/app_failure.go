package update

func archivedAppFailureCount(currentVersion string, archived *Operation) int {
	if archived == nil || archived.App == nil || archived.App.Phase != PhaseFailed ||
		archived.App.ConsecutiveFailures < 1 {
		return 0
	}
	comparison, err := compareVersions(currentVersion, archived.App.ToVersion)
	if err == nil && comparison >= 0 {
		return 0
	}
	return archived.App.ConsecutiveFailures
}
