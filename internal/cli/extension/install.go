package extension

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	huh "charm.land/huh/v2"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/spf13/cobra"
)

const noneLabel = "(none)"

func newInstallCommand(deps commandDeps) *cobra.Command {
	var yes bool
	var remote string
	var ref string
	var subdir string

	cmd := &cobra.Command{
		Use:          "install <source>",
		Short:        "Install an extension into the user scope",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallCommand(cmd, deps, args[0], yes, installSourceOptions{
				Remote: installRemote(remote),
				Ref:    ref,
				Subdir: subdir,
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the install confirmation prompt")
	cmd.Flags().StringVar(&remote, "remote", string(installRemoteLocal), "Extension source type: local or github")
	cmd.Flags().StringVar(&ref, "ref", "", "Git ref to install when using --remote github")
	cmd.Flags().
		StringVar(&subdir, "subdir", "", "Repository subdirectory containing the extension when using --remote github")
	return cmd
}

func runInstallCommand(
	cmd *cobra.Command,
	deps commandDeps,
	rawSource string,
	yes bool,
	options installSourceOptions,
) (err error) {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	env, err := deps.resolveEnv(ctx)
	if err != nil {
		return err
	}

	resolvedSource, err := resolveInstallSourceForCommand(ctx, deps, rawSource, options)
	if err != nil {
		return err
	}
	if resolvedSource.CleanupSource != nil {
		defer func() {
			cleanupErr := resolvedSource.CleanupSource()
			if cleanupErr == nil {
				return
			}
			if err == nil {
				err = fmt.Errorf("cleanup install source: %w", cleanupErr)
				return
			}
			err = errors.Join(err, fmt.Errorf("cleanup install source: %w", cleanupErr))
		}()
	}

	manifest, installPath, prompt, err := prepareInstallRequest(ctx, deps, env, resolvedSource)
	if err != nil {
		return err
	}

	if err := writeInstallPlan(cmd, prompt); err != nil {
		return err
	}
	if err := confirmInstall(cmd, deps, prompt, yes); err != nil {
		return err
	}
	if err := installResolvedSource(deps, resolvedSource, installPath, manifest.Extension.Name); err != nil {
		return err
	}
	if err := disableInstalledUserExtension(ctx, env, installPath, manifest.Extension.Name, deps); err != nil {
		return err
	}
	return writeInstallSummary(cmd, prompt, installPath)
}

func newUninstallCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:          "uninstall <name>",
		Short:        "Remove a user-scoped extension from the local machine",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstallCommand(cmd, deps, args[0])
		},
	}
}

func runUninstallCommand(cmd *cobra.Command, deps commandDeps, rawName string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	name, err := normalizeExtensionName(rawName)
	if err != nil {
		return err
	}

	env, err := deps.resolveEnv(ctx)
	if err != nil {
		return err
	}

	userPath := filepath.Join(userExtensionsRoot(env.homeDir), name)
	exists, err := deps.pathExists(userPath)
	if err != nil {
		return fmt.Errorf("inspect user extension path %q: %w", userPath, err)
	}
	if exists {
		if err := deps.removeAll(userPath); err != nil {
			return fmt.Errorf("remove user extension %q: %w", name, err)
		}
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"Uninstalled user extension %q from %s.\n",
			name,
			userPath,
		); err != nil {
			return fmt.Errorf("write uninstall summary: %w", err)
		}
		return nil
	}

	result, err := deps.discoverAll(ctx, env)
	if err != nil {
		return err
	}
	if entry, ok := findEffectiveExtension(result, name); ok {
		switch entry.Ref.Source {
		case extensions.SourceBundled:
			return fmt.Errorf("refuse to uninstall bundled extension %q", name)
		case extensions.SourceWorkspace:
			return fmt.Errorf("refuse to uninstall workspace extension %q from %s", name, entry.ExtensionDir)
		}
	}

	return fmt.Errorf("user extension %q is not installed", name)
}

func writeInstallPlan(cmd *cobra.Command, prompt installPrompt) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"Extension: %s\nSource: %s\nInstall path: %s\nCapabilities: %s\nSetup assets: %s\n",
		prompt.Name,
		prompt.Source,
		prompt.InstallPath,
		renderCapabilities(prompt.Capabilities),
		renderSetupAssets(prompt.SetupAssets),
	); err != nil {
		return fmt.Errorf("write install plan: %w", err)
	}
	return nil
}

func manifestSetupAssets(manifest *extensions.Manifest) []string {
	if manifest == nil {
		return nil
	}

	setupAssets := make([]string, 0, 2)
	if len(manifest.Resources.Skills) > 0 {
		setupAssets = append(setupAssets, "skills")
	}
	if len(manifest.Resources.Agents) > 0 {
		setupAssets = append(setupAssets, "reusable agents")
	}
	return setupAssets
}

func renderSetupAssets(values []string) string {
	if len(values) == 0 {
		return noneLabel
	}
	return strings.Join(values, ", ")
}

