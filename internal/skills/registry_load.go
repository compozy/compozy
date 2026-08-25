package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/skillscan"
)

func (r *Registry) loadGlobalSkills(
	ctx context.Context,
	disabledSkills []string,
	cfg RegistryConfig,
) (map[string]*Skill, map[string]filesnap.Snapshot, []SkillDiagnostic, error) {
	skills := make(map[string]*Skill)
	snapshots := make(map[string]filesnap.Snapshot)
	diagnostics := make([]SkillDiagnostic, 0)

	if err := r.loadBundledSkills(ctx, cfg.BundledFS, skills, disabledSkills, &diagnostics); err != nil {
		return nil, nil, nil, err
	}
	if err := r.loadConfiguredGlobalRoots(
		ctx, cfg.GlobalSkillRoots, skills, snapshots, disabledSkills, &diagnostics,
	); err != nil {
		return nil, nil, nil, err
	}

	return skills, snapshots, diagnostics, nil
}

type configuredRootScan struct {
	spec   compozyconfig.SkillRootSpec
	result skillscan.DirectoryResult
}

func (r *Registry) loadConfiguredGlobalRoots(
	ctx context.Context,
	roots []compozyconfig.SkillRootSpec,
	dst map[string]*Skill,
	snapshots map[string]filesnap.Snapshot,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	trustedRoots := make([]string, 0, len(roots))
	for _, root := range roots {
		trustedRoots = append(trustedRoots, root.Dir)
	}
	scans := make([]configuredRootScan, 0, len(roots))
	for _, root := range roots {
		if err := checkRegistryContext(ctx); err != nil {
			return err
		}
		result, err := skillscan.ScanDirectoryWithin(root.Dir, trustedRoots)
		if err != nil {
			return fmt.Errorf("skills: scan configured root %q: %w", root.Dir, err)
		}
		if err := r.emitSkillScanEvents(ctx, root, result.Stats); err != nil {
			return err
		}
		maps.Copy(snapshots, result.Snapshots)
		if err := recordSidecarSnapshots(result.Paths, snapshots); err != nil {
			return err
		}
		scans = append(scans, configuredRootScan{spec: root, result: result})
	}

	selected := selectedRootPaths(scans)
	for index, scan := range scans {
		if err := r.loadRootSkillPaths(
			ctx,
			selected[index],
			scan.spec,
			dst,
			disabledSkills,
			diagnostics,
		); err != nil {
			return err
		}
	}
	return nil
}

func selectedRootPaths(scans []configuredRootScan) map[int][]string {
	type selection struct {
		index             int
		path              string
		firstLevelSymlink bool
	}

	selected := make(map[int][]string, len(scans))
	winners := make(map[string]selection)
	for index := len(scans) - 1; index >= 0; index-- {
		for _, path := range scans[index].result.Paths {
			realPath := scans[index].result.RealPaths[path]
			candidate := selection{
				index:             index,
				path:              path,
				firstLevelSymlink: pathUsesFirstLevelSymlink(scans[index].spec.Dir, path),
			}
			winner, exists := winners[realPath]
			if exists && (!winner.firstLevelSymlink || candidate.firstLevelSymlink) {
				continue
			}
			winners[realPath] = candidate
		}
	}
	for _, winner := range winners {
		selected[winner.index] = append(selected[winner.index], winner.path)
	}
	for index := range scans {
		slices.Sort(selected[index])
	}
	return selected
}

func pathUsesFirstLevelSymlink(root string, skillPath string) bool {
	relative, err := filepath.Rel(root, skillPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	first, _, _ := strings.Cut(relative, string(filepath.Separator))
	info, err := os.Lstat(filepath.Join(root, first))
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func (r *Registry) loadRootSkillPaths(
	ctx context.Context,
	paths []string,
	root compozyconfig.SkillRootSpec,
	dst map[string]*Skill,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	for _, skillPath := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return err
		}
		skill, content, err := parseSkillFileDocument(skillPath)
		if err != nil {
			return err
		}
		if err := r.assignRootAndProvenance(skill, root); err != nil {
			return err
		}
		if !r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, diagnostics) {
			continue
		}
	}
	return nil
}

func (r *Registry) loadWorkspaceSkills(
	ctx context.Context,
	paths []workspaceSkillPath,
	disabledSkills []string,
) (map[string]*Skill, []SkillDiagnostic, error) {
	skills := make(map[string]*Skill)
	diagnostics := make([]SkillDiagnostic, 0)

	for _, path := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return nil, nil, err
		}

		skill, content, err := parseSkillFileDocument(path.filePath)
		if err != nil {
			return nil, nil, err
		}
		skill.Source = path.source
		skill.Origin = path.origin
		skill.RootID = path.rootID
		skill.RootDir = path.rootDir
		skill.ResourceScope = path.resourceScope
		refreshSkillHookDecls(skill)
		if !r.processSkillWithDiagnostics(skills, skill, content, disabledSkills, &diagnostics) {
			continue
		}
	}

	return skills, diagnostics, nil
}

