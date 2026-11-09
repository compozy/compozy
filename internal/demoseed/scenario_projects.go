package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// Northstar splits its work across several repositories, each registered as its own project.
const (
	workspaceKeyCheckout = "checkout-web"
	workspaceKeyRisk     = "risk-engine"
	workspaceKeyInfra    = "platform-infra"
	workspaceKeyDocs     = "docs-site"
)

func extraWorkspaces(clock timeline) []workspaceStory {
	specs := []struct {
		key, agent string
		age        int
	}{
		{workspaceKeyCheckout, agentCheckoutEngineer, 28},
		{workspaceKeyRisk, agentFraudAnalyst, 26},
		{workspaceKeyInfra, agentPlatformEngineer, 22},
		{workspaceKeyDocs, agentDocsSteward, 18},
	}
	stories := make([]workspaceStory, 0, len(specs))
	for index, spec := range specs {
		stories = append(stories, workspaceStory{
			Key: spec.key, Name: spec.key, Relative: "workspaces/northstar-pay/" + spec.key,
			DefaultAgent: spec.agent,
			CreatedAt:    clock.daysAgo(spec.age), UpdatedAt: clock.minutesAgo(9 + index*13),
		})
	}
	return stories
}

// projectAgentCopies gives each new project the on-demand definitions its sessions run.
func projectAgentCopies(base []agentStory) []agentStory {
	assignments := map[string][]string{
		workspaceKeyCheckout: {agentCheckoutEngineer, agentReleaseManager, agentDocsSteward},
		workspaceKeyRisk:     {agentFraudAnalyst, agentPlatformEngineer},
		workspaceKeyInfra:    {agentPlatformEngineer, agentReleaseManager},
		workspaceKeyDocs:     {agentDocsSteward},
	}
	copies := []agentStory{}
	for _, key := range []string{workspaceKeyCheckout, workspaceKeyRisk, workspaceKeyInfra, workspaceKeyDocs} {
		for _, name := range assignments[key] {
			for _, agent := range base {
				if agent.Name == name {
					agent.WorkspaceKey = key
					copies = append(copies, agent)
					break
				}
			}
		}
	}
	return copies
}

// quickSpec is one finished session described compactly; ages are minutes before now.
type quickSpec struct {
	id, workspace, agent, name, prompt, command, output, summary string
	endedAgo, runtime                                            int
	failure, laterReason                                         string
	laterInHours                                                 int
	forkOf                                                       string
}

