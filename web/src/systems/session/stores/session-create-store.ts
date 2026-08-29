import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";
import type { TerminalQuote } from "@/systems/terminal/parts";

import type { EntityMode } from "@compozy/ui";

import {
  ADVANCED_DEFAULTS,
  applySessionAgentSelection,
  DEFAULT_SESSION_AGENT_NAME,
  EMPTY_SESSION_CREATE_DRAFT,
  type SessionCreateDialogDraft,
} from "../lib/session-create-draft";
import type { SessionEnvironmentTarget } from "../lib/session-environment-target";
import type { CreateSessionParams, SessionPayload } from "../types";

interface SessionCreateNavigationTarget {
  attempt: number;
  session: SessionPayload;
}

type SessionCreateOperation =
  | { status: "idle" }
  | { agentName: string; attempt: number; status: "submitting"; workspaceId: string }
  | { status: "navigation-pending"; target: SessionCreateNavigationTarget }
  | { attempt: number; status: "navigating" };

/**
 * A submit held back until the chosen environment exists.
 *
 * It carries the whole request the operator already approved — only the worktree
 * id is still unknown — so resuming it needs nothing from the live draft, which
 * the operator remains free to edit while the checkout is built.
 */
interface SessionCreatePendingSubmit {
  agentName: string;
  workspaceId: string;
  pendingPrompt: string | null;
  /** Quote claimed when this attempt was armed — not the live pending slot. */
  terminalQuote: TerminalQuote | null;
  previousEnvironment: SessionEnvironmentTarget;
  request: CreateSessionParams;
}

interface SessionCreateStoreContext {
  open: boolean;
  restoreFocusOnClose: boolean;
  mode: EntityMode;
  draft: SessionCreateDialogDraft;
  submitError: string | null;
  attempt: number;
  operation: SessionCreateOperation;
  /** Non-null while a worktree is materializing for a submit the operator already asked for. */
  pendingSubmit: SessionCreatePendingSubmit | null;
  /** Prompt already sent from the composer while the fallback waits for agent selection. */
  pendingPrompt: string | null;
}

type SessionCreateStoreEvents = {
  agentSelected: { agentName: string; workspaceId: string };
  /** The submit is waiting on the environment; the dialog stays open and editable. */
  environmentAwaited: SessionCreatePendingSubmit;
  /** The environment stopped being pending — ready, failed, or canceled. */
  environmentSettled: {};
  fallbackPromptStaged: { prompt: string };
  dialogOpened: {
    agentName: string;
    workspaceId: string;
    /** Optional composer prompt supplied by the surface that opened creation. */
    pendingPrompt?: string;
    /** Preselects where the session will run; omitted opens at the workspace root. */
    environment?: SessionEnvironmentTarget;
  };
  dialogOpenChanged: { open: boolean };
  modeSelected: { mode: EntityMode };
  navigationCompleted: { attempt: number };
  navigationFailed: { attempt: number; message: string };
  navigationRequested: { attempt: number; execute: () => Promise<void> };
  networkParticipationSelected: Pick<
    SessionCreateDialogDraft,
    "networkParticipationMode" | "networkChannelId" | "networkChannelStrategy"
  >;
  environmentSelected: { environment: SessionEnvironmentTarget };
  environmentRestored: { environment: SessionEnvironmentTarget };
  sessionNameChanged: { sessionName: string };
  submissionFailed: { attempt: number; message: string };
  submissionRequested: {
    agentName: string;
    execute: () => Promise<SessionPayload>;
    navigate: (session: SessionPayload) => Promise<void>;
    workspaceId: string;
  };
  sessionCreated: { attempt: number; session: SessionPayload };
  /**
   * Palette prompt fallback: same single-flight submitting state as the dialog,
   * without opening it or asking the dialog to navigate.
   */
  fallbackRequested: { agentName: string; workspaceId: string };
  fallbackCompleted: { attempt: number };
  validationFailed: { message: string };
};

export const sessionCreateStoreLogic = createStoreLogic<
  SessionCreateStoreContext,
  SessionCreateStoreEvents
