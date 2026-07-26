package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/compozy/agh/internal/api/spec"
	"github.com/compozy/agh/internal/codegen/jsbin"
	"github.com/compozy/agh/internal/codegen/sdkts"
	"github.com/compozy/agh/internal/fileutil"
)

const (
	subcommandCheck = "check"
)

const defaultSDKContractsPath = "sdk/typescript/src/generated/contracts.ts"
const defaultLoopEnumsPath = "web/src/generated/loop-enums.ts"
const defaultLifecycleMatrixPath = "packages/site/content/runtime/core/configuration/lifecycle-matrix.mdx"
const defaultNativeToolCatalogPath = "internal/tools/builtin/testdata/native-tool-catalog.json"

var ErrStaleGeneratedFile = errors.New("generated file is stale")

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals()...)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	return runWithPaths(ctx, args, spec.DefaultPath, defaultSDKContractsPath)
}

func runWithPaths(ctx context.Context, args []string, openapiPath string, sdkContractsPath string) error {
	if len(args) == 0 {
		return fmt.Errorf(
			"usage: agh-codegen <openapi|sdk-contracts|loop-enums|lifecycle-matrix|native-tool-catalog|all|check>",
		)
	}
	loopEnumsPath := loopEnumsPathFor(openapiPath)
	lifecycleMatrixPath := lifecycleMatrixPathFor(openapiPath)
	nativeToolCatalogPath := nativeToolCatalogPathFor(openapiPath)

	switch args[0] {
	case "openapi":
		return writeOpenAPI(openapiPath)
	case "sdk-contracts":
		return writeSDKContracts(ctx, sdkContractsPath)
	case "loop-enums":
		return writeLoopEnums(ctx, loopEnumsPath)
	case "lifecycle-matrix":
		return writeLifecycleMatrix(lifecycleMatrixPath)
	case "native-tool-catalog":
		return writeNativeToolCatalog(nativeToolCatalogPath)
	case "all":
		return writeAll(ctx, openapiPath, sdkContractsPath, lifecycleMatrixPath, nativeToolCatalogPath)
	case subcommandCheck:
		if err := checkOpenAPI(openapiPath); err != nil {
			return err
		}
		if err := checkSDKContracts(ctx, sdkContractsPath); err != nil {
			return err
		}
		if err := checkLoopEnums(ctx, loopEnumsPath); err != nil {
			return err
		}
		if err := checkLifecycleMatrix(lifecycleMatrixPath); err != nil {
			return err
		}
		return checkNativeToolCatalog(nativeToolCatalogPath)
	default:
		return fmt.Errorf("unknown codegen target %q", args[0])
	}
}

func writeOpenAPI(path string) error {
	content, err := marshalOpenAPI()
	if err != nil {
		return fmt.Errorf("write openapi to %q: %w", path, err)
	}
	if err := publishGeneratedFile(path, content); err != nil {
		return fmt.Errorf("write openapi to %q: %w", path, err)
	}
	return nil
}

func writeSDKContracts(ctx context.Context, path string) error {
	content, err := generateFormattedSDKContracts(ctx, path)
	if err != nil {
		return err
	}
	if err := publishGeneratedFile(path, content); err != nil {
		return fmt.Errorf("write sdk contracts to %q: %w", path, err)
	}
	return nil
}

func writeAll(
	ctx context.Context,
	openapiPath string,
	sdkContractsPath string,
	lifecycleMatrixPath string,
	nativeToolCatalogPath string,
) error {
	if err := writeAllWith(
		ctx,
		openapiPath,
		sdkContractsPath,
		marshalOpenAPI,
		generateFormattedSDKContracts,
		publishGeneratedFile,
	); err != nil {
		return err
	}
	if err := writeLifecycleMatrix(lifecycleMatrixPath); err != nil {
		return err
	}
	if err := writeLoopEnums(ctx, loopEnumsPathFor(openapiPath)); err != nil {
		return err
	}
	return writeNativeToolCatalog(nativeToolCatalogPath)
}

func writeAllWith(
	ctx context.Context,
	openapiPath string,
	sdkContractsPath string,
	generateOpenAPI func() ([]byte, error),
	generateSDK func(context.Context, string) ([]byte, error),
	publishFile func(string, []byte) error,
) error {
	openapiContent, err := generateOpenAPI()
	if err != nil {
		return fmt.Errorf("write openapi to %q: %w", openapiPath, err)
	}

	sdkContent, err := generateSDK(ctx, sdkContractsPath)
	if err != nil {
		return err
	}

	previousOpenAPI, openapiExisted, err := readGeneratedFile(openapiPath)
	if err != nil {
		return fmt.Errorf("read existing openapi %q: %w", openapiPath, err)
	}

	if err := publishFile(openapiPath, openapiContent); err != nil {
		return fmt.Errorf("write openapi to %q: %w", openapiPath, err)
	}
	if err := publishFile(sdkContractsPath, sdkContent); err != nil {
		if restoreErr := restoreGeneratedFile(
			openapiPath,
			previousOpenAPI,
			openapiExisted,
			publishFile,
		); restoreErr != nil {
			return fmt.Errorf(
				"write sdk contracts to %q: %w; restore openapi %q: %v",
				sdkContractsPath,
				err,
				openapiPath,
				restoreErr,
			)
		}
		return fmt.Errorf("write sdk contracts to %q: %w", sdkContractsPath, err)
	}

	return nil
}

func publishGeneratedFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", filepath.Dir(path), err)
	}
	if err := fileutil.AtomicWriteFile(path, content, 0o600); err != nil {
		return err
	}
	return nil
}

func readGeneratedFile(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		return content, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, err
}

func restoreGeneratedFile(path string, content []byte, existed bool, publishFile func(string, []byte) error) error {
	if existed {
		return publishFile(path, content)
	}
	if err := fileutil.AtomicRemoveFile(path); err != nil {
		return err
	}
	return nil
}

func checkOpenAPI(path string) error {
	want, err := marshalOpenAPI()
	if err != nil {
		return err
	}
	return checkJSONFile(path, want)
}

func checkSDKContracts(ctx context.Context, path string) error {
	content, err := generateFormattedSDKContracts(ctx, path)
	if err != nil {
		return err
	}
	return checkFile(path, content)
}

func writeLifecycleMatrix(path string) error {
	content := generateLifecycleMatrixMDX()
	if err := publishGeneratedFile(path, content); err != nil {
		return fmt.Errorf("write lifecycle matrix to %q: %w", path, err)
	}
	return nil
}

func checkLifecycleMatrix(path string) error {
	return checkFile(path, generateLifecycleMatrixMDX())
}

func writeNativeToolCatalog(path string) error {
	content, err := generateNativeToolCatalog()
	if err != nil {
		return err
	}
	if err := publishGeneratedFile(path, content); err != nil {
		return fmt.Errorf("write native tool catalog to %q: %w", path, err)
	}
	return nil
}

func checkNativeToolCatalog(path string) error {
	content, err := generateNativeToolCatalog()
	if err != nil {
		return err
	}
	return checkJSONFile(path, content)
}

func marshalOpenAPI() ([]byte, error) {
	data, err := spec.Render()
	if err != nil {
		return nil, fmt.Errorf("render openapi: %w", err)
	}
	return data, nil
}

func checkFile(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s is missing; run codegen", path)
		}
		return fmt.Errorf("read %q: %w", path, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s: %w; run codegen", path, ErrStaleGeneratedFile)
	}
	return nil
}

func checkJSONFile(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s is missing; run codegen", path)
		}
		return fmt.Errorf("read %q: %w", path, err)
	}

	gotCanonical, err := canonicalJSON(got)
	if err != nil {
		return fmt.Errorf("decode %q: %w", path, err)
	}
	wantCanonical, err := canonicalJSON(want)
	if err != nil {
		return fmt.Errorf("decode generated json for %q: %w", path, err)
	}
	if !bytes.Equal(gotCanonical, wantCanonical) {
		return fmt.Errorf("%s: %w; run codegen", path, ErrStaleGeneratedFile)
	}
	return nil
}

func canonicalJSON(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func generateFormattedSDKContracts(ctx context.Context, path string) ([]byte, error) {
	content, err := sdkts.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate sdk contracts: %w", err)
	}
	formatted, err := formatTypeScript(ctx, path, []byte(content))
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

func lifecycleMatrixPathFor(openapiPath string) string {
	if filepath.Clean(openapiPath) == filepath.Clean(spec.DefaultPath) {
		return defaultLifecycleMatrixPath
	}
	return filepath.Join(filepath.Dir(openapiPath), "lifecycle-matrix.mdx")
}

func nativeToolCatalogPathFor(openapiPath string) string {
	if filepath.Clean(openapiPath) == filepath.Clean(spec.DefaultPath) {
		return defaultNativeToolCatalogPath
	}
	return filepath.Join(filepath.Dir(openapiPath), "native-tool-catalog.json")
}

func loopEnumsPathFor(openapiPath string) string {
	if filepath.Clean(openapiPath) == filepath.Clean(spec.DefaultPath) {
		return defaultLoopEnumsPath
	}
	return filepath.Join(filepath.Dir(openapiPath), "loop-enums.ts")
}

func formatTypeScript(ctx context.Context, path string, content []byte) ([]byte, error) {
	argv := jsbin.Argv(".", "oxfmt", "--stdin-filepath", path)
	//nolint:gosec // argv[0] comes from jsbin.Argv with a constant tool name: workspace shim or bunx fallback.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(content)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return nil, fmt.Errorf("format typescript %q with oxfmt: %w", path, err)
		}
		return nil, fmt.Errorf("format typescript %q with oxfmt: %w: %s", path, err, detail)
	}
	return stdout.Bytes(), nil
}

func removeTemporaryFile(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove temporary file %q: %w", path, err)
}
