package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/skillscan"
)

func (w *Watcher) snapshotRoots(ctx context.Context) (map[string]filesnap.Snapshot, error) {
	roots, agentRoots, err := w.currentRoots(ctx)
	if err != nil {
		return nil, err
	}

	snapshots := make(map[string]filesnap.Snapshot)
	for _, root := range roots {
		if err := checkRegistryContext(ctx); err != nil {
			return nil, err
		}

		_, isAgentRoot := agentRoots[root]
		rootSnapshots, err := w.snapshotRoot(ctx, root, isAgentRoot)
		if err != nil {
			return nil, fmt.Errorf("skills: scan watcher root %q: %w", root, err)
		}
		maps.Copy(snapshots, rootSnapshots)
	}
	for root := range w.discoveries {
		if !slices.Contains(roots, root) {
			delete(w.discoveries, root)
		}
	}
	for root := range w.agentDiscoveries {
		if !slices.Contains(roots, root) {
			delete(w.agentDiscoveries, root)
		}
	}

	return snapshots, nil
}

func (w *Watcher) snapshotRoot(ctx context.Context, root string, agentRoot bool) (map[string]filesnap.Snapshot, error) {
	if agentRoot {
		return w.snapshotAgentRoot(root)
	}
	previous := w.discoveries[root]
	if !previous.Unchanged(ctx, root, []string{root}) {
		current, err := skillscan.ScanDirectory(root)
		if err != nil {
			return nil, err
		}
		if w.discoveries == nil {
			w.discoveries = make(map[string]skillscan.DirectoryResult)
		}
		w.discoveries[root] = current
		previous = current
	}
	snapshots := filesnap.Clone(previous.Snapshots)
	if err := recordSidecarSnapshots(previous.Paths, snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

type watcherAgentDiscovery struct {
	paths        []string
	dependencies map[string]filesnap.Snapshot
}

func (w *Watcher) snapshotAgentRoot(root string) (map[string]filesnap.Snapshot, error) {
	previous, ok := w.agentDiscoveries[root]
	unchanged := ok && len(previous.dependencies) > 0
	for path, expected := range previous.dependencies {
		current, err := filesnap.FromPath(path)
		if errors.Is(err, fs.ErrNotExist) && expected.Size == -1 {
			continue
		}
		if err != nil || !current.Equal(expected) {
			unchanged = false
			break
		}
	}
	if !unchanged {
		current, err := scanWatcherAgentRoot(root)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				delete(w.agentDiscoveries, root)
				return map[string]filesnap.Snapshot{}, nil
			}
			return nil, err
		}
		if w.agentDiscoveries == nil {
			w.agentDiscoveries = make(map[string]watcherAgentDiscovery)
		}
		w.agentDiscoveries[root] = current
		previous = current
	}
	snapshots := make(map[string]filesnap.Snapshot)
	var skillPaths []string
	for _, path := range previous.paths {
		snapshots[path] = previous.dependencies[path]
		if filepath.Base(path) == skillFileName {
			skillPaths = append(skillPaths, path)
		}
	}
	if err := recordSidecarSnapshots(skillPaths, snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func scanWatcherAgentRoot(root string) (watcherAgentDiscovery, error) {
	current := watcherAgentDiscovery{dependencies: make(map[string]filesnap.Snapshot)}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) && path != root {
			current.dependencies[path] = filesnap.Snapshot{Size: -1}
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		definition := entry.Name() == "AGENT.md" || entry.Name() == skillFileName
		if !entry.IsDir() && !definition {
			return nil
		}
		snapshot, err := filesnap.FromPath(path)
		if errors.Is(err, fs.ErrNotExist) && path != root {
			current.dependencies[path] = filesnap.Snapshot{Size: -1}
			return nil
		}
		if err != nil {
			return err
		}
		current.dependencies[path] = snapshot
		if definition && !entry.IsDir() {
			current.paths = append(current.paths, path)
		}
		return nil
	})
	return current, err
}
