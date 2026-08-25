package skills

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
)

// ExposeManager owns skill-link lifecycle for one exact configured root projection.
type ExposeManager struct {
	store  store.SkillExposureRepository
	roots  []compozyconfig.SkillRootSpec
	events store.EventSummaryStore
	logger *slog.Logger
	fs     exposureFS
	mu     sync.Mutex
}

// ExposeManagerOption customizes exposure I/O and observability.
type ExposeManagerOption func(*ExposeManager)

// WithExposureEventStore records canonical exposure lifecycle events.
func WithExposureEventStore(events store.EventSummaryStore) ExposeManagerOption {
	return func(manager *ExposeManager) { manager.events = events }
}

// WithExposureLogger sets the logger used for best-effort event write failures.
func WithExposureLogger(logger *slog.Logger) ExposeManagerOption {
	return func(manager *ExposeManager) { manager.logger = logger }
}

func withExposureFS(filesystem exposureFS) ExposeManagerOption {
	return func(manager *ExposeManager) { manager.fs = filesystem }
}

// NewExposeManager constructs a manager scoped to one effective source projection.
func NewExposeManager(
	repository store.SkillExposureRepository,
	roots []compozyconfig.SkillRootSpec,
	options ...ExposeManagerOption,
) *ExposeManager {
	manager := &ExposeManager{
		store:  repository,
		roots:  append([]compozyconfig.SkillRootSpec(nil), roots...),
		fs:     osExposureFS{},
		logger: slog.Default(),
	}
	for _, option := range options {
		if option != nil {
			option(manager)
		}
	}
	if manager.fs == nil {
		manager.fs = osExposureFS{}
	}
	if manager.logger == nil {
		manager.logger = slog.Default()
	}
	return manager
}

type exposureOwner struct {
	scope       store.SkillExposureOwnerScope
	workspaceID string
	resource    resources.ResourceScope
}

func (m *ExposeManager) prepareSkill(skill *Skill) (exposureOwner, string, error) {
	if skill == nil {
		return exposureOwner{}, "", newExposureError(
			ExposureCodeSkillNotExposable, "", "", "skill is required", nil,
		)
	}
	if skill.Source == SourceBundled {
		return exposureOwner{}, "", newExposureError(
			ExposureCodeSkillNotExposable,
			"",
			"",
			"bundled skills have no on-disk home; copy it with `compozy skill create` first",
			nil,
		)
	}
	owner, name, err := m.prepareSkillIdentity(skill)
	if err != nil {
		return exposureOwner{}, "", err
	}
	info, err := m.fs.Stat(skill.Dir)
	if err != nil {
		return exposureOwner{}, "", newExposureError(
			ExposureCodeSkillNotExposable,
			"",
			skill.Dir,
			fmt.Sprintf("skill %q has no accessible on-disk home", name),
			err,
		)
	}
	if !info.IsDir() {
		return exposureOwner{}, "", newExposureError(
			ExposureCodeSkillNotExposable,
			"",
			skill.Dir,
			fmt.Sprintf("skill %q on-disk home is not a directory", name),
			nil,
		)
	}
	canonicalDir, err := m.fs.EvalSymlinks(skill.Dir)
	if err != nil {
		return exposureOwner{}, "", fmt.Errorf("skills: resolve canonical directory for %q: %w", name, err)
	}
	return owner, canonicalDir, nil
}

func (m *ExposeManager) prepareSkillIdentity(skill *Skill) (exposureOwner, string, error) {
	if skill == nil {
		return exposureOwner{}, "", newExposureError(
			ExposureCodeSkillNotExposable, "", "", "skill is required", nil,
		)
	}
	scope := skill.ResourceScope.Normalize()
	owner, err := exposureOwnerFromScope(scope)
	if err != nil {
		return exposureOwner{}, "", err
	}
	name := strings.TrimSpace(skill.Meta.Name)
	if err := validateExposeName(name); err != nil {
		return exposureOwner{}, "", err
	}
	if m == nil || m.store == nil {
		return exposureOwner{}, "", errors.New("skills: exposure repository is required")
	}
	return owner, name, nil
}

