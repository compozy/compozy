//go:build !race

package testutil

// raceTimeScale leaves wall-clock budgets unchanged in plain builds.
const raceTimeScale = 1
