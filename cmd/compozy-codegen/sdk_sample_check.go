package main

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"maps"
	"os"
	"slices"
)

func checkSDKSampleManifest(path string, expected []byte) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read SDK sample manifest: %w", err)
	}
	if bytes.Equal(content, expected) {
		return nil
	}
	var got, want sdkSampleManifest
	if err := json.Unmarshal(content, &got); err != nil {
		return fmt.Errorf("%s: %w; decode SDK samples: %w", path, ErrStaleGeneratedFile, err)
	}
	if err := json.Unmarshal(expected, &want); err != nil {
		return fmt.Errorf("decode expected SDK samples: %w", err)
	}
	for _, operation := range slices.Sorted(maps.Keys(want.Operations)) {
		if sample, exists := got.Operations[operation]; !exists || sample != want.Operations[operation] {
			return fmt.Errorf("SDK sample %s: %w; run codegen", operation, ErrStaleGeneratedFile)
		}
	}
	for _, operation := range slices.Sorted(maps.Keys(got.Operations)) {
		if _, exists := want.Operations[operation]; !exists {
			return fmt.Errorf("SDK sample %s: %w; obsolete operation, run codegen", operation, ErrStaleGeneratedFile)
		}
	}
	return fmt.Errorf("%s: %w; run codegen", path, ErrStaleGeneratedFile)
}
