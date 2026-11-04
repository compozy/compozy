package desktoprelease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

func (a *Authority) channelState(ctx context.Context, branch string) (string, ChannelCommit, error) {
	sha, err := a.backend.ChannelRef(ctx, branch)
	if errors.Is(err, ErrChannelRefNotFound) {
		return "", ChannelCommit{}, nil
	}
	if err != nil {
		return "", ChannelCommit{}, err
	}
	commit, err := a.backend.ReadCommit(ctx, sha)
	if err != nil {
		return "", ChannelCommit{}, err
	}
	return sha, commit, nil
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
	for _, name := range []string{ManifestMac, ManifestLinux, ManifestWindows} {
		manifest, decodeErr := decodeUpdateManifest(files[path.Join(ChannelDirectory, name)], name)
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
