//go:build mage

package main

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/codegen/openapits"
	"github.com/compozy/compozy/internal/codegen/storeschema"
)

func Codegen() error {
	ctx := context.Background()
	if err := storeschema.GenerateAtlas(ctx, "."); err != nil {
		return err
	}
	if err := storeschema.GenerateSQLC(ctx, "."); err != nil {
		return err
	}
	if err := DaytonaSidecars(); err != nil {
		return err
	}
	if err := runCommandInDir(context.Background(), ".", "go", "run", "./cmd/compozy-codegen", "host-api"); err != nil {
		return err
	}
	if err := runCommandInDir(context.Background(), ".", "go", "run", "./cmd/compozy-codegen", "all"); err != nil {
		return err
	}
	artifacts, err := webOpenAPIArtifactsForGeneration()
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := openapits.Generate(context.Background(), artifact); err != nil {
			return err
		}
	}
	if err := CLIDocs(); err != nil {
		return err
	}
	if err := SyncFontSizeClasses(); err != nil {
		return err
	}
	if err := SyncLucideIcons(); err != nil {
		return err
	}
	if err := SyncEmojibase(); err != nil {
		return err
	}
	return SyncDesignMD()
}

func CodegenCheck() error {
	ctx := context.Background()
	if err := DepsCheck(); err != nil {
		return err
	}
	if err := storeschema.CheckAtlas(ctx, "."); err != nil {
		return err
	}
	if err := storeschema.CheckSQLC(ctx, "."); err != nil {
		return err
	}
	if err := daytonaSidecarsCheckStamped(); err != nil {
		return err
	}
	if err := runCommandInDir(
		context.Background(),
		".",
		"go",
		"run",
		"./cmd/compozy-codegen",
		"host-api",
		"--check",
	); err != nil {
		return err
	}
	if err := runCommandInDir(context.Background(), ".", "go", "run", "./cmd/compozy-codegen", "check"); err != nil {
		return err
	}
	if err := runCommandInDir(context.Background(), ".", "go", "run", "./cmd/compozy-manifest-check"); err != nil {
		return err
	}
	artifacts, err := webOpenAPIArtifactsForCheck()
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := openapits.Check(context.Background(), artifact); err != nil {
			return err
		}
	}
	if err := CLIDocsCheck(); err != nil {
		return err
	}
	if err := MigrationGuideCheck(); err != nil {
		return err
	}
	if err := SyncFontSizeClassesCheck(); err != nil {
		return err
	}
	if err := SyncLucideIconsCheck(); err != nil {
		return err
	}
	if err := SyncEmojibaseCheck(); err != nil {
		return err
	}
	return SyncDesignMDCheck()
}

// MigrationGuideCheck verifies parity between the root and site migration guides.
func MigrationGuideCheck() error {
	if err := runCommandInDir(
		context.Background(),
		".",
		"bash",
		"scripts/verify-migration-guide-parity.sh",
	); err != nil {
		return fmt.Errorf("check migration guide parity: %w", err)
	}
	return nil
}

// SyncFontSizeClasses refreshes the generated tailwind-merge font-size class group.
func SyncFontSizeClasses() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", fontSizeSyncScriptPath, "--write")
}

// SyncFontSizeClassesCheck verifies the generated tailwind-merge font-size class group.
func SyncFontSizeClassesCheck() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", fontSizeSyncScriptPath, "--check")
}

// SyncLucideIcons refreshes the Lucide icon allowlist embedded by internal/profile.
func SyncLucideIcons() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", lucideIconsSyncScriptPath, "--write")
}

// SyncLucideIconsCheck verifies the Lucide icon allowlist embedded by internal/profile.
func SyncLucideIconsCheck() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", lucideIconsSyncScriptPath, "--check")
}

// SyncEmojibase mirrors picker Emojibase JSON into web/public for native serving.
func SyncEmojibase() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", emojibaseSyncScriptPath, "--write")
}

// SyncEmojibaseCheck verifies the mirrored Emojibase JSON in web/public.
func SyncEmojibaseCheck() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", emojibaseSyncScriptPath, "--check")
}

// SyncDesignMD refreshes generated DESIGN.md token frontmatter and tables.
func SyncDesignMD() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", designSyncScriptPath, "--write")
}

// SyncDesignMDCheck verifies generated DESIGN.md token frontmatter and tables.
func SyncDesignMDCheck() error {
	return runCommandInDir(
		context.Background(),
		".",
		"bun",
		"run",
		designSyncScriptPath,
		"--check",
		"--audit-site",
	)
}
