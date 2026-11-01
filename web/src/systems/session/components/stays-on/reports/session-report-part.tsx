/** One `session.report` in the transcript. It starts no turn; its choices send ordinary prompts. */
import { useAuiState } from "@assistant-ui/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { History } from "@compozy/ui/icons";
import { use, useState, type ReactNode } from "react";

import { Button, Marker } from "@compozy/ui";

import {
  agentInstanceAwayOptions,
  agentInstanceCheckInsOptions,
  agentInstanceReportsOptions,
  useAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

import { useReportSource } from "../../../hooks/use-report-source";
import { useMarkSeenOnLeave } from "../../../hooks/use-mark-seen-on-leave";
import { countWord, deriveAwayDigest, reportClock } from "../../../lib/session-away";
import { SessionReportSendContext } from "../../../lib/session-report-send";
import { pickedProposal, reportPayload } from "../../../lib/session-report";
import type { SessionReportData } from "../../../types";
import { CheckTicks } from "../stays-on-hover-card";
import { AwayDigest } from "./away-digest";
import { reportAgent, type ReportAgent } from "../../../lib/session-report-identity";
import { Notice, SettledAsk, ShoulderTap } from "./report-entry";

const MAYBE_NOT_RECEIVED = (label: string) =>
  `This may not have reached ${label}. If you picked one earlier, check its reply first.`;

function AskButton({ prompt, children }: { prompt: string; children: ReactNode }) {
  const send = use(SessionReportSendContext);
  if (!send) return null;
  return (
    <Button onClick={() => send(prompt)} size="sm" type="button" variant="secondary">
      {children}
    </Button>
  );
}

function Proposal({
  agent,
  data,
  instanceId,
}: {
  agent: ReportAgent;
  data: SessionReportData;
  instanceId: string;
}) {
  const send = use(SessionReportSendContext);
  const proposals = reportPayload(data).proposals;
  const picked = useAuiState(state =>
    pickedProposal(state.thread.messages, proposals, data.created_at)
  );
  const { data: reports } = useQuery(agentInstanceReportsOptions(instanceId));
  const state = reports?.find(report => report.id === data.report_id)?.state;
  return (
    <ShoulderTap
      agent={agent}
      ask={
        proposals.length > 1
          ? `I found ${countWord(proposals.length).toLowerCase()} things worth doing. Which one first?`
          : "I found something worth doing. Should I?"
      }
      choices={proposals}
      chosen={picked}
      footnote={state === "indeterminate" ? MAYBE_NOT_RECEIVED(agent.label) : undefined}
      onChoose={send ? choice => send(choice.title) : undefined}
      when={reportClock(data.created_at)}
    />
  );
}

function FailureReport({
  agent,
  data,
  instance,
}: {
  agent: ReportAgent;
  data: SessionReportData;
  instance: AgentInstancePayload | undefined;
}) {
  const navigate = useNavigate();
  const { detail } = reportPayload(data);
  const { data: checkIns } = useQuery({
    ...agentInstanceCheckInsOptions(data.instance_id),
    enabled: Boolean(instance),
  });
  const ticks = [...(checkIns?.check_ins ?? [])].reverse();
  const streak = instance?.failure_streak ?? 0;
  const body = [
    detail || "The check stopped before it was done.",
    streak > 1 ? `That's ${streak} in a row.` : "",
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <Notice
      actions={
        <>
          <AskButton prompt="Try that check again and tell me what stopped it.">
            Try again
          </AskButton>
          {instance ? (
            <Button
              onClick={() =>
                void navigate({ to: "/agents/$name", params: { name: instance.agent_name } })
              }
              size="sm"
              type="button"
              variant="ghost"
            >
              See check-ins
            </Button>
          ) : null}
        </>
      }
      agent={agent}
      body={body}
      detail={
        ticks.length > 0 ? (
          <div className="flex max-w-xs flex-col gap-1.5">
            <CheckTicks checkIns={ticks} />
            <span className="text-eyebrow text-subtle">
              After 10 failures in a row, check-ins pause on their own.
            </span>
          </div>
        ) : null
      }
      state="failed"
      title="I couldn't finish this check"
      when={reportClock(data.created_at)}
    />
  );
}

function ExpiredApproval({
  data,
  instances,
}: {
  data: SessionReportData;
  instances: readonly AgentInstancePayload[];
}) {
  const payload = reportPayload(data);
  const { agent, session } = useReportSource(payload.sessionId, instances);
  const provider = payload.expiredBy === "provider";
  const subject = payload.tool;
  return (
    <SettledAsk
      actions={
        <AskButton
          prompt={subject ? `Try ${subject} again, I'm here now.` : "Try that again, I'm here now."}
        >
          Ask it to try again
        </AskButton>
      }
      agent={agent}
      outcome={provider ? "Didn't happen" : "Didn't run"}
      session={session}
      subject={subject}
      title={
        provider
          ? `${agent.label}'s request ended early`
          : `${agent.label}'s request went unanswered`
      }
      when={reportClock(data.created_at)}
      why={
        provider
          ? "The agent stopped waiting for an answer before you got to it."
          : "Nobody answered in time, so it wasn't allowed."
      }
    />
  );
}

function InterruptedReport({
  agent,
  data,
  instances,
}: {
  agent: ReportAgent;
  data: SessionReportData;
  instances: readonly AgentInstancePayload[];
}) {
  const source = useReportSource(reportPayload(data).sessionId, instances);
  return (
    <Notice
      actions={
        <AskButton prompt="Pick the work back up where it stopped.">Pick it back up</AskButton>
      }
      agent={agent}
      body={`CompozyOS restarted while ${source.agent.label} was waiting for your answer, so its work stopped there.`}
      title={`${source.agent.label}'s work stopped`}
      when={reportClock(data.created_at)}
    />
  );
}

function ReportBody({
  agent,
  data,
  instance,
  instances,
}: {
  agent: ReportAgent;
  data: SessionReportData;
  instance: AgentInstancePayload | undefined;
  instances: readonly AgentInstancePayload[];
}) {
  switch (data.kind) {
    case "proposal":
      return <Proposal agent={agent} data={data} instanceId={data.instance_id} />;
    case "failure":
      return <FailureReport agent={agent} data={data} instance={instance} />;
    case "approval_expired":
      return <ExpiredApproval data={data} instances={instances} />;
    case "interrupted":
      return <InterruptedReport agent={agent} data={data} instances={instances} />;
    case "rotation_failed":
      return (
        <Notice
          agent={agent}
          body="I tried to start a fresh conversation and couldn't, so we stay here. You don't need to do anything."
          title="Still in this conversation"
          when={reportClock(data.created_at)}
        />
      );
    case "rotation_deferred":
      return (
        <Marker icon={<History aria-hidden="true" className="size-3" />}>
          A fresh conversation is waiting for other work to finish. That's taken over an hour.
        </Marker>
      );
    default:
      return null;
  }
}

export function SessionReportPart({ data }: { data: SessionReportData }) {
  const instances = useAgentInstances();
  const instance = instances.find(candidate => candidate.id === data.instance_id);
  const agent = reportAgent(instance, "Your agent");
  const { data: away } = useQuery(agentInstanceAwayOptions(data.instance_id));
  const [now] = useState(() => Date.now());
  const digest = away ? deriveAwayDigest(away, agent.label, now) : null;
  const leads = digest?.firstReportId === data.report_id;
  useMarkSeenOnLeave(data.instance_id, leads);
  return (
    <div className="flex flex-col gap-5 py-2" data-testid={`session-report-${data.kind}`}>
      {leads && digest ? <AwayDigest agent={agent} digest={digest} /> : null}
      {digest?.foldedIds.has(data.report_id) ? null : (
        <ReportBody agent={agent} data={data} instance={instance} instances={instances} />
      )}
    </div>
  );
}
