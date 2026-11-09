package demoseed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	instanceActiveLimit  = 10
	checkInRetention     = 30 * 24 * time.Hour
	autoAcceptPerDay     = 3
	pickQueueCap         = 20
	checkInSourceTimer   = "timer"
	operatorApprovalVia  = "web"
	pauseReasonOperator  = "manual"
	reasonDigestEmpty    = "digest_empty"
	reasonDigestSame     = "digest_unchanged"
	pickReplyDelay       = 40 * time.Second
	autoPickDispatchWait = 90 * time.Second
)

// alwaysOnCounts summarizes what the always-on replay wrote.
type alwaysOnCounts struct {
	instances int
	checkIns  int
	reports   int
	picks     int
	// transcriptEvents counts the main-session events written.
	transcriptEvents int
}

// instanceRun carries one instance through its replay.
type instanceRun struct {
	story      instanceStory
	agent      agentStory
	record     workspaceRecord
	instance   agentinstance.Instance
	policy     store.SessionPermissionPolicy
	dir        string
	authority  string
	expected   string
	hasSources bool
	// picks maps a check time to the queue entries its proposals started.
	picks map[time.Time]bool
	// admissions are the operator prompts, admitted once the reports before them are published.
	admissions []operatorAdmission
	// events is how many main-session events were written.
	events int
}

func seedAlwaysOn(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (alwaysOnCounts, error) {
	defer state.writeClock.reset()
	if err := writeWorkLog(state); err != nil {
		return alwaysOnCounts{}, err
	}
	if err := writeSourceFeeds(state); err != nil {
		return alwaysOnCounts{}, err
	}
	counts := alwaysOnCounts{}
	for _, story := range scenarioInstances(state.clock) {
		run, err := startInstance(ctx, db, state, story)
		if err != nil {
			return alwaysOnCounts{}, err
		}
		checks, picks, err := replayCheckIns(ctx, db, state, run)
		if err != nil {
			return alwaysOnCounts{}, err
		}
		reports, err := writeMainSession(ctx, db, state, run)
		if err != nil {
			return alwaysOnCounts{}, err
		}
		if err := settleInstance(ctx, db, state, run); err != nil {
			return alwaysOnCounts{}, err
		}
		if err := writeAgentNotes(state, run); err != nil {
			return alwaysOnCounts{}, err
		}
		counts.instances++
		counts.checkIns += checks
		counts.reports += reports
		counts.picks += picks
		counts.transcriptEvents += run.events
	}
	return counts, nil
}

func agentFor(story instanceStory) (agentStory, error) {
	if story.Bundled {
		return agentStory{
			Name: story.Agent, WorkspaceKey: story.WorkspaceKey, Permissions: bundledAgentPermissions[story.Agent],
			Tools: []string{
				"compozy__worktree_create", "compozy__session_create", "compozy__session_history",
				"compozy__session_prompt", toolMonitorCreate,
			},
		}, nil
	}
	for _, agent := range alwaysOnAgents() {
		if agent.Name == story.Agent && agent.WorkspaceKey == story.WorkspaceKey {
			return agent, nil
		}
	}
	return agentStory{}, fmt.Errorf("demo seed: always-on agent %q is not defined", story.Agent)
}

// startInstance turns the agent on the way the operator does: admit, start its main session, bind.
func startInstance(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	story instanceStory,
) (*instanceRun, error) {
	agent, err := agentFor(story)
	if err != nil {
		return nil, err
	}
	record, err := state.recordFor(story.WorkspaceKey)
	if err != nil {
		return nil, err
	}
	// The ceiling the daemon resolves at creation: the definition's tools and the project root.
	// The seed workspaces carry no skills or MCP servers, so those stay empty.
	policy := store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{
		Tools: agent.Tools, WorkspacePaths: []string{record.RootDir},
	})
	ceiling, err := store.EncodeSessionPermissionPolicy(policy)
	if err != nil {
		return nil, fmt.Errorf("demo seed: encode permission ceiling for %q: %w", story.Agent, err)
	}
	state.writeClock.at(story.CreatedAt)
	instance, created, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
		ProfileID: store.DefaultProfileID, AgentName: story.Agent, WorkspaceID: record.ID,
		PermissionCeiling: ceiling, CreatedByKind: string(workspaceaccess.ActorHuman),
	}, instanceActiveLimit)
	if err != nil {
		return nil, fmt.Errorf("demo seed: turn on %q: %w", story.Agent, err)
	}
	if !created {
		return nil, fmt.Errorf("demo seed: %q already has a live instance in %q", story.Agent, record.Name)
	}
	if err := registerMainSession(ctx, db, record, story, instance.ID, policy); err != nil {
		return nil, err
	}
	instance, err = db.Instances().BindMain(ctx, instance.ID, story.MainID)
	if err != nil {
		return nil, fmt.Errorf("demo seed: bind main session for %q: %w", story.Agent, err)
	}
	run := &instanceRun{
		story: story, agent: agent, record: record, instance: instance, policy: policy,
		dir:        filepath.Join(record.RootDir, config.DirName, config.AgentsDirName, story.Agent),
		hasSources: len(agent.Scripts) > 0 || story.Agent == agentIssueTriager,
		picks:      map[time.Time]bool{},
	}
	if story.Bundled {
		return run, nil
	}
	if err := approveCheckInAuthority(ctx, db, state, run); err != nil {
		return nil, err
	}
	return run, nil
}

