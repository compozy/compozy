import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";
import type { AgentPayload } from "../types";

type AgentCreateMode = { kind: "create" } | { kind: "duplicate"; source: AgentPayload };

interface AgentCreateFlowBase {
  draft: AgentCreateDialogDraft;
  nextAttempt: number;
}

type AgentCreateFlow =
  | (AgentCreateFlowBase & { status: "closed" })
  | (AgentCreateFlowBase &
      AgentCreateMode &
      (
        | { status: "editing" }
        | { status: "submitting"; attempt: number }
        | { status: "failed"; error: string }
        | { status: "navigation-pending"; attempt: number; agentName: string }
        | { status: "navigating"; attempt: number; agentName: string }
      ));

type AgentCreateDialogEvents = {
  createOpened: { draft: AgentCreateDialogDraft };
  dialogDismissed: { draft: AgentCreateDialogDraft };
  draftChanged: { draft: AgentCreateDialogDraft };
  duplicateOpened: { draft: AgentCreateDialogDraft; source: AgentPayload };
  navigationCompleted: { attempt: number; draft: AgentCreateDialogDraft };
  navigationFailed: { attempt: number; error: string };
  navigationRequested: {
    attempt: number;
    draft: AgentCreateDialogDraft;
    execute: () => Promise<void>;
  };
  submissionFailed: { attempt: number; error: string };
  submissionRejected: { error: string };
  submissionRequested: {
    execute: () => Promise<AgentPayload>;
    failureMessage: string;
  };
  submissionSucceeded: { agentName: string; attempt: number };
};

export interface AgentCreateDialogLogicInput {
  initialDraft: AgentCreateDialogDraft;
}

function editingFlow(
  draft: AgentCreateDialogDraft,
  nextAttempt: number,
  mode: AgentCreateMode
): Extract<AgentCreateFlow, { status: "editing" }> {
  return { ...mode, draft, nextAttempt, status: "editing" };
}

function failedFlow(
  context: Exclude<AgentCreateFlow, { status: "closed" }>,
  error: string
): Extract<AgentCreateFlow, { status: "failed" }> {
  const base = {
    draft: context.draft,
    error,
    nextAttempt: context.nextAttempt,
    status: "failed" as const,
  };
  return context.kind === "duplicate"
    ? { ...base, kind: "duplicate", source: context.source }
    : { ...base, kind: "create" };
}

export const agentCreateDialogLogic = createStoreLogic<
  AgentCreateFlow,
  AgentCreateDialogEvents,
  Record<never, never>,
  AgentCreateDialogLogicInput
>({
  context: input => ({
    draft: input.initialDraft,
    nextAttempt: 0,
    status: "closed",
  }),
  on: {
    createOpened: (context, event) =>
      editingFlow(event.draft, context.nextAttempt, { kind: "create" }),
    duplicateOpened: (context, event) =>
      editingFlow(event.draft, context.nextAttempt, { kind: "duplicate", source: event.source }),
    dialogDismissed: (context, event) => ({
      draft: event.draft,
      nextAttempt: context.nextAttempt,
      status: "closed",
    }),
    draftChanged: (context, event) => {
      if (context.status !== "editing" && context.status !== "failed") return;
      const flow = editingFlow(event.draft, context.nextAttempt, context);
      return flow;
    },
    submissionRejected: (context, event) => {
      if (context.status !== "editing" && context.status !== "failed") return;
      return failedFlow(context, event.error);
    },
    submissionRequested: (context, event, enqueue) => {
      if (context.status !== "editing" && context.status !== "failed") return;
      const attempt = context.nextAttempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          const agent = await event.execute();
          trigger.submissionSucceeded({ agentName: agent.name, attempt });
        } catch (cause) {
          trigger.submissionFailed({
            attempt,
            error: errorMessage(cause, event.failureMessage),
          });
        }
      });
      return { ...context, attempt, nextAttempt: attempt, status: "submitting" };
    },
    submissionSucceeded: (context, event) => {
      if (context.status !== "submitting" || context.attempt !== event.attempt) return;
      return { ...context, agentName: event.agentName, status: "navigation-pending" };
    },
    submissionFailed: (context, event, enqueue) => {
      if (context.status !== "submitting" || context.attempt !== event.attempt) return;
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
      return failedFlow(context, event.error);
    },
    navigationRequested: (context, event, enqueue) => {
      if (context.status !== "navigation-pending" || context.attempt !== event.attempt) return;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.navigationCompleted({ attempt: event.attempt, draft: event.draft });
        } catch (error) {
          trigger.navigationFailed({
            attempt: event.attempt,
            error: errorMessage(error, "Failed to open the created agent."),
          });
        }
      });
      return { ...context, status: "navigating" };
    },
    navigationCompleted: (context, event) => {
      if (context.status !== "navigating" || context.attempt !== event.attempt) return;
      return {
        draft: event.draft,
        nextAttempt: context.nextAttempt,
        status: "closed",
      };
    },
    navigationFailed: (context, event, enqueue) => {
      if (context.status !== "navigating" || context.attempt !== event.attempt) return;
      enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
      return {
        draft: context.draft,
        nextAttempt: context.nextAttempt,
        status: "closed",
      };
    },
  },
});

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim().length > 0 ? error.message : fallback;
}

export type { AgentCreateFlow, AgentCreateMode };
