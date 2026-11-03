import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { hasAgentRuntimeOverride, resolveAgentRuntimeValue } from "../lib/agent-effective-runtime";
import {
  buildSettingsDraftFromAgent,
  buildUpdateAgentParams,
  type AgentSettingsDraft,
} from "../lib/agent-settings-draft";
import { agentKeys } from "../lib/query-keys";
import {
  agentRuntimeSaveLogic,
  runtimeSaveBlocked,
  type RuntimeIntended,
  type RuntimeSaveIO,
} from "../stores/agent-runtime-save-store";
import type { AgentPayload } from "../types";
import { useAgentWindowScope } from "./use-agent-window-scope";
import { useUpdateAgent } from "./use-agents";
import { type RuntimeSpeed } from "@/lib/api-contract";
import type { RuntimeSelectorValue } from "@/systems/runtime";
import { useProfileReadScope } from "@/systems/profiles";

export { describeError } from "../stores/agent-runtime-save-store";

const DEFAULT_RUNTIME: Partial<AgentSettingsDraft> = {
  provider: "",
  model: "",
  reasoningEffort: "",
  speed: "",
  acpOptions: [],
};

const SETUP_NEEDS_FIXING = "This agent's setup needs fixing before it can change.";

/** What the row shows: the latest choice while it saves, else server truth. */
function runtimeView(agent: AgentPayload | undefined, pending: RuntimeIntended | null) {
  const value: RuntimeSelectorValue = pending?.value ?? resolveAgentRuntimeValue(agent);
  const speed: RuntimeSpeed =
    pending?.speed ?? agent?.effective_runtime?.speed ?? agent?.speed ?? "normal";
  // A pending pick overrides; a pending return to the default keeps the saved state until it lands.
  if (pending?.value) return { value, speed, overridden: true };
  const overridden = agent ? hasAgentRuntimeOverride(buildSettingsDraftFromAgent(agent)) : false;
  return { value, speed, overridden };
}

const speedNotice = (next: RuntimeSpeed) =>
  next === "fast" ? "Fast turned on" : "Fast turned off";

/**
 * Immediate saves for the agent's runtime. The save flow lives in `agentRuntimeSaveLogic`; this
 * hook supplies its I/O (the digest-guarded update and the conflict reread) and the view.
 */
export function useAgentRuntimeSave(agent: AgentPayload | undefined) {
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  // The window's scope: saves go to its project, the cache write to the key `useAgent` reads.
  const { workspace: cacheWorkspace, projectId: workspaceId } = useAgentWindowScope();
  const updateAgent = useUpdateAgent();
  const store = useStore(agentRuntimeSaveLogic);
  // Every save, queued choice and message belongs to one agent in one scope.
  const key = JSON.stringify([destination, cacheWorkspace, agent?.name ?? ""]);
  const entry = useSelector(store, snapshot => snapshot.context.entries[key]);
  const { value, speed, overridden } = runtimeView(agent, entry?.intended ?? null);
  const feedback = entry?.feedback ?? null;

  const io = (name: string): RuntimeSaveIO => {
    const detailKey = agentKeys.detail(name, cacheWorkspace, destination);
    return {
      save: (base, patch) => {
        const params = buildUpdateAgentParams(
          { ...buildSettingsDraftFromAgent(base), ...patch },
          workspaceId
        );
        if (!params) return Promise.reject(new Error(SETUP_NEEDS_FIXING));
        return updateAgent.mutateAsync({ name, params, cacheWorkspace, profile: destination });
      },
      // Refetch the exact view key only. Do not write the rejected choice into cache:
      // invalidate+refetch restores server truth under the same key useAgent reads.
      reread: async () => {
        await queryClient.invalidateQueries({ queryKey: detailKey });
        await queryClient.refetchQueries(
          { queryKey: detailKey, exact: true },
          { throwOnError: true }
        );
      },
    };
  };

  const choose = (
    intended: RuntimeIntended,
    patch: Partial<AgentSettingsDraft>,
    notice: string
  ) => {
    if (!agent) return;
    store.trigger.chosen({ owner: key, base: agent, intended, patch, notice, io: io(agent.name) });
  };

  const onChange = (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) =>
    choose(
      { value: next, speed: normalizedSpeed ?? speed },
      {
        provider: next.provider,
        model: next.model ?? "",
        reasoningEffort: next.reasoning_effort,
        acpOptions: next.acp_options ?? [],
        ...(normalizedSpeed ? { speed: normalizedSpeed } : {}),
      },
      "Model updated"
    );

  const onReload = () => {
    if (agent) store.trigger.rereadRequested({ owner: key, io: io(agent.name) });
  };

  return {
    value,
    speed,
    overridden,
    onChange,
    onSpeedChange: (next: RuntimeSpeed) =>
      choose({ value, speed: next }, { speed: next }, speedNotice(next)),
    // The server resolves the default's speed, so the current one stays until the reread.
    onUseDefault: () => choose({ value: null, speed }, DEFAULT_RUNTIME, "Back to your default"),
    /** A return to the default or a conflict reread is running or failed; picks wait for it. */
    isPending: runtimeSaveBlocked(entry),
    error: feedback?.error ?? null,
    conflictMessage: feedback?.conflict ?? null,
    /** Retries a failed conflict reread; null when nothing needs it. */
    onReload: feedback?.reread === "failed" ? onReload : null,
    onDismissError: () => store.trigger.dismissed({ owner: key }),
  };
}
