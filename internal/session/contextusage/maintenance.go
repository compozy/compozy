package contextusage

import "math"

type MaintenanceAction string

const (
	MaintenanceNone    MaintenanceAction = "none"
	MaintenanceCompact MaintenanceAction = "compact"
	MaintenanceRotate  MaintenanceAction = "rotate"
)

// DecideMaintenance leaves execution and dependency admission to the caller.
func DecideMaintenance(known bool, used, size int64, threshold float64, hasCompaction bool) MaintenanceAction {
	if !known || size <= 0 || used < 0 || threshold <= 0 || math.IsNaN(threshold) ||
		float64(used)/float64(size) < threshold {
		return MaintenanceNone
	}
	if hasCompaction {
		return MaintenanceCompact
	}
	return MaintenanceRotate
}
