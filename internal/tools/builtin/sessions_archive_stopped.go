package builtin

import (
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionArchiveStoppedDescriptor() toolspkg.Descriptor {
	return nativeDescriptor(
		toolspkg.ToolIDSessionArchiveStopped,
		"session_archive_stopped",
		"Session Archive Stopped",
		"Archive every stopped thread of a project in one call, reversibly. A thread archives whole, root "+
			"and every child; threads that are running, waiting on you, have queued input, are pinned, in "+
			"Later, an always-on instance's main session, or contain the calling session are skipped and "+
			"counted by reason. dry_run=true previews without archiving. The result lists every archived "+
			"thread with its session_ids, root first; to undo, call compozy__session_unarchive on each of "+
			"its restore_ids in order (subagents restore with their parent).",
		sessionArchiveStoppedInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, descriptorKeywordArchive},
		[]string{"archive stopped sessions", "tidy session list", "archive finished threads"},
	)
}

const sessionArchiveStoppedInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{
			"type":"string",
			"description":"Workspace id, name, or path; defaults to the caller's workspace."
		},
		"all_workspaces":{
			"type":"boolean",
			"description":"Archive stopped threads in every project; needs unrestricted workspace access."
		},
		"dry_run":{
			"type":"boolean",
			"default":false,
			"description":"Preview what would be archived without archiving anything."
		}
	},
	"additionalProperties":false
}`
