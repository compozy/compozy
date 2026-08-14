package github

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

var githubTemplatePaths = []string{
	".github/pull_request_template.md",
	".github/pull_request_template.txt",
	"pull_request_template.md",
	"pull_request_template.txt",
	"docs/pull_request_template.md",
	"docs/pull_request_template.txt",
}

const githubProviderName = "github"

type Provider struct {
	credentials credentialResolver
	api         githubClient
	now         func() time.Time
}

type repositoryPayload struct {
	DefaultBranch string `json:"default_branch"`
}

type pullPayload struct {
	Number   int        `json:"number"`
	HTMLURL  string     `json:"html_url"`
	State    string     `json:"state"`
	MergedAt *time.Time `json:"merged_at"`
}

func newProvider() *Provider {
	return &Provider{credentials: newCredentialResolver(), api: newGitHubClient(), now: time.Now}
}

func (p *Provider) Capabilities(
	ctx context.Context,
	request extensioncontract.ForgeCapabilitiesRequest,
) (extensioncontract.ForgeCapabilitiesResponse, error) {
	repo, served := selectRepository(request.RemoteURLs)
	if !served {
		return extensioncontract.ForgeCapabilitiesResponse{
			Cause: extensioncontract.ForgeCauseUnsupportedRemote,
		}, nil
	}
	response := extensioncontract.ForgeCapabilitiesResponse{
		Served: true, Provider: githubProviderName, ServedRemote: repo.url,
		RequestNoun: "PR", OpenActionLabel: "Open PR", ViewActionLabel: "View PR",
		SupportsDraft: true, CompareURLTemplate: repo.url + "/compare/{base}...{head}",
		TemplatePaths: append([]string(nil), githubTemplatePaths...),
	}
	credential, err := p.credentials.resolve(ctx)
	if err != nil {
		return response, err
	}
	if credential.token == "" {
		response.Cause = extensioncontract.ForgeCauseCredentialAbsent
		return response, nil
	}
	response.CredentialSource = credential.source
	var repositoryDetails repositoryPayload
	if err := p.api.request(
		ctx, http.MethodGet, repositoryAPIPath(repo, ""), credential.token, nil, &repositoryDetails,
	); err != nil {
		if response.Cause = githubErrorCause(err); response.Cause != "" {
			return response, nil
		}
		return response, err
	}
	response.Available = true
	response.DefaultBranch = strings.TrimSpace(repositoryDetails.DefaultBranch)
	return response, nil
}

func (p *Provider) Status(
	ctx context.Context,
	request extensioncontract.ForgeStatusRequest,
) (extensioncontract.ForgeStatusResponse, error) {
	repo, ok := selectRepository(request.RemoteURLs)
	if !ok {
		return extensioncontract.ForgeStatusResponse{Cause: extensioncontract.ForgeCauseUnsupportedRemote}, nil
	}
	credential, err := p.credentials.resolve(ctx)
	if err != nil {
		return extensioncontract.ForgeStatusResponse{}, err
	}
	if credential.token == "" {
		return extensioncontract.ForgeStatusResponse{Cause: extensioncontract.ForgeCauseCredentialAbsent}, nil
	}
	pulls, err := p.listPulls(ctx, repo, request.Branch, "all", credential.token)
	if err != nil {
		if cause := githubErrorCause(err); cause != "" {
			return extensioncontract.ForgeStatusResponse{Cause: cause}, nil
		}
		return extensioncontract.ForgeStatusResponse{}, err
	}
	response := extensioncontract.ForgeStatusResponse{Provider: githubProviderName, FetchedAt: p.now().UTC()}
	if len(pulls) == 0 {
		return response, nil
	}
	pull := pulls[0]
	state, merged := pull.State, pull.MergedAt != nil
	if merged {
		state = "merged"
	}
	response.PRNumber, response.PRState = &pull.Number, &state
	response.PRURL, response.Merged = pull.HTMLURL, &merged
	return response, nil
}

func (p *Provider) CreatePR(
	ctx context.Context,
	request extensioncontract.ForgePRCreateRequest,
) (extensioncontract.ForgePRCreateResponse, error) {
	repo, ok := selectRepository(request.RemoteURLs)
	if !ok {
		return extensioncontract.ForgePRCreateResponse{Cause: extensioncontract.ForgeCauseUnsupportedRemote}, nil
	}
	credential, err := p.credentials.resolve(ctx)
	if err != nil {
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	if credential.token == "" {
		return extensioncontract.ForgePRCreateResponse{Cause: extensioncontract.ForgeCauseCredentialAbsent}, nil
	}
	existing, err := p.listPulls(ctx, repo, request.Head, "open", credential.token)
	if err != nil {
		if cause := githubErrorCause(err); cause != "" {
			return extensioncontract.ForgePRCreateResponse{Cause: cause}, nil
		}
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	if len(existing) > 0 {
		if !validPullPayload(existing[0]) {
			return extensioncontract.ForgePRCreateResponse{}, errors.New("github forge: invalid existing pull request")
		}
		return pullCreateResponse("opened_existing", existing[0]), nil
	}
	var created pullPayload
	err = p.api.request(ctx, http.MethodPost, repositoryAPIPath(repo, "/pulls"), credential.token, map[string]any{
		"head": request.Head, "base": request.Base, "title": request.Title,
		"body": request.Body, "draft": request.Draft,
	}, &created)
	if err != nil {
		if cause := githubErrorCause(err); cause != "" {
			return extensioncontract.ForgePRCreateResponse{Cause: cause}, nil
		}
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	if !validPullPayload(created) {
		return extensioncontract.ForgePRCreateResponse{}, errors.New("github forge: invalid created pull request")
	}
	return pullCreateResponse("created", created), nil
}

func (p *Provider) listPulls(
	ctx context.Context,
	repo repository,
	branch string,
	state string,
	token string,
) ([]pullPayload, error) {
	var pulls []pullPayload
	err := p.api.request(ctx, http.MethodGet, pullListPath(repo, branch, state), token, nil, &pulls)
	return pulls, err
}

func pullCreateResponse(status string, pull pullPayload) extensioncontract.ForgePRCreateResponse {
	return extensioncontract.ForgePRCreateResponse{
		Status: status, Number: pull.Number, URL: pull.HTMLURL,
	}
}

func validPullPayload(pull pullPayload) bool {
	return pull.Number > 0 && strings.TrimSpace(pull.HTMLURL) != ""
}

func githubErrorCause(err error) string {
	var apiErr *githubAPIError
	if errors.As(err, &apiErr) {
		return apiErr.cause
	}
	return ""
}
