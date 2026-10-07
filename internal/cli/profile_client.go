package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type profileResolutionClient interface {
	ListProfiles(context.Context) ([]contract.Profile, error)
	ListProfileSelections(context.Context) ([]contract.ProfileSelection, error)
}

type profileClientAPI interface {
	profileResolutionClient
	CreateProfile(context.Context, contract.CreateProfileRequest) (contract.Profile, error)
	UpdateProfile(context.Context, string, contract.UpdateProfileRequest) (contract.Profile, error)
	PrepareProfileRename(context.Context, string, string) (contract.RenameProfilePlan, error)
	RenameProfile(context.Context, string, contract.RenameProfileRequest) (contract.RenameProfileResponse, error)
	PrepareProfileArchive(context.Context, string) (contract.ArchiveProfilePlan, error)
	ArchiveProfile(context.Context, string, string) (contract.ArchiveProfileResponse, error)
	UnarchiveProfile(context.Context, string) (contract.UnarchiveProfileResponse, error)
	PrepareProfileDelete(context.Context, string) (contract.DeleteProfilePlan, error)
	DeleteProfile(context.Context, string, string) (contract.DeleteProfileResponse, error)
	PutProfileSelection(context.Context, contract.ProfileSelection) (contract.ProfileSelection, error)
	ListProfileOperations(context.Context) ([]contract.ProfileOperation, error)
	RetryProfileOperation(context.Context, string) (contract.ProfileOperation, error)
}

func optionalProfileResolutionClient(client any) profileResolutionClient {
	profiles, ok := client.(profileResolutionClient)
	if !ok {
		return nil
	}
	return profiles
}

func profileResolutionClientFromDeps(deps commandDeps) (profileResolutionClient, DaemonClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, nil, err
	}
	profiles, ok := client.(profileResolutionClient)
	if !ok {
		return nil, nil, newProfileUnavailableError()
	}
	return profiles, client, nil
}

func profileClientFromDeps(deps commandDeps) (profileClientAPI, DaemonClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, nil, err
	}
	profiles, ok := client.(profileClientAPI)
	if !ok {
		return nil, nil, newProfileUnavailableError()
	}
	return profiles, client, nil
}

func newProfileUnavailableError() error {
	return &profileCommandError{payload: contract.ProfileErrorPayload{Error: contract.ProfileError{
		Code:    "profile_unavailable",
		Message: "profile client is unavailable",
		Action:  "update the CompozyOS client and retry",
	}}}
}

func (c *daemonClient) ListProfiles(ctx context.Context) ([]contract.Profile, error) {
	return c.profileClientJSON[[]contract.Profile](ctx, http.MethodGet, "/api/profiles", nil, nil)
}

func (c *daemonClient) CreateProfile(
	ctx context.Context,
	request contract.CreateProfileRequest,
) (contract.Profile, error) {
	return c.profileClientJSON[contract.Profile](ctx, http.MethodPost, "/api/profiles", nil, request)
}

func (c *daemonClient) UpdateProfile(
	ctx context.Context,
	name string,
	request contract.UpdateProfileRequest,
) (contract.Profile, error) {
	return c.profileClientJSON[contract.Profile](ctx, http.MethodPatch, profilePath(name), nil, request)
}

func (c *daemonClient) PrepareProfileRename(
	ctx context.Context,
	name, newName string,
) (contract.RenameProfilePlan, error) {
	query := url.Values{"new_name": []string{strings.TrimSpace(newName)}}
	return c.profileClientJSON[contract.RenameProfilePlan](
		ctx,
		http.MethodGet,
		profilePath(name)+"/rename-plan",
		query,
		nil,
	)
}

func (c *daemonClient) RenameProfile(
	ctx context.Context,
	name string,
	request contract.RenameProfileRequest,
) (contract.RenameProfileResponse, error) {
	return c.profileClientJSON[contract.RenameProfileResponse](
		ctx,
		http.MethodPost,
		profilePath(name)+"/rename",
		nil,
		request,
	)
}

func (c *daemonClient) PrepareProfileArchive(ctx context.Context, name string) (contract.ArchiveProfilePlan, error) {
	return c.profileClientJSON[contract.ArchiveProfilePlan](
		ctx,
		http.MethodGet,
		profilePath(name)+"/archive-plan",
		nil,
		nil,
	)
}

func (c *daemonClient) ArchiveProfile(
	ctx context.Context,
	name, revision string,
) (contract.ArchiveProfileResponse, error) {
	request := contract.ProfilePlanRequest{PlanRevision: revision}
	return c.profileClientJSON[contract.ArchiveProfileResponse](
		ctx,
		http.MethodPost,
		profilePath(name)+"/archive",
		nil,
		request,
	)
}

func (c *daemonClient) UnarchiveProfile(ctx context.Context, name string) (contract.UnarchiveProfileResponse, error) {
	return c.profileClientJSON[contract.UnarchiveProfileResponse](
		ctx,
		http.MethodPost,
		profilePath(name)+"/unarchive",
		nil,
		struct{}{},
	)
}

func (c *daemonClient) PrepareProfileDelete(ctx context.Context, name string) (contract.DeleteProfilePlan, error) {
	return c.profileClientJSON[contract.DeleteProfilePlan](
		ctx,
		http.MethodGet,
		profilePath(name)+"/delete-plan",
		nil,
		nil,
	)
}

func (c *daemonClient) DeleteProfile(
	ctx context.Context,
	name, revision string,
) (contract.DeleteProfileResponse, error) {
	query := url.Values{"plan_revision": []string{strings.TrimSpace(revision)}}
	return c.profileClientJSON[contract.DeleteProfileResponse](ctx, http.MethodDelete, profilePath(name), query, nil)
}

func (c *daemonClient) ListProfileSelections(ctx context.Context) ([]contract.ProfileSelection, error) {
	return c.profileClientJSON[[]contract.ProfileSelection](ctx, http.MethodGet, "/api/profiles/selection", nil, nil)
}

func (c *daemonClient) PutProfileSelection(
	ctx context.Context,
	request contract.ProfileSelection,
) (contract.ProfileSelection, error) {
	return c.profileClientJSON[contract.ProfileSelection](ctx, http.MethodPut, "/api/profiles/selection", nil, request)
}

func (c *daemonClient) ListProfileOperations(ctx context.Context) ([]contract.ProfileOperation, error) {
	return c.profileClientJSON[[]contract.ProfileOperation](ctx, http.MethodGet, "/api/profiles/ops", nil, nil)
}

func (c *daemonClient) RetryProfileOperation(ctx context.Context, id string) (contract.ProfileOperation, error) {
	path := "/api/profiles/ops/" + url.PathEscape(strings.TrimSpace(id)) + "/retry"
	return c.profileClientJSON[contract.ProfileOperation](ctx, http.MethodPost, path, nil, nil)
}

func (c *daemonClient) profileClientJSON[T any](
	ctx context.Context,
	method, path string,
	query url.Values,
	request any,
) (T, error) {
	var response T
	if err := c.doJSON(ctx, method, path, query, request, &response); err != nil {
		return response, err
	}
	return response, nil
}

func profilePath(name string) string {
	return "/api/profiles/" + url.PathEscape(strings.TrimSpace(name))
}

var _ profileClientAPI = (*daemonClient)(nil)
