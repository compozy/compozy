import { useQueries, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { notifyUser } from "@/lib/user-feedback";
import { useProfileReadScope } from "@/systems/profiles";
import {
  getSessionDisplayTitle,
  useSessionCreateActions,
  useSignedOutProviders,
  type SessionPayload,
} from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import { agentAppearanceFromPayload } from "../lib/agent-appearance";
import { isOrphanAgent } from "../lib/agent-instance-wire";
import { GLOBAL_SCOPE, type AgentInstanceCapacity } from "../lib/agent-instance";
import { formatAgentOwner, isAgentReadOnly } from "../lib/agent-ownership";
import {
  agentActivityView,
  agentBehaviorSummary,
  agentCanResume,
  agentHeadline,
  agentModelSummary,
  agentToolsSummary,
  type AgentEditorPage,
  type AgentProfilePanel,
} from "../lib/agent-profile";
import { agentFindings } from "../lib/agent-reports";
import { buildSettingsDraftFromAgent, buildUpdateAgentParams } from "../lib/agent-settings-draft";
import {
  agentAddablePlaces,
  agentOverallState,
  agentPlaceFromInstance,
  type AgentTeamPlace,
} from "../lib/agent-team";
import { formatRelativeClock } from "../lib/agent-time";
import {
  agentActivityOptions,
  agentInstanceReportsOptions,
  agentTeamOptions,
} from "../lib/query-options";
import { isAgentSessionFailure } from "../lib/session-status";
import type { AgentPayload, UpdateAgentParams } from "../types";
import type { AgentWorkItem } from "../components/agent-profile/agent-profile-work";
import type { AgentHeroIdentity } from "../components/agent-profile/agent-profile-hero";
import type { AgentAddPlaceActions } from "../components/agent-team/agent-team-parts";
import {
  useAgentInstanceActions,
  useAgentInstanceCapacity,
  useAgentInstances,
} from "./use-agent-instances";
import { useAgentSessions } from "./use-agent-sessions";
import { useAgentWindowScope } from "./use-agent-window-scope";
import { useUpdateAgent } from "./use-agents";

const ACTIVITY_DAYS = 30;
const WORK_SHOWN = 5;

function workItem(session: SessionPayload): AgentWorkItem {
  const failed = isAgentSessionFailure(session);
  return {
    id: session.id,
    title: getSessionDisplayTitle(session),
    detail: failed ? "Didn't finish" : session.state === "active" ? "Still going" : "Finished",
    when: formatRelativeClock(session.updated_at) ?? "",
    tone: failed ? "destructive" : session.state === "active" ? "neutral" : "success",
  };
}

function useFindings(places: readonly AgentTeamPlace[]) {
  const results = useQueries({
    queries: places.map(place => agentInstanceReportsOptions(place.instance)),
  });
  return agentFindings(
    places.flatMap((place, index) =>
      (results[index]?.data ?? []).map(report => ({ place, report }))
    )
  );
}

function useAddPlace(agent: AgentPayload, capacity: AgentInstanceCapacity): AgentAddPlaceActions {
  const navigate = useNavigate();
  const { workspaces } = useActiveWorkspace();
  const actions = useAgentInstanceActions();
  const { destination } = useProfileReadScope();
  const { data: team } = useQuery(agentTeamOptions("", destination));
  const scopes = team?.find(
    item => item.name === agent.name && item.definition_digest === agent.definition_digest
  )?.scopes;
  const places = [{ id: GLOBAL_SCOPE, name: "Global" }, ...workspaces];
  return {
    places: scopes ? agentAddablePlaces(places, scopes) : places,
    capacity,
    signedOut: useSignedOutProviders(),
    onAdd: (agent, place) => actions.add({ agent, workspace: place }),
    onSignIn: () => void navigate({ to: "/settings/providers" }),
  };
}

function useIdentitySave(agent: AgentPayload) {
  const update = useUpdateAgent();
  const { destination } = useProfileReadScope();
  const { workspace, projectId } = useAgentWindowScope();
  return (patch: Partial<UpdateAgentParams["agent"]>) => {
    const params = buildUpdateAgentParams(buildSettingsDraftFromAgent(agent), projectId);
    if (!params) {
      notifyUser({
        message: "This agent's setup needs fixing before it can change.",
        tone: "error",
      });
      return;
    }
    update.mutate(
      {
        name: agent.name,
        profile: destination,
        cacheWorkspace: workspace,
        params: { ...params, agent: { ...params.agent, ...patch } },
      },
      {
        onError: error =>
          notifyUser({
            message: error instanceof Error ? error.message : "Couldn't save this agent.",
            tone: "error",
          }),
      }
    );
  };
}

function checkInsSummary(places: readonly AgentTeamPlace[]) {
  const on = places.filter(place => place.instance.checks_enabled).length;
  if (on === 0) return "Off";
  return `On in ${on} ${on === 1 ? "project" : "projects"}`;
}

/** The profile's view model: one agent, its live places, and what they've done. */
export function useAgentProfile(input: {
  agent: AgentPayload;
  panel: AgentProfilePanel | null;
  setPanel: (panel: AgentProfilePanel | null) => void;
  active: number;
  memorySummary: string;
}) {
  const { agent, panel, setPanel, active } = input;
  const navigate = useNavigate();
  const { workspaces } = useActiveWorkspace();
  const { destination } = useProfileReadScope();
  const { workspace, projectId, bindingSearch } = useAgentWindowScope();
  const { openForAgent } = useSessionCreateActions();
  const instances = useAgentInstances();
  const capacity = useAgentInstanceCapacity();
  const actions = useAgentInstanceActions();
  const names = new Map(workspaces.map(item => [item.id, item.name]));
  const places = instances.flatMap(instance =>
    instance.agent_name === agent.name ? [agentPlaceFromInstance(instance, names)] : []
  );
  const staysOn = agent.kind === "stays_on" || places.length > 0;
  const state = agentOverallState({ active, kind: staysOn ? "stays_on" : "on_demand", places });
  const activityQuery = useQuery(
    agentActivityOptions(agent.name, ACTIVITY_DAYS, workspace, destination)
  );
  // `undefined` while loading, `null` when no route answers: the two say different things.
  const activity = activityQuery.isSuccess ? activityQuery.data : undefined;
  const { sessions } = useAgentSessions(workspace, agent.name);
  const findings = useFindings(staysOn ? places : []);
  const work = sessions.slice(0, WORK_SHOWN).map(workItem);
  const packageOwned = isAgentReadOnly(agent);
  const readOnly = packageOwned || isOrphanAgent(agent);
  const identity: AgentHeroIdentity = {
    name: agent.name,
    label: agent.label,
    description: agent.description ?? "",
    appearance: agentAppearanceFromPayload(agent.resolved_appearance, agent.name),
    provider: agent.provider || null,
    staysOn,
    readOnly,
    ...(packageOwned ? { owner: formatAgentOwner(agent) } : {}),
  };
  const openSessionById = (id: string) =>
    void navigate({ to: "/agents/$name/sessions/$id", params: { name: agent.name, id } });
  const openSession = async (place: AgentTeamPlace) => {
    const current = place.instance.main_session_id
      ? place.instance
      : await actions.ensureMain(place.instance).catch(() => null);
    if (current?.main_session_id) openSessionById(current.main_session_id);
  };
  const saveIdentity = useIdentitySave(agent);
  return {
    identity,
    places,
    state,
    panel,
    setPanel,
    activity,
    findings,
    work,
    diagnostics: agent.diagnostics ?? [],
    headline: agentHeadline({ staysOn, active, places, state }),
    fresh:
      work.length === 0 &&
      findings.length === 0 &&
      (activity ? agentActivityView(activity, ACTIVITY_DAYS).empty : false),
    capacity,
    add: useAddPlace(agent, capacity),
    newSessionDisabled: projectId === null,
    summaries: {
      behavior: agentBehaviorSummary(agent),
      model: agentModelSummary(agent),
      checkIns: checkInsSummary(places),
      memory: input.memorySummary,
      tools: agentToolsSummary(agent),
    },
    placeActions: {
      onOpen: (place: AgentTeamPlace) => void openSession(place),
      onToggle: (place: AgentTeamPlace) =>
        place.instance.state === "paused"
          ? actions.resume(place.instance)
          : actions.pause(place.instance),
      onArchive: (place: AgentTeamPlace) => actions.archive(place.instance),
    },
    onResumeAll: () => {
      for (const place of places) {
        if (place.instance.state === "paused" && agentCanResume(place.pauseReason)) {
          actions.resume(place.instance);
        }
      }
    },
    onSaveIdentity: saveIdentity,
    onNewSession: () => openForAgent(agent.name),
    onOpenSession: openSessionById,
    onOpenFinding: (finding: { sessionId?: string | null }) => {
      if (finding.sessionId) openSessionById(finding.sessionId);
    },
    onOpenWhere: () =>
      void navigate({
        to: "/agents/$name/$page",
        params: { name: agent.name, page: "where" },
        search: bindingSearch,
      }),
    onOpenEditor: (page: AgentEditorPage) =>
      void navigate({
        to: "/agents/$name/$page",
        params: { name: agent.name, page },
        search: bindingSearch,
      }),
  };
}

export type UseAgentProfile = ReturnType<typeof useAgentProfile>;
