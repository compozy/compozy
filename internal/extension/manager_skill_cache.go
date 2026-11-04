package extensionpkg

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

// skillResourceOwnerTTL matches the skills registry workspace cache lifetime.
const skillResourceOwnerTTL = 10 * time.Minute

// skillResourceOwner is the file set an instance key loaded last and when.
type skillResourceOwner struct {
	files    map[string]struct{}
	lastUsed time.Time
}

type skillResourceCacheEntry struct {
	source    skillspkg.SkillSource
	snapshots map[string]filesnap.Snapshot
	skill     *skillspkg.Skill
}

func (m *Manager) cachedSkillResource(
	file string,
	source skillspkg.SkillSource,
	load func(string, skillspkg.SkillSource) (*skillspkg.Skill, error),
) (*skillspkg.Skill, error) {
	snapshots := make(map[string]filesnap.Snapshot, 2)
	for _, name := range []string{filepath.Base(file), compozyconfig.MCPJSONName} {
		path := filepath.Join(filepath.Dir(file), name)
		snapshot, err := filesnap.FromPath(path)
		if errors.Is(err, os.ErrNotExist) && name != filepath.Base(file) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if _, err := resolveResourcePathWithinRoot(filepath.Dir(file), name, "skill"); err != nil {
			return nil, err
		}
		snapshots[path] = snapshot
	}
	m.skillResourceCacheMu.Lock()
	previous, found := m.skillResourceCache[file]
	if found && previous.source == source && filesnap.Equal(previous.snapshots, snapshots) {
		skill := cloneSkillSnapshot(previous.skill)
		m.skillResourceCacheMu.Unlock()
		return skill, nil
	}
	m.skillResourceCacheMu.Unlock()
	skill, err := load(file, source)
	if err != nil {
		return nil, err
	}
	m.skillResourceCacheMu.Lock()
	if m.skillResourceCache == nil {
		m.skillResourceCache = make(map[string]skillResourceCacheEntry)
	}
	m.skillResourceCache[file] = skillResourceCacheEntry{
		source: source, snapshots: snapshots, skill: cloneSkillSnapshot(skill),
	}
	m.skillResourceCacheMu.Unlock()
	return skill, nil
}

// retainSkillResources replaces or merges loaded files, pruning unowned and idle parses.
func (m *Manager) retainSkillResources(owner InstanceKey, files map[string]struct{}, merge bool) {
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	m.skillResourceCacheMu.Lock()
	defer m.skillResourceCacheMu.Unlock()
	previous := m.skillResourceOwners[owner].files
	if merge {
		maps.Copy(files, previous)
	}
	released := make(map[string]struct{})
	for file := range previous {
		if _, kept := files[file]; !kept {
			released[file] = struct{}{}
		}
	}
	if len(files) == 0 {
		delete(m.skillResourceOwners, owner)
	} else {
		if m.skillResourceOwners == nil {
			m.skillResourceOwners = make(map[InstanceKey]skillResourceOwner)
		}
		m.skillResourceOwners[owner] = skillResourceOwner{files: files, lastUsed: now}
	}
	cutoff := now.Add(-skillResourceOwnerTTL)
	maps.DeleteFunc(m.skillResourceOwners, func(_ InstanceKey, entry skillResourceOwner) bool {
		if !entry.lastUsed.Before(cutoff) {
			return false
		}
		maps.Copy(released, entry.files)
		return true
	})
	if len(released) == 0 {
		return
	}
	for _, entry := range m.skillResourceOwners {
		for file := range entry.files {
			delete(released, file)
		}
	}
	for file := range released {
		delete(m.skillResourceCache, file)
	}
}

// releaseSkillResources drops owner's cached parses once its resources unload.
func (m *Manager) releaseSkillResources(owner InstanceKey) {
	m.retainSkillResources(owner, nil, false)
}
