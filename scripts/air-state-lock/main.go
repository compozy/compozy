package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) < 4 || os.Args[2] != "--" {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/air-state-lock <lock-file> -- <command> [args...]")
		os.Exit(2)
	}

	lockPath := filepath.Clean(os.Args[1])
	if lockPath != os.Args[1] || filepath.Base(lockPath) != "dev-owner.lock" {
		fatalf("Air state lock path must be normalized and end with dev-owner.lock")
	}
	if err := runLocked(context.Background(), lockPath, os.Args[3:]); err != nil {
		fatalf("%v", err)
	}
}

func runLocked(ctx context.Context, lockPath string, command []string) (returnErr error) {
	// #nosec G703 -- the normalized operator-owned path is constrained to the fixed lock basename.
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open Air state lock: %w", err)
	}
	defer func() {
		if err := lockFile.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close Air state lock: %w", err))
		}
	}()

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("acquire Air state lock: %w", err)
	}
	// The helper owns the lock; commands and their descendants must not inherit it.
	if _, err := unix.FcntlInt(lockFile.Fd(), unix.F_SETFD, unix.FD_CLOEXEC); err != nil {
		return fmt.Errorf("mark Air state lock close-on-exec: %w", err)
	}

	commandPath, err := exec.LookPath(command[0])
	if err != nil {
		return fmt.Errorf("resolve locked command %q: %w", command[0], err)
	}
	// #nosec G702 -- this internal helper intentionally executes argv supplied by fixed repository scripts.
	lockedCommand := exec.CommandContext(ctx, commandPath, command[1:]...)
	lockedCommand.Stdin = os.Stdin
	lockedCommand.Stdout = os.Stdout
	lockedCommand.Stderr = os.Stderr
	if err := lockedCommand.Run(); err != nil {
		return fmt.Errorf("execute locked command %q: %w", command[0], err)
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "dev: "+format+"\n", args...)
	os.Exit(1)
}
