import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { isAgentDigestConflict } from "../adapters/agent-api";
import { hasAgentRuntimeOverride, resolveAgentRuntimeValue } from "../lib/agent-effective-runtime";
import {
  buildSettingsDraftFromAgent,
  buildUpdateAgentParams,
  type AgentSettingsDraft,
} from "../lib/agent-settings-draft";
import { agentKeys } from "../lib/query-keys";
import type { AgentPayload } from "../types";
import { useAgentWindowScope } from "./use-agent-window-scope";
import { useUpdateAgent } from "./use-agents";
import { type RuntimeSpeed } from "@/lib/api-contract";
import type { RuntimeSelectorValue } from "@/systems/runtime";
import { useProfileReadScope } from "@/systems/profiles";

interface Feedback {
  key: string;
  error?: string;
  conflict?: string;
  /** The conflict's server reread is still running; picks wait for the fresh digest. */
  refreshing?: boolean;
}

interface Intended {
  value: RuntimeSelectorValue | null;
  speed: RuntimeSpeed;
}

const DEFAULT_RUNTIME: Partial<AgentSettingsDraft> = {
  provider: "",
  model: "",
  reasoningEffort: "",
  speed: "",
  acpOptions: [],
};

export function describeError(fallback: string, error: unknown): string {
  return error instanceof Error && error.message.trim().length > 0 ? error.message : fallback;
}

/**
 * Immediate saves for the agent's runtime. Shows the latest choice while it saves; a choice made
 * during a save applies on the saved definition of the same agent, and a failed save restores
 * server truth.
 */
export function useAgentRuntimeSave(agent: AgentPayload | undefined) {
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  // The window's scope: saves go to its project, the cache write to the key `useAgent` reads.
  const { workspace: cacheWorkspace, projectId: workspaceId } = useAgentWindowScope();
  const updateAgent = useUpdateAgent();
  // Every save, queued choice and message belongs to one agent in one scope, so switching
  // agents while a save runs never lands a choice, or its error, on the other agent.
  const key = JSON.stringify([destination, cacheWorkspace, agent?.name ?? ""]);
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  // The latest choice, shown while its save runs so the next control builds on it.
  // `value: null` is a pending return to the default. One entry per agent with a save running.
  const [intended, setIntended] = useState<Readonly<Record<string, Intended>>>({});
  // Agents with a save in flight, each with the choice made meanwhile, if any.
  const saving = useRef(new Map<string, Partial<AgentSettingsDraft> | null>());
  // What the latest choice for each agent changed, named in the toast once its save lands.
  const notice = useRef(new Map<string, string>());

  const pending = intended[key] ?? null;
  const serverValue = resolveAgentRuntimeValue(agent);
  const value: RuntimeSelectorValue = pending?.value ?? serverValue;
  const speed = pending?.speed ?? agent?.effective_runtime?.speed ?? agent?.speed ?? "normal";
  const overridden = pending
    ? pending.value !== null
    : agent
      ? hasAgentRuntimeOverride(buildSettingsDraftFromAgent(agent))
      : false;
  const shown = feedback?.key === key ? feedback : null;

  const settle = (owner: string) =>
    setIntended(current => {
      if (!(owner in current)) return current;
      const { [owner]: _settled, ...rest } = current;
      return rest;
    });

  const save = (owner: string, patch: Partial<AgentSettingsDraft>, base: AgentPayload) => {
    if (saving.current.has(owner)) {
      saving.current.set(owner, { ...saving.current.get(owner), ...patch });
      return;
    }
    const params = buildUpdateAgentParams(
      { ...buildSettingsDraftFromAgent(base), ...patch },
      workspaceId
    );
    if (!params) {
      settle(owner);
      setFeedback({ key: owner, error: "This agent's setup needs fixing before it can change." });
      return;
    }

    setFeedback(current => (current?.key === owner ? null : current));
    const detailKey = agentKeys.detail(base.name, cacheWorkspace, destination);
    saving.current.set(owner, null);
    updateAgent.mutateAsync({ name: base.name, params, cacheWorkspace, profile: destination }).then(
      saved => {
        const next = saving.current.get(owner);
        saving.current.delete(owner);
        if (next) {
          save(owner, next, saved);
          return;
        }
        settle(owner);
        toast.success(notice.current.get(owner) ?? "Model updated");
        notice.current.delete(owner);
      },
      (mutationError: unknown) => {
        // A choice waiting on a definition that didn't save is dropped with it.
        saving.current.delete(owner);
        settle(owner);
        if (isAgentDigestConflict(mutationError)) {
          // Refetch the exact view key only. Do not write the rejected choice
          // into cache — mutation onSuccess never ran, and invalidate+refetch
          // restores server truth under the same key useAgent reads.
          const conflict = "This agent changed somewhere else. Pick the model again.";
          setFeedback({ key: owner, conflict, refreshing: true });
          void queryClient
            .invalidateQueries({ queryKey: detailKey })
            .then(() => queryClient.refetchQueries({ queryKey: detailKey }))
            .finally(() => {
              setFeedback(current =>
                current?.key === owner && current.refreshing ? { key: owner, conflict } : current
              );
            });
          return;
        }
        setFeedback({
          key: owner,
          error: describeError("Couldn't update the model", mutationError),
        });
      }
    );
  };

  const choose = (
    next: Intended["value"],
    nextSpeed: RuntimeSpeed,
    patch: Partial<AgentSettingsDraft>,
    message: string
  ) => {
    if (!agent) return;
    notice.current.set(key, message);
    setIntended(current => ({ ...current, [key]: { value: next, speed: nextSpeed } }));
    save(key, patch, agent);
  };

  const onChange = (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) =>
    choose(
      next,
      normalizedSpeed ?? speed,
      {
        provider: next.provider,
        model: next.model ?? "",
        reasoningEffort: next.reasoning_effort,
        acpOptions: next.acp_options ?? [],
        ...(normalizedSpeed ? { speed: normalizedSpeed } : {}),
      },
      "Model updated"
    );

  return {
    value,
    speed,
    overridden,
    onChange,
    onSpeedChange: (next: RuntimeSpeed) =>
      choose(value, next, { speed: next }, next === "fast" ? "Fast turned on" : "Fast turned off"),
    // The server resolves the default's speed, so the current one stays until the reread.
    onUseDefault: () => choose(null, speed, DEFAULT_RUNTIME, "Back to your default"),
    /** A return to the default or a conflict reread is running; the selector waits for it. */
    isPending: (pending !== null && pending.value === null) || Boolean(shown?.refreshing),
    error: shown?.error ?? null,
    conflictMessage: shown?.conflict ?? null,
    onDismissError: () => setFeedback(null),
  };
}