func writeSetupHint(cmd *cobra.Command, setupAssets []string, message string) error {
	if len(setupAssets) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"This extension ships %s.\n%s\n",
		renderSetupAssets(setupAssets),
		message,
	); err != nil {
		return fmt.Errorf("write setup hint: %w", err)
	}
	return nil
}

func resolveInstallSourceForCommand(
	ctx context.Context,
	deps commandDeps,
	rawSource string,
	options installSourceOptions,
) (resolvedInstallSource, error) {
	resolveSource := deps.resolveInstallSource
	if resolveSource == nil {
		resolveSource = resolveInstallSource
	}
	return resolveSource(ctx, rawSource, options)
}

func prepareInstallRequest(
	ctx context.Context,
	deps commandDeps,
	env commandEnv,
	resolvedSource resolvedInstallSource,
) (*extensions.Manifest, string, installPrompt, error) {
	manifest, err := deps.loadManifest(ctx, resolvedSource.SourcePath)
	if err != nil {
		return nil, "", installPrompt{}, fmt.Errorf(
			"load extension manifest from %q: %w",
			resolvedSource.SourcePath,
			err,
		)
	}

	installPath := filepath.Join(userExtensionsRoot(env.homeDir), manifest.Extension.Name)
	if sameInstallPath(resolvedSource.SourcePath, installPath) {
		return nil, "", installPrompt{}, fmt.Errorf(
			"extension %q is already installed at %s",
			manifest.Extension.Name,
			installPath,
		)
	}
	if err := ensureInstallTargetAvailable(deps, installPath, manifest.Extension.Name); err != nil {
		return nil, "", installPrompt{}, err
	}

	return manifest, installPath, installPrompt{
		Name:         manifest.Extension.Name,
		Source:       resolvedSource.DisplaySource,
		InstallPath:  installPath,
		Capabilities: manifest.Security.Capabilities,
		SetupAssets:  manifestSetupAssets(manifest),
	}, nil
}

func installResolvedSource(
	deps commandDeps,
	resolvedSource resolvedInstallSource,
	installPath string,
	name string,
) error {
	installPathExisted, err := deps.pathExists(installPath)
	if err != nil {
		return fmt.Errorf("inspect install target %q before copy: %w", installPath, err)
	}
	if err := deps.copyDir(resolvedSource.SourcePath, installPath); err != nil {
		return cleanupFailedInstall(
			deps,
			installPath,
			!installPathExisted,
			fmt.Errorf("copy extension into user scope: %w", err),
		)
	}

	if resolvedSource.InstallOrigin == nil {
		return nil
	}
	if err := deps.writeInstallOrigin(installPath, *resolvedSource.InstallOrigin); err != nil {
		return cleanupFailedInstall(
			deps,
			installPath,
			true,
			fmt.Errorf("record install provenance for %q: %w", name, err),
		)
	}
	return nil
}

func disableInstalledUserExtension(
	ctx context.Context,
	env commandEnv,
	installPath string,
	name string,
	deps commandDeps,
) error {
	ref := extensions.Ref{Name: name, Source: extensions.SourceUser}
	if err := env.store.Disable(ctx, ref); err != nil {
		return cleanupFailedInstall(
			deps,
			installPath,
			true,
			fmt.Errorf("record initial disabled state for %q: %w", name, err),
		)
	}
	return nil
}

func cleanupFailedInstall(deps commandDeps, installPath string, shouldRemove bool, cause error) error {
	if !shouldRemove {
		return cause
	}
	cleanupErr := deps.removeAll(installPath)
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(cause, fmt.Errorf("cleanup failed at %q: %w", installPath, cleanupErr))
}

func writeInstallSummary(cmd *cobra.Command, prompt installPrompt, installPath string) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"Installed extension %q into %s.\nLocal state recorded as disabled; run `compozy ext enable %s` to activate it on this machine.\n",
		prompt.Name,
		installPath,
		prompt.Name,
	); err != nil {
		return fmt.Errorf("write install summary: %w", err)
	}
	return writeSetupHint(
		cmd,
		prompt.SetupAssets,
		"After enabling it, run `compozy setup` to install its setup assets.",
	)
}

func confirmInstall(cmd *cobra.Command, deps commandDeps, prompt installPrompt, yes bool) error {
	if yes {
		return nil
	}
	if !deps.isInteractive() {
		return fmt.Errorf("%s requires --yes in non-interactive mode", cmd.CommandPath())
	}

	confirmed, err := deps.confirmInstall(cmd, prompt)
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("extension install canceled")
	}
	return nil
}

func confirmInstallPrompt(_ *cobra.Command, prompt installPrompt) (bool, error) {
	confirmed := false
	field := huh.NewConfirm().
		Key("confirm").
		Title(fmt.Sprintf("Install extension %q?", prompt.Name)).
		Description(
			fmt.Sprintf(
				"The extension requests: %s. It will stay disabled until you explicitly enable it on this machine.",
				renderCapabilities(prompt.Capabilities),
			),
		).
		Value(&confirmed)
	if err := runPromptField(field); err != nil {
		return false, fmt.Errorf("confirm extension install: %w", err)
	}
	return confirmed, nil
}

