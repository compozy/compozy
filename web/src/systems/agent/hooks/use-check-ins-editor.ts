import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";

import { useProfileReadScope } from "@/systems/profiles";

import { AGENT_PAUSE_COPY, agentInstanceResumeBlocked } from "../lib/agent-instance";
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
import { useAgentInstanceActions, useAgentInstances } from "./use-agent-instances";
import { useEditorDocument } from "./use-editor-document";
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
  return (
    instances.find(item => item.id === placeId) ??
    instances.find(item => matchesWorkspace(item, workspace)) ??
    instances[0] ??
    null
  );
}

/** Check-ins' file, plus how the agent stands in the project picked on the page. */
export function useCheckInsEditor(
  agent: AgentPayload,
  workspace: string | null,
  payload: AgentHeartbeatPayload,
  readOnly: boolean
) {
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  const put = usePutAgentHeartbeat();
  const validate = useValidateAgentHeartbeat();
  const scope = { name: agent.name, cacheWorkspace: workspace, profile: destination };
  const selector = workspace ? { workspace_id: workspace } : {};

  const document = useEditorDocument({
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

  const instances = useAgentInstances().filter(item => item.agent_name === agent.name);
  const [placeId, setPlaceId] = useState<string | null>(null);
  const instance = pickInstance(instances, placeId, workspace);

  const [limit, setLimit] = useState(false);
  const [fresh, setFresh] = useState(false);
  const now = useMinuteClock();
  const checkIns = useAgentInstanceDayCheckIns(instance?.id ?? null);
  const setChecks = useSetAgentInstanceChecks();
  const { resume } = useAgentInstanceActions();
  const tryCheck = useTryCheck(instance?.id ?? null);

  const floor = floorStop(payload.config_provenance?.subset.min_interval);
  const settings = { ...document.form, stop: Math.max(document.form.stop, floor) };
  const interval = HOW_OFTEN[settings.stop]!.minutes;
  const standing = instance ? checkInStanding(instance, interval) : "off";
  const nowHour = hourInZone(now, settings.timezone);
  const scheduled =
    document.changes === 0 && !fresh && instance?.next_wake_at
      ? scheduledNextCheckIn(instance.next_wake_at, now)
      : null;

  const turnOn = (on: boolean) => {
    if (!instance) return;
    setChecks.mutate({ id: instance.id, enabled: on });
    if (!on) return;
    setFresh(true);
    // Checks only run with a file that allows them, so turning them on writes one if needed.
    const { baseline, form } = document;
    if (readOnly || baseline.block || (!baseline.missing && form.kept.enabled)) return;
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
    standing,
    nowhere: instance === null,
    /** Never checked in and still off: the page leads with turning it on. */
    never: instance !== null && standing === "off" && !instance.last_wake_at,
    on: instance?.checks_enabled === true,
    paused: standing === "paused" || standing === "failed",
    pauseCopy: instance?.pause_reason ? AGENT_PAUSE_COPY[instance.pause_reason] : undefined,
    resumeBlocked: instance ? agentInstanceResumeBlocked(instance) : false,
    failedInARow: instance?.failure_streak ?? 0,
    slowedBy: instance ? slowedMinutes(instance, interval) : null,
    next: scheduled ?? previewNextCheckIn(settings, nowHour, fresh),
    activity: checkIns === null ? null : checkInActivity(checkIns, now),
    tryCheck,
    turnOn,
    resume: () => {
      if (!instance) return;
      resume(instance.id);
      toast.message("Check-ins are back on");
    },
  };
}

export type CheckInsEditorModel = ReturnType<typeof useCheckInsEditor>;
