import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { toast } from "@compozy/ui";

import { useProfileReadScope } from "@/systems/profiles";

import { agentInstanceResumeBlocked, agentPauseCopy } from "../lib/agent-instance";
import { checkInActivity, checkInStanding, slowedMinutes } from "../lib/agent-check-in-activity";
import {
  HOW_OFTEN,
  checkInsFromPayload,
  composeCheckInsFile,
  countCheckInChanges,
  floorStop,
  hourInZone,
  previewNextCheckIn,
  scheduledNextCheckIn,
} from "../lib/agent-check-in-form";
import type { CheckInSource } from "../lib/agent-check-in-sources";
import { fileProblem } from "../lib/agent-frontmatter";
import { agentHeartbeatOptions } from "../lib/query-options";
import type { AgentHeartbeatPayload, AgentInstancePayload, AgentPayload } from "../types";
import { buildAuthoredFileResourceKey } from "./use-agent-authored-file-editor";
import { usePutAgentHeartbeat, useValidateAgentHeartbeat } from "./use-agent-heartbeat";
import {
  useAgentInstanceDayCheckIns,
  useSetAgentInstanceChecks,
  useTryCheck,
} from "./use-agent-instance-checks";
import { useAgentInstanceActions, useAgentDefinitionInstances } from "./use-agent-instances";
import { useEditorDocument } from "./use-editor-document";
import { useApproveHeartbeatScript, useHeartbeatSourceCatalogQuery } from "./use-heartbeat-sources";
import { useMinuteClock } from "./use-minute-clock";

function matchesWorkspace(instance: AgentInstancePayload, workspace: string | null) {
  if (workspace === null) return false;
  return workspace === "@user" ? instance.workspace_id === "" : instance.workspace_id === workspace;
}

function pickInstance(
  instances: readonly AgentInstancePayload[],
  placeId: string | null,
  workspace: string | null
): AgentInstancePayload | null {
  if (placeId) return instances.find(item => item.id === placeId) ?? null;
  // The route selects the authored definition; its placements may be in other projects.
  return instances.find(item => matchesWorkspace(item, workspace)) ?? instances[0] ?? null;
}

function useCheckInsDocument(
  agent: AgentPayload,
  workspace: string | null,
  payload: AgentHeartbeatPayload
) {
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  const put = usePutAgentHeartbeat();
  const validate = useValidateAgentHeartbeat();
  const scope = { name: agent.name, cacheWorkspace: workspace, profile: destination };
  const selector = workspace ? { workspace_id: workspace } : {};

  return useEditorDocument({
    resourceKey: buildAuthoredFileResourceKey(workspace, agent.name, "heartbeat", destination),
    snapshot: checkInsFromPayload(payload),
    compose: composeCheckInsFile,
    countChanges: countCheckInChanges,
    savedCopy: "Saved",
    validate: async body => {
      const result = await validate.mutateAsync({ ...scope, params: { body, ...selector } });
      return result.valid
        ? undefined
        : (fileProblem(result.diagnostics) ?? "Fix the file to save.");
    },
    write: async (body, expected_digest) => {
      const result = await put.mutateAsync({
        ...scope,
        params: { body, expected_digest, ...selector },
      });
      return checkInsFromPayload(result.heartbeat);
    },
    reread: async () =>
      checkInsFromPayload(
        await queryClient.fetchQuery({
          ...agentHeartbeatOptions(agent.name, workspace, destination),
          staleTime: 0,
        })
      ),
  });
}

/** Everything the page says about the agent's standing; no instance reads as off and nowhere. */
function instanceFacts(instance: AgentInstancePayload | null, interval: number) {
  if (!instance) {
    return {
      standing: "off" as const,
      never: false,
      on: false,
      pauseCopy: undefined,
      resumeBlocked: false,
      failedInARow: 0,
      slowedBy: null,
    };
  }
  const standing = checkInStanding(instance, interval);
  return {
    standing,
    /** Never checked in and still off: the page leads with turning it on. */
    never: standing === "off" && !instance.last_wake_at,
    on: instance.checks_enabled ?? false,
    pauseCopy: agentPauseCopy(instance) || undefined,
    resumeBlocked: agentInstanceResumeBlocked(instance),
    failedInARow: instance.failure_streak ?? 0,
    slowedBy: slowedMinutes(instance, interval),
  };
}

