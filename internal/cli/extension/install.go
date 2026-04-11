package extension

import (
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

func newInstallCommand(deps commandDeps) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:          "install <path>",
		Short:        "Install an extension into the user scope",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallCommand(cmd, deps, args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the install confirmation prompt")
	return cmd
}

func runInstallCommand(cmd *cobra.Command, deps commandDeps, rawPath string, yes bool) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	env, err := deps.resolveEnv(ctx)
	if err != nil {
		return err
	}

	sourcePath, err := resolveSourcePath(rawPath)
	if err != nil {
		return err
	}

	manifest, err := deps.loadManifest(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("load extension manifest from %q: %w", sourcePath, err)
	}

	installPath := filepath.Join(userExtensionsRoot(env.homeDir), manifest.Extension.Name)
	if sameInstallPath(sourcePath, installPath) {
		return fmt.Errorf("extension %q is already installed at %s", manifest.Extension.Name, installPath)
	}
	if err := ensureInstallTargetAvailable(deps, installPath, manifest.Extension.Name); err != nil {
		return err
	}

	if err := writeInstallPlan(cmd, installPrompt{
		Name:         manifest.Extension.Name,
		SourcePath:   sourcePath,
		InstallPath:  installPath,
		Capabilities: manifest.Security.Capabilities,
	}); err != nil {
		return err
	}

	if err := confirmInstall(cmd, deps, installPrompt{
		Name:         manifest.Extension.Name,
		SourcePath:   sourcePath,
		InstallPath:  installPath,
		Capabilities: manifest.Security.Capabilities,
	}, yes); err != nil {
		return err
	}

	if err := deps.copyDir(sourcePath, installPath); err != nil {
		return fmt.Errorf("copy extension into user scope: %w", err)
	}

	ref := extensions.Ref{Name: manifest.Extension.Name, Source: extensions.SourceUser}
	if err := env.store.Disable(ctx, ref); err != nil {
		cleanupErr := deps.removeAll(installPath)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup failed at %q: %w", installPath, cleanupErr))
		}
		return fmt.Errorf("record initial disabled state for %q: %w", manifest.Extension.Name, err)
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"Installed extension %q into %s.\nLocal state recorded as disabled; run `compozy ext enable %s` to activate it on this machine.\n",
		manifest.Extension.Name,
		installPath,
		manifest.Extension.Name,
	); err != nil {
		return fmt.Errorf("write install summary: %w", err)
	}

	return nil
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
		"Extension: %s\nSource path: %s\nInstall path: %s\nCapabilities: %s\n",
		prompt.Name,
		prompt.SourcePath,
		prompt.InstallPath,
		renderCapabilities(prompt.Capabilities),
	); err != nil {
		return fmt.Errorf("write install plan: %w", err)
	}
	return nil
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

func resolveSourcePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", fmt.Errorf("extension path is required")
	}

	absolutePath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve extension path %q: %w", trimmed, err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("stat extension path %q: %w", absolutePath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension path %q must be a directory", absolutePath)
	}
	return absolutePath, nil
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
		return copySymlink(path, targetPath)
	case entry.IsDir():
		if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
			return fmt.Errorf("create directory %q: %w", targetPath, err)
		}
		return nil
	default:
		return copyRegularFile(path, targetPath, info.Mode().Perm())
	}
}

func copySymlink(sourcePath string, targetPath string) error {
	linkTarget, err := os.Readlink(sourcePath)
	if err != nil {
		return fmt.Errorf("read symlink %q: %w", sourcePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create symlink parent %q: %w", filepath.Dir(targetPath), err)
	}
	if err := os.Symlink(linkTarget, targetPath); err != nil {
		return fmt.Errorf("create symlink %q: %w", targetPath, err)
	}
	return nil
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
