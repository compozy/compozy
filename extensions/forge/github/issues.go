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
	MaxNumber int `json:"max_number"`
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
	issues, baseline, err := p.fetchIssues(ctx, repo, credential.token, state.MaxNumber, initial, backfill, now)
	if err != nil {
		return compozysdk.HeartbeatSourceResult{}, issueSourceError(err)
	}
	if initial {
		state.MaxNumber = baseline
	}
	return issueBatch(issues, params, repo, state, req.Limits)
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
		if len(summary) > budget {
			summary = summary[:budget]
			for !utf8.ValidString(summary) {
				summary = summary[:len(summary)-1]
			}
		}
		budget -= len(summary)
		attributes, err := json.Marshal(map[string]any{
			"number": issue.Number, "author": issue.Author.Login, "labels": issue.labelNames(),
			"body_excerpt": string(excerpt), "linked_open_prs": issue.linkedPRs(),
		})
		if err != nil {
			return compozysdk.HeartbeatSourceResult{}, fmt.Errorf("github issues: encode attributes: %w", err)
		}
		result.Items = append(result.Items, compozysdk.HeartbeatSourceItem{
			Key: fmt.Sprintf(
				"%s/%s#%d",
				repo.owner,
				repo.name,
				issue.Number,
			),
			Title:      issue.Title,
			Summary:    summary,
			URL:        issue.URL,
			Attributes: attributes,
		})
		state.MaxNumber = max(state.MaxNumber, issue.Number)
	}
	result.State = []byte(fmt.Sprintf(`{"max_number":%d}`, state.MaxNumber))
	return result, nil
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
