package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/demoseed"
)

func main() {
	os.Exit(exitCode())
}

func exitCode() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) (err error) {
	defaultHome, err := defaultDemoHome()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("demo-seed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	homeDir := flags.String("home", defaultHome, "CompozyOS home to populate")
	replace := flags.Bool("replace", false, "replace only the existing Northstar Pay scenario")
	jsonOutput := flags.Bool("json", false, "print the result as JSON")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("demo seed: parse flags: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("demo seed: unexpected arguments: %v", flags.Args())
	}

	paths, err := config.ResolveHomePathsFrom(*homeDir)
	if err != nil {
		return fmt.Errorf("demo seed: resolve home: %w", err)
	}
	lock, err := daemon.AcquireLock(paths.DaemonLock, os.Getpid())
	if err != nil {
		if errors.Is(err, daemon.ErrAlreadyRunning) {
			return fmt.Errorf("demo seed: stop the daemon using %s before seeding: %w", paths.HomeDir, err)
		}
		return fmt.Errorf("demo seed: acquire daemon lock: %w", err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("demo seed: release daemon lock: %w", releaseErr))
		}
	}()

	result, err := demoseed.Seed(ctx, demoseed.Options{HomeDir: paths.HomeDir, Replace: *replace})
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("demo seed: encode result: %w", err)
		}
		return nil
	}
	return printResult(stdout, result)
}

func defaultDemoHome() (string, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("demo seed: resolve user home: %w", err)
	}
	return filepath.Join(userHome, ".agh"), nil
}

func printResult(output io.Writer, result demoseed.Result) error {
	if _, err := fmt.Fprintf(
		output,
		"Northstar Pay workspace is ready.\n\nHome: %s\nDatabase: %s\nWorkspace: %s\n\n",
		result.HomeDir,
		result.DatabaseFile,
		result.WorkspaceName,
	); err != nil {
		return fmt.Errorf("demo seed: print result header: %w", err)
	}
	if _, err := fmt.Fprintln(output, "Suggested recording path:"); err != nil {
		return fmt.Errorf("demo seed: print recording path heading: %w", err)
	}
	for index, path := range result.SuggestedWebPath {
		if _, err := fmt.Fprintf(output, "  %d. %s\n", index+1, path); err != nil {
			return fmt.Errorf("demo seed: print recording path: %w", err)
		}
	}
	if _, err := fmt.Fprintf(
		output,
		"\nCreated %d agents, %d sessions, %d tasks, and %d Network messages.\n",
		result.Counts.Agents,
		result.Counts.Sessions,
		result.Counts.Tasks,
		result.Counts.NetworkMessages,
	); err != nil {
		return fmt.Errorf("demo seed: print result summary: %w", err)
	}
	return nil
}
