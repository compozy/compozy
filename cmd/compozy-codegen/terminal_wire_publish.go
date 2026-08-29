package main

import (
	"errors"
	"fmt"
	"slices"
)

type generatedArtifact struct {
	path    string
	content []byte
}

type generatedArtifactSnapshot struct {
	path    string
	content []byte
	existed bool
}

func publishGeneratedArtifactSet(
	artifacts []generatedArtifact,
	publish func(string, []byte) error,
) error {
	snapshots := make([]generatedArtifactSnapshot, 0, len(artifacts))
	seenPaths := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.path == "" {
			return errors.New("generated output path is empty")
		}
		if _, exists := seenPaths[artifact.path]; exists {
			return fmt.Errorf("generated output path %q is duplicated", artifact.path)
		}
		seenPaths[artifact.path] = struct{}{}
		content, existed, err := readGeneratedFile(artifact.path)
		if err != nil {
			return fmt.Errorf("snapshot generated output %q: %w", artifact.path, err)
		}
		snapshots = append(snapshots, generatedArtifactSnapshot{
			path: artifact.path, content: content, existed: existed,
		})
	}

	for index, artifact := range artifacts {
		if err := publish(artifact.path, artifact.content); err != nil {
			return rollbackGeneratedArtifactSet(err, snapshots[:index+1])
		}
	}
	return nil
}

func rollbackGeneratedArtifactSet(publishErr error, snapshots []generatedArtifactSnapshot) error {
	result := publishErr
	for _, snapshot := range slices.Backward(snapshots) {
		if err := restoreGeneratedFile(
			snapshot.path,
			snapshot.content,
			snapshot.existed,
			publishGeneratedFile,
		); err != nil {
			result = errors.Join(result, fmt.Errorf("restore generated output %q: %w", snapshot.path, err))
		}
	}
	return result
}
