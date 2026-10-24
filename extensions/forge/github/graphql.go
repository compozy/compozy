package github

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const graphFactsNone = "none"
const graphCheckCancelled = "CANCELLED" //nolint:misspell // GitHub's GraphQL enum spelling.

const pullQuery = `query($owner:String!,$name:String!,$branch:String!,$states:[PullRequestState!]!,$after:String){
 repository(owner:$owner,name:$name){pullRequests(
 headRefName:$branch,states:$states,first:50,after:$after,orderBy:{field:UPDATED_AT,direction:DESC}){
 nodes{number url state isDraft merged headRefOid baseRefName updatedAt
 reviewDecision mergeable headRepository{nameWithOwner}}
 pageInfo{hasNextPage endCursor}}}}`

const checksQuery = `query($owner:String!,$name:String!,$oid:GitObjectID!){repository(owner:$owner,name:$name){object(oid:$oid){... on Commit{oid statusCheckRollup{contexts(first:20){totalCount checkRunCountsByState{state count} statusContextCountsByState{state count} nodes{... on CheckRun{name conclusion status} ... on StatusContext{context state}}}}}}}}`

type graphPull struct {
	Number         int
	URL            string
	State          string
	IsDraft        bool
	Merged         bool
	HeadRefOID     string
	BaseRefName    string
	UpdatedAt      time.Time
	ReviewDecision string
	Mergeable      string
	HeadRepository *struct{ NameWithOwner string }
}

type graphPage struct {
	Nodes    []graphPull
	PageInfo struct {
		HasNextPage bool
		EndCursor   string
	}
}

type graphCount struct {
	State string
	Count int
}
type graphContexts struct {
	TotalCount                 int
	CheckRunCountsByState      []graphCount
	StatusContextCountsByState []graphCount
	Nodes                      []struct{ Name, Conclusion, Status, Context, State string }
}

type graphResponse struct {
	Data struct {
		Repository *struct {
			PullRequests graphPage
			Object       *struct {
				OID               string
				StatusCheckRollup *struct{ Contexts graphContexts }
			}
		}
	}
	Errors []struct{ Type string }
}

func (p *Provider) graphRequest(
	ctx context.Context,
	token, query string,
	variables map[string]any,
) (graphResponse, error) {
	var result graphResponse
	if err := p.api.request(
		ctx,
		http.MethodPost,
		"/graphql",
		token,
		map[string]any{"query": query, "variables": variables},
		&result,
	); err != nil {
		return result, err
	}
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			if e.Type == "RATE_LIMITED" {
				return result, &githubAPIError{cause: extensioncontract.ForgeCauseRateLimited, status: 429}
			}
		}
		return result, errors.New("github forge: GraphQL observation failed")
	}
	if result.Data.Repository == nil {
		return result, errors.New("github forge: repository unavailable")
	}
	return result, nil
}

func (p *Provider) graphStatus(
	ctx context.Context,
	repo repository,
	branch, token string,
) (extensioncontract.ForgeStatusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	response := extensioncontract.ForgeStatusResponse{
		Provider:  githubProviderName,
		FetchedAt: p.now().UTC(),
		Selection: "complete",
	}
	pull, incomplete, err := p.selectGraphPull(ctx, repo, branch, token)
	if err != nil {
		return response, err
	}
	if pull == nil {
		if incomplete {
			response.Selection = "incomplete"
		}
		return response, nil
	}
	state := strings.ToLower(pull.State)
	response.PRNumber, response.PRState = &pull.Number, &state
	response.PRURL, response.Merged, response.Draft = pull.URL, &pull.Merged, &pull.IsDraft
	response.Head, response.Base, response.HeadSHA = branch, pull.BaseRefName, pull.HeadRefOID
	response.UpdatedAt = &pull.UpdatedAt
	response.ReviewDecision = strings.ToLower(pull.ReviewDecision)
	if response.ReviewDecision == "" {
		response.ReviewDecision = graphFactsNone
	}
	response.Mergeable = strings.ToLower(pull.Mergeable)
	if response.Mergeable == "" {
		response.Mergeable = "unknown"
	}
	result, err := p.graphRequest(
		ctx,
		token,
		checksQuery,
		map[string]any{"owner": repo.owner, "name": repo.name, "oid": pull.HeadRefOID},
	)
	if err != nil {
		return response, err
	}
	object := result.Data.Repository.Object
	if object == nil || object.OID != pull.HeadRefOID {
		return response, errors.New("github forge: checks head mismatch")
	}
	response.Checks = &extensioncontract.ForgeChecks{State: graphFactsNone, FailedNames: []string{}}
	if object.StatusCheckRollup != nil {
		response.Checks = aggregateGraphChecks(object.StatusCheckRollup.Contexts)
	}
	return response, nil
}

