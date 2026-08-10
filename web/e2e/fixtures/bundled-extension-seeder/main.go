package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) (err error) {
	flags := flag.NewFlagSet("bundled-extension-seeder", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	homeDir := flags.String("home", "", "isolated CompozyOS home")
	sourceDir := flags.String("source", "", "bundled extension source directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*homeDir) == "" || strings.TrimSpace(*sourceDir) == "" {
		return errors.New("bundled extension seeder requires --home and --source")
	}

	homePaths, err := compozyconfig.ResolveHomePathsFrom(*homeDir)
	if err != nil {
		return fmt.Errorf("resolve isolated home: %w", err)
	}
	store, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
	if err != nil {
		return fmt.Errorf("open isolated global database: %w", err)
	}
	defer func() {
		err = errors.Join(err, store.Close(ctx))
	}()

	manifest, err := extensionpkg.LoadManifest(*sourceDir)
	if err != nil {
		return fmt.Errorf("load bundled extension manifest: %w", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(*sourceDir)
	if err != nil {
		return fmt.Errorf("compute bundled extension checksum: %w", err)
	}
	registry := extensionpkg.NewRegistry(store.DB())
	if err := extensionpkg.InstallLocalManaged(
		homePaths,
		registry,
		manifest,
		*sourceDir,
		checksum,
		extensionpkg.WithInstallSource(extensionpkg.SourceBundled),
	); err != nil {
		return fmt.Errorf("install bundled extension: %w", err)
	}
	return nil
}
