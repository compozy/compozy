//go:build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

const productLanguageCheckPath = "scripts/check-product-language.sh"

// ProductLanguageCheck rejects the retired product name on public surfaces.
func ProductLanguageCheck() error {
	if err := sh.RunV("bash", productLanguageCheckPath); err != nil {
		return fmt.Errorf("check product language: %w", err)
	}
	return nil
}
