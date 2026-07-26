//go:build mage

package main

import (
	"fmt"
	"os"
)

const codegenCheckedEnvVar = "AGH_CODEGEN_CHECKED"

// markCodegenChecked exports the pipeline marker consumed by
// scripts/codegen-check.sh so turbo's //#codegen-check root task becomes a
// no-op inside a verify that already ran the full mage CodegenCheck.
// Standalone turbo runs never see the marker and keep the full gate.
func markCodegenChecked() error {
	if err := os.Setenv(codegenCheckedEnvVar, "1"); err != nil {
		return fmt.Errorf("set %s: %w", codegenCheckedEnvVar, err)
	}
	return nil
}