func ensureInstallTargetAvailable(deps commandDeps, installPath string, name string) error {
	exists, err := deps.pathExists(installPath)
	if err != nil {
		return fmt.Errorf("inspect install target %q: %w", installPath, err)
	}
	if exists {
		return fmt.Errorf("user extension %q already exists at %s", name, installPath)
	}
	return nil
}

func userExtensionsRoot(homeDir string) string {
	return filepath.Join(homeDir, ".compozy", "extensions")
}

func sameInstallPath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func copyDirectoryTree(sourceDir string, destDir string) error {
	if err := validateCopyTarget(sourceDir, destDir); err != nil {
		return err
	}

	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return copyDirectoryEntry(sourceDir, destDir, path, entry)
	})
}

func validateCopyTarget(sourceDir string, destDir string) error {
	source := filepath.Clean(sourceDir)
	dest := filepath.Clean(destDir)
	if sameInstallPath(source, dest) {
		return fmt.Errorf("source and destination must differ")
	}

	relative, err := filepath.Rel(source, dest)
	if err != nil {
		return fmt.Errorf("compare copy paths: %w", err)
	}
	if relative == "." || (!strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && relative != "..") {
		return fmt.Errorf("destination %q must not be inside source %q", dest, source)
	}
	return nil
}

func copyDirectoryEntry(sourceDir string, destDir string, path string, entry fs.DirEntry) error {
	relativePath, err := filepath.Rel(sourceDir, path)
	if err != nil {
		return fmt.Errorf("resolve copied path %q: %w", path, err)
	}
	targetPath := filepath.Join(destDir, relativePath)

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("inspect %q: %w", path, err)
	}

	switch {
	case entry.Type()&os.ModeSymlink != 0:
		return copySymlink(sourceDir, destDir, path, targetPath)
	case entry.IsDir():
		if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
			return fmt.Errorf("create directory %q: %w", targetPath, err)
		}
		return nil
	default:
		return copyRegularFile(path, targetPath, info.Mode().Perm())
	}
}

func copySymlink(sourceRoot string, destRoot string, sourcePath string, targetPath string) error {
	linkTarget, err := os.Readlink(sourcePath)
	if err != nil {
		return fmt.Errorf("read symlink %q: %w", sourcePath, err)
	}

	safeTarget, err := sanitizedSymlinkTarget(sourceRoot, destRoot, sourcePath, targetPath, linkTarget)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create symlink parent %q: %w", filepath.Dir(targetPath), err)
	}
	if err := os.Symlink(safeTarget, targetPath); err != nil {
		return fmt.Errorf("create symlink %q: %w", targetPath, err)
	}
	return nil
}

func sanitizedSymlinkTarget(
	sourceRoot string,
	destRoot string,
	sourcePath string,
	targetPath string,
	linkTarget string,
) (string, error) {
	resolvedSourceRoot, err := filepath.EvalSymlinks(filepath.Clean(sourceRoot))
	if err != nil {
		return "", fmt.Errorf("resolve extension root %q: %w", sourceRoot, err)
	}

	resolvedTarget := linkTarget
	if !filepath.IsAbs(resolvedTarget) {
		resolvedTarget = filepath.Join(filepath.Dir(sourcePath), resolvedTarget)
	}
	resolvedTarget, err = filepath.EvalSymlinks(filepath.Clean(resolvedTarget))
	if err != nil {
		return "", fmt.Errorf("resolve symlink target %q for %q: %w", linkTarget, sourcePath, err)
	}

	relativeToRoot, err := filepath.Rel(resolvedSourceRoot, resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("compare symlink target %q against root %q: %w", resolvedTarget, resolvedSourceRoot, err)
	}
	if pathEscapesRoot(relativeToRoot) {
		return "", fmt.Errorf("symlink %q points outside extension root: %q", sourcePath, linkTarget)
	}

	destResolvedTarget := filepath.Join(destRoot, relativeToRoot)
	safeTarget, err := filepath.Rel(filepath.Dir(targetPath), destResolvedTarget)
	if err != nil {
		return "", fmt.Errorf("build destination symlink target for %q: %w", targetPath, err)
	}
	return filepath.Clean(safeTarget), nil
}

func pathEscapesRoot(relativePath string) bool {
	return relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator))
}

func copyRegularFile(sourcePath string, targetPath string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory %q: %w", filepath.Dir(targetPath), err)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("create destination file %q: %w", targetPath, err)
	}

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		closeErr := targetFile.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return fmt.Errorf("copy file %q: %w", sourcePath, err)
	}
	if err := targetFile.Close(); err != nil {
		return fmt.Errorf("close destination file %q: %w", targetPath, err)
	}
	return nil
}