func exposureOwnerFromScope(scope resources.ResourceScope) (exposureOwner, error) {
	switch scope.Kind.Normalize() {
	case resources.ResourceScopeKindUser:
		return exposureOwner{scope: store.SkillExposureOwnerUser, resource: scope}, nil
	case resources.ResourceScopeKindWorkspace:
		if strings.TrimSpace(scope.ID) == "" {
			return exposureOwner{}, newExposureError(
				ExposureCodeSkillNotExposable, "", "", "workspace-owned skill is missing its workspace id", nil,
			)
		}
		return exposureOwner{
			scope: store.SkillExposureOwnerWorkspace, workspaceID: strings.TrimSpace(scope.ID), resource: scope,
		}, nil
	case resources.ResourceScopeKindProfile, resources.ResourceScopeKindWorkspaceProfile:
		return exposureOwner{}, newExposureError(
			ExposureCodeProfileSkillNotExposable,
			"",
			"",
			"profile-owned skills cannot be exposed into shared provider roots",
			nil,
		)
	default:
		return exposureOwner{}, newExposureError(
			ExposureCodeSkillNotExposable, "", "", "skill ownership scope is not exposable", nil,
		)
	}
}

func (m *ExposeManager) targetRoot(owner exposureOwner, target string) (compozyconfig.SkillRootSpec, error) {
	trimmed := strings.TrimSpace(target)
	for _, root := range m.roots {
		if root.SourceSlug == trimmed && root.Kind == compozyconfig.RootKindCustom {
			return compozyconfig.SkillRootSpec{}, newExposureError(
				ExposureCodeTargetInvalid,
				trimmed,
				root.Dir,
				"expose targets are presets; custom sources cannot receive links",
				nil,
			)
		}
	}
	if !knownExposePreset(trimmed) {
		return compozyconfig.SkillRootSpec{}, newExposureError(
			ExposureCodeTargetInvalid,
			trimmed,
			"",
			fmt.Sprintf("expose target %q is not a supported preset", trimmed),
			nil,
		)
	}
	for _, root := range m.roots {
		if root.SourceSlug != trimmed || root.Kind != compozyconfig.RootKindPreset ||
			!sameExposureScope(root.ResourceScope.Normalize(), owner.resource) {
			continue
		}
		return root, nil
	}
	enabled := m.enabledTargets(owner)
	return compozyconfig.SkillRootSpec{}, newExposureError(
		ExposureCodeTargetDisabled,
		trimmed,
		"",
		fmt.Sprintf("expose target %q is disabled; enabled targets: %s", trimmed, strings.Join(enabled, ", ")),
		nil,
	)
}

func (m *ExposeManager) enabledTargets(owner exposureOwner) []string {
	enabled := make([]string, 0)
	for _, root := range m.roots {
		if root.Kind != compozyconfig.RootKindPreset || !knownExposePreset(root.SourceSlug) ||
			!sameExposureScope(root.ResourceScope.Normalize(), owner.resource) || slices.Contains(enabled, root.SourceSlug) {
			continue
		}
		enabled = append(enabled, root.SourceSlug)
	}
	slices.Sort(enabled)
	if len(enabled) == 0 {
		return []string{"none"}
	}
	return enabled
}

func knownExposePreset(target string) bool {
	for _, preset := range compozyconfig.SkillSourcePresets() {
		if preset.Slug == target && !preset.AlwaysOn {
			return true
		}
	}
	return false
}

func sameExposureScope(left resources.ResourceScope, right resources.ResourceScope) bool {
	if left.Kind.Normalize() != right.Kind.Normalize() {
		return false
	}
	if left.Kind.Normalize() == resources.ResourceScopeKindUser {
		return true
	}
	return strings.TrimSpace(left.ID) == strings.TrimSpace(right.ID)
}
