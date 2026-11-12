//go:build race

package testutil

// raceTimeScale stretches wall-clock budgets for race-instrumented builds,
// which CI runs with -parallel=4 on four vCPUs.
const raceTimeScale = 4
