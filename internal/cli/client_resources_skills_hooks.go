package cli

import (
	"context"

	"net/http"
	"net/url"

	"strings"
)

func (c *unixSocketClient) ListResources(ctx context.Context, query ResourceListQuery) ([]ResourceRecord, error) {
	var response struct {
		Records []ResourceRecord `json:"records"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/resources", resourceListValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Records, nil
}

func (c *unixSocketClient) GetResource(ctx context.Context, kind string, id string) (ResourceRecord, error) {
	var response struct {
		Record ResourceRecord `json:"record"`
	}
	path := "/api/resources/" + url.PathEscape(strings.TrimSpace(kind)) + "/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return ResourceRecord{}, err
	}
	return response.Record, nil
}

func (c *unixSocketClient) PutResource(
	ctx context.Context,
	kind string,
	id string,
	request ResourcePutRequest,
) (ResourceRecord, error) {
	var response struct {
		Record ResourceRecord `json:"record"`
	}
	path := "/api/resources/" + url.PathEscape(strings.TrimSpace(kind)) + "/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return ResourceRecord{}, err
	}
	return response.Record, nil
}

func (c *unixSocketClient) DeleteResource(
	ctx context.Context,
	kind string,
	id string,
	request ResourceDeleteRequest,
) error {
	path := "/api/resources/" + url.PathEscape(strings.TrimSpace(kind)) + "/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSON(ctx, http.MethodDelete, path, nil, request, nil)
}

func (c *unixSocketClient) ListSkills(ctx context.Context, query SkillQuery) ([]SkillRecord, error) {
	var response struct {
		Skills []SkillRecord `json:"skills"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/skills", skillValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Skills, nil
}

func (c *unixSocketClient) GetSkill(ctx context.Context, name string, query SkillQuery) (SkillRecord, error) {
	var response struct {
		Skill SkillRecord `json:"skill"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/skills/"+url.PathEscape(strings.TrimSpace(name)),
		skillValues(query),
		nil,
		&response,
	); err != nil {
		return SkillRecord{}, err
	}
	return response.Skill, nil
}

func (c *unixSocketClient) GetSkillContent(ctx context.Context, name string, query SkillQuery) (string, error) {
	var response struct {
		Content string `json:"content"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/skills/"+url.PathEscape(strings.TrimSpace(name))+"/content",
		skillValues(query),
		nil,
		&response,
	); err != nil {
		return "", err
	}
	return response.Content, nil
}

func (c *unixSocketClient) GetSkillShadows(
	ctx context.Context,
	name string,
	query SkillQuery,
) (SkillShadowsRecord, error) {
	var response SkillShadowsRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/skills/"+url.PathEscape(strings.TrimSpace(name))+"/shadows",
		skillValues(query),
		nil,
		&response,
	); err != nil {
		return SkillShadowsRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) EnableSkill(ctx context.Context, name string, query SkillQuery) (SkillActionRecord, error) {
	return c.skillAction(ctx, strings.TrimSpace(name), "enable", query)
}

func (c *unixSocketClient) DisableSkill(ctx context.Context, name string, query SkillQuery) (SkillActionRecord, error) {
	return c.skillAction(ctx, strings.TrimSpace(name), "disable", query)
}

func (c *unixSocketClient) InstallSkillMarketplace(
	ctx context.Context,
	request SkillMarketplaceInstallRequest,
) (SkillMarketplaceInstallRecord, error) {
	var response struct {
		Skill SkillMarketplaceInstallRecord `json:"skill"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/skills/marketplace/install", nil, request, &response); err != nil {
		return SkillMarketplaceInstallRecord{}, err
	}
	return response.Skill, nil
}

func (c *unixSocketClient) UpdateSkillMarketplace(
	ctx context.Context,
	request SkillMarketplaceUpdateRequest,
) ([]SkillMarketplaceUpdateRecord, error) {
	var response struct {
		Skills []SkillMarketplaceUpdateRecord `json:"skills"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/skills/marketplace/update", nil, request, &response); err != nil {
		return nil, err
	}
	return response.Skills, nil
}

func (c *unixSocketClient) RemoveSkillMarketplace(
	ctx context.Context,
	name string,
) (SkillMarketplaceRemoveRecord, error) {
	var response struct {
		Skill SkillMarketplaceRemoveRecord `json:"skill"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		"/api/skills/marketplace/"+url.PathEscape(strings.TrimSpace(name)),
		nil,
		nil,
		&response,
	); err != nil {
		return SkillMarketplaceRemoveRecord{}, err
	}
	return response.Skill, nil
}

func (c *unixSocketClient) HookCatalog(ctx context.Context, query HookCatalogQuery) ([]HookCatalogRecord, error) {
	var response struct {
		Hooks []HookCatalogRecord `json:"hooks"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/hooks/catalog",
		hookCatalogValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Hooks, nil
}

func (c *unixSocketClient) HookRuns(
	ctx context.Context,
	workspaceRef string,
	query HookRunsQuery,
) ([]HookRunRecord, error) {
	var response struct {
		Runs []HookRunRecord `json:"runs"`
	}
	path, err := hooksBasePath(workspaceRef)
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path+"/runs", hookRunsValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

func (c *unixSocketClient) HookEvents(ctx context.Context, query HookEventsQuery) ([]HookEventRecord, error) {
	var response struct {
		Events []HookEventRecord `json:"events"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/hooks/events", hookEventsValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Events, nil
}

func (c *unixSocketClient) ListLogs(ctx context.Context, query LogsListQuery) ([]LogEventRecord, error) {
	var response struct {
		Events []LogEventRecord `json:"events"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/logs",
		logsListValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Events, nil
}

func (c *unixSocketClient) StreamLogs(
	ctx context.Context,
	query LogsListQuery,
	lastEventID string,
	handler SSEHandler,
) error {
	return c.doSSE(
		ctx,
		"/api/logs/stream",
		logsListValues(query),
		lastEventID,
		handler,
	)
}
