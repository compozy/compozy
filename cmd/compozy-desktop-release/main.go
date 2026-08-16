package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
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
		writeOperatorError(err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: compozy-desktop-release <publish|repair|assert-inventory|assert-version>")
	}
	switch args[0] {
	case "publish":
		return runPublish(ctx, args[1:])
	case "repair":
		return runRepair(ctx, args[1:])
	case "assert-inventory":
		return runAssertInventory(ctx, args[1:])
	case "assert-version":
		return runAssertVersion(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runPublish(ctx context.Context, args []string) error {
	flags := newFlagSet("publish")
	common := addAuthorityFlags(flags)
	assetDir := flags.String("asset-dir", "", "directory containing the exact desktop release inventory")
	channelDir := flags.String("channel-dir", "", "directory containing merged updater manifests")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	authority, err := newAuthority(common)
	if err != nil {
		return err
	}
	result, err := authority.Publish(ctx, desktoprelease.PublishRequest{
		OperationID: common.operationID, Channel: common.channel, Version: common.version,
		AssetDir: *assetDir, ChannelDir: *channelDir, PublishedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func runRepair(ctx context.Context, args []string) error {
	flags := newFlagSet("repair")
	common := addAuthorityFlags(flags)
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	authority, err := newAuthority(common)
	if err != nil {
		return err
	}
	result, err := authority.Repair(ctx, desktoprelease.RepairRequest{
		OperationID: common.operationID, Channel: common.channel, Version: common.version,
		PublishedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	return writeJSON(result)
}

type authorityFlags struct {
	operationID string
	channel     string
	version     string
	repository  string
	token       string
	apiURL      string
	output      string
}

func addAuthorityFlags(flags *flag.FlagSet) *authorityFlags {
	values := &authorityFlags{}
	flags.StringVar(&values.operationID, "operation-id", "", "idempotent release operation identity")
	flags.StringVar(&values.channel, "channel", desktoprelease.ChannelBeta, "desktop release channel")
	flags.StringVar(&values.version, "version", "", "strict unprefixed release SemVer")
	flags.StringVar(&values.repository, "repository", os.Getenv("GITHUB_REPOSITORY"), "GitHub owner/name")
	flags.StringVar(&values.token, "token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	flags.StringVar(&values.apiURL, "api-url", "https://api.github.com", "GitHub API base URL")
	flags.StringVar(&values.output, "output", "json", "structured output format")
	flags.StringVar(&values.output, "o", "json", "structured output format")
	return values
}

func newAuthority(values *authorityFlags) (*desktoprelease.Authority, error) {
	if values.output != "json" {
		return nil, fmt.Errorf("desktop release: output format must be json")
	}
	backend, err := desktoprelease.NewGitHubBackend(
		&http.Client{Timeout: 60 * time.Second},
		values.apiURL,
		values.repository,
		values.token,
	)
	if err != nil {
		return nil, err
	}
	return desktoprelease.NewAuthority(backend)
}

func runAssertInventory(ctx context.Context, args []string) error {
	flags := newFlagSet("assert-inventory")
	version := flags.String("version", "", "release version")
	dir := flags.String("dir", "", "normalized desktop artifact directory")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	return desktoprelease.AssertExactDesktopInventory(ctx, *dir, *version)
}

func runAssertVersion(args []string) error {
	flags := newFlagSet("assert-version")
	candidate := flags.String("candidate", "", "candidate release version")
	live := flags.String("live", "", "live channel version")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	return desktoprelease.AssertStrictlyGreater(*candidate, *live)
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

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("desktop release: write JSON output: %w", err)
	}
	return nil
}

func writeOperatorError(err error) {
	payload := map[string]any{
		"error": map[string]string{
			"code":    string(desktoprelease.CodeOf(err)),
			"message": err.Error(),
		},
	}
	if encodeErr := json.NewEncoder(os.Stderr).Encode(payload); encodeErr != nil {
		fmt.Fprintf(os.Stderr, "desktop release: write error output: %v; original: %v\n", encodeErr, err)
	}
}
