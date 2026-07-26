//go:build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ReleaseWebAssetsSync() error {
	return releaseWebAssetsSync(context.Background())
}

func releaseWebAssetsSync(ctx context.Context) error {
	if err := WebAssetsDeterminismCheck(); err != nil {
		return err
	}
	buildDigest, err := directoryDigest(webDistDir)
	if err != nil {
		return fmt.Errorf("digest local web build: %w", err)
	}
	sourceCommit, err := gitCommandOutput(ctx, ".", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve release source commit: %w", err)
	}
	token := webAssetsPublishToken()
	if token == "" {
		return fmt.Errorf(
			"%s or %s is required to publish %s",
			webAssetsTokenEnvVar,
			releaseTokenEnvVar,
			webAssetsModulePath,
		)
	}
	gitCredentials, err := newWebAssetsGitCredentials(token)
	if err != nil {
		return err
	}
	defer gitCredentials.cleanup()

	assetsRepoDir, err := os.MkdirTemp("", "agh-web-assets-release-sync-")
	if err != nil {
		return fmt.Errorf("create web assets sync repo dir: %w", err)
	}
	defer os.RemoveAll(assetsRepoDir)

	if err := cloneWebAssetsRepository(ctx, assetsRepoDir, gitCredentials.env); err != nil {
		return err
	}
	metadata := webAssetsMetadata{
		BuildDigest:      buildDigest,
		SourceRepository: webAssetsSourceRepository,
		SourceCommit:     sourceCommit,
	}
	tag, err := publishWebAssetsModule(ctx, assetsRepoDir, metadata, gitCredentials.env)
	if err != nil {
		return err
	}
	if err := waitForWebAssetsPublic(ctx, tag); err != nil {
		return err
	}
	if err := pinWebAssetsModule(ctx, tag); err != nil {
		return err
	}
	fmt.Printf("Pinned %s@%s for web assets digest %s\n", webAssetsModulePath, tag, buildDigest)
	return nil
}

func webAssetsPublishToken() string {
	if token := strings.TrimSpace(os.Getenv(webAssetsTokenEnvVar)); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv(releaseTokenEnvVar))
}

func newWebAssetsGitCredentials(token string) (*webAssetsGitCredentials, error) {
	askpassDir, err := os.MkdirTemp("", "agh-web-assets-askpass-")
	if err != nil {
		return nil, fmt.Errorf("create web assets git credentials dir: %w", err)
	}

	askpassPath := filepath.Join(askpassDir, "askpass.sh")
	askpassScript := strings.Join([]string{
		"#!/bin/sh",
		"case \"$1\" in",
		"*Username*) printf '%s\\n' x-access-token ;;",
		"*Password*) printf '%s\\n' \"$AGH_WEB_ASSETS_GIT_TOKEN\" ;;",
		"*) printf '\\n' ;;",
		"esac",
		"",
	}, "\n")
	if err := os.WriteFile(askpassPath, []byte(askpassScript), 0o700); err != nil {
		return nil, fmt.Errorf("write web assets git credentials helper: %w", err)
	}
	return &webAssetsGitCredentials{
		dir: askpassDir,
		env: map[string]string{
			"AGH_WEB_ASSETS_GIT_TOKEN": token,
			"GIT_ASKPASS":              askpassPath,
			"GIT_TERMINAL_PROMPT":      "0",
		},
	}, nil
}

func (c *webAssetsGitCredentials) cleanup() {
	if c == nil || c.dir == "" {
		return
	}
	if err := os.RemoveAll(c.dir); err != nil {
		fmt.Printf("Warning: remove web assets git credentials dir %s: %v\n", c.dir, err)
	}
}

func cloneWebAssetsRepository(ctx context.Context, destDir string, gitEnv map[string]string) error {
	if err := runCommandInDirWithEnv(
		ctx,
		".",
		gitEnv,
		"git",
		"clone",
		"--no-single-branch",
		webAssetsRemoteURL,
		destDir,
	); err != nil {
		return fmt.Errorf("clone %s: %w", webAssetsRemoteURL, err)
	}
	return nil
}

