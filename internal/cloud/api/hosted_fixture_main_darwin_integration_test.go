// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && darwin

package api_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(runHostedFixtureProcess(m.Run))
}