/**
 * What can read into check-ins, and the edits to the file's `context.sources`. Adding or editing
 * writes the file through the editor's save; removing waits in the unsaved changes.
 */
function useCheckInSources(
  agent: AgentPayload,
  workspace: string | null,
  document: ReturnType<typeof useCheckInsDocument>,
  enabled: boolean
) {
  const { destination } = useProfileReadScope();
  const query = useHeartbeatSourceCatalogQuery(agent.name, workspace, destination, enabled);
  const approve = useApproveHeartbeatScript(agent.name, workspace, destination);
  const navigate = useNavigate();
  const catalog = query.data ?? null;
  const { form } = document;
  return {
    catalog,
    actions: {
      catalogState: query.isError
        ? ("error" as const)
        : catalog
          ? ("ready" as const)
          : ("loading" as const),
      retryCatalog: () => void query.refetch(),
      /** Writes `source` in place of `replacing`, or at the end. */
      save: (source: CheckInSource, replacing?: string) => {
        const next = replacing
          ? form.sources.map(item => (item.id === replacing ? source : item))
          : [...form.sources, source];
        document.setForm({ ...form, sources: next });
        void document.save();
      },
      remove: (id: string) =>
        document.setForm({ ...form, sources: form.sources.filter(item => item.id !== id) }),
      approve: (script: string, sha256: string) => approve.mutate({ script, sha256 }),
      /** The script an approval is on its way for. */
      approving: approve.isPending ? (approve.variables?.script ?? null) : null,
      openExtensions: () => void navigate({ to: "/settings/extensions" }),
    },
  };
}

export type CheckInSourceActions = ReturnType<typeof useCheckInSources>["actions"];

/** Check-ins' file, plus how the agent stands in the project picked on the page. */
export function useCheckInsEditor(
  agent: AgentPayload,
  workspace: string | null,
  payload: AgentHeartbeatPayload,
  readOnly: boolean
) {
  const document = useCheckInsDocument(agent, workspace, payload);
  const sources = useCheckInSources(
    agent,
    workspace,
    document,
    // Titles need it when the file has sources; Add needs its kinds and limit when editable.
    document.form.sources.length > 0 || !readOnly
  );

  const instances = useAgentDefinitionInstances(agent);
  const [placeId, setPlaceId] = useState<string | null>(null);
  const instance = pickInstance(instances, placeId, workspace);

  const [limit, setLimit] = useState(false);
  const [fresh, setFresh] = useState(false);
  const now = useMinuteClock();
  const checkIns = useAgentInstanceDayCheckIns(instance);
  const setChecks = useSetAgentInstanceChecks();
  const { resume } = useAgentInstanceActions();
  const tryCheck = useTryCheck(instance);

  const floor = floorStop(payload.config_provenance?.subset.min_interval);
  const settings = { ...document.form, stop: Math.max(document.form.stop, floor) };
  const interval = HOW_OFTEN[settings.stop]!.minutes;
  const facts = instanceFacts(instance, interval);
  const nowHour = hourInZone(now, settings.timezone);
  const scheduled =
    document.changes === 0 && !fresh && instance?.next_wake_at
      ? scheduledNextCheckIn(instance.next_wake_at, now)
      : null;

  const turnOn = (on: boolean) => {
    if (!instance) return;
    setChecks.mutate({ instance, enabled: on });
    if (!on) return;
    setFresh(true);
    // No file already allows checks on the config's timer, so only a file saying no needs a write.
    const { baseline, form } = document;
    if (readOnly || baseline.block || baseline.missing || form.kept.enabled) return;
    document.setForm({ ...form, kept: { ...form.kept, enabled: true } });
    void document.save();
  };

  return {
    document,
    settings,
    floor,
    limit,
    setLimit,
    readOnly,
    nowHour,
    instance,
    places: instances,
    setPlace: setPlaceId,
    ...facts,
    nowhere: instance === null,
    paused: facts.standing === "paused" || facts.standing === "failed",
    next: scheduled ?? previewNextCheckIn(settings, nowHour, fresh),
    activity: checkIns === null ? null : checkInActivity(checkIns, now),
    tryCheck,
    catalog: sources.catalog,
    sources: sources.actions,
    /** The minute clock the page reads relative times against. */
    now,
    turnOn,
    resume: () => {
      if (!instance) return;
      resume(instance);
      toast.message("Check-ins are back on");
    },
  };
}

export type CheckInsEditorModel = ReturnType<typeof useCheckInsEditor>;
