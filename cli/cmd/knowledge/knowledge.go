package knowledge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	clihelpers "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Cmd creates the root knowledge command group.
func Cmd() *cobra.Command {
	root := &cobra.Command{Use: "knowledge", Short: "Manage knowledge bases"}
	root.AddCommand(
		newListCommand(),
		newGetCommand(),
		newApplyCommand(),
		newDeleteCommand(),
		newIngestCommand(),
		newQueryCommand(),
	)
	return root
}

func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(fmt.Sprintf("failed to mark flag %q as required: %v", name, err))
	}
}

func knowledgeRequest(
	ctx context.Context,
	client api.AuthClient,
	method,
	path string,
	params url.Values,
	body any,
	headers map[string]string,
) (*models.APIResponse, error) {
	if client == nil {
		return nil, errors.New("auth client not initialized")
	}
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}
	switch method {
	case http.MethodGet:
		return api.CallGETDecode(ctx, client, path)
	case http.MethodPost:
		return api.CallPOSTDecode(ctx, client, path, body)
	case http.MethodPut:
		return api.CallPUTDecode(ctx, client, path, body, headers)
	case http.MethodDelete:
		if err := api.CallDELETE(ctx, client, path, headers); err != nil {
			return nil, err
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported method %s", method)
	}
}

func printSuccess(cmdObj *cobra.Command, data any) error {
	formatter := clihelpers.NewJSONFormatter(true)
	output, err := formatter.FormatSuccess(data, nil)
	if err != nil {
		return err
	}
	fmt.Fprint(cmdObj.OutOrStdout(), output)
	return nil
}

func getProjectFlag(cmdObj *cobra.Command) (string, error) {
	project, err := cmdObj.Flags().GetString("project")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(project), nil
}

func formatStrongETag(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	withoutQuotes := strings.Trim(trimmed, "\"")
	if withoutQuotes == "" {
		return "", fmt.Errorf("etag must not be empty")
	}
	if strings.HasPrefix(withoutQuotes, "W/") {
		return "", fmt.Errorf("weak ETags are not supported")
	}
	return strconv.Quote(withoutQuotes), nil
}

func getKnowledgeQueryText(cmdObj *cobra.Command) (string, error) {
	queryText, err := cmdObj.Flags().GetString("query")
	if err != nil {
		return "", err
	}
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return "", fmt.Errorf("query text is required")
	}
	return queryText, nil
}

func getKnowledgeTopK(cmdObj *cobra.Command) (int, error) {
	topK, err := cmdObj.Flags().GetInt("top-k")
	if err != nil {
		return 0, err
	}
	if topK < 0 || topK > 50 {
		return 0, fmt.Errorf("top-k must be between 0 and 50")
	}
	return topK, nil
}

func getKnowledgeMinScore(cmdObj *cobra.Command) (float64, bool, error) {
	minScore, err := cmdObj.Flags().GetFloat64("min-score")
	if err != nil {
		return 0, false, err
	}
	switch {
	case minScore == -1:
		return 0, false, nil
	case minScore < 0 || minScore > 1:
		return 0, false, fmt.Errorf("min-score must be between 0 and 1")
	default:
		return minScore, true, nil
	}
}

func getKnowledgeFilters(cmdObj *cobra.Command) (map[string]string, error) {
	return cmdObj.Flags().GetStringToString("filter")
}

func newListCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List knowledge bases",
		RunE:  runKnowledgeList,
	}
	c.Flags().Int("limit", 0, "Maximum number of items to return (server default when omitted)")
	c.Flags().String("cursor", "", "Opaque pagination cursor returned by previous calls")
	c.Flags().String("project", "", "Project override")
	clihelpers.AddGlobalFlags(c)
	return c
}

func newGetCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "get <knowledge-id>",
		Short: "Get a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE:  runKnowledgeGet,
	}
	c.Flags().String("project", "", "Project override")
	clihelpers.AddGlobalFlags(c)
	return c
}

func newApplyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "apply",
		Short: "Create or update a knowledge base from file",
		RunE:  runKnowledgeApply,
	}
	c.Flags().String("file", "", "Path to JSON or YAML file describing the knowledge base")
	c.Flags().String("if-match", "", "Optional strong ETag to enforce optimistic concurrency")
	c.Flags().String("project", "", "Project override")
	mustMarkFlagRequired(c, "file")
	clihelpers.AddGlobalFlags(c)
	return c
}

func newDeleteCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "delete <knowledge-id>",
		Short: "Delete a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE:  runKnowledgeDelete,
	}
	c.Flags().String("project", "", "Project override")
	c.Flags().String("if-match", "", "Optional strong ETag to enforce optimistic concurrency")
	clihelpers.AddGlobalFlags(c)
	return c
}

func newIngestCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "ingest <knowledge-id>",
		Short: "Ingest sources for a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE:  runKnowledgeIngest,
	}
	c.Flags().String("project", "", "Project override")
	c.Flags().String("strategy", "upsert", "Ingestion strategy: upsert or replace")
	clihelpers.AddGlobalFlags(c)
	return c
}

func newQueryCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "query <knowledge-id>",
		Short: "Run an ad-hoc query against a knowledge base",
		Args:  cobra.ExactArgs(1),
		RunE:  runKnowledgeQuery,
	}
	c.Flags().String("project", "", "Project override")
	c.Flags().String("query", "", "Query text")
	c.Flags().Int("top-k", 0, "Override retrieval top_k (<=50)")
	c.Flags().Float64("min-score", -1, "Override retrieval minimum score between 0 and 1")
	c.Flags().StringToString("filter", nil, "Equality filters in key=value form")
	mustMarkFlagRequired(c, "query")
	clihelpers.AddGlobalFlags(c)
	return c
}

func runKnowledgeList(cmdObj *cobra.Command, _ []string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: knowledgeListHandler,
		TUI:  knowledgeListHandler,
	}, nil)
}

func runKnowledgeGet(cmdObj *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeGetHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
		TUI: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeGetHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
	}, nil)
}

func runKnowledgeApply(cmdObj *cobra.Command, _ []string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: knowledgeApplyHandler,
		TUI:  knowledgeApplyHandler,
	}, nil)
}

func runKnowledgeDelete(cmdObj *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeDeleteHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
		TUI: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeDeleteHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
	}, nil)
}

func runKnowledgeIngest(cmdObj *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeIngestHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
		TUI: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeIngestHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
	}, nil)
}

func runKnowledgeQuery(cmdObj *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeQueryHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
		TUI: func(ctx context.Context, c *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return knowledgeQueryHandler(ctx, c, executor.GetAuthClient(), args[0])
		},
	}, nil)
}

func knowledgeListHandler(ctx context.Context, cmdObj *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	limit, err := cmdObj.Flags().GetInt("limit")
	if err != nil {
		return err
	}
	cursor, err := cmdObj.Flags().GetString("cursor")
	if err != nil {
		return err
	}
	project, err := getProjectFlag(cmdObj)
	if err != nil {
		return err
	}
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if project != "" {
		params.Set("project", project)
	}
	resp, err := knowledgeRequest(ctx, executor.GetAuthClient(), http.MethodGet, "/knowledge-bases", params, nil, nil)
	if err != nil {
		return err
	}
	return printSuccess(cmdObj, resp.Data)
}

func knowledgeGetHandler(ctx context.Context, cmdObj *cobra.Command, client api.AuthClient, id string) error {
	project, err := getProjectFlag(cmdObj)
	if err != nil {
		return err
	}
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	path := fmt.Sprintf("/knowledge-bases/%s", url.PathEscape(id))
	resp, err := knowledgeRequest(ctx, client, http.MethodGet, path, params, nil, nil)
	if err != nil {
		return err
	}
	return printSuccess(cmdObj, resp.Data)
}

func knowledgeApplyHandler(
	ctx context.Context,
	cmdObj *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	client := executor.GetAuthClient()
	if client == nil {
		return errors.New("auth client not initialized")
	}
	payload, id, err := readKnowledgeApplyPayload(cmdObj)
	if err != nil {
		return err
	}
	params, headers, path, err := buildKnowledgeApplyRequest(cmdObj, id)
	if err != nil {
		return err
	}
	resp, err := knowledgeRequest(ctx, client, http.MethodPut, path, params, payload, headers)
	if err != nil {
		return err
	}
	return printSuccess(cmdObj, resp.Data)
}

