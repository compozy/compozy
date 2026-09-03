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

type generatedPublicationCompanion struct {
	snapshot func() (func() error, error)
	publish  func() error
}

func publishGeneratedArtifactSet(
	artifacts []generatedArtifact,
	publish func(string, []byte) error,
) error {
	return publishGeneratedArtifactTransaction(artifacts, publish, nil)
}

func publishGeneratedArtifactTransaction(
	artifacts []generatedArtifact,
	publish func(string, []byte) error,
	companion *generatedPublicationCompanion,
) error {
	snapshots, err := snapshotGeneratedArtifacts(artifacts)
	if err != nil {
		return err
	}
	var restoreCompanion func() error
	if companion != nil {
		restoreCompanion, err = companion.snapshot()
		if err != nil {
			return err
		}
	}
	for index, artifact := range artifacts {
		if err := publish(artifact.path, artifact.content); err != nil {
			return rollbackGeneratedArtifactSet(err, snapshots[:index+1])
		}
	}
	if companion == nil {
		return nil
	}
	if err := companion.publish(); err != nil {
		if restoreErr := restoreCompanion(); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
		return rollbackGeneratedArtifactSet(err, snapshots)
	}
	return nil
}

func snapshotGeneratedArtifacts(artifacts []generatedArtifact) ([]generatedArtifactSnapshot, error) {
	snapshots := make([]generatedArtifactSnapshot, 0, len(artifacts))
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.path == "" {
			return nil, errors.New("generated output path is empty")
		}
		if _, exists := seen[artifact.path]; exists {
			return nil, fmt.Errorf("generated output path %q is duplicated", artifact.path)
		}
		seen[artifact.path] = struct{}{}
		content, existed, err := readGeneratedFile(artifact.path)
		if err != nil {
			return nil, fmt.Errorf("snapshot generated output %q: %w", artifact.path, err)
		}
		snapshots = append(snapshots, generatedArtifactSnapshot{
			path: artifact.path, content: content, existed: existed,
		})
	}
	return snapshots, nil
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
