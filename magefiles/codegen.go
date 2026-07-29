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
	if err := runCommandInDir(context.Background(), ".", "go", "run", "./cmd/compozy-codegen", "check"); err != nil {
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

// SyncDesignMD refreshes generated DESIGN.md token frontmatter and tables.
func SyncDesignMD() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", designSyncScriptPath, "--write")
}

// SyncDesignMDCheck verifies generated DESIGN.md token frontmatter and tables.
func SyncDesignMDCheck() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", designSyncScriptPath, "--check")
}
