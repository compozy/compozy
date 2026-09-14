package extensionpkg

import (
	"context"
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

// PrepareDevelopmentGeneration validates portable source or builds a native bundle.
func PrepareDevelopmentGeneration(ctx context.Context, sourceDir string) (*BuildResult, error) {
	if ctx == nil {
		return nil, errors.New("extension: development context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := normalizeBuildRequest(BuildRequest{SourceDir: sourceDir})
	if err != nil {
		return nil, err
	}
	document, err := readPortableDevManifest(req.SourceDir)
	if err != nil {
		return nil, err
	}
	if document == nil {
		return BuildBundle(ctx, req)
	}
	manifest, err := loadAgentPluginDocument(req.SourceDir, "", document)
	if err != nil {
		return nil, err
	}
	hash, err := ComputeDirectoryChecksum(req.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("extension: checksum portable development source: %w", err)
	}
	return &BuildResult{
		GenerationDir:  req.SourceDir,
		GenerationHash: hash,
		ManifestPath:   document.Path,
		Manifest:       manifest,
	}, nil
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
	document, err := readPortableDevManifest(originPath)
	if err != nil {
		return nil, err
	}
	if document != nil {
		return verifyPortableDevGeneration(originPath, hash, document, resolvePortableDataDir)
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

func readPortableDevManifest(originPath string) (*agentplugin.ManifestDocument, error) {
	for _, name := range []string{manifestTOMLFileName, manifestJSONFileName} {
		exists, err := fileExists(filepath.Join(originPath, name))
		if err != nil {
			return nil, fmt.Errorf("extension: inspect development manifest %q: %w", name, err)
		}
		if exists {
			return nil, nil
		}
	}
	document, err := agentplugin.ReadManifest(originPath)
	if missing, ok := errors.AsType[*agentplugin.NotManifestError](err); ok && missing != nil {
		return nil, nil
	}
	return document, err
}

func verifyPortableDevGeneration(
	originPath string,
	hash string,
	document *agentplugin.ManifestDocument,
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
	name, err := document.Name()
	if err != nil {
		return nil, fmt.Errorf("%w: read portable generation %q name: %w", ErrExtensionGenerationInvalid, hash, err)
	}
	if resolveDataDir == nil {
		return nil, errors.New("extension: Agent Plugins development data path resolver is required")
	}
	dataDir, err := resolveDataDir(name)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: resolve portable generation %q data directory: %w",
			ErrExtensionGenerationInvalid,
			hash,
			err,
		)
	}
	manifest, err := loadAgentPluginDocument(originPath, dataDir, document)
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
		ManifestPath:             document.Path,
		Manifest:                 manifest,
		NetworkRequirementDigest: networkDigest,
	}, nil
}
