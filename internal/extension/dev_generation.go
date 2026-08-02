package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var generationHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type verifiedDevGeneration struct {
	OriginPath               string
	GenerationDir            string
	GenerationHash           string
	ManifestPath             string
	Manifest                 *Manifest
	NetworkRequirementDigest string
}

func canonicalizeDevOrigin(workspaceRoot, originPath string) (string, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return "", errors.New("extension: workspace root is required")
	}
	origin := strings.TrimSpace(originPath)
	if origin == "" {
		return "", errors.New("extension: development origin path is required")
	}
	if !filepath.IsAbs(origin) {
		origin = filepath.Join(root, origin)
	}
	canonicalRoot, err := canonicalBuildPath(root)
	if err != nil {
		return "", fmt.Errorf("extension: canonicalize workspace root: %w", err)
	}
	canonicalOrigin, err := canonicalBuildPath(origin)
	if err != nil {
		return "", fmt.Errorf("extension: canonicalize development origin: %w", err)
	}
	contained, err := buildPathContains(canonicalRoot, canonicalOrigin)
	if err != nil {
		return "", fmt.Errorf("extension: compare development origin to workspace root: %w", err)
	}
	if !contained {
		return "", fmt.Errorf(
			"extension: development origin %q escapes workspace root %q",
			canonicalOrigin,
			canonicalRoot,
		)
	}
	info, err := os.Stat(canonicalOrigin)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s: %w", ErrExtensionDevOriginMissing, canonicalOrigin, err)
		}
		return "", fmt.Errorf("extension: stat development origin %q: %w", canonicalOrigin, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension: development origin %q is not a directory", canonicalOrigin)
	}
	return canonicalOrigin, nil
}

func verifyDevGeneration(originPath, generationHash string) (*verifiedDevGeneration, error) {
	hash := strings.TrimSpace(generationHash)
	if !generationHashPattern.MatchString(hash) {
		return nil, fmt.Errorf(
			"%w: generation hash must be 64 lowercase hexadecimal characters",
			ErrExtensionGenerationInvalid,
		)
	}
	generationDir := filepath.Join(originPath, "dist", generationPrefix+hash)
	distRoot, err := filepath.EvalSymlinks(filepath.Join(originPath, "dist"))
	if err != nil {
		return nil, fmt.Errorf("%w: resolve generation root: %v", ErrExtensionGenerationInvalid, err)
	}
	canonicalDir, err := filepath.EvalSymlinks(generationDir)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve generation %q: %v", ErrExtensionGenerationInvalid, hash, err)
	}
	contained, err := buildPathContains(distRoot, canonicalDir)
	if err != nil || !contained {
		return nil, fmt.Errorf("%w: generation %q escapes the development origin", ErrExtensionGenerationInvalid, hash)
	}
	actualHash, err := ComputeDirectoryChecksum(canonicalDir)
	if err != nil {
		return nil, fmt.Errorf("%w: verify generation %q: %v", ErrExtensionGenerationInvalid, hash, err)
	}
	if actualHash != hash {
		return nil, fmt.Errorf(
			"%w: generation %q digest mismatch (actual %s)",
			ErrExtensionGenerationInvalid,
			hash,
			actualHash,
		)
	}
	manifest, err := LoadManifest(canonicalDir)
	if err != nil {
		return nil, fmt.Errorf("%w: load generation %q manifest: %v", ErrExtensionGenerationInvalid, hash, err)
	}
	networkDigest, err := NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: digest generation %q network requirement: %v",
			ErrExtensionGenerationInvalid,
			hash,
			err,
		)
	}
	return &verifiedDevGeneration{
		OriginPath:               originPath,
		GenerationDir:            canonicalDir,
		GenerationHash:           hash,
		ManifestPath:             bundleManifestPath(canonicalDir),
		Manifest:                 manifest,
		NetworkRequirementDigest: networkDigest,
	}, nil
}
