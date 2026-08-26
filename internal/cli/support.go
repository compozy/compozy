package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	supportCommandKey      = "support"
	supportBundleKey       = "bundle"
	supportOperationIDKey  = "operation_id"
	supportDownloadPathKey = "path"
	supportSizeBytesKey    = "size_bytes"
	supportBundlesDirName  = "support-bundles"
	supportStatusCompleted = "completed"
	supportStatusFailed    = "failed"
	supportStatusValue     = "Status"

	defaultSupportBundleTimeout = 5 * time.Minute
)

type supportBundleOptions struct {
	outputPath string
	yes        bool
	noStatus   bool
}

type supportBundleResult struct {
	Operation SupportBundleOperationRecord `json:"operation"`
	Path      string                       `json:"path"`
}

type supportBundleStatusClient interface {
	GetSupportBundle(context.Context, string) (SupportBundleOperationRecord, error)
}

type supportBundleDownloadClient interface {
	DownloadSupportBundle(context.Context, string, io.Writer) error
}

func newSupportCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   supportCommandKey,
		Short: "Create daemon-owned support artifacts",
	}
	cmd.AddCommand(newSupportBundleCommand(deps))
	return cmd
}

func newSupportBundleCommand(deps commandDeps) *cobra.Command {
	var opts supportBundleOptions
	cmd := &cobra.Command{
		Use:   supportBundleKey,
		Short: "Create and download a redacted support bundle",
		Example: strings.Join([]string{
			"  # Create a bundle and write it under $COMPOZY_HOME/support-bundles",
			"  compozy support bundle --yes",
			"",
			"  # Write the daemon-built bundle to a specific path",
			"  compozy support bundle --yes --output /tmp/compozy-support-bundle.tar.gz",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSupportBundleCommand(cmd, deps, opts)
		},
	}
	cmd.Flags().StringVar(&opts.outputPath, outputFlagName, "", "Path or directory for the downloaded bundle")
	cmd.Flags().BoolVar(&opts.yes, yesFlagName, false, "Confirm support bundle creation without an interactive prompt")
	cmd.Flags().BoolVar(
		&opts.noStatus,
		"no-status",
		false,
		"Omit status.json, doctor.json, providers.json, and runtime status snapshots",
	)
	return cmd
}

func runSupportBundleCommand(cmd *cobra.Command, deps commandDeps, opts supportBundleOptions) error {
	confirmed, err := confirmSupportBundleCreation(cmd, opts.yes)
	if err != nil {
		return err
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	includeStatus := !opts.noStatus
	created, err := client.CreateSupportBundle(cmd.Context(), CreateSupportBundleRequest{
		Yes:           confirmed,
		IncludeStatus: &includeStatus,
	})
	if err != nil {
		return err
	}
	operation, err := waitForSupportBundle(cmd.Context(), deps, client, created.OperationID)
	if err != nil {
		return err
	}
	path, err := resolveSupportBundleOutputPath(deps, opts.outputPath, operation)
	if err != nil {
		return err
	}
	if err := downloadSupportBundle(cmd.Context(), client, operation.OperationID, path); err != nil {
		return err
	}
	return writeSupportBundleResult(cmd, supportBundleResult{
		Operation: operation,
		Path:      path,
	})
}

func confirmSupportBundleCreation(cmd *cobra.Command, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return false, err
	}
	if mode != OutputHuman {
		return false, errors.New("cli: support bundle creation requires --yes for structured output")
	}
	input := cmd.InOrStdin()
	if !supportBundleInputIsTerminal(input) {
		return false, errors.New("cli: support bundle creation requires --yes when stdin is not interactive")
	}
	message := "Support bundles include redacted config, log tail, provider metadata, " +
		"event summaries, and status artifacts. Create support bundle? [y/N] "
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), message); err != nil {
		return false, fmt.Errorf("cli: write support bundle consent prompt: %w", err)
	}
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("cli: read support bundle consent: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != yesFlagName {
		return false, errors.New("cli: support bundle creation declined")
	}
	return true, nil
}

func supportBundleInputIsTerminal(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	rawConn, err := file.SyscallConn()
	if err != nil {
		return false
	}
	isTerminal := false
	if err := rawConn.Control(func(fd uintptr) {
		if fd <= uintptr(math.MaxInt) {
			isTerminal = term.IsTerminal(int(fd))
		}
	}); err != nil {
		return false
	}
	return isTerminal
}

func resolveInheritedOutputFormat(cmd *cobra.Command) (OutputFormat, error) {
	if cmd == nil {
		return "", errors.New("cli: command is required")
	}
	root := cmd.Root()
	if root == nil {
		return "", errors.New("cli: root command is required")
	}
	flags := root.PersistentFlags()
	value, err := flags.GetString(outputFlagName)
	if err != nil {
		return "", fmt.Errorf("cli: read inherited output flag: %w", err)
	}
	jsonEnabled, err := flags.GetBool(jsonFlagName)
	if err == nil && jsonEnabled {
		outputFlag := flags.Lookup(outputFlagName)
		normalized := OutputFormat(strings.ToLower(strings.TrimSpace(value)))
		if outputFlag != nil && outputFlag.Changed && normalized != "" && normalized != OutputJSON {
			return "", errors.New("cli: --json cannot be combined with a non-json output format")
		}
		return OutputJSON, nil
	}
	switch OutputFormat(strings.ToLower(strings.TrimSpace(value))) {
	case "", OutputHuman:
		return OutputHuman, nil
	case OutputJSON:
		return OutputJSON, nil
	case OutputJSONL:
		return OutputJSONL, nil
	case OutputToon:
		return OutputToon, nil
	default:
		return "", fmt.Errorf("cli: invalid output format %q", value)
	}
}