var quickSpecs = []quickSpec{
	{
		id: "checkout_apple_pay", workspace: workspaceKeyCheckout, agent: agentCheckoutEngineer,
		name:     "Fix Apple Pay sheet closing on 3DS",
		prompt:   "The Apple Pay sheet closes when the 3DS challenge opens. Find out why and fix it.",
		command:  "bun run test src/payment/apple-pay.test.ts",
		output:   "12 passed (12)",
		summary:  "The sheet closed because the challenge iframe took focus. It now waits for the sheet to settle.",
		endedAgo: 26, runtime: 19,
	},
	{
		id: "checkout_boleto_tz", workspace: workspaceKeyCheckout, agent: agentCheckoutEngineer,
		name:     "Show boleto expiry in the buyer's zone",
		prompt:   "Issue #438: the boleto receipt shows expiry in UTC. Use the buyer's time zone.",
		command:  "bun run test src/receipt/boleto-receipt.test.tsx",
		output:   "8 passed (8)",
		summary:  "The receipt now formats expiry in the buyer's zone, with a test for a late-night boleto.",
		endedAgo: 140, runtime: 11,
	},
	{
		id: "checkout_retry_banner", workspace: workspaceKeyCheckout, agent: agentCheckoutEngineer,
		name:     "Clear the retry banner after 3DS succeeds",
		prompt:   "Issue #447: the card retry banner stays after a successful challenge.",
		command:  "bun run test src/payment/retry-banner.test.tsx",
		output:   "FAIL renders nothing once the challenge succeeded",
		summary:  "The banner reads a stale attempt id. The fix needs the attempt store change first.",
		endedAgo: 52, runtime: 8, failure: "Tests still fail on the stale attempt id; stopped before opening a PR.",
	},
	{
		id: "checkout_a11y_sweep", workspace: workspaceKeyCheckout, agent: agentDocsSteward,
		name:     "Audit checkout focus order",
		prompt:   "Walk the checkout form by keyboard and list every focus-order problem.",
		command:  "bun run axe src/routes/checkout",
		output:   "2 violations: focus-order-semantics, label",
		summary:  "Two problems: the installment select comes after submit, and the CVV field has no label.",
		endedAgo: 310, runtime: 14,
	},
	{
		id: "checkout_bundle_budget", workspace: workspaceKeyCheckout, agent: agentReleaseManager,
		name:     "Trim the checkout bundle under budget",
		prompt:   "The checkout bundle is over the 180 kB budget. Find what grew.",
		command:  "bun run size-limit",
		output:   "checkout: 193.4 kB (limit 180 kB)",
		summary:  "The date library came back through the receipt page. Parked until the receipts folder lands.",
		endedAgo: 1500, runtime: 16,
		laterReason: "Waiting for the receipt migration to land before cutting the date library.", laterInHours: 30,
	},
	{
		id: "checkout_apple_pay_fork", workspace: workspaceKeyCheckout, agent: agentCheckoutEngineer,
		name:     "Fix Apple Pay sheet closing on 3DS (try focus trap)",
		prompt:   "Try the focus-trap approach instead and compare it with the settle delay.",
		command:  "bun run test src/payment/apple-pay.test.ts",
		output:   "12 passed (12)",
		summary:  "The focus trap also works but needs a Safari-only branch. The settle delay stays the simpler fix.",
		endedAgo: 21, runtime: 5, forkOf: "checkout_apple_pay",
	},
	{
		id: "risk_velocity_rule", workspace: workspaceKeyRisk, agent: agentFraudAnalyst,
		name:     "Tune the card velocity rule for Pix",
		prompt:   "The velocity rule flags Pix refunds as card retries. Separate them.",
		command:  "go test ./rules/velocity/...",
		output:   "ok  \tnorthstar/risk/rules/velocity\t0.214s",
		summary:  "Pix refunds now carry their own rail in the velocity key. False flags drop to zero in the replay.",
		endedAgo: 74, runtime: 23,
	},
	{
		id: "risk_merchant_cluster", workspace: workspaceKeyRisk, agent: agentFraudAnalyst,
		name:     "Cluster the retry-loop merchants",
		prompt:   "Group merchants whose chargebacks come from a retry loop rather than fraud.",
		command:  "python scripts/cluster_merchants.py --since 14d",
		output:   "3 clusters, 41 merchants",
		summary:  "Three clusters. All 41 merchants share the same integration version with the retry bug.",
		endedAgo: 410, runtime: 37,
	},
	{
		id: "risk_model_backtest", workspace: workspaceKeyRisk, agent: agentPlatformEngineer,
		name:     "Backtest the dispute model on October",
		prompt:   "Backtest the new dispute model against October and compare with production.",
		command:  "make backtest MONTH=2026-10",
		output:   "exit status 137: killed",
		summary:  "The backtest ran out of memory on the Mexico partition.",
		endedAgo: 18, runtime: 41, failure: "Backtest killed on the Mexico partition: out of memory.",
	},
	{
		id: "infra_queue_alerts", workspace: workspaceKeyInfra, agent: agentPlatformEngineer,
		name:     "Add queue depth alerts for the cutover",
		prompt:   "Add alerts on settlement queue depth before the cutover starts.",
		command:  "terraform plan -target=module.settlement_alerts",
		output:   "Plan: 3 to add, 0 to change, 0 to destroy.",
		summary:  "Three alerts planned: depth, oldest message age, and dead letters.",
		endedAgo: 96, runtime: 13,
	},
	{
		id: "infra_rds_upgrade", workspace: workspaceKeyInfra, agent: agentPlatformEngineer,
		name:     "Plan the ledger database minor upgrade",
		prompt:   "Plan the ledger database minor upgrade and list what needs a restart.",
		command:  "terraform plan -target=aws_rds_cluster.ledger",
		output:   "Plan: 0 to add, 1 to change, 0 to destroy.",
		summary:  "One in-place change with a reader failover. Parked until the payment freeze ends.",
		endedAgo: 2200, runtime: 21,
		laterReason: "Payment freeze until Monday. Apply during the Tuesday maintenance window.", laterInHours: 44,
	},
	{
		id: "infra_ci_cache", workspace: workspaceKeyInfra, agent: agentReleaseManager,
		name:     "Cut CI time on the integration lane",
		prompt:   "The integration lane takes 31 minutes. Find the slowest steps.",
		command:  "gh run view --json jobs -q '.jobs[].name'",
		output:   "integration-1 integration-2 integration-3",
		summary:  "The module cache missed on every run because the key included the commit. Fixed the key.",
		endedAgo: 620, runtime: 28,
	},
	{
		id: "docs_rollback_refresh", workspace: workspaceKeyDocs, agent: agentDocsSteward,
		name:     "Refresh the settlement rollback runbook",
		prompt:   "The rollback runbook still names the old queue. Bring it up to date.",
		command:  "grep -rn settlement-v1 runbooks/",
		output:   "runbooks/settlement-rollback.md:14: settlement-v1",
		summary:  "Replaced the old queue name and added the dead-letter check to step 4.",
		endedAgo: 170, runtime: 9,
	},
	{
		id: "docs_api_changelog", workspace: workspaceKeyDocs, agent: agentDocsSteward,
		name:     "Write the November API changelog",
		prompt:   "Write the November API changelog from the merged pull requests.",
		command:  "gh pr list --state merged --search 'merged:>2026-11-01' --json title",
		output:   "14 pull requests",
		summary:  "Drafted the changelog. Two entries need a product name from the launch team.",
		endedAgo: 880, runtime: 17,
	},
	{
		id: "docs_link_check", workspace: workspaceKeyDocs, agent: agentDocsSteward,
		name:     "Fix broken links in the partner guide",
		prompt:   "Run the link checker on the partner guide and fix what is broken.",
		command:  "bun run links partner-guide",
		output:   "4 broken links",
		summary:  "Fixed four links. One partner page was removed, so I linked its replacement.",
		endedAgo: 2900, runtime: 7,
	},
	{
		id: "platform_retry_fork", workspace: workspaceKeyPlatform, agent: agentCheckoutEngineer,
		name:     "Cap settlement retries per partner window (sliding)",
		prompt:   "Try a sliding window instead of fixed minutes and compare the behavior.",
		command:  "go test ./services/settlement/retry/...",
		output:   "ok  \tnorthstar/services/settlement/retry\t0.388s",
		summary:  "The sliding window smooths bursts but costs a lock per partner. The fixed window stays in #482.",
		endedAgo: 47, runtime: 12,
	},
}

