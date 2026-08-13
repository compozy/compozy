package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/desktoprelease"
)

func main() {
	os.Exit(execute(os.Args[1:]))
}

func execute(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(
			"usage: compozy-desktop-release " +
				"<build-feed|canonicalize-json|channel-config|assert-inventory|assert-version|assert-comparator>",
		)
	}
	switch args[0] {
	case "build-feed":
		return runBuildFeed(ctx, args[1:])
	case "canonicalize-json":
		return runCanonicalizeJSON(args[1:])
	case "channel-config":
		return runChannelConfig(args[1:])
	case "assert-inventory":
		return runAssertInventory(ctx, args[1:])
	case "assert-version":
		return runAssertVersion(args[1:])
	case "assert-comparator":
		return runAssertComparator(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runCanonicalizeJSON(args []string) error {
	flags := flag.NewFlagSet("canonicalize-json", flag.ContinueOnError)
	input := flags.String("input", "", "input JSON path")
	output := flags.String("output", "", "canonical JSON output path")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	contents, err := os.ReadFile(*input)
	if err != nil {
		return fmt.Errorf("read json input: %w", err)
	}
	canonical, err := desktoprelease.CanonicalizeJSON(contents)
	if err != nil {
		return fmt.Errorf("canonicalize json input: %w", err)
	}
	if err := os.WriteFile(*output, canonical, 0o600); err != nil {
		return fmt.Errorf("write canonical json output: %w", err)
	}
	return nil
}

func runBuildFeed(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("build-feed", flag.ContinueOnError)
	version := flags.String("version", "", "release version")
	publishedAt := flags.String("published-at", "", "RFC3339 publication time")
	desktopDir := flags.String("desktop-dir", "", "normalized desktop artifact directory")
	runtimeDir := flags.String("runtime-dir", "", "runtime archive directory")
	repoRoot := flags.String("repo-root", ".", "repository root")
	outputDir := flags.String("output-dir", "", "feed output directory")
	baseURL := flags.String("base-url", desktoprelease.DistributionOrigin, "distribution origin")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	stamp, err := time.Parse(time.RFC3339, *publishedAt)
	if err != nil {
		return fmt.Errorf("published-at must be RFC3339: %w", err)
	}
	return desktoprelease.BuildFeeds(ctx, desktoprelease.BuildRequest{
		Version:     *version,
		PublishedAt: stamp,
		DesktopDir:  *desktopDir,
		RuntimeDir:  *runtimeDir,
		RepoRoot:    *repoRoot,
		OutputDir:   *outputDir,
		BaseURL:     *baseURL,
	})
}

func runChannelConfig(args []string) error {
	flags := flag.NewFlagSet("channel-config", flag.ContinueOnError)
	version := flags.String("version", "", "release version")
	channel := flags.String("channel", "", "desktop channel")
	publicKey := flags.String("public-key", "", "base64 minisign public key")
	previousPublicKey := flags.String("previous-public-key", "", "previous base64 minisign public key")
	azureEndpoint := flags.String("azure-endpoint", "", "Azure Artifact Signing endpoint")
	output := flags.String("output", "desktop/src-tauri/tauri.channel.conf.json", "output path")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	return desktoprelease.WriteChannelConfig(desktoprelease.ChannelConfigRequest{
		Version:           *version,
		Channel:           *channel,
		PublicKey:         *publicKey,
		PreviousPublicKey: *previousPublicKey,
		AzureEndpoint:     *azureEndpoint,
		OutputPath:        *output,
	})
}

func runAssertInventory(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("assert-inventory", flag.ContinueOnError)
	version := flags.String("version", "", "release version")
	dir := flags.String("dir", "", "normalized desktop artifact directory")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	return desktoprelease.AssertExactDesktopInventory(ctx, *dir, *version)
}

func runAssertVersion(args []string) error {
	flags := flag.NewFlagSet("assert-version", flag.ContinueOnError)
	candidate := flags.String("candidate", "", "candidate release version")
	live := flags.String("live", "", "live feed version")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	return desktoprelease.AssertStrictlyGreater(*candidate, *live)
}

func runAssertComparator(args []string) error {
	flags := flag.NewFlagSet("assert-comparator", flag.ContinueOnError)
	sourcePath := flags.String("source", "desktop/src-tauri/src/update/app_update.rs", "updater source path")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	contents, err := os.ReadFile(*sourcePath)
	if err != nil {
		return fmt.Errorf("read updater source: %w", err)
	}
	return desktoprelease.AssertDefaultComparator(string(contents))
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%s: unexpected positional arguments: %v", flags.Name(), flags.Args())
	}
	return nil
}
