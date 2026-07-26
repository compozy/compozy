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

	"github.com/compozy/agh/internal/api/contract"

	mcppkg "github.com/compozy/agh/internal/mcp"

	"github.com/compozy/agh/internal/sse"
)

var _ DaemonClient = (*unixSocketClient)(nil)
var _ mcppkg.HostedProxyClient = (*unixSocketClient)(nil)

var errStopSSE = sse.ErrStop

// NewClient constructs a daemon client that talks HTTP over a Unix domain socket.
func NewClient(socketPath string) (DaemonClient, error) {
	path := strings.TrimSpace(socketPath)
	if path == "" {
		return nil, errors.New("cli: daemon socket path is required")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", path)
		},
	}

	return &unixSocketClient{
		socketPath:   path,
		httpClient:   &http.Client{Transport: transport, Timeout: defaultUnixSocketClientTimeout},
		streamClient: &http.Client{Transport: transport},
	}, nil
}

func (c *unixSocketClient) TriggerSettingsRestart(ctx context.Context) (SettingsRestartActionRecord, error) {
	var response SettingsRestartActionRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/settings/actions/restart", nil, nil, &response); err != nil {
		return SettingsRestartActionRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetSettingsRestartStatus(
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

func (c *unixSocketClient) CreateSupportBundle(
	ctx context.Context,
	request CreateSupportBundleRequest,
) (SupportBundleOperationRecord, error) {
	var response contract.SupportBundleOperationResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/support/bundles", nil, request, &response); err != nil {
		return SupportBundleOperationRecord{}, err
	}
	return response.Operation, nil
}

func (c *unixSocketClient) GetSupportBundle(
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

func (c *unixSocketClient) DownloadSupportBundle(
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

func (c *unixSocketClient) GetSettingsUpdate(ctx context.Context) (SettingsUpdateRecord, error) {
	var response SettingsUpdateRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/update", nil, nil, &response); err != nil {
		return SettingsUpdateRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) UpdateSettingsSkills(
	ctx context.Context,
	request UpdateSettingsSkillsRequest,
) (SettingsMutationRecord, error) {
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPatch, "/api/settings/skills", nil, request, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ReloadSettings(ctx context.Context) (SettingsMutationRecord, error) {
	var response SettingsMutationRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/settings/reload", nil, nil, &response); err != nil {
		return SettingsMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListSettingsApplyRecords(
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

func (c *unixSocketClient) ListVaultSecrets(ctx context.Context, query VaultListQuery) ([]VaultRecord, error) {
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

func (c *unixSocketClient) GetVaultSecret(ctx context.Context, ref string) (VaultRecord, error) {
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

func (c *unixSocketClient) PutVaultSecret(
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

func (c *unixSocketClient) DeleteVaultSecret(ctx context.Context, ref string) error {
	trimmedRef, err := requireVaultRef(ref)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/vault/secrets", vaultRefValues(trimmedRef), nil, nil)
}
