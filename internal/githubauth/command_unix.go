//go:build !windows

package githubauth

import (
	"os/exec"

	"github.com/compozy/compozy/internal/procutil"
)

func configureCredentialCommand(command *exec.Cmd) {
	procutil.ConfigureCommandSession(command)
}