func quickSessionID(id string) string { return "sess_ns_" + id }

func agentRuntime(workspace, name string) (string, string) {
	for _, agent := range scenarioAgents() {
		if agent.Name == name && agent.WorkspaceKey == workspace {
			return agent.Provider, agent.Model
		}
	}
	return providerClaude, modelClaudeOpus
}

func quickSteps(spec quickSpec) []transcriptStep {
	return []transcriptStep{
		operatorSays(spec.prompt),
		shellStep(spec.command, spec.output),
		agentSays(spec.summary),
	}
}

func quickSession(clock timeline, spec quickSpec) boardSessionStory {
	provider, model := agentRuntime(spec.workspace, spec.agent)
	ended := clock.minutesAgo(spec.endedAgo)
	input := int64(spec.runtime) * 1900
	output := int64(spec.runtime) * 310
	story := boardSessionStory{
		ID: quickSessionID(spec.id), Name: spec.name, WorkspaceKey: spec.workspace, AgentName: spec.agent,
		Provider: provider, Model: model, SessionType: string(session.SessionTypeUser),
		StartedAt: ended.Add(-time.Duration(spec.runtime) * time.Minute), EndedAt: ended,
		Input: input, Output: output, CostUSD: float64(input+output) / 50000,
		Steps: quickSteps(spec)}
	if spec.failure != "" {
		story.StopReason = string(store.StopError)
		story.StopDetail = spec.failure
		story.Failure = &sessionFailureStory{Kind: string(store.FailurePrompt), Summary: spec.failure}
	}
	if spec.laterReason != "" {
		story.Later = &boardLater{
			Reason:   spec.laterReason,
			ParkedAt: ended.Add(10 * time.Minute),
			ReturnAt: clock.Now().Add(time.Duration(spec.laterInHours) * time.Hour),
		}
	}
	if spec.forkOf != "" {
		story.ParentID = quickForkSource(spec.forkOf)
		story.LineageKind = store.LineageKindFork
		story.OriginAgent = spec.agent
	}
	return story
}

// quickForkSource resolves a fork's source, which may be another quick session or a named one.
func quickForkSource(id string) string { return quickSessionID(id) }

func projectSessions(clock timeline) []boardSessionStory {
	stories := make([]boardSessionStory, 0, len(quickSpecs))
	for _, spec := range quickSpecs {
		story := quickSession(clock, spec)
		if spec.id == "platform_retry_fork" {
			story.ParentID = sessionRetryBudgetID
			story.LineageKind = store.LineageKindFork
			story.OriginAgent = spec.agent
		}
		stories = append(stories, story)
	}
	return stories
}

func projectFolderStories() []sessionFolderStory {
	return []sessionFolderStory{
		{ID: "sfd_ns_receipts", WorkspaceKey: workspaceKeyCheckout, Name: "Receipts", SessionIDs: []string{
			quickSessionID("checkout_boleto_tz"), treeRootID(treeReceipts),
		}},
		{ID: "sfd_ns_payment_form", WorkspaceKey: workspaceKeyCheckout, Name: "Payment form", SessionIDs: []string{
			quickSessionID("checkout_apple_pay"), quickSessionID("checkout_retry_banner"),
		}},
		{ID: "sfd_ns_thresholds", WorkspaceKey: workspaceKeyRisk, Name: "Thresholds", SessionIDs: []string{
			quickSessionID("risk_velocity_rule"), treeRootID(treeThresholds),
		}},
		{ID: "sfd_ns_cutover", WorkspaceKey: workspaceKeyInfra, Name: "Queue cutover", SessionIDs: []string{
			quickSessionID("infra_queue_alerts"), treeRootID(treeCutover),
		}},
		{ID: "sfd_ns_runbooks", WorkspaceKey: workspaceKeyDocs, Name: "Runbooks", SessionIDs: []string{
			quickSessionID("docs_rollback_refresh"),
		}},
	}
}
