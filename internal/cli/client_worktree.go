package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type WorktreeRecord = contract.WorktreePayload
type WorktreeListRecord = contract.WorktreesResponse
type WorktreeInspectRecord = contract.WorktreeInspectionResponse
type WorktreeStatusRecord = contract.WorktreeStatusResponse
type WorktreeExitPlanRecord = contract.WorktreeExitPlanResponse
type WorktreeExitOperationRecord = contract.WorktreeExitOperationResponse
type WorktreeCreateRequest = contract.CreateWorktreeRequest
type WorktreeExitActionRequest = contract.RunWorktreeExitActionRequest

type worktreeClientAPI interface {
	ListWorktrees(context.Context, string, bool) (WorktreeListRecord, error)
	CreateWorktree(context.Context, string, WorktreeCreateRequest) (WorktreeRecord, error)
	CancelWorktreeCreate(context.Context, string, string) error
	AdoptWorktree(context.Context, string, string) (WorktreeRecord, error)
	InspectWorktree(context.Context, string, string) (WorktreeInspectRecord, error)
	GetWorktreeStatus(context.Context, string, string, bool, bool) (WorktreeStatusRecord, error)
	GetWorktreeExitPlan(context.Context, string, string) (WorktreeExitPlanRecord, error)
	RunWorktreeExitAction(
		context.Context, string, string, WorktreeExitActionRequest,
	) (WorktreeExitOperationRecord, error)
	CancelWorktreeExitAction(context.Context, string, string, string) error
	RemoveWorktree(context.Context, string, string, bool) error
	DismissWorktree(context.Context, string, string) error
}

func (c *daemonClient) ListWorktrees(
	ctx context.Context,
	workspaceRef string,
	refresh bool,
) (WorktreeListRecord, error) {
	var response WorktreeListRecord
	query := url.Values{}
	if refresh {
		query.Set("refresh", strconv.FormatBool(refresh))
	}
	err := c.doJSON(ctx, http.MethodGet, worktreeCollectionPath(workspaceRef), query, nil, &response)
	return response, err
}

func (c *daemonClient) CreateWorktree(
	ctx context.Context,
	workspaceRef string,
	request WorktreeCreateRequest,
) (WorktreeRecord, error) {
	var response contract.WorktreeResponse
	err := c.doJSON(ctx, http.MethodPost, worktreeCollectionPath(workspaceRef), nil, request, &response)
	return response.Worktree, err
}

func (c *daemonClient) CancelWorktreeCreate(ctx context.Context, workspaceRef, ref string) error {
	return c.doJSON(ctx, http.MethodPost, worktreeItemPath(workspaceRef, ref)+"/cancel", nil, nil, nil)
}

func (c *daemonClient) AdoptWorktree(
	ctx context.Context,
	workspaceRef string,
	path string,
) (WorktreeRecord, error) {
	var response contract.WorktreeResponse
	err := c.doJSON(
		ctx, http.MethodPost, worktreeCollectionPath(workspaceRef)+"/adopt", nil,
		contract.AdoptWorktreeRequest{Path: path}, &response,
	)
	return response.Worktree, err
}

func (c *daemonClient) InspectWorktree(
	ctx context.Context,
	workspaceRef string,
	ref string,
) (WorktreeInspectRecord, error) {
	var response WorktreeInspectRecord
	err := c.doJSON(ctx, http.MethodGet, worktreeItemPath(workspaceRef, ref), nil, nil, &response)
	return response, err
}

func (c *daemonClient) GetWorktreeStatus(
	ctx context.Context,
	workspaceRef string,
	ref string,
	refresh bool,
	forge bool,
) (WorktreeStatusRecord, error) {
	query := url.Values{}
	if refresh {
		query.Set("refresh", strconv.FormatBool(refresh))
	}
	if forge {
		query.Set("forge", strconv.FormatBool(forge))
	}
	var response WorktreeStatusRecord
	err := c.doJSON(ctx, http.MethodGet, worktreeItemPath(workspaceRef, ref)+"/status", query, nil, &response)
	return response, err
}

func (c *daemonClient) GetWorktreeExitPlan(
	ctx context.Context,
	workspaceRef string,
	ref string,
) (WorktreeExitPlanRecord, error) {
	var response WorktreeExitPlanRecord
	err := c.doJSON(ctx, http.MethodGet, worktreeItemPath(workspaceRef, ref)+"/exit", nil, nil, &response)
	return response, err
}

func (c *daemonClient) RunWorktreeExitAction(
	ctx context.Context,
	workspaceRef string,
	ref string,
	request WorktreeExitActionRequest,
) (WorktreeExitOperationRecord, error) {
	var response WorktreeExitOperationRecord
	err := c.doJSON(
		ctx, http.MethodPost, worktreeItemPath(workspaceRef, ref)+"/exit/actions", nil, request, &response,
	)
	return response, err
}

func (c *daemonClient) CancelWorktreeExitAction(
	ctx context.Context,
	workspaceRef string,
	ref string,
	opID string,
) error {
	return c.doJSON(
		ctx, http.MethodPost, worktreeItemPath(workspaceRef, ref)+"/exit/cancel", nil,
		contract.CancelWorktreeExitActionRequest{OperationID: strings.TrimSpace(opID)}, nil,
	)
}

func (c *daemonClient) RemoveWorktree(ctx context.Context, workspaceRef, ref string, force bool) error {
	query := url.Values{}
	if force {
		query.Set("force", strconv.FormatBool(force))
	}
	return c.doJSON(ctx, http.MethodDelete, worktreeItemPath(workspaceRef, ref), query, nil, nil)
}

func (c *daemonClient) DismissWorktree(ctx context.Context, workspaceRef, ref string) error {
	return c.doJSON(ctx, http.MethodPost, worktreeItemPath(workspaceRef, ref)+"/dismiss", nil, nil, nil)
}

func worktreeCollectionPath(workspaceRef string) string {
	return "/api/workspaces/" + url.PathEscape(strings.TrimSpace(workspaceRef)) + "/worktrees"
}

func worktreeItemPath(workspaceRef, ref string) string {
	return worktreeCollectionPath(workspaceRef) + "/" + url.PathEscape(strings.TrimSpace(ref))
}
