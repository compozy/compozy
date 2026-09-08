//go:build mage

package main

import (
	"fmt"
	"os"
)

const codegenCheckedEnvVar = "COMPOZY_CODEGEN_CHECKED"

// markCodegenChecked exports the pipeline marker consumed by
// scripts/codegen-check.sh so turbo's //#codegen-check root task becomes a
// no-op in a pipeline that already passed the full mage CodegenCheck, including
// Verify and E2E asset preparation. Standalone turbo runs validate gate evidence.
func markCodegenChecked() error {
	if err := os.Setenv(codegenCheckedEnvVar, "1"); err != nil {
		return fmt.Errorf("set %s: %w", codegenCheckedEnvVar, err)
	}
	return nil
}
