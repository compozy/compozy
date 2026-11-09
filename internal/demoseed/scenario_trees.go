package demoseed

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// A coordinator session spawns workers and delegates to subagents; workers delegate in turn,
// so the list shows the same two-level trees a team running parallel agents ends up with.
const (
	treeLedger     = "ledger_v2"
	treeReceipts   = "receipt_tokens"
	treeThresholds = "mx_thresholds"
	treeCutover    = "queue_cutover"
	treeGoNoGo     = "mx_go_no_go"
)

type treeWorker struct {
	title, agent, failure string
	subagents             []string
}

type treeSpec struct {
	id, workspace, agent, name, prompt, summary string
	endedAgo                                    int
	workers                                     []treeWorker
	subagents                                   []string
	subagentAgent                               string
}

func ledgerWorkers() []treeWorker {
	return []treeWorker{
		{
			"Slice 01: ledger entry schema",
			agentCodeImplementer,
			"",
			[]string{"Map current entry columns", "Draft the migration"},
		},
		{
			"Slice 02: double-entry writer",
			agentCodeImplementer,
			"",
			[]string{"Trace current writers", "Write balance invariants", "Review posting order"},
		},
		{
			"Slice 03: partner balance view",
			agentCodeImplementer,
			"",
			[]string{"Profile the balance query", "Check index coverage"},
		},
		{
			"Slice 04: refund postings",
			agentCodeImplementer,
			"",
			[]string{"List refund paths", "Cover partial refunds", "Check reversal order"},
		},
		{
			"Slice 05: chargeback postings",
			agentCodeImplementer,
			"",
			[]string{"Map dispute states", "Write chargeback fixtures"},
		},
		{
			"Slice 06: settlement export",
			agentCodeImplementer,
			"",
			[]string{"Compare export totals", "Update golden files"},
		},
		{
			"Slice 07: backfill job",
			agentCodeImplementer,
			"Backfill aborted: duplicate entry ids in the October partition.",
			[]string{"Size the backfill", "Find duplicate entries", "Draft the dedupe plan"},
		},
		{
			"Slice 08: read cutover flag",
			agentCodeImplementer,
			"",
			[]string{"Trace ledger readers", "Add the shadow read"},
		},
		{
			"Slice 09: reconciliation report",
			agentCodeImplementer,
			"",
			[]string{"Define report columns", "Check partner statements"},
		},
		{"Integrate slices and verify", agentReviewer, "", []string{"Review migration safety", "Review rollback path"}},
	}
}

func treeSpecs() []treeSpec {
	return []treeSpec{
		{
			id:            treeLedger,
			workspace:     workspaceKeyPlatform,
			agent:         agentSpecConductor,
			name:          "Deliver the settlement ledger v2 spec",
			prompt:        "Deliver docs/specs/ledger-v2. Ten slices, each in its own worktree, one integration PR.",
			summary:       "Nine slices are integrated. Slice 07 failed on duplicate entries in October and needs a decision.",
			endedAgo:      36,
			workers:       ledgerWorkers(),
			subagentAgent: agentReviewer,
			subagents: []string{
				"Review the slice plan", "Check migration ordering", "Audit partner balance math",
				"Review the rollback path", "Compare against ledger v1", "Check reconciliation coverage",
				"Review the backfill plan", "Summarize open risks",
			},
		},
		{
			id: treeReceipts, workspace: workspaceKeyCheckout, agent: agentCheckoutEngineer,
			name:     "Migrate receipt pages to the new tokens",
			prompt:   "Move the receipt pages to the new design tokens, one worker per page.",
			summary:  "All three receipt pages use the new tokens and the visual diffs are clean.",
			endedAgo: 64,
			workers: []treeWorker{
				{"Migrate the card receipt", agentCheckoutEngineer, "", []string{"List hard-coded colors"}},
				{"Migrate the boleto receipt", agentCheckoutEngineer, "", []string{"Check print styles"}},
				{"Migrate the Pix receipt", agentCheckoutEngineer, "", nil},
			},
			subagents: []string{"Compare visual diffs", "Check contrast"}, subagentAgent: agentDocsSteward,
		},
		{
			id:        treeThresholds,
			workspace: workspaceKeyRisk,
			agent:     agentFraudAnalyst,
			name:      "Recalibrate dispute thresholds for Mexico",
			prompt:    "Recalibrate the Mexico dispute thresholds using the last 60 days.",
			summary:   "The new threshold is 0.38%. It would have caught both real spikes with no false holds.",
			endedAgo:  128,
			subagents: []string{
				"Pull 60 days of disputes",
				"Separate issuer noise",
				"Replay candidate thresholds",
				"Write the recommendation",
			},
			subagentAgent: agentPlatformEngineer,
		},
		{
			id: treeCutover, workspace: workspaceKeyInfra, agent: agentPlatformEngineer,
			name:     "Move settlement workers to the new queue",
			prompt:   "Plan and rehearse the settlement queue cutover in staging.",
			summary:  "Staging cut over cleanly. Production waits for Wednesday's window.",
			endedAgo: 240,
			workers: []treeWorker{
				{
					"Rehearse the cutover in staging",
					agentPlatformEngineer,
					"",
					[]string{"Drain the old queue", "Verify dead letters"},
				},
				{"Write the rollback steps", agentReleaseManager, "", nil},
			},
		},
		{
			id: treeGoNoGo, workspace: workspaceKeyLaunch, agent: agentProductLead,
			name:     "Prepare the Mexico go or no-go packet",
			prompt:   "Prepare the go or no-go packet for Mexico. One owner per evidence stream.",
			summary:  "Compliance and support are ready. Risk holds until the replay gap closes.",
			endedAgo: 300,
			workers: []treeWorker{
				{"Compliance evidence for Mexico", agentComplianceReview, "", nil},
				{"Support readiness for Mexico", agentSupportLead, "", nil},
				{"Dispute evidence for Mexico", agentFraudAnalyst, "", []string{"Summarize dispute windows"}},
			},
		},
	}
}

