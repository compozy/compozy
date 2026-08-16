package extensionpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/extension/agentplugin"
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

func verifyDevGeneration(
	originPath string,
	generationHash string,
	resolvePortableDataDir func(string) (string, error),
) (*verifiedDevGeneration, error) {
	hash := strings.TrimSpace(generationHash)
	if !generationHashPattern.MatchString(hash) {
		return nil, fmt.Errorf(
			"%w: generation hash must be 64 lowercase hexadecimal characters",
			ErrExtensionGenerationInvalid,
		)
	}
	portable, err := isPortableDevOrigin(originPath)
	if err != nil {
		return nil, err
	}
	if portable {
		return verifyPortableDevGeneration(originPath, hash, resolvePortableDataDir)
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
			"%w: digest generation %q network requirement: %w",
			ErrExtensionGenerationInvalid,
			hash,
			err,
		)
	}
	return &verifiedDevGeneration{
		OriginPath:               originPath,
		GenerationDir:            canonicalDir,
		GenerationHash:           hash,
		ManifestPath:             extensionManifestPath(canonicalDir),
		Manifest:                 manifest,
		NetworkRequirementDigest: networkDigest,
	}, nil
}

func isPortableDevOrigin(originPath string) (bool, error) {
	for _, name := range []string{manifestTOMLFileName, manifestJSONFileName} {
		exists, err := fileExists(filepath.Join(originPath, name))
		if err != nil {
			return false, fmt.Errorf("extension: inspect development manifest %q: %w", name, err)
		}
		if exists {
			return false, nil
		}
	}
	exists, err := fileExists(filepath.Join(originPath, agentPluginManifestFileName))
	if err != nil {
		return false, fmt.Errorf("extension: inspect Agent Plugins development manifest: %w", err)
	}
	return exists, nil
}

func verifyPortableDevGeneration(
	originPath string,
	hash string,
	resolveDataDir func(string) (string, error),
) (*verifiedDevGeneration, error) {
	actualHash, err := ComputeDirectoryChecksum(originPath)
	if err != nil {
		return nil, fmt.Errorf("%w: verify portable generation %q: %v", ErrExtensionGenerationInvalid, hash, err)
	}
	if actualHash != hash {
		return nil, fmt.Errorf(
			"%w: generation %q digest mismatch (actual %s)",
			ErrExtensionGenerationInvalid,
			hash,
			actualHash,
		)
	}
	name, err := agentplugin.ReadManifestName(originPath)
	if err != nil {
		return nil, fmt.Errorf("%w: read portable generation %q name: %v", ErrExtensionGenerationInvalid, hash, err)
	}
	if resolveDataDir == nil {
		return nil, errors.New("extension: Agent Plugins development data path resolver is required")
	}
	dataDir, err := resolveDataDir(name)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve portable generation %q data directory: %w", ErrExtensionGenerationInvalid, hash, err)
	}
	manifest, err := LoadManifestWithAgentPluginDataDir(originPath, dataDir)
	if err != nil {
		return nil, fmt.Errorf("%w: load portable generation %q manifest: %v", ErrExtensionGenerationInvalid, hash, err)
	}
	networkDigest, err := NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: digest portable generation %q network requirement: %w",
			ErrExtensionGenerationInvalid,
			hash,
			err,
		)
	}
	return &verifiedDevGeneration{
		OriginPath:               originPath,
		GenerationDir:            originPath,
		GenerationHash:           hash,
		ManifestPath:             filepath.Join(originPath, agentPluginManifestFileName),
		Manifest:                 manifest,
		NetworkRequirementDigest: networkDigest,
	}, nil
}
