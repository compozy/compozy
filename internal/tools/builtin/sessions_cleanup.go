package builtin

import (
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionCleanupDescriptor() toolspkg.Descriptor {
	return nativeDescriptor(
		toolspkg.ToolIDSessionCleanup,
		"session_cleanup",
		"Session Cleanup",
		"Preview or delete the profile's archived session history in bulk. action=plan with dry_run "+
			"(the default) only previews: every count is a number of sessions, and kept_live counts child "+
			"sessions still in the session list or in Later, which always stay with everything under them. "+
			"dry_run=false starts the delete in the daemon's background and needs approval; follow it with "+
			"action=status and end it early with action=stop (what it deleted stays deleted). Only archived "+
			"sessions are deleted; worktrees stay, as with a single delete. The range defaults to threads "+
			"archived more than 7 days ago; delete defaults to everything (whole threads). To delete no more "+
			"than a preview counted, send its plan.counted_at as counted_at. Retry failed threads with "+
			"session_ids set to their session_id.",
		sessionCleanupInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, descriptorKeywordArchive, descriptorKeywordDestructive},
		[]string{"clean up archived sessions", "delete archived sessions", "clear session history"},
	)
}

const sessionCleanupInputSchema = `{
	"type":"object",
	"properties":{
		"action":{
			"type":"string",
			"enum":["plan","status","stop"],
			"default":"plan",
			"description":"plan previews or starts a cleanup; status reads the running or last one; stop ends it."
		},
		"workspace":{
			"type":"string",
			"description":"Workspace id, name, path, or @user; defaults to the caller's workspace."
		},
		"all_workspaces":{
			"type":"boolean",
			"description":"Clean up every project and User scope; needs unrestricted workspace access."
		},
		"delete":{
			"type":"object",
			"properties":{
				"everything":{"type":"boolean","description":"Delete picked threads whole; includes the other two."},
				"children":{"type":"boolean","description":"Delete archived child sessions, keeping main sessions."},
				"subagents":{"type":"boolean","description":"Delete archived subagents, keeping main sessions."}
			},
			"additionalProperties":false
		},
		"older_than_days":{
			"type":"integer",
			"minimum":1,
			"maximum":36500,
			"description":"Pick threads archived more than this many days ago (default 7)."
		},
		"archived_before":{
			"type":"string",
			"format":"date-time",
			"description":"Pick threads archived before this RFC 3339 instant."
		},
		"any_time":{"type":"boolean","description":"Pick every archived thread, whenever it was archived."},
		"session_ids":{
			"type":"array",
			"maxItems":1000,
			"items":{"type":"string"},
			"description":"Limit the cleanup to these archived main sessions, as a retry of failed threads does."
		},
		"counted_at":{
			"type":"string",
			"format":"date-time",
			"description":"The plan.counted_at of the preview being confirmed; sessions archived after it stay."
		},
		"dry_run":{
			"type":"boolean",
			"default":true,
			"description":"Preview only (the default); set false to delete, which needs approval."
		}
	},
	"additionalProperties":false
}`
