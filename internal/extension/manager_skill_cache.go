package extensionpkg

import (
	"errors"
	"os"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

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
	m.skillResourceCache[file] = skillResourceCacheEntry{source: source, snapshots: snapshots, skill: cloneSkillSnapshot(skill)}
	m.skillResourceCacheMu.Unlock()
	return skill, nil
}
