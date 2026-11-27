// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && linux

package api_test

import (
	"testing"

	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	sshFixtureVersion = remotetest.SSHFixtureVersion
	sshFixtureHome    = remotetest.SSHHome
	sshFixtureUser    = remotetest.SSHUser
)

func prepareSSHFixtureAssets(t *testing.T) remotetest.SSHAssets {
	t.Helper()
	return remotetest.PrepareSSHAssets(t, "../../..")
}