// treeTaskChars is the recorded length of a seeded delegation brief.
const treeTaskChars = 96

func treeRootID(id string) string { return "sess_ns_tree_" + id }

func treeWorkerID(id string, index int) string {
	return fmt.Sprintf("%s_w%02d", treeRootID(id), index+1)
}

func treeSubSessionID(owner string, index int) string { return fmt.Sprintf("%s_a%02d", owner, index+1) }

func treeSubagentID(childSession string) string { return "sub_" + childSession }

func treeDelegateStep(childSession, title, agent string) transcriptStep {
	return pixDelegateStep(treeSubagentID(childSession), title, agent, title+".")
}

func treeSpawnStep(childID, title, agent string) transcriptStep {
	return nativeStep("compozy__session_create",
		jsonObject(map[string]any{"agent": agent, "name": title}), jsonObject(map[string]any{"session_id": childID}))
}

func treeRootSteps(spec treeSpec) []transcriptStep {
	steps := []transcriptStep{operatorSays(spec.prompt)}
	for index, worker := range spec.workers {
		steps = append(steps, treeSpawnStep(treeWorkerID(spec.id, index), worker.title, worker.agent))
	}
	for index, title := range spec.subagents {
		steps = append(steps, treeDelegateStep(treeSubSessionID(treeRootID(spec.id), index), title, spec.subagentAgent))
	}
	return append(steps, agentSays(spec.summary))
}

func treeWorkerSteps(owner string, worker treeWorker) []transcriptStep {
	steps := []transcriptStep{operatorSays(worker.title + ". Work in your own worktree and report what you verified.")}
	for index, title := range worker.subagents {
		steps = append(steps, treeDelegateStep(treeSubSessionID(owner, index), title, agentCodeImplementer))
	}
	steps = append(steps, shellStep("go test ./services/...", "ok  \tnorthstar/services\t2.914s"))
	if worker.failure != "" {
		return append(steps, agentSays(worker.failure))
	}
	return append(steps, agentSays(worker.title+" is done and verified."))
}

func treeStory(spec treeSpec, id, name, agent string, ended time.Time, runtime int) boardSessionStory {
	provider, model := agentRuntime(spec.workspace, agent)
	input := int64(runtime) * 2100
	output := int64(runtime) * 360
	return boardSessionStory{
		ID: id, Name: name, WorkspaceKey: spec.workspace, AgentName: agent, Provider: provider, Model: model,
		SessionType: string(session.SessionTypeSpawned), ParentID: treeRootID(spec.id), SpawnRole: "worker",
		StartedAt: ended.Add(-time.Duration(runtime) * time.Minute), EndedAt: ended,
		Input: input, Output: output, CostUSD: float64(input+output) / 50000}
}

