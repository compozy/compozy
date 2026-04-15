package sound

import (
	"context"
	"fmt"
	"os/exec"
)

// execRunner is the production commandRunner. It runs the requested binary
// with the given arguments and discards stdout/stderr — sound commands never
// emit output we care about.
type execRunner struct{}

// Run executes the command and returns any error from the underlying process.
func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	if name == "" {
		return fmt.Errorf("sound: empty command name")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sound: %s: %w", name, err)
	}
	return nil
}
