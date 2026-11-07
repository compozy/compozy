package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/projectfiles/changes"
	"github.com/spf13/cobra"
)

type sessionChangesOptions struct {
	Scope, Base string
	Paths       []string
	Reveal      bool
}
type sessionChangesRecord struct {
	contract.SessionChangesResponse
	Patches []contract.ChangePatchPayload `json:"patches,omitempty"`
}
type sessionChangesClient interface {
	SessionChanges(context.Context, string, sessionChangesOptions) (sessionChangesRecord, error)
}

func (c *daemonClient) SessionChanges(
	ctx context.Context,
	id string,
	options sessionChangesOptions,
) (sessionChangesRecord, error) {
	for range 2 {
		result, err := c.sessionChangesOnce(ctx, id, options)
		if errors.Is(err, changes.ErrStale) {
			continue
		}
		return result, err
	}
	return sessionChangesRecord{}, &daemonAPIError{
		statusCode: 409,
		payload: contract.ErrorPayload{
			Code:  "changes_stale",
			Error: "changes moved since revision; refetch the summary",
		},
	}
}

func (c *daemonClient) sessionChangesOnce(
	ctx context.Context,
	id string,
	options sessionChangesOptions,
) (sessionChangesRecord, error) {
	var result sessionChangesRecord
	path, err := c.sessionScopedPath(ctx, id, "/changes")
	if err != nil {
		return result, err
	}
	query := url.Values{"scope": {options.Scope}}
	if options.Base != "" {
		query.Set("base", options.Base)
	}
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &result.SessionChangesResponse); err != nil {
		return result, err
	}
	if len(options.Paths) > 0 {
		query.Set("revision", result.Revision)
		query.Set("reveal", fmt.Sprint(options.Reveal))
		for _, requested := range options.Paths {
			query.Add("path", requested)
			old := ""
			for _, file := range result.Files {
				if file.Path == requested {
					old = file.OldPath
					break
				}
			}
			query.Add("old_path", old)
		}
		var patches contract.SessionChangePatchesResponse
		if err := c.doJSON(ctx, http.MethodGet, path+"/patches", query, nil, &patches); err != nil {
			if api, ok := errors.AsType[*daemonAPIError](err); ok && api.payload.Code == "changes_stale" {
				return result, changes.ErrStale
			}
			return result, err
		}
		valid := patches.Revision == result.Revision && len(patches.Patches) == len(options.Paths)
		valid = valid && sessionChangePatchesMatch(result.SessionChangesResponse, patches.Patches)
		if !valid {
			return sessionChangesRecord{}, changes.ErrStale
		}
		result.Patches = patches.Patches
	}
	return result, nil
}

func sessionChangePatchesMatch(result contract.SessionChangesResponse, patches []contract.ChangePatchPayload) bool {
	valid := true
	for _, patch := range patches {
		// Unavailable items carry no content to fence. Keep them in the report
		// without rejecting patches for other files.
		if patch.Fingerprint == "" && patch.Patch == "" && !patch.TooLarge && !patch.Sensitive {
			continue
		}
		// Off-page patches retain the engine revision and before/after fingerprint checks.
		found := result.Truncated && patch.Fingerprint != ""
		for _, file := range result.Files {
			if file.Path == patch.Path {
				found = file.Fingerprint == patch.Fingerprint
				break
			}
		}
		valid = valid && found
	}
	return valid
}

func newSessionChangesCommand(deps commandDeps) *cobra.Command {
	var options sessionChangesOptions
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "changes <session-id>",
		Short: "Read session branch or uncommitted changes",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			changesClient, ok := client.(sessionChangesClient)
			if !ok {
				return errors.New("cli: session changes are unavailable")
			}
			paths := make([]string, 0, len(options.Paths))
			for _, path := range options.Paths {
				if path = strings.TrimSpace(path); path != "" {
					paths = append(paths, path)
				}
			}
			options.Paths = paths
			if len(paths) > 20 {
				return errors.New("request at most 20 paths (too_many_paths)")
			}
			record, err := changesClient.SessionChanges(cmd.Context(), strings.TrimSpace(args[0]), options)
			if err != nil {
				if api, ok := errors.AsType[*daemonAPIError](err); ok {
					return withCommandExitCode(1, fmt.Errorf("%s (%s)", api.Error(), api.payload.Code))
				}
				return withCommandExitCode(1, err)
			}
			return writeCommandOutput(
				cmd,
				outputBundle{
					jsonValue: record,
					jsonl:     func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, record) },
					human:     func() (string, error) { return sessionChangesTable(record), nil },
				},
			)
		},
	}
	cmd.Flags().StringVar(&options.Scope, "scope", worktreeBranchKey, "Changes scope: branch or uncommitted")
	cmd.Flags().StringVar(&options.Base, "base", "", "Comparison base ref")
	cmd.Flags().
		StringArrayVar(&options.Paths, "patch", nil, "Include a patch for a root-relative path (repeatable, at most 20)")
	cmd.Flags().BoolVar(&options.Reveal, "reveal", false, "Include sensitive patches")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print structured output")
	return cmd
}

func sessionChangesTable(record sessionChangesRecord) string {
	var out strings.Builder
	if record.Scope == worktreeBranchKey {
		fmt.Fprintf(
			&out,
			"Branch · %s → %s (merge base %.7s)",
			record.Head.Branch,
			record.Base.Ref,
			record.Base.MergeBaseSHA,
		)
	} else {
		fmt.Fprintf(&out, "Uncommitted · %s", record.Head.Branch)
	}
	if record.PullRequest != nil {
		fmt.Fprintf(&out, " · PR #%d", record.PullRequest.Number)
	}
	out.WriteByte('\n')
	table := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "STATUS\t+ADD\t−DEL\tPATH")
	for _, file := range record.Files {
		path := file.Path
		if file.OldPath != "" {
			path = file.OldPath + " → " + path
		}
		if file.Generated {
			path += " (generated)"
		}
		status := map[string]string{"added": "A", "untracked": "A", "modified": "M",
			"deleted": "D", "renamed": "R", "copied": "C", "conflicted": "U"}[file.Status]
		if status == "" {
			status = file.Status
		}
		fmt.Fprintf(table, "%s\t%d\t%d\t%s\n", status, file.Additions, file.Deletions, path)
	}
	table.Flush()
	if record.Truncated {
		noun := "files"
		if len(record.Files) == 1 {
			noun = "file"
		}
		fmt.Fprintf(&out, "Summary truncated at %d %s.\n", len(record.Files), noun)
	}
	if record.Totals.Partial {
		out.WriteString("Totals are a lower bound.\n")
	}
	fmt.Fprintf(
		&out,
		"%d files changed, +%d −%d\n",
		record.Totals.Files,
		record.Totals.Additions,
		record.Totals.Deletions,
	)
	for _, patch := range record.Patches {
		switch {
		case patch.Sensitive:
			fmt.Fprintf(&out, "%s (sensitive)\n", patch.Path)
		case patch.TooLarge:
			fmt.Fprintf(&out, "%s (patch too large to display)\n", patch.Path)
		case patch.Patch == "":
			fmt.Fprintf(&out, "%s (patch unavailable)\n", patch.Path)
		default:
			out.WriteString(patch.Patch)
		}
	}
	return out.String()
}