func readKnowledgeApplyPayload(cmdObj *cobra.Command) (map[string]any, string, error) {
	filePath, err := cmdObj.Flags().GetString("file")
	if err != nil {
		return nil, "", err
	}
	bytes, err := clihelpers.ReadFile(filePath)
	if err != nil {
		return nil, "", err
	}
	var payload map[string]any
	if err := yaml.Unmarshal(bytes, &payload); err != nil {
		return nil, "", fmt.Errorf("invalid knowledge base file: %w", err)
	}
	rawID, ok := payload["id"].(string)
	id := strings.TrimSpace(rawID)
	if !ok || id == "" {
		return nil, "", fmt.Errorf("knowledge base definition must include string id field")
	}
	return payload, id, nil
}

func buildKnowledgeApplyRequest(cmdObj *cobra.Command, id string) (url.Values, map[string]string, string, error) {
	project, err := getProjectFlag(cmdObj)
	if err != nil {
		return nil, nil, "", err
	}
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	ifMatch, err := cmdObj.Flags().GetString("if-match")
	if err != nil {
		return nil, nil, "", err
	}
	etagHeader, err := formatStrongETag(ifMatch)
	if err != nil {
		return nil, nil, "", err
	}
	headers := map[string]string{}
	if etagHeader != "" {
		headers["If-Match"] = etagHeader
	}
	path := fmt.Sprintf("/knowledge-bases/%s", url.PathEscape(id))
	return params, headers, path, nil
}

func knowledgeDeleteHandler(ctx context.Context, cmdObj *cobra.Command, client api.AuthClient, id string) error {
	project, err := getProjectFlag(cmdObj)
	if err != nil {
		return err
	}
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	ifMatch, err := cmdObj.Flags().GetString("if-match")
	if err != nil {
		return err
	}
	etagHeader, err := formatStrongETag(ifMatch)
	if err != nil {
		return err
	}
	headers := map[string]string{}
	if etagHeader != "" {
		headers["If-Match"] = etagHeader
	}
	path := fmt.Sprintf("/knowledge-bases/%s", url.PathEscape(id))
	if _, err := knowledgeRequest(ctx, client, http.MethodDelete, path, params, nil, headers); err != nil {
		return err
	}
	fmt.Fprintln(cmdObj.OutOrStdout(), "knowledge base deleted")
	return nil
}

func knowledgeIngestHandler(ctx context.Context, cmdObj *cobra.Command, client api.AuthClient, id string) error {
	project, err := getProjectFlag(cmdObj)
	if err != nil {
		return err
	}
	strategy, err := cmdObj.Flags().GetString("strategy")
	if err != nil {
		return err
	}
	normalized := strings.ToLower(strings.TrimSpace(strategy))
	switch normalized {
	case "", "upsert":
		normalized = "upsert"
	case "replace":
	default:
		return fmt.Errorf("unsupported strategy %q", strategy)
	}
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	body := map[string]any{"strategy": normalized}
	path := fmt.Sprintf("/knowledge-bases/%s/ingest", url.PathEscape(id))
	resp, err := knowledgeRequest(ctx, client, http.MethodPost, path, params, body, nil)
	if err != nil {
		return err
	}
	return printSuccess(cmdObj, resp.Data)
}

func knowledgeQueryHandler(ctx context.Context, cmdObj *cobra.Command, client api.AuthClient, id string) error {
	project, err := getProjectFlag(cmdObj)
	if err != nil {
		return err
	}
	queryText, err := getKnowledgeQueryText(cmdObj)
	if err != nil {
		return err
	}
	topK, err := getKnowledgeTopK(cmdObj)
	if err != nil {
		return err
	}
	minScore, hasMinScore, err := getKnowledgeMinScore(cmdObj)
	if err != nil {
		return err
	}
	filters, err := getKnowledgeFilters(cmdObj)
	if err != nil {
		return err
	}
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	body := map[string]any{"query": queryText}
	if topK > 0 {
		body["top_k"] = topK
	}
	if hasMinScore {
		body["min_score"] = minScore
	}
	if len(filters) > 0 {
		body["filters"] = filters
	}
	path := fmt.Sprintf("/knowledge-bases/%s/query", url.PathEscape(id))
	resp, err := knowledgeRequest(ctx, client, http.MethodPost, path, params, body, nil)
	if err != nil {
		return err
	}
	return printSuccess(cmdObj, resp.Data)
}