>({
  context: {
    open: false,
    restoreFocusOnClose: true,
    mode: "simple",
    draft: EMPTY_SESSION_CREATE_DRAFT,
    submitError: null,
    attempt: 0,
    operation: { status: "idle" },
    pendingSubmit: null,
    pendingPrompt: null,
  },
  on: {
    dialogOpened: (context, event) => {
      if (context.operation.status === "submitting") return;
      const agentName = event.agentName.trim() || DEFAULT_SESSION_AGENT_NAME;
      return {
        ...context,
        open: true,
        restoreFocusOnClose: true,
        pendingSubmit: null,
        pendingPrompt: event.pendingPrompt ?? null,
        // A preselected environment lives in Advanced, so opening there is the
        // only way the operator can see what was chosen for them.
        mode: event.environment ? "advanced" : "simple",
        draft: {
          ...applySessionAgentSelection(context.draft, agentName, event.workspaceId),
          ...(event.environment ? { environment: event.environment } : {}),
        },
        operation: { status: "idle" },
        submitError: null,
      };
    },
    dialogOpenChanged: (context, event) => {
      if (context.operation.status === "submitting") return;
      return {
        ...context,
        open: event.open,
        restoreFocusOnClose: true,
        operation: event.open ? { status: "idle" } : context.operation,
        pendingSubmit: null,
        pendingPrompt: event.open ? context.pendingPrompt : null,
        submitError: event.open ? context.submitError : null,
      };
    },
    modeSelected: (context, event) => ({
      ...context,
      mode: event.mode,
      draft: event.mode === "advanced" ? context.draft : { ...context.draft, ...ADVANCED_DEFAULTS },
      submitError: null,
    }),
    agentSelected: (context, event) => ({
      ...context,
      draft: {
        ...applySessionAgentSelection(
          context.draft,
          event.agentName.trim() || DEFAULT_SESSION_AGENT_NAME,
          event.workspaceId
        ),
        sessionName: context.draft.sessionName,
      },
      submitError: null,
    }),
    sessionNameChanged: (context, event) => ({
      ...context,
      draft: { ...context.draft, sessionName: event.sessionName },
    }),
    fallbackPromptStaged: (context, event) => ({
      ...context,
      pendingPrompt: event.prompt,
    }),
    environmentSelected: (context, event) => ({
      ...context,
      draft: { ...context.draft, environment: event.environment },
      // Changing where the session runs retracts the submit that was waiting on
      // the previous choice.
      pendingSubmit: null,
      submitError: null,
    }),
    environmentRestored: (context, event) => ({
      ...context,
      draft: { ...context.draft, environment: event.environment },
      pendingSubmit: null,
      submitError: null,
    }),
    environmentAwaited: (context, event) =>
      context.operation.status === "idle"
        ? { ...context, pendingSubmit: event, submitError: null }
        : undefined,
    environmentSettled: context =>
      context.pendingSubmit === null ? undefined : { ...context, pendingSubmit: null },
    networkParticipationSelected: (context, event) => ({
      ...context,
      draft: { ...context.draft, ...event },
      submitError: null,
    }),
    validationFailed: (context, event) => ({
      ...context,
      submitError: event.message,
    }),
    submissionRequested: (context, event, enqueue) => {
      if (context.operation.status !== "idle") return;
      const attempt = context.attempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          const session = await event.execute();
          trigger.sessionCreated({ attempt, session });
          trigger.navigationRequested({ attempt, execute: () => event.navigate(session) });
        } catch (cause) {
          trigger.submissionFailed({
            attempt,
            message: errorMessage(cause, "Failed to create session."),
          });
        }
      });
      return {
        ...context,
        attempt,
        operation: {
          agentName: event.agentName,
          attempt,
          status: "submitting",
          workspaceId: event.workspaceId,
        },
        pendingSubmit: null,
        submitError: null,
      };
    },
    submissionFailed: (context, event, enqueue) => {
      if (
        context.operation.status !== "submitting" ||
        context.operation.attempt !== event.attempt
      ) {
        return;
      }
      enqueue.effect(() => notifyUser({ message: event.message, tone: "error" }));
      return {
        ...context,
        operation: { status: "idle" },
        pendingSubmit: null,
        submitError: event.message,
      };
    },
    sessionCreated: (context, event) => {
      if (
        context.operation.status !== "submitting" ||
        context.operation.attempt !== event.attempt
      ) {
        return;
      }
      return {
        ...context,
        open: false,
        restoreFocusOnClose: false,
        mode: "simple",
        draft: EMPTY_SESSION_CREATE_DRAFT,
        operation: {
          status: "navigation-pending",
          target: { attempt: event.attempt, session: event.session },
        },
        pendingSubmit: null,
        pendingPrompt: null,
      };
    },
    fallbackRequested: (context, event) => {
      if (context.operation.status !== "idle") return;
      const attempt = context.attempt + 1;
      return {
        ...context,
        attempt,
        operation: {
          agentName: event.agentName,
          attempt,
          status: "submitting",
          workspaceId: event.workspaceId,
        },
        pendingSubmit: null,
        submitError: null,
      };
    },
    fallbackCompleted: (context, event) => {
      if (
        context.operation.status !== "submitting" ||
        context.operation.attempt !== event.attempt
      ) {
        return;
      }
      return { ...context, operation: { status: "idle" }, pendingPrompt: null };
    },
    navigationRequested: (context, event, enqueue) => {
      if (
        context.operation.status !== "navigation-pending" ||
        context.operation.target.attempt !== event.attempt
      ) {
        return;
      }
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.navigationCompleted({ attempt: event.attempt });
        } catch (cause) {
          trigger.navigationFailed({
            attempt: event.attempt,
            message: errorMessage(cause, "Failed to open the created session."),
          });
        }
      });
      return { ...context, operation: { attempt: event.attempt, status: "navigating" } };
    },
    navigationCompleted: (context, event) =>
      context.operation.status === "navigating" && context.operation.attempt === event.attempt
        ? { ...context, operation: { status: "idle" } }
        : undefined,
    navigationFailed: (context, event, enqueue) => {
      if (
        context.operation.status !== "navigating" ||
        context.operation.attempt !== event.attempt
      ) {
        return;
      }
      enqueue.effect(() => notifyUser({ message: event.message, tone: "error" }));
      return { ...context, operation: { status: "idle" }, submitError: event.message };
    },
  },
});

export type SessionCreateStore = ReturnType<typeof sessionCreateStoreLogic.createStore>;

export function createSessionCreateStore(): SessionCreateStore {
  return sessionCreateStoreLogic.createStore();
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
