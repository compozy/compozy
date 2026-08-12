package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func worktreeDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeDescriptor(
			toolspkg.ToolIDWorktreeList, "worktree_list", "Worktree List",
			"List registered and discovered worktrees for one workspace.",
			worktreeListInputSchema, toolspkg.RiskRead, true, false, false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorktrees},
			[]string{"worktree", "list"}, []string{"worktree list", "git worktrees"},
		),
		nativeDescriptor(
			toolspkg.ToolIDWorktreeInspect, "worktree_inspect", "Worktree Inspect",
			"Inspect one worktree with cached status, forge state, and agent activity.",
			worktreeRefInputSchema, toolspkg.RiskRead, true, false, false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorktrees},
			[]string{"worktree", "inspect"}, []string{"worktree inspect", "worktree status"},
		),
		nativeDescriptor(
			toolspkg.ToolIDWorktreeCreate, "worktree_create", "Worktree Create",
			"Accept one durable phased worktree creation for a workspace.",
			worktreeCreateInputSchema, toolspkg.RiskMutating, false, false, false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorktrees},
			[]string{"worktree", descriptorKeywordCreate}, []string{"create worktree", "git worktree add"},
		),
		nativeDescriptor(
			toolspkg.ToolIDWorktreeRemove, "worktree_remove", "Worktree Remove",
			"Remove one worktree after authoritative dirty, commit, identity, and session checks.",
			worktreeRemoveInputSchema, toolspkg.RiskDestructive, false, true, false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDWorktrees},
			[]string{"worktree", descriptorKeywordDestructive}, []string{"remove worktree", "delete checkout"},
		),
	}
}

const worktreeListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"refresh":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const worktreeRefInputSchema = `{
	"type":"object",
	"required":["ref"],
	"properties":{
		"workspace":{"type":"string"},
		"ref":{"type":"string"}
	},
	"additionalProperties":false
}`

const worktreeCreateInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"name":{"type":"string"},
		"branch":{"type":"string"},
		"base_ref":{"type":"string"},
		"existing_branch":{"type":"string"},
		"path":{"type":"string"}
	},
	"additionalProperties":false
}`

const worktreeRemoveInputSchema = `{
	"type":"object",
	"required":["ref"],
	"properties":{
		"workspace":{"type":"string"},
		"ref":{"type":"string"},
		"force":{"type":"boolean"}
	},
	"additionalProperties":false
}`
