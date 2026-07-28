import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import type { EntityMode } from "@compozy/ui";

import {
  ADVANCED_DEFAULTS,
  applySessionAgentSelection,
  EMPTY_SESSION_CREATE_DRAFT,
  RUNTIME_OVERRIDE_DEFAULTS,
  type SessionCreateDialogDraft,
} from "../lib/session-create-draft";
import type { SessionPayload } from "../types";

interface SessionCreateNavigationTarget {
  attempt: number;
  session: SessionPayload;
}

interface SessionCreateStoreContext {
  open: boolean;
  mode: EntityMode;
  draft: SessionCreateDialogDraft;
  submitError: string | null;
  isSubmitting: boolean;
  attempt: number;
  navigationAttempt: number | null;
  navigationTarget: SessionCreateNavigationTarget | null;
  pendingAgentName: string | null;
  pendingWorkspaceId: string | null;
}

type SessionCreateStoreEvents = {
  agentSelected: { agentName: string; workspaceId: string };
  dialogOpened: { agentName: string; workspaceId: string };
  dialogOpenChanged: { open: boolean };
  modeSelected: { mode: EntityMode };
  navigationCompleted: { attempt: number };
  navigationFailed: { attempt: number; message: string };
  navigationRequested: { attempt: number; execute: () => Promise<void> };
  networkParticipationSelected: Pick<
    SessionCreateDialogDraft,
    "networkParticipationMode" | "networkChannelId" | "networkChannelStrategy"
  >;
  promptChanged: { prompt: string };
  providerSettingsRequested: {};
  runtimeSelected: Pick<
    SessionCreateDialogDraft,
    "providerOverride" | "modelOverride" | "reasoningEffort"
  >;
  sessionNameChanged: { sessionName: string };
  submissionFailed: { attempt: number; message: string };
  submissionRequested: {
    agentName: string;
    execute: () => Promise<SessionPayload>;
    workspaceId: string;
  };
  sessionCreated: { attempt: number; session: SessionPayload };
  validationFailed: { message: string };
  workspacePathChanged: { workspacePath: string };
  workspaceSelected: { workspaceId: string };
};

export const sessionCreateStoreLogic = createStoreLogic<
  SessionCreateStoreContext,
  SessionCreateStoreEvents
>({
  context: {
    open: false,
    mode: "simple",
    draft: EMPTY_SESSION_CREATE_DRAFT,
    submitError: null,
    isSubmitting: false,
    attempt: 0,
    navigationAttempt: null,
    navigationTarget: null,
    pendingAgentName: null,
    pendingWorkspaceId: null,
  },
  on: {
    dialogOpened: (context, event) => ({
      ...context,
      open: true,
      mode: "simple",
      draft: applySessionAgentSelection(context.draft, event.agentName, event.workspaceId),
      navigationAttempt: null,
      navigationTarget: null,
      submitError: null,
    }),
    dialogOpenChanged: (context, event) => {
      if (context.isSubmitting && !event.open) return;
      return {
        ...context,
        open: event.open,
        navigationAttempt: event.open ? null : context.navigationAttempt,
        navigationTarget: event.open ? null : context.navigationTarget,
        submitError: event.open ? context.submitError : null,
      };
    },
    modeSelected: (context, event) => ({
      ...context,
      mode: event.mode,
      draft: event.mode === "advanced" ? context.draft : { ...context.draft, ...ADVANCED_DEFAULTS },
      submitError: null,
    }),
    workspaceSelected: (context, event) => ({
      ...context,
      draft: {
        ...context.draft,
        workspaceId: event.workspaceId,
        agentName: "",
        prompt: "",
        ...RUNTIME_OVERRIDE_DEFAULTS,
        ...ADVANCED_DEFAULTS,
      },
      submitError: null,
    }),
    agentSelected: (context, event) => ({
      ...context,
      draft: {
        ...applySessionAgentSelection(context.draft, event.agentName, event.workspaceId),
        sessionName: context.draft.sessionName,
      },
      submitError: null,
    }),
    sessionNameChanged: (context, event) => ({
      ...context,
      draft: { ...context.draft, sessionName: event.sessionName },
    }),
    workspacePathChanged: (context, event) => ({
      ...context,
      draft: { ...context.draft, workspacePath: event.workspacePath },
    }),
    promptChanged: (context, event) => ({
      ...context,
      draft: { ...context.draft, prompt: event.prompt },
      submitError: null,
    }),
    runtimeSelected: (context, event) => ({
      ...context,
      draft: { ...context.draft, ...event },
      submitError: null,
    }),
    networkParticipationSelected: (context, event) => ({
      ...context,
      draft: { ...context.draft, ...event },
      submitError: null,
    }),
    providerSettingsRequested: context =>
      context.isSubmitting ? undefined : { ...context, open: false },
    validationFailed: (context, event) => ({
      ...context,
      submitError: event.message,
    }),
    submissionRequested: (context, event, enqueue) => {
      if (context.isSubmitting) return;
      const attempt = context.attempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.sessionCreated({ attempt, session: await event.execute() });
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
        isSubmitting: true,
        navigationAttempt: null,
        navigationTarget: null,
        pendingAgentName: event.agentName,
        pendingWorkspaceId: event.workspaceId,
        submitError: null,
      };
    },
    submissionFailed: (context, event, enqueue) => {
      if (!context.isSubmitting || context.attempt !== event.attempt) return;
      enqueue.effect(() => notifyUser({ message: event.message, tone: "error" }));
      return {
        ...context,
        isSubmitting: false,
        pendingAgentName: null,
        pendingWorkspaceId: null,
        submitError: event.message,
      };
    },
    sessionCreated: (context, event) =>
      context.isSubmitting && context.attempt === event.attempt
        ? {
            ...context,
            open: false,
            mode: "simple",
            draft: EMPTY_SESSION_CREATE_DRAFT,
            isSubmitting: false,
            navigationTarget: { attempt: event.attempt, session: event.session },
            pendingAgentName: null,
            pendingWorkspaceId: null,
          }
        : undefined,
    navigationRequested: (context, event, enqueue) => {
      if (context.navigationTarget?.attempt !== event.attempt) return;
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
      return { ...context, navigationAttempt: event.attempt, navigationTarget: null };
    },
    navigationCompleted: (context, event) =>
      context.navigationAttempt === event.attempt
        ? { ...context, navigationAttempt: null }
        : undefined,
    navigationFailed: (context, event, enqueue) => {
      if (context.navigationAttempt !== event.attempt) return;
      enqueue.effect(() => notifyUser({ message: event.message, tone: "error" }));
      return { ...context, navigationAttempt: null, submitError: event.message };
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
