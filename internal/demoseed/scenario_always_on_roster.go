package demoseed

import (
	"fmt"
	"strings"
	"time"
)

// The always-on team beyond the four core agents: one definition per project it works in,
// so each project's sidebar row carries its own faces and each seat has its own main session.
const (
	agentPRShepherd      = "pr-shepherd"
	agentCIDoctor        = "ci-doctor"
	agentDependencyWatch = "dependency-watch"
	agentSecurityScan    = "security-scan"
	agentUXRegression    = "ux-regression"
	agentInfraDrift      = "infra-drift"
	agentRunbookKeeper   = "runbook-keeper"
)

type rosterAgent struct {
	name, display, description, color, provider, watch string
}

var rosterAgentDefs = []rosterAgent{
	{agentPRShepherd, "PR Shepherd", "Keeps open pull requests moving and flags the ones stuck on a person.",
		"#a78bfa", providerClaude, "open pull requests that have waited more than a day on a review or a red check"},
	{agentCIDoctor, "CI Doctor", "Spots flaky and newly broken checks and names the commit that started them.",
		"#34d399", providerCodex, "checks that turned red or started flaking on the main branch"},
	{agentDependencyWatch, "Dependency Watch", "Reads new dependency advisories and proposes the upgrades that matter.",
		"#60a5fa", providerClaude, "new advisories for the dependencies this project ships"},
	{agentSecurityScan, "Security Scan", "Reviews new secrets, permissions and auth changes for risky patterns.",
		"#f87171", providerCodex, "merged changes that touch secrets, permissions or authentication"},
	{agentUXRegression, "UX Regression", "Compares checkout screens against the last release and reports what moved.",
		"#f0abfc", providerClaude, "visual differences on checkout screens since the last release"},
	{agentInfraDrift, "Infra Drift", "Compares live infrastructure with Terraform and reports what drifted.",
		"#fbbf24", providerCodex, "differences between live infrastructure and the Terraform state"},
	{agentRunbookKeeper, "Runbook Keeper", "Flags runbook steps that no longer match the shipped system.",
		"#2dd4bf", providerClaude, "runbook pages whose steps no longer match merged changes"},
}

// rosterSeat is one agent working in one project.
type rosterSeat struct {
	agent, workspace, findingTitle, findingBody string
	createdDays                                 int
	needsYou, paused                            bool
}

var rosterSeats = []rosterSeat{
	{agent: agentPRShepherd, workspace: workspaceKeyPlatform, createdDays: 11, needsYou: true},
	{
		agent:        agentCIDoctor,
		workspace:    workspaceKeyPlatform,
		createdDays:  9,
		findingTitle: "settlement-replay flakes on the Mexico case",
		findingBody: "It failed 4 of the last 12 runs on main, always on the partial-refund replay. " +
			"It started with the retry-budget merge.",
	},
	{agent: agentDependencyWatch, workspace: workspaceKeyPlatform, createdDays: 7},
	{
		agent:        agentSecurityScan,
		workspace:    workspaceKeyPlatform,
		createdDays:  6,
		findingTitle: "A webhook secret is logged at debug level",
		findingBody:  "The PSP client logs the signing header when debug logging is on. It is off in production today.",
	},
	{agent: agentIssueTriager, workspace: workspaceKeyCheckout, createdDays: 10, findingTitle: "#452 has no owner",
		findingBody: "Checkout web: the installment selector resets when the card brand is detected late."},
	{agent: agentOrchestrator, workspace: workspaceKeyCheckout, createdDays: 8},
	{agent: agentPRShepherd, workspace: workspaceKeyCheckout, createdDays: 9},
	{agent: agentCIDoctor, workspace: workspaceKeyCheckout, createdDays: 5, needsYou: true},
	{
		agent:        agentUXRegression,
		workspace:    workspaceKeyCheckout,
		createdDays:  4,
		findingTitle: "The Pix QR code moved below the fold on small screens",
		findingBody:  "Since the token migration the QR code starts below the fold at 360 pixels wide.",
	},
	{
		agent:        agentRiskMonitor,
		workspace:    workspaceKeyRisk,
		createdDays:  8,
		findingTitle: "Issuer retries are inflating Brazil disputes again",
		findingBody:  "Disputes in the 08:00 window are mostly retries after soft declines. Confirmed fraud is flat.",
	},
	{agent: agentLogTriage, workspace: workspaceKeyRisk, createdDays: 7, paused: true},
	{agent: agentSecurityScan, workspace: workspaceKeyRisk, createdDays: 6},
	{agent: agentReleaseWatch, workspace: workspaceKeyLaunch, createdDays: 8},
	{agent: agentOrchestrator, workspace: workspaceKeyLaunch, createdDays: 7},
	{agent: agentInfraDrift, workspace: workspaceKeyInfra, createdDays: 10, needsYou: true},
	{agent: agentCIDoctor, workspace: workspaceKeyInfra, createdDays: 9},
	{
		agent:        agentDependencyWatch,
		workspace:    workspaceKeyInfra,
		createdDays:  6,
		findingTitle: "The Terraform AWS provider has a security release",
		findingBody:  "Version 6.41 fixes a state-file exposure in the secrets resource. We are on 6.38.",
	},
	{
		agent:        agentRunbookKeeper,
		workspace:    workspaceKeyDocs,
		createdDays:  9,
		findingTitle: "The settlement rollback runbook names the old queue",
		findingBody:  "Step 3 still drains settlement-v1. The cutover plan renames it on Wednesday.",
	},
	{agent: agentIssueTriager, workspace: workspaceKeyDocs, createdDays: 7},
	{agent: agentPRShepherd, workspace: workspaceKeyDocs, createdDays: 5, paused: true},
}