func treeOneSessions(clock timeline, spec treeSpec) []boardSessionStory {
	rootEnded := clock.minutesAgo(spec.endedAgo)
	runtime := 22 + 9*len(spec.workers) + 4*len(spec.subagents)
	root := treeStory(spec, treeRootID(spec.id), spec.name, spec.agent, rootEnded, runtime)
	root.SessionType, root.ParentID, root.SpawnRole = string(session.SessionTypeUser), "", ""
	root.Steps = treeRootSteps(spec)
	stories := []boardSessionStory{root}
	for index, worker := range spec.workers {
		id := treeWorkerID(spec.id, index)
		ended := rootEnded.Add(-time.Duration(4+index*3) * time.Minute)
		child := treeStory(spec, id, worker.title, worker.agent, ended, 11+index%5*3)
		child.Steps = treeWorkerSteps(id, worker)
		if worker.failure != "" {
			child.StopReason = string(store.StopError)
			child.StopDetail = worker.failure
			child.Failure = &sessionFailureStory{Kind: string(store.FailurePrompt), Summary: worker.failure}
		}
		stories = append(stories, child)
		for subIndex, title := range worker.subagents {
			sub := treeStory(spec, treeSubSessionID(id, subIndex), title, agentCodeImplementer,
				ended.Add(-time.Duration(2+subIndex)*time.Minute), 4+subIndex)
			sub.ParentID, sub.RootID, sub.SpawnDepth, sub.SpawnRole = id, treeRootID(
				spec.id,
			), 2, store.SubagentSpawnRole
			sub.Steps = treeSubagentSteps(title)
			stories = append(stories, sub)
		}
	}
	for index, title := range spec.subagents {
		sub := treeStory(spec, treeSubSessionID(treeRootID(spec.id), index), title, spec.subagentAgent,
			rootEnded.Add(-time.Duration(3+index)*time.Minute), 5+index)
		sub.SpawnRole = store.SubagentSpawnRole
		sub.Steps = treeSubagentSteps(title)
		stories = append(stories, sub)
	}
	return stories
}

func treeSubagentSteps(title string) []transcriptStep {
	return []transcriptStep{
		operatorSays(title + "."),
		agentSays(title + ": done, with the evidence in the notes above."),
	}
}

func treeSessions(clock timeline) []boardSessionStory {
	stories := []boardSessionStory{}
	for _, spec := range treeSpecs() {
		stories = append(stories, treeOneSessions(clock, spec)...)
	}
	return stories
}

func treeSubagentStory(
	spec treeSpec,
	stories map[string]boardSessionStory,
	parent, child, title string,
	step, depth int,
) subagentStory {
	session := stories[child]
	return subagentStory{
		ID: treeSubagentID(child), ChildID: child, Title: title, Role: subagentRoleGeneral, Agent: session.AgentName,
		Provider: session.Provider, Model: session.Model, StepIndex: step, Status: store.SubagentStatusCompleted,
		Result: title + ": done.", StartedAt: session.StartedAt, SettledAt: session.EndedAt,
		TaskChars: treeTaskChars, InputUsed: session.Input, OutputUsed: session.Output,
		ParentID: parent, Depth: depth, WorkspaceKey: spec.workspace,
	}
}

// treeSubagents lists the roster rows; StepIndex is the delegate call's place in the owner's transcript.
func treeSubagents(clock timeline) []subagentStory {
	subs := []subagentStory{}
	for _, spec := range treeSpecs() {
		stories := map[string]boardSessionStory{}
		for _, story := range treeOneSessions(clock, spec) {
			stories[story.ID] = story
		}
		root := treeRootID(spec.id)
		for index, title := range spec.subagents {
			subs = append(subs, treeSubagentStory(spec, stories, root, treeSubSessionID(root, index), title,
				1+len(spec.workers)+index, 1))
		}
		for workerIndex, worker := range spec.workers {
			owner := treeWorkerID(spec.id, workerIndex)
			for index, title := range worker.subagents {
				subs = append(
					subs,
					treeSubagentStory(spec, stories, owner, treeSubSessionID(owner, index), title, 1+index, 2),
				)
			}
		}
	}
	return subs
}
