import { createStoreLogic } from "@xstate/store";
import { toast } from "@compozy/ui";

import { isAgentDigestConflict } from "../adapters/agent-api";
import type { AgentSettingsDraft } from "../lib/agent-settings-draft";
import type { AgentPayload } from "../types";
import type { RuntimeSpeed } from "@/lib/api-contract";
import type { RuntimeSelectorValue } from "@/systems/runtime";

/** The latest choice, shown while it saves. `value: null` is a pending return to the default. */
export interface RuntimeIntended {
  value: RuntimeSelectorValue | null;
  speed: RuntimeSpeed;
}

export interface RuntimeSaveFeedback {
  error?: string;
  conflict?: string;
  /**
   * After a conflict, picks wait for the server reread to bring the fresh digest;
   * a failed reread keeps them waiting until a retry succeeds.
   */
  reread?: "running" | "failed";
}

/** The owning hook's I/O: one digest-guarded update, and the conflict reread of the agent. */
export interface RuntimeSaveIO {
  save: (base: AgentPayload, patch: Partial<AgentSettingsDraft>) => Promise<AgentPayload>;
  reread: () => Promise<void>;
}

export interface RuntimeSaveEntry {
  intended: RuntimeIntended | null;
  saving: boolean;
  /** The choice made while a save runs; it applies on that save's result. */
  queued: Partial<AgentSettingsDraft> | null;
  /** What the latest choice changed, named in the toast once its save lands. */
  notice: string | null;
  feedback: RuntimeSaveFeedback | null;
  io: RuntimeSaveIO | null;
}

/** One entry per agent in one scope, so switching agents mid-save never mixes their choices. */
export interface RuntimeSaveContext {
  entries: Readonly<Record<string, RuntimeSaveEntry>>;
}

type RuntimeSaveEvents = {
  chosen: {
    owner: string;
    base: AgentPayload;
    intended: RuntimeIntended;
    patch: Partial<AgentSettingsDraft>;
    notice: string;
    io: RuntimeSaveIO;
  };
  saveSucceeded: { owner: string; agent: AgentPayload };
  saveFailed: { owner: string; error: unknown };
  rereadRequested: { owner: string; io: RuntimeSaveIO };
  rereadSettled: { owner: string; ok: boolean };
  dismissed: { owner: string };
};

export const RUNTIME_CONFLICT = "This agent changed somewhere else. Pick the model again.";
export const RUNTIME_REREAD_FAILED = "Couldn't load this agent's latest version.";

const IDLE: RuntimeSaveEntry = {
  intended: null,
  saving: false,
  queued: null,
  notice: null,
  feedback: null,
  io: null,
};

export function describeError(fallback: string, error: unknown): string {
  return error instanceof Error && error.message.trim().length > 0 ? error.message : fallback;
}

/** Picks wait while a return to the default saves, or until a conflict reread succeeds. */
export function runtimeSaveBlocked(entry: RuntimeSaveEntry | undefined): boolean {
  if (!entry) return false;
  return entry.feedback?.reread !== undefined || (entry.saving && entry.intended?.value === null);
}

function entryOf(context: RuntimeSaveContext, owner: string): RuntimeSaveEntry {
  return context.entries[owner] ?? IDLE;
}

function put(context: RuntimeSaveContext, owner: string, entry: RuntimeSaveEntry) {
  const { [owner]: _previous, ...rest } = context.entries;
  const idle = !entry.saving && entry.intended === null && entry.feedback === null;
  return { entries: idle ? rest : { ...rest, [owner]: entry } };
}

type Trigger = {
  saveSucceeded: (event: RuntimeSaveEvents["saveSucceeded"]) => void;
  saveFailed: (event: RuntimeSaveEvents["saveFailed"]) => void;
  rereadSettled: (event: RuntimeSaveEvents["rereadSettled"]) => void;
};

const runSave =
  (owner: string, io: RuntimeSaveIO, base: AgentPayload, patch: Partial<AgentSettingsDraft>) =>
  async ({ trigger }: { trigger: Trigger }) => {
    try {
      trigger.saveSucceeded({ owner, agent: await io.save(base, patch) });
    } catch (error) {
      trigger.saveFailed({ owner, error });
    }
  };

const runReread =
  (owner: string, io: RuntimeSaveIO) =>
  async ({ trigger }: { trigger: Trigger }) => {
    try {
      await io.reread();
      trigger.rereadSettled({ owner, ok: true });
    } catch {
      trigger.rereadSettled({ owner, ok: false });
    }
  };

/** Immediate runtime saves for the agent's Model row; I/O stays in the owning hook. */
export const agentRuntimeSaveLogic = createStoreLogic<RuntimeSaveContext, RuntimeSaveEvents>({
  context: { entries: {} },
  on: {
    chosen: (context, event, enqueue) => {
      const entry = entryOf(context, event.owner);
      if (runtimeSaveBlocked(entry)) return;
      const next = { ...entry, intended: event.intended, notice: event.notice, io: event.io };
      if (entry.saving) {
        return put(context, event.owner, { ...next, queued: { ...entry.queued, ...event.patch } });
      }
      enqueue.effect(runSave(event.owner, event.io, event.base, event.patch));
      return put(context, event.owner, { ...next, saving: true, queued: null, feedback: null });
    },
    saveSucceeded: (context, event, enqueue) => {
      const entry = entryOf(context, event.owner);
      if (!entry.saving) return;
      if (entry.queued && entry.io) {
        enqueue.effect(runSave(event.owner, entry.io, event.agent, entry.queued));
        return put(context, event.owner, { ...entry, queued: null });
      }
      const notice = entry.notice ?? "Model updated";
      enqueue.effect(() => void toast.success(notice));
      return put(context, event.owner, { ...entry, saving: false, intended: null, notice: null });
    },
    saveFailed: (context, event, enqueue) => {
      const entry = entryOf(context, event.owner);
      if (!entry.saving) return;
      // A choice waiting on a definition that didn't save is dropped with it.
      const settled = { ...entry, saving: false, intended: null, queued: null, notice: null };
      if (isAgentDigestConflict(event.error) && entry.io) {
        enqueue.effect(runReread(event.owner, entry.io));
        return put(context, event.owner, {
          ...settled,
          feedback: { conflict: RUNTIME_CONFLICT, reread: "running" },
        });
      }
      const error = describeError("Couldn't update the model", event.error);
      return put(context, event.owner, { ...settled, feedback: { error } });
    },
    rereadRequested: (context, event, enqueue) => {
      const entry = entryOf(context, event.owner);
      if (entry.feedback?.reread !== "failed") return;
      enqueue.effect(runReread(event.owner, event.io));
      return put(context, event.owner, {
        ...entry,
        io: event.io,
        feedback: { conflict: RUNTIME_CONFLICT, reread: "running" },
      });
    },
    rereadSettled: (context, event) => {
      const entry = entryOf(context, event.owner);
      if (entry.feedback?.reread !== "running") return;
      const feedback: RuntimeSaveFeedback = event.ok
        ? { conflict: RUNTIME_CONFLICT }
        : { error: RUNTIME_REREAD_FAILED, reread: "failed" };
      return put(context, event.owner, { ...entry, feedback });
    },
    dismissed: (context, event) => {
      const entry = entryOf(context, event.owner);
      if (entry.feedback?.reread !== undefined) return;
      return put(context, event.owner, { ...entry, feedback: null });
    },
  },
});