func rosterHeartbeat(summary, watch string) string {
	return fmt.Sprintf(`---
version: 1
enabled: true
summary: %q
preferences:
  min_interval: "3h"
context:
  include: []
---

Look for %s. Report only what needs a decision, and stay quiet otherwise.
`, summary, watch)
}

func coreAgentFor(core []agentStory, name string) (agentStory, bool) {
	for _, agent := range core {
		if agent.Name == name {
			return agent, true
		}
	}
	return agentStory{}, false
}

func rosterDefinition(def rosterAgent, workspace string) agentStory {
	model := modelClaude
	if def.provider == providerCodex {
		model = modelCodex
	}
	return agentStory{
		Name: def.name, WorkspaceKey: workspace, Kind: agentKindAlwaysOn, DisplayName: def.display,
		Description: def.description, Appearance: faceAppearance(def.name+"-face", def.color),
		Provider: def.provider, Model: model, Permissions: approveReads, Tools: []string{toolTaskRead},
		CategoryPath: []string{categoryEngineering, "Always-on"},
		Prompt:       def.description + " Keep reports short and name the evidence every time.",
		Heartbeat:    rosterHeartbeat(def.description, def.watch),
	}
}

// rosterAgents returns one definition per seat; bundled agents need none.
func rosterAgents(core []agentStory) []agentStory {
	agents := []agentStory{}
	for _, seat := range rosterSeats {
		if seat.agent == agentOrchestrator {
			continue
		}
		if base, ok := coreAgentFor(core, seat.agent); ok {
			if base.WorkspaceKey == seat.workspace {
				continue
			}
			base.WorkspaceKey = seat.workspace
			base.Heartbeat = strings.ReplaceAll(base.Heartbeat, platformRepo, "northstar-pay/"+seat.workspace)
			agents = append(agents, base)
			continue
		}
		for _, def := range rosterAgentDefs {
			if def.name == seat.agent {
				agents = append(agents, rosterDefinition(def, seat.workspace))
			}
		}
	}
	return agents
}

func seatDisplay(agent string) string {
	for _, def := range rosterAgentDefs {
		if def.name == agent {
			return def.display
		}
	}
	return map[string]string{
		agentIssueTriager: displayIssueTriager, agentReleaseWatch: displayReleaseWatch, agentLogTriage: displayLogTriage,
		agentRiskMonitor: displayRiskMonitor, agentOrchestrator: "Orchestrator",
	}[agent]
}

func rosterSeatInstance(clock timeline, seat rosterSeat, index int) instanceStory {
	provider, model := agentRuntime(seat.workspace, seat.agent)
	created := clock.daysHoursAgo(seat.createdDays, index%7)
	story := instanceStory{
		Agent:        seat.agent,
		WorkspaceKey: seat.workspace,
		Bundled:      seat.agent == agentOrchestrator,
		MainID: fmt.Sprintf(
			"sess_ns_main_%s_%s",
			strings.ReplaceAll(seat.agent, "-", "_"),
			strings.ReplaceAll(seat.workspace, "-", "_"),
		),
		MainName:    seatDisplay(seat.agent),
		Provider:    provider,
		Model:       model,
		CreatedAt:   created,
		Interval:    3 * time.Hour,
		ActiveFrom:  0,
		ActiveTo:    24,
		QuietEvery:  2,
		SeenAt:      clock.hoursAgo(1 + index%4),
		NextCheckIn: time.Duration(17+index*6) * time.Minute,
		Turns: []mainTurn{{At: created.Add(3 * time.Minute), Steps: []transcriptStep{
			operatorSays(
				"Keep an eye on " + seat.workspace + " for me and only write when something needs a decision.",
			),
			agentSays("Watching " + seat.workspace + ". I'll write here only when something needs you."),
		}}},
	}
	if seat.findingTitle != "" {
		story.Beats = append(story.Beats, checkInBeat{
			At: clock.hoursMinutesAgo(5+index%6, 7+index), Result: checkResultReported,
			Proposals: []proposalStory{{
				Title: seat.findingTitle, Body: seat.findingBody,
				ItemKeys: []string{fmt.Sprintf("%s:%d", seat.agent, index)},
			}},
		})
	}
	if seat.needsYou {
		story.SeenAt = clock.hoursAgo(3)
		story.Beats = append(story.Beats, checkInBeat{
			At: clock.minutesAgo(33 + index), Result: checkResultFailed, Reason: reasonTriageTimeout,
			Failure: triageTimeoutFailure,
		})
	}
	if seat.paused {
		story.PauseAt = clock.daysHoursAgo(1, index%5)
		story.SeenAt = story.PauseAt
	}
	return story
}

func rosterInstances(clock timeline) []instanceStory {
	stories := make([]instanceStory, 0, len(rosterSeats))
	for index, seat := range rosterSeats {
		stories = append(stories, rosterSeatInstance(clock, seat, index))
	}
	return stories
}
