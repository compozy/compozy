package cli

import (
	"io/fs"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/skills"
)

const (
	defaultSkillName            = "new-skill"
	skillMarkdownFileName       = "SKILL.md"
	nodeModulesDirectoryName    = "node_modules"
	defaultMarketplaceRegistry  = "clawhub"
	defaultMarketplaceSearchLim = 20
)

var (
	skillXMLAttributeReplacer = strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;", `"`, "&quot;")
	skillXMLTextReplacer      = strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;")
	validSkillNamePattern     = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type skillCommandContext struct {
	bundledFS fs.FS
	registry  *skills.Registry
	skills    []*skills.Skill
}

type skillListItem struct {
	Name        string                          `json:"name"`
	Description string                          `json:"description"`
	Source      string                          `json:"source"`
	Origin      string                          `json:"origin"`
	Enabled     bool                            `json:"enabled"`
	Activation  contract.SkillActivationPayload `json:"activation"`
}

type skillViewItem struct {
	Name      string   `json:"name"`
	Source    string   `json:"source"`
	Path      string   `json:"path"`
	File      string   `json:"file,omitempty"`
	Content   string   `json:"content"`
	Resources []string `json:"resources,omitempty"`
}

type skillInfoItem struct {
	Name        string                          `json:"name"`
	Description string                          `json:"description"`
	Version     string                          `json:"version,omitempty"`
	Source      string                          `json:"source"`
	Origin      string                          `json:"origin"`
	Path        string                          `json:"path"`
	Enabled     bool                            `json:"enabled"`
	Activation  contract.SkillActivationPayload `json:"activation"`
	Metadata    map[string]any                  `json:"metadata,omitempty"`
	Resources   []string                        `json:"resources,omitempty"`
	Provenance  *SkillProvenanceRecord          `json:"provenance,omitempty"`
	Exposures   []contract.SkillExposurePayload `json:"exposures"`
}

type skillCreateItem struct {
	Name   string `json:"name"`
	Group  string `json:"group,omitempty"`
	Path   string `json:"path"`
	File   string `json:"file"`
	Source string `json:"source"`
	Status string `json:"status"`
}

type skillWhereItem struct {
	Name      string                             `json:"name"`
	Source    string                             `json:"source"`
	Origin    string                             `json:"origin"`
	Dir       string                             `json:"dir"`
	Winner    contract.SkillShadowEntryPayload   `json:"winner"`
	Shadows   []contract.SkillShadowEntryPayload `json:"shadows"`
	Exposures []contract.SkillExposurePayload    `json:"exposures"`
}

type skillInstallItem struct {
	Name               string                                              `json:"name"`
	Slug               string                                              `json:"slug"`
	Version            string                                              `json:"version,omitempty"`
	Registry           string                                              `json:"registry"`
	Path               string                                              `json:"path"`
	Hash               string                                              `json:"hash"`
	Status             string                                              `json:"status"`
	CleanupDiagnostics []contract.SkillMarketplaceCleanupDiagnosticPayload `json:"cleanup_diagnostics,omitempty"`
}

type skillRemoveItem struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

type skillUpdateItem struct {
	Name               string                                              `json:"name"`
	Slug               string                                              `json:"slug"`
	CurrentVersion     string                                              `json:"current_version,omitempty"`
	LatestVersion      string                                              `json:"latest_version,omitempty"`
	Path               string                                              `json:"path"`
	Status             string                                              `json:"status"`
	CleanupDiagnostics []contract.SkillMarketplaceCleanupDiagnosticPayload `json:"cleanup_diagnostics,omitempty"`
}
