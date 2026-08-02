package main

import (
	"fmt"

	"github.com/compozy/compozy/internal/codegen/hostapi"
)

func writeHostAPIContracts() error {
	result, err := hostapi.Generate()
	if err != nil {
		return fmt.Errorf("generate internal Host API contracts: %w", err)
	}
	for _, path := range result.FileNames() {
		if err := publishGeneratedFile(path, result.Files[path]); err != nil {
			return fmt.Errorf("write internal Host API contract %q: %w", path, err)
		}
	}
	return nil
}

func checkHostAPIContracts() error {
	result, err := hostapi.Generate()
	if err != nil {
		return fmt.Errorf("generate internal Host API contracts: %w", err)
	}
	for _, path := range result.FileNames() {
		if err := checkFile(path, result.Files[path]); err != nil {
			return err
		}
	}
	return nil
}
