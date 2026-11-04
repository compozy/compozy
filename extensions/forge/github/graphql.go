package github

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const graphFactsNone = "none"
const graphChecksUnknown = "unknown"
const graphCheckCancelled = "CANCELLED" //nolint:misspell // GitHub's GraphQL enum spelling.

const pullQuery = `query($owner:String!,$name:String!,$branch:String!,$states:[PullRequestState!]!,$after:String){
 repository(owner:$owner,name:$name){pullRequests(
 headRefName:$branch,states:$states,first:50,after:$after,orderBy:{field:UPDATED_AT,direction:DESC}){
 nodes{number title url state isDraft merged headRefOid baseRefName updatedAt
 reviewDecision mergeable headRepository{nameWithOwner}}
 pageInfo{hasNextPage endCursor}}}}`

const checksQuery = `query($owner:String!,$name:String!,$oid:GitObjectID!,$after:String){repository(owner:$owner,name:$name){object(oid:$oid){... on Commit{oid statusCheckRollup{contexts(first:100,after:$after){totalCount checkRunCountsByState{state count} statusContextCountsByState{state count} nodes{... on CheckRun{name conclusion status} ... on StatusContext{context state}} pageInfo{hasNextPage endCursor}}}}}}}`

// maxCheckPages bounds how far naming failures may page through a head's check contexts.
const maxCheckPages = 5

type graphPull struct {
	Title          string
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

func (p *graphPull) isOpen() bool { return strings.EqualFold(p.State, "OPEN") }

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
	PageInfo                   struct {
		HasNextPage bool
		EndCursor   string
	}
}

type graphResponse struct {
	Data struct {
		Repository *struct {
			PullRequests graphPage
			Issues       *graphIssuesPage
			Object       *struct {
				OID               string
				StatusCheckRollup *struct{ Contexts graphContexts }
			}
			PullRequest *graphReviewActivity
		}
	}
	Errors []graphError
}

type graphError struct {
	Type       string
	Extensions struct{ Code string }
}

// errGraphUnsupported reports a query GitHub refused for good: the token or server cannot serve it, so
// retrying would change nothing. Every other GraphQL error may be transient.
var errGraphUnsupported = errors.New("github forge: GraphQL query unsupported")

// A validation error, such as a field an older server lacks, carries no type but names its code in
// extensions. With neither, the error is an execution failure, which GitHub also leaves untyped.
func graphErrorMeansUnsupported(failure *graphError) bool {
	switch failure.Type {
	case "":
		return failure.Extensions.Code != ""
	case "FORBIDDEN", "NOT_FOUND", "INSUFFICIENT_SCOPES":
		return true
	default:
		return false
	}
}

func graphPageVariables(repo repository, cursor *string) map[string]any {
	return map[string]any{"owner": repo.owner, "name": repo.name, "after": cursor}
}

func (p *Provider) graphRequest(
	ctx context.Context,
	token, query string,
	variables map[string]any,
) (graphResponse, error) {
	var result graphResponse
	var headers http.Header
	if err := p.api.requestWithHeaders(
		ctx,
		http.MethodPost,
		"/graphql",
		token,
		map[string]any{"query": query, "variables": variables},
		&result, &headers,
	); err != nil {
		return result, err
	}
	if len(result.Errors) > 0 {
		unsupported := true
		for i := range result.Errors {
			e := &result.Errors[i]
			if e.Type == "RATE_LIMITED" {
				failure := &githubAPIError{
					cause:  extensioncontract.ForgeCauseRateLimited,
					status: http.StatusTooManyRequests,
				}
				if reset, err := strconv.ParseInt(headers.Get("X-RateLimit-Reset"), 10, 64); err == nil && reset > 0 {
					failure.retryAfter = new(time.Unix(reset, 0).UTC())
				}
				return result, failure
			}
			unsupported = unsupported && graphErrorMeansUnsupported(e)
		}
		if unsupported {
			return result, errGraphUnsupported
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
			response.Selection = pullSelectionIncomplete
		}
		return response, nil
	}
	state := strings.ToLower(pull.State)
	response.Title = pull.Title
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
	if response.Checks, err = p.readGraphChecks(ctx, repo, pull.HeadRefOID, token); err != nil {
		return response, err
	}
	if !pull.isOpen() {
		return response, nil
	}
	reviews, err := p.readGraphReviews(ctx, repo, pull.Number, token)
	switch {
	case err == nil:
		response.Reviews = reviews
	case githubErrorCause(err) != "":
		// Reviews are optional, but a limit or credential failure applies to the whole read.
		return response, err
	case !graphReviewsUnsupported(err):
		// Checks stay good, and a transient failure must not read as a forge without reviews.
		response.ReviewsUnavailable = true
	}
	return response, nil
}

func graphReviewsUnsupported(err error) bool {
	if errors.Is(err, errGraphUnsupported) {
		return true
	}
	apiErr, ok := errors.AsType[*githubAPIError](err)
	return ok && (apiErr.status == http.StatusNotFound || apiErr.status == http.StatusNotImplemented)
}

// readGraphChecks pages the head's check contexts only as far as naming its failures needs.
func (p *Provider) readGraphChecks(
	ctx context.Context,
	repo repository,
	head, token string,
) (*extensioncontract.ForgeChecks, error) {
	var checks *extensioncontract.ForgeChecks
	var cursor *string
	for range maxCheckPages {
		variables := graphPageVariables(repo, cursor)
		variables["oid"] = head
		result, err := p.graphRequest(ctx, token, checksQuery, variables)
		if err != nil {
			return nil, err
		}
		object := result.Data.Repository.Object
		if object == nil || object.OID != head {
			return nil, errors.New("github forge: checks head mismatch")
		}
		if object.StatusCheckRollup == nil {
			return &extensioncontract.ForgeChecks{State: graphFactsNone, FailedNames: []string{}}, nil
		}
		contexts := object.StatusCheckRollup.Contexts
		if checks == nil {
			checks = aggregateGraphChecks(contexts)
		}
		appendFailedNames(checks, contexts)
		wanted := min(checks.Failed, extensioncontract.MaxForgeFailedNames)
		if checks.State == graphChecksUnknown || len(checks.FailedNames) >= wanted ||
			!contexts.PageInfo.HasNextPage || contexts.PageInfo.EndCursor == "" {
			break
		}
		cursor = &contexts.PageInfo.EndCursor
	}
	return checks, nil
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
			variables := graphPageVariables(repo, cursor)
			variables["branch"], variables["states"] = branch, states
			result, err := p.graphRequest(ctx, token, pullQuery, variables)
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
	result := &extensioncontract.ForgeChecks{State: graphChecksUnknown, FailedNames: []string{}}
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
	return result
}

func appendFailedNames(checks *extensioncontract.ForgeChecks, contexts graphContexts) {
	if checks.State == graphChecksUnknown {
		return
	}
	for _, node := range contexts.Nodes {
		state, name := node.Conclusion, node.Name
		if node.Context != "" {
			state, name = node.State, node.Context
		}
		switch state {
		case "FAILURE", "ERROR", graphCheckCancelled, "TIMED_OUT", "ACTION_REQUIRED", "STALE", "STARTUP_FAILURE":
			if name != "" && len(checks.FailedNames) < extensioncontract.MaxForgeFailedNames {
				checks.FailedNames = append(checks.FailedNames, name)
			}
		}
	}
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