// approveCheckInAuthority records the operator approving the agent's scripts and actions by hash.
func approveCheckInAuthority(ctx context.Context, db *globaldb.GlobalDB, state *scenario, run *instanceRun) error {
	dir, err := filepath.EvalSymlinks(run.dir)
	if err != nil {
		return fmt.Errorf("demo seed: resolve definition folder for %q: %w", run.story.Agent, err)
	}
	run.dir = dir
	approvedAt := run.story.CreatedAt.Add(time.Minute)
	for name := range run.agent.Scripts {
		_, hash, err := source.ReadScript(dir, name)
		if err != nil {
			return fmt.Errorf("demo seed: hash source %q for %q: %w", name, run.story.Agent, err)
		}
		if err := db.Instances().ApproveScript(
			ctx, store.DefaultProfileID, dir, name, hash, operatorApprovalVia, approvedAt,
		); err != nil {
			return fmt.Errorf("demo seed: approve source %q for %q: %w", name, run.story.Agent, err)
		}
	}
	if len(run.story.Actions) == 0 {
		return nil
	}
	digest, err := heartbeat.AuthorityDigest(heartbeat.CheckInsPolicy{}, heartbeat.AuthorityActions(run.story.Actions))
	if err != nil {
		return fmt.Errorf("demo seed: digest check-in authority for %q: %w", run.story.Agent, err)
	}
	run.authority = digest
	state.writeClock.at(approvedAt)
	if err := db.Instances().ApproveAuthority(
		ctx, store.DefaultProfileID, dir, digest, operatorApprovalVia, approvedAt,
	); err != nil {
		return fmt.Errorf("demo seed: approve check-in authority for %q: %w", run.story.Agent, err)
	}
	return nil
}

type timelineEntry struct {
	at      time.Time
	routine int
	beat    *checkInBeat
	pick    bool
	turn    *mainTurn
	index   int
}

func checkInTimeline(story instanceStory, now time.Time) []timelineEntry {
	entries := []timelineEntry{}
	for index, at := range routineCheckTimes(story, now) {
		entries = append(entries, timelineEntry{at: at, routine: index + 1})
	}
	for index := range story.Turns {
		turn := &story.Turns[index]
		entries = append(entries, timelineEntry{at: turn.At, turn: turn, index: index})
	}
	for index := range story.Beats {
		beat := &story.Beats[index]
		entries = append(entries, timelineEntry{at: beat.At, beat: beat})
		if !beat.PickAt.IsZero() {
			entries = append(entries, timelineEntry{at: beat.PickAt, beat: beat, pick: true})
		}
	}
	slices.SortStableFunc(entries, func(a, b timelineEntry) int { return a.at.Compare(b.at) })
	return entries
}

// replayCheckIns drives every check through Admit/Finish or Skip, exactly as the scheduler does.
func replayCheckIns(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	run *instanceRun,
) (int, int, error) {
	entries := checkInTimeline(run.story, state.clock.Now())
	checks, picks := 0, 0
	for index, entry := range entries {
		next := nextCheckAt(entries, index, run.story, state.clock.Now())
		state.writeClock.at(entry.at)
		var err error
		switch {
		case entry.turn != nil:
			queueStoryTurn(run, entry)
		case entry.pick:
			err = pickProposal(ctx, db, run, entry)
			picks++
		case entry.beat != nil:
			var auto int
			auto, err = finishCheck(ctx, db, run, entry.at, next, *entry.beat)
			picks += auto
			checks++
		default:
			err = routineCheck(ctx, db, run, entry, next)
			checks++
		}
		if err != nil {
			return 0, 0, fmt.Errorf("demo seed: replay %q check-in at %s: %w", run.story.Agent, entry.at, err)
		}
	}
	return checks, picks, nil
}