func publishWebAssetsModule(
	ctx context.Context,
	assetsRepoDir string,
	metadata webAssetsMetadata,
	gitEnv map[string]string,
) (string, error) {
	if err := runCommandInDirWithEnv(ctx, assetsRepoDir, gitEnv, "git", "fetch", "--tags", "origin"); err != nil {
		return "", fmt.Errorf("fetch web assets tags: %w", err)
	}
	if err := runCommandInDir(ctx, assetsRepoDir, "git", "config", "user.name", "github-actions[bot]"); err != nil {
		return "", fmt.Errorf("configure web assets git user name: %w", err)
	}
	if err := runCommandInDir(
		ctx,
		assetsRepoDir,
		"git",
		"config",
		"user.email",
		"github-actions[bot]@users.noreply.github.com",
	); err != nil {
		return "", fmt.Errorf("configure web assets git user email: %w", err)
	}

	tag, err := matchingWebAssetsTag(ctx, assetsRepoDir, metadata)
	if err != nil {
		return "", err
	}
	if tag != "" {
		fmt.Printf("Reusing existing assets tag %s for %s\n", tag, metadata.SourceCommit)
		return tag, nil
	}

	if err := prepareWebAssetsRepo(webDistDir, assetsRepoDir, metadata); err != nil {
		return "", err
	}
	hasDiff, err := gitHasDiff(ctx, assetsRepoDir, webAssetsModuleDistDir, webAssetsMetadataFile)
	if err != nil {
		return "", err
	}
	if hasDiff {
		if err := runCommandInDir(
			ctx,
			assetsRepoDir,
			"git",
			"add",
			webAssetsModuleDistDir,
			webAssetsMetadataFile,
		); err != nil {
			return "", fmt.Errorf("stage web assets module update: %w", err)
		}
		message := fmt.Sprintf("build: sync AGH web assets %s", shortCommit(metadata.SourceCommit))
		if err := runCommandInDir(ctx, assetsRepoDir, "git", "commit", "-m", message); err != nil {
			return "", fmt.Errorf("commit web assets module update: %w", err)
		}
	}

	tags, err := gitTags(assetsRepoDir)
	if err != nil {
		return "", err
	}
	nextTag, err := nextWebAssetsTag(tags)
	if err != nil {
		return "", err
	}
	if err := runCommandInDir(
		ctx,
		assetsRepoDir,
		"git",
		"tag",
		"-a",
		nextTag,
		"-m",
		"AGH web assets "+metadata.SourceCommit,
	); err != nil {
		return "", fmt.Errorf("tag web assets module %s: %w", nextTag, err)
	}
	if hasDiff {
		if err := runCommandInDirWithEnv(
			ctx,
			assetsRepoDir,
			gitEnv,
			"git",
			"push",
			"origin",
			"HEAD:main",
			nextTag,
		); err != nil {
			return "", fmt.Errorf("push web assets module update %s: %w", nextTag, err)
		}
		return nextTag, nil
	}
	if err := runCommandInDirWithEnv(ctx, assetsRepoDir, gitEnv, "git", "push", "origin", nextTag); err != nil {
		return "", fmt.Errorf("push web assets module tag %s: %w", nextTag, err)
	}
	return nextTag, nil
}

func matchingWebAssetsTag(ctx context.Context, assetsRepoDir string, metadata webAssetsMetadata) (string, error) {
	output, err := gitCommandOutput(ctx, assetsRepoDir, "tag", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		return "", fmt.Errorf("list web assets tags: %w", err)
	}
	for _, line := range strings.Split(output, "\n") {
		tag := strings.TrimSpace(line)
		if tag == "" {
			continue
		}
		source, ok, err := gitShowFile(ctx, assetsRepoDir, tag, webAssetsMetadataFile)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		tagMetadata := parseWebAssetsMetadataSource(source)
		if tagMetadata.BuildDigest == metadata.BuildDigest && tagMetadata.SourceCommit == metadata.SourceCommit {
			return tag, nil
		}
	}
	return "", nil
}

func shortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}

func waitForWebAssetsPublic(ctx context.Context, version string) error {
	const attempts = 30
	const delay = 20 * time.Second

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := webAssetsPublicCheck(ctx, version); err != nil {
			lastErr = err
		} else {
			return nil
		}
		if attempt == attempts {
			break
		}
		fmt.Printf(
			"Waiting for %s@%s to resolve publicly (attempt %d/%d): %v\n",
			webAssetsModulePath,
			version,
			attempt,
			attempts,
			lastErr,
		)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("wait for public %s@%s: %w", webAssetsModulePath, version, ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf(
		"%s@%s did not resolve publicly after %d attempts: %w",
		webAssetsModulePath,
		version,
		attempts,
		lastErr,
	)
}

func pinWebAssetsModule(ctx context.Context, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("web assets module version is required")
	}
	moduleVersion := webAssetsModulePath + "@" + version
	env := webAssetsPublicModuleEnv("")
	if err := runCommandInDirWithEnv(ctx, ".", env, "go", "get", moduleVersion); err != nil {
		return fmt.Errorf("pin %s: %w", moduleVersion, err)
	}
	if err := runCommandInDirWithEnv(ctx, ".", env, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy after pinning %s: %w", moduleVersion, err)
	}
	return nil
}
