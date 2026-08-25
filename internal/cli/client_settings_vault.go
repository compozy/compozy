package cli

import (
	"context"

	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"

	mcppkg "github.com/compozy/compozy/internal/mcp"

	"github.com/compozy/compozy/internal/sse"
)

var _ DaemonClient = (*daemonClient)(nil)
var _ mcppkg.HostedProxyClient = (*daemonClient)(nil)

var errStopSSE = sse.ErrStop

func defaultCLIRedirectPolicy(_ *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// NewClient constructs a daemon client for one fully resolved connection target.
func NewClient(target ClientTarget) (DaemonClient, error) {
	if err := target.validate(); err != nil {
		return nil, err
	}

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("cli: default HTTP transport is not an HTTP transport")
	}
	transport := defaultTransport.Clone()
	switch target.kind {
	case clientTargetLocal:
		transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialUnix(
					ctx,
					clientUnixNetwork,
					nil,
					&net.UnixAddr{Name: target.socketPath, Net: clientUnixNetwork},
				)
			},
		}
	case clientTargetSSHForward:
		transport.Proxy = nil
	}
	checkRedirect := defaultCLIRedirectPolicy
	if target.isRemoteGateway() {
		checkRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &daemonClient{
		target: target,
		httpClient: &http.Client{
			Transport: transport, Timeout: defaultUnixSocketClientTimeout, CheckRedirect: checkRedirect,
		},
		streamClient: &http.Client{Transport: transport, CheckRedirect: checkRedirect},
	}, nil
}

func (c *daemonClient) TriggerSettingsRestart(ctx context.Context) (SettingsRestartActionRecord, error) {
	var response SettingsRestartActionRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/settings/actions/restart", nil, nil, &response); err != nil {
		return SettingsRestartActionRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) GetSettingsRestartStatus(
	ctx context.Context,
	operationID string,
) (SettingsRestartStatusRecord, error) {
	path := "/api/settings/actions/restart/" + url.PathEscape(strings.TrimSpace(operationID))
	var response SettingsRestartStatusRecord
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SettingsRestartStatusRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) CreateSupportBundle(
	ctx context.Context,
	request CreateSupportBundleRequest,
) (SupportBundleOperationRecord, error) {
	var response contract.SupportBundleOperationResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/support/bundles", nil, request, &response); err != nil {
		return SupportBundleOperationRecord{}, err
	}
	return response.Operation, nil
}

func (c *daemonClient) GetSupportBundle(
	ctx context.Context,
	operationID string,
) (SupportBundleOperationRecord, error) {
	path := supportBundleOperationPath(operationID)
	var response contract.SupportBundleOperationResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SupportBundleOperationRecord{}, err
	}
	return response.Operation, nil
}

func (c *daemonClient) DownloadSupportBundle(
	ctx context.Context,
	operationID string,
	dst io.Writer,
) (err error) {
	if dst == nil {
		return errors.New("cli: support bundle download writer is required")
	}
	response, err := c.doRequest(ctx, http.MethodGet, supportBundleOperationPath(operationID)+"/download", nil, nil)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			closeErr = fmt.Errorf("cli: close support bundle download response: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return readAPIError(response)
	}
	if _, err := io.Copy(dst, response.Body); err != nil {
		return fmt.Errorf("cli: download support bundle: %w", err)
	}
	return nil
}

func supportBundleOperationPath(operationID string) string {
	return "/api/support/bundles/" + url.PathEscape(strings.TrimSpace(operationID))
}

func (c *daemonClient) GetSettingsUpdate(ctx context.Context) (SettingsUpdateRecord, error) {
	var response SettingsUpdateRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/update", nil, nil, &response); err != nil {
		return SettingsUpdateRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) UpdateSettingsSkills(
	ctx context.Context,
	request UpdateSettingsSkillsRequest,
) (SettingsMutationRecord, error) {
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPatch, "/api/settings/skills", nil, request, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) UpdateSettingsSkillsAtScope(
	ctx context.Context,
	query settingsSkillsScopeQuery,
	request UpdateSettingsSkillsRequest,
) (SettingsMutationRecord, error) {
	values := settingsSkillsQueryValues(query)
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPatch, "/api/settings/skills", values, request, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) GetSettingsSkills(
	ctx context.Context,
	query settingsSkillsScopeQuery,
) (contract.SettingsSkillsResponse, error) {
	values := settingsSkillsQueryValues(query)
	var response contract.SettingsSkillsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/skills", values, nil, &response); err != nil {
		return contract.SettingsSkillsResponse{}, err
	}
	return response, nil
}

func settingsSkillsQueryValues(query settingsSkillsScopeQuery) url.Values {
	values := url.Values{}
	if query.Scope != "" {
		values.Set("scope", string(query.Scope))
	}
	if workspaceID := strings.TrimSpace(query.WorkspaceID); workspaceID != "" {
		values.Set("workspace_id", workspaceID)
	}
	if profile := strings.TrimSpace(query.Profile); profile != "" {
		values.Set("profile", profile)
	}
	return values
}

func (c *daemonClient) UpdateSettingsAttention(
	ctx context.Context,
	request UpdateSettingsAttentionRequest,
) (SettingsMutationRecord, error) {
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPatch, "/api/settings/attention", nil, request, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) UpdateSettingsShell(
	ctx context.Context,
	request UpdateSettingsShellRequest,
) (SettingsMutationRecord, error) {
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPatch, "/api/settings/shell", nil, request, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ReloadSettings(ctx context.Context) (SettingsMutationRecord, error) {
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/settings/reload", nil, nil, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ListSettingsApplyRecords(
	ctx context.Context,
	query SettingsApplyHistoryQuery,
) (SettingsApplyHistoryRecord, error) {
	values := url.Values{}
	if strings.TrimSpace(query.Status) != "" {
		values.Set("status", strings.TrimSpace(query.Status))
	}
	if strings.TrimSpace(query.Actor) != "" {
		values.Set("actor", strings.TrimSpace(query.Actor))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var response SettingsApplyHistoryRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/apply", values, nil, &response); err != nil {
		return SettingsApplyHistoryRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ListVaultSecrets(ctx context.Context, query VaultListQuery) ([]VaultRecord, error) {
	var response struct {
		Secrets []VaultRecord `json:"secrets"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/vault/secrets",
		vaultListValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Secrets, nil
}

func (c *daemonClient) GetVaultSecret(ctx context.Context, ref string) (VaultRecord, error) {
	trimmedRef, err := requireVaultRef(ref)
	if err != nil {
		return VaultRecord{}, err
	}
	var response struct {
		Secret VaultRecord `json:"secret"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/vault/secrets/metadata",
		vaultRefValues(trimmedRef),
		nil,
		&response,
	); err != nil {
		return VaultRecord{}, err
	}
	return response.Secret, nil
}

func (c *daemonClient) PutVaultSecret(
	ctx context.Context,
	request PutVaultSecretRequest,
) (VaultRecord, error) {
	var response struct {
		Secret VaultRecord `json:"secret"`
	}
	if err := c.doJSON(ctx, http.MethodPut, "/api/vault/secrets", nil, request, &response); err != nil {
		return VaultRecord{}, err
	}
	return response.Secret, nil
}

func (c *daemonClient) DeleteVaultSecret(ctx context.Context, ref string) error {
	trimmedRef, err := requireVaultRef(ref)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/vault/secrets", vaultRefValues(trimmedRef), nil, nil)
}
