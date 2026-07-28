import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import { AgentApiError, isAgentDigestConflict } from "../adapters/agent-api";
import {
  buildSettingsDraftFromAgent,
  buildUpdateAgentParams,
  isAgentSettingsDraftDirty,
  type AgentSettingsDraft,
} from "../lib/agent-settings-draft";
import type { AgentPayload, UpdateAgentParams } from "../types";

interface AgentSettingsEditorBase {
  resourceKey: string;
  requestId: number;
  baseline: AgentPayload | null;
  draft: AgentSettingsDraft | null;
}

interface AgentSettingsEditorReady extends AgentSettingsEditorBase {
  phase: "ready";
}

interface AgentSettingsEditorSaving extends AgentSettingsEditorBase {
  phase: "saving";
}

interface AgentSettingsEditorReloading extends AgentSettingsEditorBase {
  phase: "reloading";
}

interface AgentSettingsEditorConflict extends AgentSettingsEditorBase {
  phase: "conflict";
  error: string;
}

interface AgentSettingsEditorDenied extends AgentSettingsEditorBase {
  phase: "denied";
}

interface AgentSettingsEditorFailed extends AgentSettingsEditorBase {
  phase: "failed";
  error: string;
}

export type AgentSettingsEditorState =
  | AgentSettingsEditorReady
  | AgentSettingsEditorSaving
  | AgentSettingsEditorReloading
  | AgentSettingsEditorConflict
  | AgentSettingsEditorDenied
  | AgentSettingsEditorFailed;

export interface AgentSettingsEditorInput {
  resourceKey: string;
  agent: AgentPayload | undefined;
}

export interface AgentSettingsSaveInput {
  cacheWorkspace: string | null;
  name: string;
  params: UpdateAgentParams;
}

type AgentSettingsEditorEventPayloadMap = {
  draftReplaced: { draft: AgentSettingsDraft };
  draftPatched: { patch: Partial<AgentSettingsDraft> };
  discardRequested: {};
  saveRequested: {
    name: string;
    save: (input: AgentSettingsSaveInput) => Promise<AgentPayload>;
    workspaceId: string | null;
  };
  saveSucceeded: { requestId: number; agent: AgentPayload };
  saveFailed: {
    requestId: number;
    kind: "conflict" | "denied" | "failed";
    error: string;
  };
  reloadRequested: { reload: () => Promise<AgentPayload | undefined> };
  reloadSucceeded: { requestId: number; agent: AgentPayload | undefined };
  reloadFailed: { requestId: number; error: string };
};

export function createAgentSettingsEditorLogic() {
  return createStoreLogic<
    AgentSettingsEditorState,
    AgentSettingsEditorEventPayloadMap,
    Record<never, never>,
    AgentSettingsEditorInput
  >({
    context: input => seedState(input.resourceKey, input.agent, 0),
    on: {
      draftReplaced: (context, event) => {
        if (context.phase === "saving" || context.phase === "reloading") return;
        return { ...toReady(context), draft: event.draft };
      },
      draftPatched: (context, event) => {
        if (context.phase === "saving" || context.phase === "reloading" || !context.draft) return;
        return { ...toReady(context), draft: { ...context.draft, ...event.patch } };
      },
      discardRequested: context => {
        if (context.phase === "saving" || context.phase === "reloading" || !context.baseline)
          return;
        return {
          ...toReady(context),
          draft: buildSettingsDraftFromAgent(context.baseline),
        };
      },
      saveRequested: (context, event, enqueue) => {
        if (
          context.phase === "saving" ||
          context.phase === "reloading" ||
          !context.draft ||
          !isDirty(context)
        )
          return;
        const params = buildUpdateAgentParams(context.draft, event.workspaceId);
        if (!params) return;
        const requestId = context.requestId + 1;
        const input = {
          cacheWorkspace: event.workspaceId,
          name: event.name,
          params,
        };
        enqueue.effect(async ({ trigger }) => {
          try {
            trigger.saveSucceeded({ requestId, agent: await event.save(input) });
          } catch (error) {
            const conflict = isAgentDigestConflict(error);
            trigger.saveFailed({
              requestId,
              kind: conflict ? "conflict" : isAgentPermissionDenied(error) ? "denied" : "failed",
              error: conflict
                ? "This agent changed elsewhere. Reload the latest definition, then retry your edits."
                : errorMessage(error, "Couldn't save agent"),
            });
          }
        });
        return { ...toReady(context), phase: "saving", requestId };
      },
      saveSucceeded: (context, event, enqueue) => {
        if (context.phase !== "saving" || context.requestId !== event.requestId) return;
        enqueue.effect(() => notifyUser({ message: "Changes saved", tone: "success" }));
        return seedState(context.resourceKey, event.agent, context.requestId);
      },
      saveFailed: (context, event) => {
        if (context.phase !== "saving" || context.requestId !== event.requestId) return;
        if (event.kind === "conflict") return { ...context, phase: "conflict", error: event.error };
        if (event.kind === "denied") return { ...context, phase: "denied" };
        return { ...context, phase: "failed", error: event.error };
      },
      reloadRequested: (context, event, enqueue) => {
        if (context.phase === "saving" || context.phase === "reloading") return;
        const requestId = context.requestId + 1;
        enqueue.effect(async ({ trigger }) => {
          try {
            trigger.reloadSucceeded({ requestId, agent: await event.reload() });
          } catch (error) {
            trigger.reloadFailed({
              requestId,
              error: errorMessage(error, "Couldn't reload agent"),
            });
          }
        });
        return { ...toReady(context), phase: "reloading", requestId };
      },
      reloadSucceeded: (context, event) => {
        if (context.phase !== "reloading" || context.requestId !== event.requestId) return;
        return seedState(context.resourceKey, event.agent, context.requestId);
      },
      reloadFailed: (context, event) => {
        if (context.phase !== "reloading" || context.requestId !== event.requestId) return;
        return { ...context, phase: "failed", error: event.error };
      },
    },
  });
}

function seedState(
  resourceKey: string,
  agent: AgentPayload | undefined,
  requestId: number
): AgentSettingsEditorReady {
  return {
    phase: "ready",
    resourceKey,
    requestId,
    baseline: agent ?? null,
    draft: agent ? buildSettingsDraftFromAgent(agent) : null,
  };
}

export function shouldAdoptAgentSettingsSource(
  context: AgentSettingsEditorState,
  agent: AgentPayload | undefined
): boolean {
  if (!agent || context.baseline?.definition_digest === agent.definition_digest) return false;
  return context.phase !== "saving" && context.phase !== "reloading" && !isDirty(context);
}

function isDirty(context: AgentSettingsEditorState): boolean {
  return Boolean(
    context.baseline && context.draft && isAgentSettingsDraftDirty(context.draft, context.baseline)
  );
}

function toReady(context: AgentSettingsEditorState): AgentSettingsEditorReady {
  const {
    phase: _phase,
    error: _error,
    ...ready
  } = context as AgentSettingsEditorState & {
    error?: string;
  };
  return { ...ready, phase: "ready" };
}

function isAgentPermissionDenied(error: unknown): boolean {
  return error instanceof AgentApiError && error.status === 403;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