func (r *Registry) loadBundledSkills(
	ctx context.Context,
	bundledFS fs.FS,
	dst map[string]*Skill,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	if bundledFS == nil {
		return nil
	}

	paths, err := scanBundledFS(bundledFS)
	if err != nil {
		return err
	}

	for _, skillPath := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return err
		}

		skill, content, err := parseBundledSkillDocument(bundledFS, skillPath)
		if err != nil {
			return err
		}
		if !r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, diagnostics) {
			continue
		}
	}

	return nil
}

func (r *Registry) loadDirectorySkills(
	ctx context.Context,
	dir string,
	source SkillSource,
	dst map[string]*Skill,
	snapshots map[string]filesnap.Snapshot,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	root := strings.TrimSpace(dir)
	if root == "" {
		return nil
	}

	paths, dirSnapshots, err := scanDirectoryWithSnapshots(root)
	if err != nil {
		return err
	}
	maps.Copy(snapshots, dirSnapshots)
	if err := recordSidecarSnapshots(paths, snapshots); err != nil {
		return err
	}

	return r.loadSkillPaths(ctx, paths, source, dst, disabledSkills, diagnostics)
}

func (r *Registry) loadSkillPaths(
	ctx context.Context,
	paths []string,
	source SkillSource,
	dst map[string]*Skill,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	for _, skillPath := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return err
		}

		skill, content, err := parseSkillFileDocument(skillPath)
		if err != nil {
			return err
		}
		if err := r.assignSourceAndProvenance(skill, source); err != nil {
			return err
		}
		if !r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, diagnostics) {
			continue
		}
	}

	return nil
}

func (r *Registry) processSkill(dst map[string]*Skill, skill *Skill, content string, disabledSkills []string) bool {
	return r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, nil)
}

func (r *Registry) processSkillWithDiagnostics(
	dst map[string]*Skill,
	skill *Skill,
	content string,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) bool {
	r.applyDisabled(skill, disabledSkills)

	verifyErr := r.verifyMarketplaceSkill(skill)
	var warnings []Warning
	if skill.Source != SourceBundled {
		warnings = VerifyContent(content)
	}
	r.logVerificationWarnings(skill, warnings)
	if verifyErr != nil {
		appendSkillDiagnostic(diagnostics, skillVerificationFailedDiagnostic(skill, verifyErr, warnings))
		return false
	}
	if hasCriticalWarning(warnings) {
		appendSkillDiagnostic(diagnostics, skillVerificationFailedDiagnostic(skill, nil, warnings))
		return false
	}

	skill.Diagnostics.VerificationStatus = verificationStatusForWarnings(warnings)
	skill.Diagnostics.Warnings = cloneWarnings(warnings)
	r.overlaySkill(dst, skill)
	return true
}

func (r *Registry) assignSourceAndProvenance(skill *Skill, source SkillSource) error {
	if skill == nil {
		return errors.New("skills: skill is required")
	}

	skill.Source = source
	if source != SourceUser {
		refreshSkillHookDecls(skill)
		return nil
	}

	hasSidecar, err := HasSidecar(skill.Dir)
	if err != nil {
		return err
	}
	if !hasSidecar {
		refreshSkillHookDecls(skill)
		return nil
	}

	provenance, err := ReadSidecar(skill.Dir)
	if err != nil {
		return err
	}

	skill.Source = SourceMarketplace
	skill.Provenance = provenance
	skill.InstalledFrom = strings.TrimSpace(provenance.Slug)
	refreshSkillHookDecls(skill)

	return nil
}

func (r *Registry) assignRootAndProvenance(skill *Skill, root compozyconfig.SkillRootSpec) error {
	assignSkillRoot(skill, root)
	if skill.Source != SourceUser {
		refreshSkillHookDecls(skill)
		return nil
	}
	hasSidecar, err := HasSidecar(skill.Dir)
	if err != nil {
		return err
	}
	if !hasSidecar {
		refreshSkillHookDecls(skill)
		return nil
	}
	provenance, err := ReadSidecar(skill.Dir)
	if err != nil {
		return err
	}
	skill.Source = SourceMarketplace
	skill.Provenance = provenance
	skill.InstalledFrom = strings.TrimSpace(provenance.Slug)
	refreshSkillHookDecls(skill)
	return nil
}

func (r *Registry) verifyMarketplaceSkill(skill *Skill) error {
	if skill == nil || skill.Source != SourceMarketplace || skill.Provenance == nil {
		return nil
	}

	err := VerifyHash(skill.Dir, skill.Provenance)
	if err == nil {
		return nil
	}

	if mismatch, ok := errors.AsType[*HashMismatchError](err); ok {
		r.logger.Warn(
			"skills: marketplace skill hash mismatch",
			"skill_name", skill.Meta.Name,
			"expected_hash", mismatch.ExpectedHash,
			"actual_hash", mismatch.ActualHash,
			"path", skill.FilePath,
		)
		return err
	}

	r.logger.Warn(
		"skills: marketplace skill hash verification failed",
		"skill_name", skill.Meta.Name,
		"path", skill.FilePath,
		"error", err,
	)

	return err
}
