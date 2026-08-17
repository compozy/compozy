package desktoprelease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (a *Authority) channelState(ctx context.Context, branch string) (string, Generation, error) {
	sha, err := a.backend.ChannelRef(ctx, branch)
	if errors.Is(err, ErrChannelRefNotFound) {
		return "", Generation{}, nil
	}
	if err != nil {
		return "", Generation{}, err
	}
	commit, err := a.backend.ReadCommit(ctx, sha)
	if err != nil {
		return "", Generation{}, err
	}
	return sha, commit.Generation, nil
}

func prepareChannelFiles(dir, assetDir, repository string, generation Generation) (map[string][]byte, error) {
	contents, err := canonicalJSON(generation)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, GenerationFile), contents, 0o600); err != nil {
		return nil, fmt.Errorf("desktop release: write generation file: %w", err)
	}
	files, err := LoadChannelFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{ManifestMac, ManifestLinux} {
		manifest, decodeErr := decodeUpdateManifest(files[filepath.Join(ChannelDirectory, name)], name)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if manifest.Version != generation.Version {
			return nil, fmt.Errorf(
				"desktop release: manifest %s version %s does not match generation %s",
				name,
				manifest.Version,
				generation.Version,
			)
		}
		if inventoryErr := validateManifestInventory(
			manifest,
			name,
			generation.Version,
			repository,
			assetDir,
		); inventoryErr != nil {
			return nil, inventoryErr
		}
	}
	return files, nil
}
