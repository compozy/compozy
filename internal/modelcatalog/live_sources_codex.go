package modelcatalog

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/subprocess"
)

// CodexModelProbe reads account-scoped model capabilities from the native Codex CLI.
type CodexModelProbe interface {
	InspectCodexModels(context.Context, DiscoveryCommandRequest) ([]CodexModelMetadata, error)
}

// CodexModelMetadata is the capability subset of the Codex model/list response.
type CodexModelMetadata struct {
	ID                        string `json:"id"`
	Model                     string `json:"model"`
	DisplayName               string `json:"displayName"`
	Hidden                    bool   `json:"hidden"`
	IsDefault                 bool   `json:"isDefault"`
	SupportedReasoningEfforts []struct {
		ReasoningEffort string `json:"reasoningEffort"`
	} `json:"supportedReasoningEfforts"`
	DefaultReasoningEffort string `json:"defaultReasoningEffort"`
}

// AppServerCodexModelProbe uses a short-lived app server without creating a thread.
type AppServerCodexModelProbe struct{}

var _ CodexModelProbe = AppServerCodexModelProbe{}

// InspectCodexModels reads every page and joins the managed subprocess on every exit.
func (AppServerCodexModelProbe) InspectCodexModels(
	ctx context.Context, req DiscoveryCommandRequest,
) (_ []CodexModelMetadata, err error) {
	proc, err := subprocess.Launch(ctx, subprocess.LaunchConfig{
		Command: req.Command, Args: []string{"app-server"}, Dir: req.Dir, Env: req.Env,
		DisableTransport: true, ShutdownTimeout: time.Second, StderrTransform: RedactString,
	})
	if err != nil {
		return nil, fmt.Errorf("model catalog: start Codex model discovery: %w", err)
	}
	stop := func() error {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return proc.Shutdown(stopCtx)
	}
	stopped := make(chan error, 1)
	cancelStop := context.AfterFunc(ctx, func() { stopped <- stop() })
	defer func() {
		var stopErr error
		if cancelStop() {
			stopErr = stop()
		} else {
			stopErr = <-stopped
		}
		err = errors.Join(err, ctx.Err(), stopErr)
	}()
	client := codexCatalogClient{writer: json.NewEncoder(proc.Stdin()), reader: bufio.NewScanner(proc.Stdout())}
	client.reader.Buffer(make([]byte, 4096), maxLiveDiscoveryPayloadSize)
	if err := client.call("initialize", map[string]any{
		"clientInfo": map[string]string{"name": "compozy-model-catalog", "version": "1"},
	}, nil); err != nil {
		return nil, err
	}
	if err := client.writer.Encode(map[string]string{"method": "initialized"}); err != nil {
		return nil, fmt.Errorf("model catalog: initialize Codex notifications: %w", err)
	}
	return client.listModels()
}

type codexCatalogClient struct {
	writer *json.Encoder
	reader *bufio.Scanner
	nextID int
}

func (c *codexCatalogClient) call(method string, params any, result any) error {
	c.nextID++
	if err := c.writer.Encode(map[string]any{"id": c.nextID, "method": method, "params": params}); err != nil {
		return fmt.Errorf("model catalog: send Codex %s: %w", method, err)
	}
	for c.reader.Scan() {
		var response struct {
			ID     json.RawMessage      `json:"id"`
			Method string               `json:"method"`
			Result json.RawMessage      `json:"result"`
			Error  *subprocess.RPCError `json:"error"`
		}
		if err := json.Unmarshal(c.reader.Bytes(), &response); err != nil {
			return fmt.Errorf("model catalog: decode Codex %s response: %w", method, err)
		}
		if response.Method != "" {
			if len(response.ID) > 0 && string(response.ID) != "null" {
				return fmt.Errorf(
					"model catalog: unexpected Codex server request %q during %s",
					response.Method,
					method,
				)
			}
			continue
		}
		id := strings.Trim(string(response.ID), "\"")
		if id != strconv.Itoa(c.nextID) {
			continue
		}
		if response.Error != nil {
			return fmt.Errorf("model catalog: Codex %s: %s", method, RedactString(response.Error.Error()))
		}
		if result == nil {
			return nil
		}
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("model catalog: decode Codex %s result: %w", method, err)
		}
		return nil
	}
	return fmt.Errorf("model catalog: read Codex %s: %w", method, errors.Join(io.ErrUnexpectedEOF, c.reader.Err()))
}

func (c *codexCatalogClient) listModels() ([]CodexModelMetadata, error) {
	var models []CodexModelMetadata
	var cursor *string
	seen := make(map[string]bool)
	for {
		var page struct {
			Data       []CodexModelMetadata `json:"data"`
			NextCursor *string              `json:"nextCursor"`
		}
		if err := c.call("model/list", map[string]any{"includeHidden": true, "cursor": cursor}, &page); err != nil {
			return nil, err
		}
		models = append(models, page.Data...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			return models, nil
		}
		if seen[*page.NextCursor] {
			return nil, errors.New("model catalog: Codex model/list repeated its continuation cursor")
		}
		seen[*page.NextCursor] = true
		cursor = page.NextCursor
	}
}

func (s *LiveProviderSource) listCodex(
	ctx context.Context,
	env []string,
	timeout time.Duration,
	now time.Time,
) ([]ModelRow, error) {
	models, err := s.codexProbe.InspectCodexModels(ctx, DiscoveryCommandRequest{
		ProviderID: s.providerID, Command: firstEnvValue(env, "CODEX_PATH"),
		Dir: s.workingDir, Env: env, Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]ModelRow, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.Model)
		if id == "" {
			id = strings.TrimSpace(model.ID)
		}
		if id == "" {
			return nil, errors.New("model catalog: Codex model/list returned a model without an identifier")
		}
		efforts := make([]ReasoningEffort, 0, len(model.SupportedReasoningEfforts))
		for _, supported := range model.SupportedReasoningEfforts {
			effort, ok := normalizeReasoningEffort(supported.ReasoningEffort)
			if !ok {
				return nil, fmt.Errorf("model catalog: Codex model %q advertised a malformed effort identifier", id)
			}
			if !slices.Contains(efforts, effort) {
				efforts = append(efforts, effort)
			}
		}

		row := ModelRow{
			ProviderID: s.providerID, ModelID: id, DisplayName: model.DisplayName,
			SourceID: s.sourceID, SourceKind: SourceKindProviderLive, Priority: PriorityProviderLive,
			Available: new(true), Hidden: new(model.Hidden), RefreshedAt: now,
			ExplicitlyCurated: !model.Hidden, Featured: new(model.IsDefault),
			SupportsReasoning: new(len(model.SupportedReasoningEfforts) > 0), ReasoningEfforts: efforts,
		}
		if effort, ok := normalizeReasoningEffort(
			model.DefaultReasoningEffort,
		); ok &&
			slices.Contains(efforts, effort) {
			row.DefaultReasoningEffort = new(effort)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, errors.New("model catalog: Codex model/list returned no models")
	}
	sortModelRowsByID(rows)
	return rows, nil
}
