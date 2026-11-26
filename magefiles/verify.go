//go:build mage

package main

import (
	"context"
	"fmt"
	"time"
)

func Verify(ctx context.Context) error {
	release := acquireVerifyLock()
	defer release()
	return runMageSteps(verifySteps(ctx))
}

func verifySteps(ctx context.Context) []mageStep {
	return []mageStep{
		{name: "CodegenCheck", run: CodegenCheck},
		{name: "InstallerCheck", run: InstallerCheck},
		{name: "ProductLanguageCheck", run: ProductLanguageCheck},
		{name: "SourceSize", run: SourceSize},
		{name: "BunLint", run: BunLint},
		{name: "BunTypecheck", run: BunTypecheck},
		{name: "BunTest", run: BunTest},
		{name: "WebBuild", run: WebBuild},
		{name: "FmtCheck", run: FmtCheck},
		{name: "GoLint", run: GoLint},
		{name: "Test", run: Test},
		{name: "buildGo", run: buildGo},
		{name: "Boundaries", run: func() error { return Boundaries(ctx) }},
	}
}

func runMageSteps(steps []mageStep) error {
	for _, step := range steps {
		startedAt := time.Now()
		fmt.Printf("==> %s\n", step.name)
		if err := step.run(); err != nil {
			fmt.Printf("<== %s FAILED (%s)\n", step.name, time.Since(startedAt).Round(time.Millisecond))
			return err
		}
		fmt.Printf("<== %s OK (%s)\n", step.name, time.Since(startedAt).Round(time.Millisecond))
	}
	return nil
}
