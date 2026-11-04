package github

import (
	"context"
	"errors"
	"time"
)

const issuesQuery = `query($owner:String!,$name:String!,$after:String){repository(owner:$owner,name:$name){
 issues(states:OPEN,orderBy:{field:CREATED_AT,direction:DESC},first:50,after:$after){
 nodes{__typename number title body url createdAt author{login} labels(first:100){nodes{name}}
 closedByPullRequestsReferences(first:5,includeClosedPrs:false){nodes{number title url state}}
 timelineItems(first:10,itemTypes:[CROSS_REFERENCED_EVENT]){nodes{
 ... on CrossReferencedEvent{source{__typename ... on PullRequest{number title url state}}}}}}
 pageInfo{hasNextPage endCursor}}}}`

type issuePull struct {
	Typename string `json:"__typename"`
	Number   int
	Title    string
	URL      string
	State    string
}
type graphIssue struct {
	Typename                       string `json:"__typename"`
	Number                         int
	Title                          string
	Body                           string
	URL                            string
	CreatedAt                      time.Time
	Author                         struct{ Login string }
	Labels                         struct{ Nodes []struct{ Name string } }
	ClosedByPullRequestsReferences struct{ Nodes []issuePull }
	TimelineItems                  struct{ Nodes []struct{ Source issuePull } }
}
type graphIssuesPage struct {
	Nodes    []graphIssue
	PageInfo struct {
		HasNextPage bool
		EndCursor   string
	}
}
type linkedIssuePR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Closes bool   `json:"closes"`
}

func (i graphIssue) labelNames() []string {
	names := make([]string, 0, len(i.Labels.Nodes))
	for _, label := range i.Labels.Nodes {
		names = append(names, label.Name)
	}
	return names
}
func (i graphIssue) linkedPRs() []linkedIssuePR {
	result := []linkedIssuePR{}
	seen := map[string]bool{}
	appendPR := func(pr issuePull, closes bool) {
		if pr.State != "OPEN" || pr.URL == "" || seen[pr.URL] {
			return
		}
		seen[pr.URL] = true
		result = append(result, linkedIssuePR{Number: pr.Number, Title: pr.Title, URL: pr.URL, Closes: closes})
	}
	for _, pr := range i.ClosedByPullRequestsReferences.Nodes {
		appendPR(pr, true)
	}
	for _, event := range i.TimelineItems.Nodes {
		if event.Source.Typename == "PullRequest" {
			appendPR(event.Source, false)
		}
	}
	return result
}

func (p *Provider) fetchIssues(
	ctx context.Context,
	repo repository,
	token string,
	watermark int,
	initial bool,
	backfill time.Duration,
	now time.Time,
) ([]graphIssue, int, error) {
	var cursor *string
	var issues []graphIssue
	baseline := watermark
	for {
		response, err := p.graphRequest(ctx, token, issuesQuery, graphPageVariables(repo, cursor))
		if err != nil {
			return nil, baseline, err
		}
		page := response.Data.Repository.Issues
		if page == nil {
			return nil, baseline, errors.New("github issues: missing issue collection")
		}
		for _, issue := range page.Nodes {
			if issue.Typename != "Issue" {
				continue
			}
			if initial && backfill == 0 {
				baseline = max(baseline, issue.Number)
				continue
			}
			if issue.Number <= watermark {
				return issues, baseline, nil
			}
			if initial && issue.CreatedAt.Before(now.Add(-backfill)) {
				return issues, max(baseline, issue.Number), nil
			}
			issues = append(issues, issue)
		}
		if (initial && backfill == 0) || !page.PageInfo.HasNextPage {
			return issues, baseline, nil
		}
		if page.PageInfo.EndCursor == "" || (cursor != nil && *cursor == page.PageInfo.EndCursor) {
			return nil, baseline, errors.New("github issues: pagination did not advance")
		}
		cursor = new(page.PageInfo.EndCursor)
	}
}