// queueStoryTurn queues an operator turn of the main-session conversation for admission.
func queueStoryTurn(run *instanceRun, entry timelineEntry) {
	if len(entry.turn.Steps) == 0 || entry.turn.Steps[0].Kind != stepUser {
		return
	}
	key := fmt.Sprintf("%s_turn_%02d", run.story.MainID, entry.index+1)
	run.admissions = append(run.admissions, operatorAdmission{key: key, text: entry.turn.Steps[0].Text, at: entry.at})
}

func nextCheckAt(entries []timelineEntry, index int, story instanceStory, now time.Time) time.Time {
	for _, entry := range entries[index+1:] {
		if !entry.pick && entry.turn == nil {
			return entry.at
		}
	}
	if !story.PauseAt.IsZero() {
		return story.PauseAt.Add(story.Interval)
	}
	return now.Add(story.NextCheckIn)
}

func routineCheck(
	ctx context.Context,
	db *globaldb.GlobalDB,
	run *instanceRun,
	entry timelineEntry,
	next time.Time,
) error {
	if run.story.QuietEvery > 0 && entry.routine%run.story.QuietEvery == 0 {
		_, err := finishCheck(ctx, db, run, entry.at, next, checkInBeat{Result: checkResultQuiet})
		return err
	}
	reason := reasonDigestSame
	if run.hasSources {
		reason = reasonDigestEmpty
	}
	changed, err := db.Instances().Skip(ctx, heartbeat.SkipRequest{
		InstanceID: run.instance.ID, ExpectedDue: run.expected, Reason: reason,
		Now: entry.at, NextWakeAt: next, ExpiresAt: entry.at.Add(checkInRetention),
	})
	if err != nil {
		return err
	}
	if !changed {
		return errors.New("the scheduler would not skip this check")
	}
	run.expected = store.FormatTimestamp(next)
	return nil
}

// finishCheck admits one check, then finishes it with the beat's outcome and report.
func finishCheck(
	ctx context.Context,
	db *globaldb.GlobalDB,
	run *instanceRun,
	at time.Time,
	next time.Time,
	beat checkInBeat,
) (int, error) {
	occurrence, err := store.NewID(checkInSourceTimer)
	if err != nil {
		return 0, err
	}
	admitted, err := db.Instances().Admit(ctx, heartbeat.AdmitRequest{
		ExpectedDue: run.expected, InstanceID: run.instance.ID, OccurrenceID: occurrence,
		Now: at, NextWakeAt: next, MaxConcurrent: instanceActiveLimit,
	})
	if err != nil {
		return 0, err
	}
	if !admitted {
		return 0, errors.New("the scheduler would not admit this check")
	}
	report, err := beatReport(beat)
	if err != nil {
		return 0, err
	}
	reason := beat.Reason
	if reason == "" {
		reason = beat.Result
	}
	result, err := db.Instances().Finish(ctx, heartbeat.FinishRequest{
		Actions: run.story.Actions, AuthorityDigest: run.authority, DefinitionDir: run.dir,
		AutoAcceptPerDay: autoAcceptPerDay, QueueCap: pickQueueCap, ToolsStatus: "unavailable",
		HasSources: run.hasSources, FatigueLimit: 10,
		InstanceID: run.instance.ID, OccurrenceID: occurrence, Source: checkInSourceTimer,
		Result: beat.Result, Reason: reason, Detail: beat.Detail, Digest: occurrence,
		Now: at, NextWakeAt: next, ExpiresAt: at.Add(checkInRetention), Report: report,
	})
	if err != nil {
		return 0, err
	}
	run.expected = store.FormatTimestamp(next)
	for _, pick := range result.Picks {
		// The daemon dispatches an auto-accepted pick into the main session right away.
		if err := dispatchPick(ctx, db, run, pick.QueueEntryID, at.Add(autoPickDispatchWait)); err != nil {
			return 0, err
		}
	}
	if len(result.Picks) > 0 {
		run.picks[at] = true
	}
	return len(result.Picks), nil
}

func beatReport(beat checkInBeat) (*agentinstance.ReportInput, error) {
	switch {
	case beat.Failure != "":
		return &agentinstance.ReportInput{Kind: "failure", Payload: []byte(beat.Failure)}, nil
	case len(beat.Proposals) > 0:
		proposals := make([]heartbeat.TriageProposal, 0, len(beat.Proposals))
		for _, proposal := range beat.Proposals {
			proposals = append(proposals, heartbeat.TriageProposal{
				Title: proposal.Title, Body: proposal.Body, ItemKeys: proposal.ItemKeys, Action: proposal.Action,
			})
		}
		payload, err := json.Marshal(map[string]any{"proposals": proposals})
		if err != nil {
			return nil, err
		}
		return &agentinstance.ReportInput{Kind: agentinstance.ReportKindProposal, Payload: payload}, nil
	default:
		return nil, nil
	}
}
