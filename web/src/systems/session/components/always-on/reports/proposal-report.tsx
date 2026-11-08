/**
 * A check-in's proposals and its activity. A pick goes through the daemon, which queues it into
 * the main session; the report's live state (picks, auto-accept, tool calls) comes from the
 * instance's projected report read, and older reports read their pick from the chat.
 */
import { useAuiState } from "@assistant-ui/react";
import { use } from "react";

import {
  ReportToolCalls,
  checkInSourceRefTitle,
  proposalPickFailure,
  useHeartbeatSourceCatalog,
  useProposalPick,
  type AgentInstancePayload,
} from "@/systems/agent";

import { countWord, reportClock } from "../../../lib/session-away";
import { SessionReportSendContext } from "../../../lib/session-report-send";
import {
  pickedProposal,
  reportPayload,
  type ReportPick,
  type ReportProposal,
} from "../../../lib/session-report";
import type { ReportAgent } from "../../../lib/session-report-identity";
import type { SessionReportData } from "../../../types";
import { ReportEventSourceLine, ReportItemSourcesLine } from "./report-event-source";
import { Notice, ShoulderTap, type ProposalChoice } from "./report-entry";

const MAYBE_NOT_PICKED = (label: string) =>
  `This may not have reached ${label}. If you picked one earlier, check its reply first.`;

/** Why an action-tagged proposal still waits for you. */
const WAITING_NOTE: Record<string, string> = {
  inactive: "The agent was paused",
  no_main: "The agent has no main session",
  limit: "Daily auto-accept limit reached",
  queue_full: "The agent is busy; pick it when ready",
  unapproved: "Waiting for your approval of check-in changes",
};

function proposalChoice(proposal: ReportProposal, index: number, picks: readonly ReportPick[]) {
  const auto =
    picks.some(pick => pick.index === index && pick.actor === "auto_accept") ||
    proposal.autoAcceptStatus === "picked";
  const choice: ProposalChoice = { title: proposal.title, body: proposal.body };
  if (auto)
    return { ...choice, autoAccepted: { action: proposal.action?.title ?? proposal.title } };
  const note = WAITING_NOTE[proposal.autoAcceptStatus ?? ""];
  return note ? { ...choice, note } : choice;
}

/** Where a proposal came from: the event behind it, or the check-in source items it cites. */
function ProposalSource({
  data,
  instance,
}: {
  data: SessionReportData;
  instance: AgentInstancePayload | undefined;
}) {
  const { source, itemSources } = reportPayload(data);
  const catalog = useHeartbeatSourceCatalog(
    instance?.agent_name ?? "",
    instance?.workspace_id || null,
    instance?.profile_name || "default",
    itemSources.length > 0
  );
  if (itemSources.length > 0) {
    return (
      <ReportItemSourcesLine
        sources={itemSources}
        titleOf={group => checkInSourceRefTitle(group.sourceRef, group.sourceId, catalog)}
      />
    );
  }
  return source ? <ReportEventSourceLine source={source} /> : null;
}

function ask(count: number, allDone: boolean) {
  if (allDone) {
    return count > 1
      ? `I found ${countWord(count).toLowerCase()} things worth doing and started on them.`
      : "I found something worth doing and started on it.";
  }
  return count > 1
    ? `I found ${countWord(count).toLowerCase()} things worth doing. Which one first?`
    : "I found something worth doing. Should I?";
}

export function ProposalReport({
  agent,
  data,
  instance,
  unsure,
}: {
  agent: ReportAgent;
  data: SessionReportData;
  instance: AgentInstancePayload | undefined;
  unsure: boolean;
}) {
  // A transcript that can't send, such as a conversation that moved on, offers no pick.
  const live = use(SessionReportSendContext) !== null;
  const payload = reportPayload(data);
  const { proposals, picks, picksTracked, source, itemSources } = payload;
  const inferred = useAuiState(state =>
    picksTracked ? undefined : pickedProposal(state.thread.messages, proposals, data.created_at)
  );
  const pick = useProposalPick();
  const mine = pick.variables?.reportId === data.report_id;
  const operatorPick = picks.find(item => item.actor === "operator");
  // A pick that just went through holds until the reread shows it.
  const justPicked = mine && pick.isSuccess ? proposals[pick.variables.proposal]?.title : undefined;
  const chosen = operatorPick ? proposals[operatorPick.index]?.title : (justPicked ?? inferred);
  const choices = proposals.map((proposal, index) => proposalChoice(proposal, index, picks));
  const allDone = choices.length > 0 && choices.every(choice => choice.autoAccepted);
  const failure = mine && pick.isError ? proposalPickFailure(pick.error) : null;
  return (
    <ShoulderTap
      agent={agent}
      ask={ask(proposals.length, allDone)}
      choices={choices}
      chosen={chosen}
      error={
        failure ? (
          <>
            {failure.words}
            {failure.code ? <span className="text-subtle"> ({failure.code})</span> : null}
          </>
        ) : undefined
      }
      footer={
        <ReportToolCalls
          instance={instance}
          occurrence={payload.occurrenceId}
          summary={payload.toolCalls}
        />
      }
      footnote={
        unsure
          ? MAYBE_NOT_PICKED(agent.label)
          : allDone
            ? "Auto-accepted actions run in the agent's main session with its own permissions."
            : undefined
      }
      onChoose={
        live && instance
          ? (_choice, index) => pick.mutate({ instance, reportId: data.report_id, proposal: index })
          : undefined
      }
      picking={mine && pick.isPending ? pick.variables?.proposal : undefined}
      source={
        source || itemSources.length > 0 ? (
          <ProposalSource data={data} instance={instance} />
        ) : undefined
      }
      when={reportClock(data.created_at)}
    />
  );
}

function activityTitle(done: number) {
  if (done === 0) return "The check-in may have changed something";
  return `The check-in made ${done} ${done === 1 ? "change" : "changes"}`;
}

/** A check-in that changed things with its tools and asks nothing: what it did, listed open. */
export function ActivityReport({
  agent,
  data,
  instance,
}: {
  agent: ReportAgent;
  data: SessionReportData;
  instance: AgentInstancePayload | undefined;
}) {
  const { toolCalls, occurrenceId } = reportPayload(data);
  const maybe = toolCalls?.writes_maybe ?? 0;
  return (
    <Notice
      agent={agent}
      body={
        maybe > 0
          ? `${maybe} ${maybe === 1 ? "call" : "calls"} may have changed something.`
          : "Here's what it did."
      }
      detail={
        <ReportToolCalls
          defaultOpen
          instance={instance}
          occurrence={occurrenceId}
          summary={toolCalls}
        />
      }
      title={toolCalls ? activityTitle(toolCalls.writes_done) : "The check-in made changes"}
      when={reportClock(data.created_at)}
    />
  );
}