func (p *Provider) selectGraphPull(
	ctx context.Context,
	repo repository,
	branch, token string,
) (*graphPull, bool, error) {
	incomplete := false
	for _, states := range [][]string{{"OPEN"}, {"MERGED", "CLOSED"}} {
		var cursor *string
		for range 2 {
			result, err := p.graphRequest(ctx, token, pullQuery, map[string]any{
				"owner": repo.owner, "name": repo.name, "branch": branch, "states": states, "after": cursor,
			})
			if err != nil {
				return nil, false, err
			}
			page := result.Data.Repository.PullRequests
			for _, pull := range page.Nodes {
				if pull.HeadRepository != nil &&
					strings.EqualFold(pull.HeadRepository.NameWithOwner, repo.owner+"/"+repo.name) {
					// An incomplete open population cannot justify choosing a terminal PR.
					if incomplete {
						return nil, true, nil
					}
					return &pull, false, nil
				}
			}
			if !page.PageInfo.HasNextPage {
				cursor = nil
				break
			}
			cursor = &page.PageInfo.EndCursor
			if *cursor == "" {
				return nil, false, errors.New("github forge: missing pagination cursor")
			}
		}
		if cursor != nil {
			// A second full page with a continuation leaves selection incomplete.
			incomplete = true
		}
	}
	return nil, incomplete, nil
}

func aggregateGraphChecks(contexts graphContexts) *extensioncontract.ForgeChecks {
	result := &extensioncontract.ForgeChecks{State: "unknown", FailedNames: []string{}}
	if contexts.CheckRunCountsByState == nil || contexts.StatusContextCountsByState == nil {
		return result
	}
	for _, group := range [][]graphCount{contexts.CheckRunCountsByState, contexts.StatusContextCountsByState} {
		for _, count := range group {
			switch count.State {
			case "SUCCESS", "NEUTRAL", "SKIPPED":
				result.Passed += count.Count
			case "FAILURE", "ERROR", graphCheckCancelled, "TIMED_OUT", "ACTION_REQUIRED", "STALE", "STARTUP_FAILURE":
				result.Failed += count.Count
			default:
				result.Pending += count.Count
			}
		}
	}
	if result.Passed+result.Failed+result.Pending != contexts.TotalCount {
		return result
	}
	switch {
	case result.Failed > 0:
		result.State = "failing"
	case result.Pending > 0:
		result.State = "pending"
	case result.Passed > 0:
		result.State = "passing"
	default:
		result.State = graphFactsNone
	}
	for _, node := range contexts.Nodes {
		state, name := node.Conclusion, node.Name
		if node.Context != "" {
			state, name = node.State, node.Context
		}
		switch state {
		case "FAILURE", "ERROR", graphCheckCancelled, "TIMED_OUT", "ACTION_REQUIRED", "STALE", "STARTUP_FAILURE":
			if name != "" && len(result.FailedNames) < 5 {
				result.FailedNames = append(result.FailedNames, name)
			}
		}
	}
	return result
}

func (p *Provider) tryGraphStatus(
	ctx context.Context,
	repo repository,
	branch, token string,
) (extensioncontract.ForgeStatusResponse, bool, error) {
	response, err := p.graphStatus(ctx, repo, branch, token)
	if err == nil {
		return response, true, nil
	}
	apiErr, ok := errors.AsType[*githubAPIError](err)
	if ok && (apiErr.status == http.StatusNotFound || apiErr.status == http.StatusNotImplemented) {
		return response, false, nil
	}
	if cause := githubErrorCause(err); cause != "" {
		response.Cause = cause
		if ok {
			response.RetryAfter = apiErr.retryAfter
		}
		return response, true, nil
	}
	return extensioncontract.ForgeStatusResponse{}, true, err
}
