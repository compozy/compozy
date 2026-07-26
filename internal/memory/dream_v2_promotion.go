package memory

import (
	"path/filepath"

	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func dreamPromotionCandidate(
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	artifactPath string,
	at time.Time,
) memcontract.Candidate {
	scope := workspace.scope.Normalize()
	if scope == "" {
		scope = memcontract.ScopeGlobal
	}
	nameDate := at.UTC().Format("2006-01-02")
	content := renderDreamPromotionContent(run, artifactPath)
	return memcontract.Candidate{
		WorkspaceID: workspace.id,
		Scope:       scope,
		Origin:      memcontract.OriginDreaming,
		Content:     content,
		Frontmatter: memcontract.Header{
			Name:        "Dreaming synthesis " + nameDate,
			Description: "Auto-curated from repeated recall signals.",
			Type:        memcontract.TypeProject,
			Scope:       scope,
			Provenance: &memcontract.Provenance{
				SourceActor: memcontract.OriginDreaming,
				Confidence:  "high",
				CreatedAt:   at.UTC(),
				UpdatedAt:   at.UTC(),
			},
		},
		Entity:    "dreaming synthesis",
		Attribute: "recurring memory themes",
		Metadata: map[string]string{
			decisionMetadataTargetFilenameKey: "project_dreaming_" + at.UTC().Format("20060102") + ".md",
			"run_id":                          strings.TrimSpace(run.runID),
			"artifact_path":                   artifactPath,
			dreamV2PromptVersionKey:           dreamPromptVersion,
		},
		SubmittedAt: at.UTC(),
	}
}

func renderDreamPromotionContent(run dreamSignalGateResult, artifactPath string) string {
	var builder strings.Builder
	builder.WriteString("Recurring memory themes promoted by the dreaming runtime.\n\n")
	builder.WriteString("Run: ")
	builder.WriteString(strings.TrimSpace(run.runID))
	builder.WriteString("\n")
	builder.WriteString("Artifact: ")
	builder.WriteString(filepath.Base(strings.TrimSpace(artifactPath)))
	builder.WriteString("\n\n")
	for _, candidate := range run.candidates {
		builder.WriteString("- ")
		builder.WriteString(cleanDreamPromotionLine(candidate))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func cleanDreamPromotionLine(candidate DreamCandidate) string {
	title := firstNonEmpty(candidate.Title, candidate.Slug, "memory candidate")
	body := diagnostics.RedactAndBound(cleanSnippet(candidate.Body), 220)
	if body == "" {
		return title
	}
	return title + ": " + body
}
