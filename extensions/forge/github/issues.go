package github

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

const issuesDescription = "Open issues created since the last check, with linked open pull requests."
const issuesParamsSchema = `{
 "type":"object",
 "properties":{
  "repo":{"type":"string","pattern":"^[\\w.-]+/[\\w.-]+$"},
  "labels":{"type":"array","items":{"type":"string"}},
  "exclude_labels":{"type":"array","items":{"type":"string"}},
  "backfill":{"type":"string"}
 },
 "additionalProperties":false
}`

var issueRepoPattern = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

type issuesParams struct {
	Repo          string   `json:"repo"`
	Labels        []string `json:"labels"`
	ExcludeLabels []string `json:"exclude_labels"`
	Backfill      string   `json:"backfill"`
}
type issuesState struct {
	MaxNumber     int       `json:"max_number"`
	Cursor        string    `json:"cursor,omitempty"`
	BackfillSince time.Time `json:"backfill_since,omitzero"`
}

func registerIssuesSource(extension *compozysdk.Extension, provider *Provider) error {
	return extension.HeartbeatSource("issues", compozysdk.HeartbeatSourceKind{
		Title: "New GitHub issues", Description: issuesDescription, ParamsSchema: []byte(issuesParamsSchema),
	}, provider.pollIssues)
}

func sourceError(code, message string) error {
	return compozysdk.NewRPCError(-32000, message, map[string]string{"code": code})
}

func (p *Provider) pollIssues(
	ctx context.Context,
	req compozysdk.HeartbeatSourceRequest,
) (compozysdk.HeartbeatSourceResult, error) {
	ctx, cancel := context.WithTimeout(ctx, githubRequestLimit)
	defer cancel()
	params, repo, err := resolveIssuesParams(ctx, req)
	if err != nil {
		return compozysdk.HeartbeatSourceResult{}, err
	}
	var state issuesState
	initial := len(req.State) == 0 || string(req.State) == "null"
	if !initial && (json.Unmarshal(req.State, &state) != nil || state.MaxNumber < 0) {
		return compozysdk.HeartbeatSourceResult{}, sourceError(
			"source_output_invalid",
			"invalid GitHub issue watermark",
		)
	}
	credential, err := p.credentials.resolve(ctx)
	if err != nil {
		return compozysdk.HeartbeatSourceResult{}, err
	}
	if credential.token == "" {
		return compozysdk.HeartbeatSourceResult{}, sourceError(
			"source_credential_absent",
			"GitHub sign-in missing; run gh auth login or bind GITHUB_TOKEN",
		)
	}
	backfill, err := time.ParseDuration(params.Backfill)
	if params.Backfill == "" {
		backfill, err = 0, nil
	}
	if err != nil || backfill < 0 {
		return compozysdk.HeartbeatSourceResult{}, sourceError(
			"source_params_invalid",
			"backfill must be a non-negative duration such as 24h",
		)
	}
	now := req.Now
	if now.IsZero() {
		now = p.now()
	}
	if initial && backfill > 0 {
		state.BackfillSince = now.Add(-backfill)
	}
	if !state.BackfillSince.IsZero() {
		backfill = now.Sub(state.BackfillSince)
		initial = true
	}
	issues, baseline, cursor, more, err := p.fetchIssues(
		ctx,
		repo,
		credential.token,
		state.MaxNumber,
		initial,
		backfill,
		now,
		state.Cursor,
	)
	if err != nil {
		return compozysdk.HeartbeatSourceResult{}, issueSourceError(err)
	}
	if initial {
		state.MaxNumber = baseline
	}
	result, err := issueBatch(issues, params, repo, state, req.Limits)
	if err != nil {
		return result, err
	}
	return finishIssuesPage(result, cursor, more)
}

func finishIssuesPage(
	result compozysdk.HeartbeatSourceResult,
	cursor string,
	more bool,
) (compozysdk.HeartbeatSourceResult, error) {
	var state issuesState
	var err error
	if err := json.Unmarshal(result.State, &state); err != nil {
		return result, err
	}
	if !result.More {
		state.Cursor = ""
		if more {
			state.Cursor = cursor
		}
	}
	if !more {
		state.BackfillSince = time.Time{}
	}
	result.More = result.More || more
	result.State, err = json.Marshal(state)
	return result, err
}