func waitForSupportBundle(
	ctx context.Context,
	deps commandDeps,
	client supportBundleStatusClient,
	operationID string,
) (SupportBundleOperationRecord, error) {
	if err := requirePollingContext(ctx); err != nil {
		return SupportBundleOperationRecord{}, err
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return SupportBundleOperationRecord{}, errors.New("cli: support bundle operation id is required")
	}
	return pollUntil(
		ctx,
		defaultSupportBundleTimeout,
		deps.pollInterval,
		nil,
		"cli: support bundle did not complete before timeout",
		func(pollCtx context.Context, _ pollEvent) (SupportBundleOperationRecord, bool, error) {
			operation, err := client.GetSupportBundle(pollCtx, operationID)
			if err != nil {
				return SupportBundleOperationRecord{}, false, nil
			}
			switch strings.TrimSpace(operation.Status) {
			case supportStatusCompleted:
				return operation, true, nil
			case supportStatusFailed:
				reason := strings.TrimSpace(operation.FailureReason)
				if reason == "" {
					reason = "support bundle creation failed"
				}
				return operation, true, errors.New("cli: " + reason)
			default:
				return SupportBundleOperationRecord{}, false, nil
			}
		},
	)
}

func resolveSupportBundleOutputPath(
	deps commandDeps,
	rawPath string,
	operation SupportBundleOperationRecord,
) (string, error) {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return "", err
	}
	fileName := strings.TrimSpace(operation.FileName)
	if fileName == "" {
		fileName = fmt.Sprintf("compozy-support-bundle-%s.tar.gz", time.Now().UTC().Format("20060102T150405Z"))
	}

	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return filepath.Join(homePaths.HomeDir, supportBundlesDirName, fileName), nil
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("cli: resolve support bundle output path: %w", err)
	}
	info, err := os.Stat(resolved)
	if err == nil && info.IsDir() {
		return filepath.Join(resolved, fileName), nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cli: inspect support bundle output path: %w", err)
	}
	return resolved, nil
}

func downloadSupportBundle(
	ctx context.Context,
	client supportBundleDownloadClient,
	operationID string,
	path string,
) (err error) {
	if strings.TrimSpace(path) == "" {
		return errors.New("cli: support bundle output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cli: create support bundle output directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("cli: support bundle output already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: inspect support bundle output: %w", err)
	}

	tmpPath := path + ".tmp-" + strings.TrimSpace(operationID)
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("cli: create support bundle temp file: %w", err)
	}
	committed := false
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			closeErr = fmt.Errorf("cli: close support bundle temp file: %w", closeErr)
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
		if !committed {
			if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				removeErr = fmt.Errorf("cli: remove support bundle temp file: %w", removeErr)
				if err == nil {
					err = removeErr
				} else {
					err = errors.Join(err, removeErr)
				}
			}
		}
	}()

	if err := client.DownloadSupportBundle(ctx, operationID, file); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("cli: sync support bundle temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("cli: move support bundle into place: %w", err)
	}
	committed = true
	return nil
}

func supportBundleResultBundle(result supportBundleResult) outputBundle {
	operation := result.Operation
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			rows := []keyValue{
				{Label: authoredContextOperationValue, Value: stringOrDash(operation.OperationID)},
				{Label: supportStatusValue, Value: stringOrDash(operation.Status)},
				{Label: automationPathValue, Value: stringOrDash(result.Path)},
				{Label: "Size", Value: fmt.Sprintf("%d", operation.SizeBytes)},
				{Label: "Created", Value: stringOrDash(formatTime(operation.CreatedAt))},
			}
			if operation.CompletedAt != nil {
				rows = append(rows, keyValue{
					Label: cliCompletedValue,
					Value: stringOrDash(formatTime(*operation.CompletedAt)),
				})
			}
			return renderHumanSection("Support bundle", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject("support_bundle", []string{
				supportOperationIDKey,
				daemonStatusKey,
				supportDownloadPathKey,
				supportSizeBytesKey,
				automationCreatedAtKey,
				"completed_at",
			}, []string{
				operation.OperationID,
				operation.Status,
				result.Path,
				fmt.Sprintf("%d", operation.SizeBytes),
				formatTime(operation.CreatedAt),
				formatOptionalTime(operation.CompletedAt),
			}), nil
		},
	}
}

func writeSupportBundleResult(cmd *cobra.Command, result supportBundleResult) error {
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return err
	}
	bundle := supportBundleResultBundle(result)
	switch mode {
	case OutputJSON:
		return writeJSON(cmd, result)
	case OutputJSONL:
		return writeJSONLine(cmd, result)
	case OutputToon:
		rendered, err := bundle.toon()
		if err != nil {
			return err
		}
		return writeRawCommandOutput(cmd, rendered)
	default:
		rendered, err := bundle.human()
		if err != nil {
			return err
		}
		return writeRawCommandOutput(cmd, rendered)
	}
}
