package desktoprelease

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
)

func (a *Authority) publishAndVerifyInventory(
	ctx context.Context,
	dir, version string,
) ([]Artifact, error) {
	local, err := InspectDesktopInventory(ctx, dir, version)
	if err != nil {
		return nil, errorWithCode(ErrorInventoryIncomplete, err)
	}
	compatibilityArtifact, err := inspectArtifact(filepath.Join(dir, CompatibilityFile))
	if err != nil {
		return nil, errorWithCode(ErrorInventoryIncomplete, err)
	}
	local = append(local, compatibilityArtifact)
	if err := VerifyChecksumsCatalog(ctx, filepath.Join(dir, ChecksumsFile), local); err != nil {
		return nil, errorWithCode(ErrorVerificationFailed, err)
	}
	for _, artifact := range local {
		if err := a.backend.UploadAsset(ctx, version, filepath.Join(dir, artifact.Name), artifact); err != nil {
			return nil, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	return a.verifyArtifacts(ctx, version, local)
}

func (a *Authority) verifyRemoteInventory(ctx context.Context, version string) ([]Artifact, error) {
	remote, err := a.backend.ReleaseInventory(ctx, version)
	if err != nil {
		return nil, errorWithCode(ErrorVerificationFailed, err)
	}
	required := append(DesktopArtifactNames(version), CompatibilityFile)
	byName := make(map[string]Artifact, len(remote))
	for _, artifact := range remote {
		if artifact.Size <= 0 {
			return nil, errorWithCode(
				ErrorInventoryIncomplete,
				fmt.Errorf("desktop release: release %s has empty artifact %s", version, artifact.Name),
			)
		}
		byName[artifact.Name] = artifact
	}
	artifacts := make([]Artifact, 0, len(required))
	for _, name := range required {
		artifact, ok := byName[name]
		if !ok {
			return nil, errorWithCode(
				ErrorInventoryIncomplete,
				fmt.Errorf("desktop release: release %s is missing %s", version, name),
			)
		}
		artifacts = append(artifacts, artifact)
	}
	return a.verifyArtifacts(ctx, version, artifacts)
}

func (a *Authority) verifyArtifacts(ctx context.Context, version string, artifacts []Artifact) ([]Artifact, error) {
	for _, artifact := range artifacts {
		if err := a.backend.VerifyAsset(ctx, version, artifact); err != nil {
			return nil, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
	return artifacts, nil
}