func resolveIssuesParams(ctx context.Context, req compozysdk.HeartbeatSourceRequest) (issuesParams, repository, error) {
	var params issuesParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params, json.RejectUnknownMembers(true)); err != nil {
			return params, repository{}, sourceError("source_params_invalid", "invalid GitHub issue parameters")
		}
	}
	if params.Repo != "" {
		repo, ok := repositoryFromPath(params.Repo)
		if !ok || !issueRepoPattern.MatchString(params.Repo) {
			return params, repository{}, sourceError("source_params_invalid", "set with.repo to owner/repo")
		}
		return params, repo, nil
	}
	if req.Workspace.RootDir != "" {
		command := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
		command.Dir = req.Workspace.RootDir
		if output, err := command.Output(); err == nil {
			if repo, ok := parseRepository(strings.TrimSpace(string(output))); ok {
				return params, repo, nil
			}
		}
	}
	return params, repository{}, sourceError(
		"source_params_invalid",
		"GitHub origin unavailable; set with.repo to owner/repo",
	)
}

func issueSourceError(err error) error {
	switch githubErrorCause(err) {
	case "credential_expired":
		return sourceError(
			"source_credential_expired",
			"GitHub sign-in expired; run gh auth login or update GITHUB_TOKEN",
		)
	case "rate_limited":
		return sourceError("source_rate_limited", "GitHub rate limit reached; retry later")
	default:
		return err
	}
}

func issueBatch(
	issues []graphIssue,
	params issuesParams,
	repo repository,
	state issuesState,
	limits compozysdk.HeartbeatSourceLimits,
) (compozysdk.HeartbeatSourceResult, error) {
	slices.SortFunc(issues, func(a, b graphIssue) int { return a.Number - b.Number })
	result := compozysdk.HeartbeatSourceResult{Items: []compozysdk.HeartbeatSourceItem{}}
	budget := max(0, limits.MaxSummaryBytes)
	for _, issue := range issues {
		if !issueMatchesLabels(issue, params) {
			state.MaxNumber = max(state.MaxNumber, issue.Number)
			continue
		}
		if len(result.Items) >= max(0, limits.MaxItems) {
			result.More = true
			break
		}
		excerpt := []rune(issue.Body)
		excerpt = excerpt[:min(400, len(excerpt))]
		summary := string(excerpt)
		summary = issueBoundBytes(summary, min(budget, 1024))
		budget -= len(summary)
		attributes, err := issueAttributes(issue, string(excerpt))
		if err != nil {
			return compozysdk.HeartbeatSourceResult{}, err
		}
		result.Items = append(result.Items, compozysdk.HeartbeatSourceItem{
			Key: fmt.Sprintf(
				"%s/%s#%d",
				repo.owner,
				repo.name,
				issue.Number,
			),
			Title:      string([]rune(issue.Title)[:min(200, utf8.RuneCountInString(issue.Title))]),
			Summary:    summary,
			URL:        issue.URL,
			Attributes: attributes,
		})
		state.MaxNumber = max(state.MaxNumber, issue.Number)
	}
	var err error
	result.State, err = json.Marshal(state)
	return result, err
}

func issueMatchesLabels(issue graphIssue, params issuesParams) bool {
	labels := issue.labelNames()
	for _, label := range params.ExcludeLabels {
		if slices.ContainsFunc(labels, func(value string) bool { return strings.EqualFold(label, value) }) {
			return false
		}
	}
	for _, label := range params.Labels {
		if !slices.ContainsFunc(labels, func(value string) bool { return strings.EqualFold(label, value) }) {
			return false
		}
	}
	return true
}

func issueBoundBytes(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	text = text[:limit]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

func issueAttributes(issue graphIssue, excerpt string) ([]byte, error) {
	labels, prs := issue.labelNames(), issue.linkedPRs()
	omitted := 0
	for {
		attrs := map[string]any{
			"number":          issue.Number,
			"author":          issueBoundBytes(issue.Author.Login, 200),
			"labels":          labels,
			"body_excerpt":    excerpt,
			"linked_open_prs": prs,
			"omitted":         omitted,
		}
		data, err := json.Marshal(attrs, json.Deterministic(true))
		if err != nil {
			return nil, fmt.Errorf("github issues: encode attributes: %w", err)
		}
		if len(data) <= 2048 {
			return data, nil
		}
		switch {
		case len(prs) > 0:
			prs = prs[:len(prs)-1]
			omitted++
		case len(labels) > 0:
			labels = labels[:len(labels)-1]
			omitted++
		default:
			excerpt = issueBoundBytes(excerpt, len(excerpt)/2)
		}
	}
}
