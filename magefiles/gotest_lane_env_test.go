//go:build mage

// Suite: Go test lane environment overrides
// Invariant: An explicit valid package cap wins and malformed caps fall back safely.
// Boundary IN: Process-environment parsing for the Mage unit-test lane.
// Boundary OUT: Pure capacity and shard selection in gotest_lane_test.go.

package main

import (
	"runtime"
	"strconv"
	"testing"
)

// not parallel: t.Setenv mutates the process environment for each subtest.
func TestGoUnitTestPackageLimit(t *testing.T) {
	defaultLimit := goUnitTestPackageLimitFor(runtime.GOMAXPROCS(0), goUnitTestParallelism)
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "Should default to the combined lane capacity when unset",
			value: "",
			want:  strconv.Itoa(defaultLimit),
		},
		{name: "Should honor a valid override", value: "2", want: "2"},
		{name: "Should ignore a non-numeric override", value: "many", want: strconv.Itoa(defaultLimit)},
		{name: "Should ignore a non-positive override", value: "0", want: strconv.Itoa(defaultLimit)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(goTestPackageLimitEnvVar, tc.value)
			if got := goUnitTestPackageLimit(); got != tc.want {
				t.Fatalf("goUnitTestPackageLimit() with %q = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
