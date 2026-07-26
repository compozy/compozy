package openapits

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/codegen/jsbin"
)

// Artifact describes one checked-in OpenAPI document and its generated type output.
type Artifact struct {
	SpecPath   string
	OutputPath string
}

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type temporaryOutputFile interface {
	Name() string
	Close() error
}

type temporaryOutputFactory func(dir string, pattern string) (temporaryOutputFile, error)

type execRunner struct{}

var _ commandRunner = execRunner{}

const openAPITypescriptTool = "openapi-typescript"

//go:embed generate.mjs
var openAPITSTypeGenerator string

// Run resolves JS tools to workspace-pinned binaries. OpenAPI generation uses
// an embedded Bun wrapper so marked mixed-shape maps remain assignable in the
// generated TypeScript without weakening the source OpenAPI schema.
func (execRunner) Run(ctx context.Context, tool string, args ...string) error {
	if tool == openAPITypescriptTool {
		argv := append([]string{"--eval", openAPITSTypeGenerator, "--"}, args...)
		return runCommand(ctx, "bun", argv...)
	}
	argv := jsbin.Argv(".", tool, args...)
	return runCommand(ctx, argv[0], argv[1:]...)
}

func createOSTemporaryOutput(dir string, pattern string) (temporaryOutputFile, error) {
	return os.CreateTemp(dir, pattern)
}

// ErrInvalidArtifact reports an unusable OpenAPI TypeScript generation artifact.
var ErrInvalidArtifact = errors.New("invalid openapi types artifact")

// ErrStaleGeneratedFile reports that the committed generated file no longer matches the source spec.
var ErrStaleGeneratedFile = errors.New("generated file is stale")

// ErrMissingGeneratedFile reports that the committed generated file does not exist.
var ErrMissingGeneratedFile = errors.New("generated file is missing")

// Generate runs openapi-typescript for one artifact and formats the generated output with oxfmt.
func Generate(ctx context.Context, artifact Artifact) error {
	return generateWithRunner(ctx, artifact, execRunner{})
}

func generateWithRunner(ctx context.Context, artifact Artifact, runner commandRunner) (err error) {
	if err := artifact.validate(); err != nil {
		return err
	}
	outputDir := filepath.Dir(artifact.OutputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", outputDir, err)
	}

	return withClosedTemporaryOutput(
		outputDir,
		".openapi-types-*.d.ts",
		artifact.OutputPath,
		createOSTemporaryOutput,
		func(tempPath string) error {
			if err := runner.Run(ctx, openAPITypescriptTool, artifact.SpecPath, "-o", tempPath); err != nil {
				return fmt.Errorf("generate %q from %q: %w", artifact.OutputPath, artifact.SpecPath, err)
			}

			if err := runner.Run(ctx, "oxfmt", tempPath); err != nil {
				return fmt.Errorf("format %q: %w", artifact.OutputPath, err)
			}

			if err := os.Rename(tempPath, artifact.OutputPath); err != nil {
				return fmt.Errorf("publish %q: %w", artifact.OutputPath, err)
			}

			return nil
		},
	)
}

// Check regenerates one artifact into a temporary file and fails when the checked-in output differs.
func Check(ctx context.Context, artifact Artifact) (err error) {
	if err := artifact.validate(); err != nil {
		return err
	}
	return withClosedTemporaryOutput(
		"",
		"openapi-types-*.d.ts",
		artifact.OutputPath,
		createOSTemporaryOutput,
		func(tempPath string) error {
			if err := Generate(ctx, Artifact{
				SpecPath:   artifact.SpecPath,
				OutputPath: tempPath,
			}); err != nil {
				return err
			}

			want, err := os.ReadFile(tempPath)
			if err != nil {
				return fmt.Errorf("read generated output %q: %w", tempPath, err)
			}

			return checkGeneratedFile(artifact.OutputPath, want)
		},
	)
}

func withClosedTemporaryOutput(
	dir string,
	pattern string,
	outputPath string,
	create temporaryOutputFactory,
	use func(tempPath string) error,
) (err error) {
	file, err := create(dir, pattern)
	if err != nil {
		return fmt.Errorf("create temporary output for %q: %w", outputPath, err)
	}
	tempPath := file.Name()
	defer func() {
		if removeErr := removeFile(tempPath); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
	}()

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary output for %q: %w", outputPath, err)
	}
	return use(tempPath)
}

func (artifact Artifact) validate() error {
	switch {
	case artifact.SpecPath == "":
		return fmt.Errorf("%w: spec path is required", ErrInvalidArtifact)
	case artifact.OutputPath == "":
		return fmt.Errorf("%w: output path is required", ErrInvalidArtifact)
	case filepath.Clean(artifact.SpecPath) == filepath.Clean(artifact.OutputPath):
		return fmt.Errorf("%w: spec path and output path must differ", ErrInvalidArtifact)
	default:
		return nil
	}
}

func checkGeneratedFile(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s: %w; run codegen", path, ErrMissingGeneratedFile)
		}
		return fmt.Errorf("read %q: %w", path, err)
	}

	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s: %w; run codegen", path, ErrStaleGeneratedFile)
	}

	return nil
}

func removeFile(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove temporary output %q: %w", path, err)
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), ctxErr)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, detail)
	}

	return nil
}
