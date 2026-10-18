package openapits

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// SDKArtifact describes the two generated SDK jobs and their shared source.
type SDKArtifact struct {
	SpecPath   string
	ConfigPath string
	OutputPath string
}

//go:embed generate-sdk.mjs
var sdkGenerator string

// GenerateSDK publishes the public resource SDK and internal stream bindings.
func GenerateSDK(ctx context.Context, artifact SDKArtifact) error {
	return withSDKOutput(ctx, artifact, func(generated string) error {
		return publishSDKOutput(generated, artifact.OutputPath, os.Rename)
	})
}

// CheckSDK rejects missing, changed, or obsolete generated SDK files.
func CheckSDK(ctx context.Context, artifact SDKArtifact) error {
	return withSDKOutput(ctx, artifact, func(generated string) error {
		for _, job := range []string{"sdk", "sse"} {
			if err := checkSDKDirectory(filepath.Join(artifact.OutputPath, job), filepath.Join(generated, job)); err != nil {
				return err
			}
		}
		return nil
	})
}

func withSDKOutput(ctx context.Context, artifact SDKArtifact, use func(string) error) (err error) {
	if err := artifact.validate(); err != nil {
		return err
	}
	parent := filepath.Dir(artifact.OutputPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create SDK parent directory: %w", err)
	}
	dir, err := os.MkdirTemp(parent, ".app-sdk-*")
	if err != nil {
		return fmt.Errorf("create temporary SDK output: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove temporary SDK output: %w", removeErr))
		}
	}()
	if err := runCommand(ctx, "bun", "--eval", sdkGenerator, "--", artifact.SpecPath, artifact.ConfigPath, dir); err != nil {
		return fmt.Errorf("generate TypeScript SDK: %w", err)
	}
	if err := (execRunner{}).Run(ctx, "oxfmt", dir); err != nil {
		return fmt.Errorf("format TypeScript SDK: %w", err)
	}
	return use(dir)
}

func (artifact SDKArtifact) validate() error {
	if err := (Artifact{SpecPath: artifact.SpecPath, OutputPath: artifact.OutputPath}).validate(); err != nil {
		return err
	}
	if artifact.ConfigPath == "" {
		return fmt.Errorf("%w: SDK config path is required", ErrInvalidArtifact)
	}
	return nil
}

func checkSDKDirectory(committed, generated string) error {
	files := make(map[string]bool)
	if err := filepath.WalkDir(generated, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk generated SDK output: %w", walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(generated, path)
		if err != nil {
			return fmt.Errorf("resolve generated SDK file: %w", err)
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read generated SDK file: %w", err)
		}
		files[relative] = true
		return checkGeneratedFile(filepath.Join(committed, relative), want)
	}); err != nil {
		return err
	}
	return filepath.WalkDir(committed, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk committed SDK output: %w", walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(committed, path)
		if err != nil {
			return fmt.Errorf("resolve committed SDK file: %w", err)
		}
		if !files[relative] {
			return fmt.Errorf("%s: %w; obsolete SDK file, run codegen", path, ErrStaleGeneratedFile)
		}
		return nil
	})
}

type sdkPublication struct {
	job       string
	backedUp  bool
	published bool
}

func publishSDKOutput(generated, output string, rename func(string, string) error) (err error) {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return fmt.Errorf("create SDK output: %w", err)
	}
	backup, err := os.MkdirTemp(filepath.Dir(output), ".app-sdk-backup-*")
	if err != nil {
		return fmt.Errorf("create SDK backup: %w", err)
	}
	jobs := []sdkPublication{{job: "sdk"}, {job: "sse"}}
	defer func() {
		if err != nil {
			if rollbackErr := rollbackSDKPublication(output, backup, jobs); rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("SDK rollback failed; backups retained at %s: %w", backup, rollbackErr))
				return
			}
		}
		if cleanupErr := os.RemoveAll(backup); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove SDK backup: %w", cleanupErr))
		}
	}()
	for index := range jobs {
		job := &jobs[index]
		destination := filepath.Join(output, job.job)
		if _, statErr := os.Stat(destination); statErr == nil {
			if err := rename(destination, filepath.Join(backup, job.job)); err != nil {
				return fmt.Errorf("back up SDK job %q: %w", job.job, err)
			}
			job.backedUp = true
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspect SDK job %q: %w", job.job, statErr)
		}
	}
	for index := range jobs {
		job := &jobs[index]
		if err := rename(filepath.Join(generated, job.job), filepath.Join(output, job.job)); err != nil {
			return fmt.Errorf("publish SDK job %q: %w", job.job, err)
		}
		job.published = true
	}
	return nil
}

func rollbackSDKPublication(output, backup string, jobs []sdkPublication) (err error) {
	for index := len(jobs) - 1; index >= 0; index-- {
		job := jobs[index]
		destination := filepath.Join(output, job.job)
		if job.published {
			if removeErr := os.RemoveAll(destination); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("remove new SDK job %q: %w", job.job, removeErr))
				continue
			}
		}
		if job.backedUp {
			if renameErr := os.Rename(filepath.Join(backup, job.job), destination); renameErr != nil {
				err = errors.Join(err, fmt.Errorf("restore SDK job %q: %w", job.job, renameErr))
			}
		}
	}
	return err
}
