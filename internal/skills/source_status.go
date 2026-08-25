package skills

import (
	"context"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/skillscan"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// SkillSourceCollision reports one name shadow resolved across source roots.
type SkillSourceCollision struct {
	Name          string
	WinnerRootID  string
	QualifiedForm string
}

// SkillSourceSkippedLink explains why a first-level directory link was excluded.
type SkillSourceSkippedLink = skillscan.SkippedLink

// SkillSourceVerification summarizes verifier outcomes for one root.
type SkillSourceVerification struct {
	Blocked int
	Warned  int
}

// SkillSourceRootStatus is the daemon-owned measurement for one configured root.
type SkillSourceRootStatus struct {
	Spec          compozyconfig.SkillRootSpec
	Exists        bool
	Readable      bool
	ScannedCount  int
	SkillCount    int
	Truncated     bool
	SkippedLinks  []skillscan.SkippedLink
	Collisions    []SkillSourceCollision
	Verification  SkillSourceVerification
	NativeReaders []string
}

// SkillSourceRoots returns the exact projection roots and their current measurements.
func (r *Registry) SkillSourceRoots(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]SkillSourceRootStatus, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}
	cfg := r.registryConfigSnapshot(ctx)
	roots := append([]compozyconfig.SkillRootSpec(nil), cfg.GlobalSkillRoots...)
	if resolved != nil {
		roots = append(roots, workspaceResolvedSkillRoots(resolved)...)
	}
	trusted := make([]string, 0, len(roots))
	for _, root := range roots {
		trusted = append(trusted, root.Dir)
	}

	var (
		skills []*Skill
		err    error
	)
	if resolved == nil {
		skills = r.List()
	} else {
		skills, err = r.ForWorkspace(ctx, resolved)
		if err != nil {
			return nil, err
		}
	}
	diagnostics, err := r.SkillDiagnostics(ctx, resolved, "")
	if err != nil {
		return nil, err
	}
	statuses := make([]SkillSourceRootStatus, 0, len(roots))
	for _, root := range roots {
		result, scanErr := skillscan.ScanDirectoryWithin(root.Dir, trusted)
		if scanErr != nil {
			return nil, scanErr
		}
		status := SkillSourceRootStatus{
			Spec: root, Exists: result.Stats.Exists, Readable: result.Stats.Readable,
			ScannedCount: result.Stats.ScannedCount, Truncated: result.Stats.Truncated,
			SkippedLinks:  append([]skillscan.SkippedLink(nil), result.Stats.SkippedLinks...),
			NativeReaders: nativeReadersForRoot(root),
		}
		populateRootRuntimeStatus(&status, skills, diagnostics)
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func populateRootRuntimeStatus(
	status *SkillSourceRootStatus,
	skills []*Skill,
	diagnostics []SkillDiagnostic,
) {
	if status == nil {
		return
	}
	rootID := status.Spec.RootID()
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		if skill.RootID == rootID {
			status.SkillCount++
		}
		for _, shadow := range skill.Diagnostics.ShadowedDefinitions {
			if !pathWithinSkillRoot(shadow.Path, status.Spec.Dir) {
				continue
			}
			status.Collisions = append(status.Collisions, SkillSourceCollision{
				Name: strings.TrimSpace(skill.Meta.Name), WinnerRootID: skill.RootID,
				QualifiedForm: qualifiedSkillSourceName(status.Spec.SourceSlug, skill.Meta.Name),
			})
		}
	}
	for _, diagnostic := range diagnostics {
		if !pathWithinSkillRoot(diagnostic.Path, status.Spec.Dir) {
			continue
		}
		switch diagnostic.VerificationStatus {
		case SkillVerificationStatusFailed:
			status.Verification.Blocked++
		case SkillVerificationStatusWarning:
			status.Verification.Warned++
		}
	}
}

func nativeReadersForRoot(root compozyconfig.SkillRootSpec) []string {
	for _, preset := range compozyconfig.SkillSourcePresets() {
		if preset.Slug != root.SourceSlug {
			continue
		}
		if root.ResourceScope.Kind.Normalize() == resources.ResourceScopeKindUser {
			return append([]string(nil), preset.GlobalNativeReaders...)
		}
		return append([]string(nil), preset.WorkspaceNativeReaders...)
	}
	return []string{}
}

func qualifiedSkillSourceName(sourceSlug string, name string) string {
	origin := strings.TrimSpace(sourceSlug)
	if origin == "" {
		origin = compozyconfig.SkillSourceCompozy
	}
	return origin + ":" + strings.TrimSpace(name)
}

func pathWithinSkillRoot(path string, root string) bool {
	relative, err := filepath.Rel(strings.TrimSpace(root), strings.TrimSpace(path))
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
