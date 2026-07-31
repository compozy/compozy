import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import type {
  SettingsMCPAuthBeginMode,
  SettingsMCPAuthBeginResponse,
  SettingsMCPAuthExchangeRequest,
  SettingsMCPAuthFilter,
  SettingsMCPAuthStatusResponse,
} from "../types";
import {
  isAbsoluteMCPRedirectURL,
  normalizeMCPAuthScopes,
  normalizeMCPRedirectURL,
} from "../lib/mcp-authorize-model";

export interface MCPAuthorizePriorStatus {
  status: string;
  tokenPresent: boolean;
}

/** Scope elevation may only be sent after an explicit operator confirmation. */
export interface MCPAuthorizeScopeApproval {
  approvedScopes: string[];
  approveScopeEscalation: boolean;
}

interface MCPAuthorizeIdleState {
  phase: "idle";
  attemptId: number;
}

interface MCPAuthorizeAttempt {
  attemptId: number;
  dismissOnConfirmation: boolean;
  server: string;
  filter: SettingsMCPAuthFilter;
  prior: MCPAuthorizePriorStatus;
  mode: SettingsMCPAuthBeginMode;
  scopeApproval?: MCPAuthorizeScopeApproval;
}

interface MCPAuthorizeBeginningState extends MCPAuthorizeAttempt {
  phase: "beginning";
}

interface MCPAuthorizeScopeReviewState extends MCPAuthorizeAttempt {
  phase: "reviewing_scopes";
  scopeApproval: MCPAuthorizeScopeApproval;
}

interface MCPAuthorizeWaitingState extends MCPAuthorizeAttempt {
  phase: "waiting";
  begin: SettingsMCPAuthBeginResponse;
}

interface MCPAuthorizeManualState extends MCPAuthorizeAttempt {
  phase: "manual";
  begin: SettingsMCPAuthBeginResponse;
}

interface MCPAuthorizeExchangingState extends MCPAuthorizeAttempt {
  phase: "exchanging";
  begin: SettingsMCPAuthBeginResponse;
}

interface MCPAuthorizeConfirmedState extends MCPAuthorizeAttempt {
  phase: "confirmed";
  begin: SettingsMCPAuthBeginResponse;
}

interface MCPAuthorizeBeginFailedState extends MCPAuthorizeAttempt {
  phase: "failed";
  stage: "begin";
  error: string;
}

interface MCPAuthorizeCompletionFailedState extends MCPAuthorizeAttempt {
  phase: "failed";
  stage: "completion";
  begin: SettingsMCPAuthBeginResponse;
  error: string;
}

export type MCPAuthorizeState =
  | MCPAuthorizeIdleState
  | MCPAuthorizeScopeReviewState
  | MCPAuthorizeBeginningState
  | MCPAuthorizeWaitingState
  | MCPAuthorizeManualState
  | MCPAuthorizeExchangingState
  | MCPAuthorizeConfirmedState
  | MCPAuthorizeBeginFailedState
  | MCPAuthorizeCompletionFailedState;

export type MCPAuthorizeBegin = (input: {
  filter: SettingsMCPAuthFilter;
  mode: SettingsMCPAuthBeginMode;
  server: string;
  scopeApproval?: MCPAuthorizeScopeApproval;
}) => Promise<SettingsMCPAuthBeginResponse>;

export type MCPAuthorizeExchange = (input: {
  body: SettingsMCPAuthExchangeRequest;
  filter: SettingsMCPAuthFilter;
  server: string;
}) => Promise<SettingsMCPAuthStatusResponse>;

type MCPAuthorizeEventPayloadMap = {
  scopeEscalationRequested: {
    approvedScopes: string[];
    dismissOnConfirmation: boolean;
    server: string;
    filter: SettingsMCPAuthFilter;
    prior: MCPAuthorizePriorStatus;
  };
  scopeEscalationConfirmed: { begin: MCPAuthorizeBegin };
  authorizeRequested: {
    begin: MCPAuthorizeBegin;
    dismissOnConfirmation: boolean;
    server: string;
    filter: SettingsMCPAuthFilter;
    prior: MCPAuthorizePriorStatus;
    mode: SettingsMCPAuthBeginMode;
    scopeApproval?: MCPAuthorizeScopeApproval;
  };
  manualAuthorizationRequested: { begin: MCPAuthorizeBegin };
  manualRedirectSubmitted: {
    exchange: MCPAuthorizeExchange;
    onConfirmed: (server: string) => void;
    value: string;
  };
  manualCompletionRetried: {
    exchange: MCPAuthorizeExchange;
    onConfirmed: (server: string) => void;
    value: string;
  };
  beginSucceeded: { attemptId: number; begin: SettingsMCPAuthBeginResponse };
  beginFailed: { attemptId: number; error: string };
  exchangeSucceeded: {
    attemptId: number;
    onConfirmed: (server: string) => void;
    status: SettingsMCPAuthStatusResponse;
  };
  exchangeFailed: { attemptId: number; error: string };
  statusObserved: {
    onConfirmed: (server: string) => void;
    status: string;
    tokenPresent: boolean;
  };
  authorizationCancelled: {};
  beginRetried: { begin: MCPAuthorizeBegin };
};

export function createMCPAuthorizeLogic() {
  return createStoreLogic<MCPAuthorizeState, MCPAuthorizeEventPayloadMap>({
    context: { phase: "idle", attemptId: 0 },
    on: {
      scopeEscalationRequested: (context, event) => {
        if (context.phase !== "idle") return;
        const approvedScopes = normalizeMCPAuthScopes(event.approvedScopes);
        const scopeApproval: MCPAuthorizeScopeApproval = {
          approveScopeEscalation: true,
          approvedScopes,
        };
        if (!hasExplicitScopeApproval(scopeApproval)) return;
        return {
          phase: "reviewing_scopes",
          attemptId: context.attemptId,
          dismissOnConfirmation: event.dismissOnConfirmation,
          server: event.server,
          filter: event.filter,
          prior: event.prior,
          mode: "automatic",
          scopeApproval,
        };
      },
      scopeEscalationConfirmed: (context, event, enqueue) => {
        if (context.phase !== "reviewing_scopes") return;
        const attemptId = context.attemptId + 1;
        enqueueBegin(
          event.begin,
          enqueue,
          attemptId,
          context.server,
          context.filter,
          context.mode,
          context.scopeApproval
        );
        return { ...context, phase: "beginning", attemptId };
      },
      authorizeRequested: (context, event, enqueue) => {
        if (context.phase !== "idle") return;
        const scopeApproval = event.scopeApproval
          ? {
              approveScopeEscalation: event.scopeApproval.approveScopeEscalation,
              approvedScopes: normalizeMCPAuthScopes(event.scopeApproval.approvedScopes),
            }
          : undefined;
        if (scopeApproval && !hasExplicitScopeApproval(scopeApproval)) return;
        const attemptId = context.attemptId + 1;
        enqueueBegin(
          event.begin,
          enqueue,
          attemptId,
          event.server,
          event.filter,
          event.mode,
          scopeApproval
        );
        return {
          phase: "beginning",
          attemptId,
          dismissOnConfirmation: event.dismissOnConfirmation,
          server: event.server,
          filter: event.filter,
          prior: event.prior,
          mode: event.mode,
          scopeApproval,
        };
      },
      manualAuthorizationRequested: (context, event, enqueue) => {
        if (
          context.phase === "idle" ||
          context.phase === "beginning" ||
          context.phase === "reviewing_scopes"
        ) {
          return;
        }
        const attemptId = context.attemptId + 1;
        enqueueBegin(
          event.begin,
          enqueue,
          attemptId,
          context.server,
          context.filter,
          "manual",
          context.scopeApproval
        );
        return {
          phase: "beginning",
          attemptId,
          dismissOnConfirmation: context.dismissOnConfirmation,
          server: context.server,
          filter: context.filter,
          prior: context.prior,
          mode: "manual",
          scopeApproval: context.scopeApproval,
        };
      },
      manualRedirectSubmitted: (context, event, enqueue) => {
        if (context.phase !== "manual") return;
        if (!isAbsoluteMCPRedirectURL(event.value)) {
          return {
            ...context,
            phase: "failed",
            stage: "completion",
            error: "Callback URL must be a complete HTTP or HTTPS URL.",
          };
        }
        enqueueExchange(event.exchange, enqueue, context, event.value, event.onConfirmed);
        return { ...context, phase: "exchanging" };
      },
      manualCompletionRetried: (context, event, enqueue) => {
        if (
          context.phase !== "failed" ||
          context.stage !== "completion" ||
          !isAbsoluteMCPRedirectURL(event.value)
        ) {
          return;
        }
        enqueueExchange(event.exchange, enqueue, context, event.value, event.onConfirmed);
        return { ...context, phase: "exchanging" };
      },
      beginSucceeded: (context, event) => {
        if (context.phase !== "beginning" || context.attemptId !== event.attemptId) return;
        return {
          ...context,
          phase: context.mode === "manual" ? "manual" : "waiting",
          begin: event.begin,
        };
      },
      beginFailed: (context, event) => {
        if (context.phase !== "beginning" || context.attemptId !== event.attemptId) return;
        return { ...context, phase: "failed", stage: "begin", error: event.error };
      },
      exchangeSucceeded: (context, event, enqueue) => {
        if (context.phase !== "exchanging" || context.attemptId !== event.attemptId) return;
        if (isConfirmed(event.status.status, Boolean(event.status.token_present))) {
          return confirmAuthorization(context, enqueue, event.onConfirmed);
        }
        return {
          ...context,
          phase: "failed",
          stage: "completion",
          error: "The provider did not return a confirmed credential.",
        };
      },
      exchangeFailed: (context, event) => {
        if (context.phase !== "exchanging" || context.attemptId !== event.attemptId) return;
        return { ...context, phase: "failed", stage: "completion", error: event.error };
      },
      statusObserved: (context, event, enqueue) => {
        if (
          context.phase !== "waiting" &&
          context.phase !== "manual" &&
          context.phase !== "exchanging"
        ) {
          return;
        }
        if (!isConfirmed(event.status, event.tokenPresent)) return;
        return confirmAuthorization(context, enqueue, event.onConfirmed);
      },
      authorizationCancelled: context => ({ phase: "idle", attemptId: context.attemptId }),
      beginRetried: (context, event, enqueue) => {
        if (context.phase !== "failed") return;
        const attemptId = context.attemptId + 1;
        enqueueBegin(
          event.begin,
          enqueue,
          attemptId,
          context.server,
          context.filter,
          context.mode,
          context.scopeApproval
        );
        return {
          phase: "beginning",
          attemptId,
          dismissOnConfirmation: context.dismissOnConfirmation,
          server: context.server,
          filter: context.filter,
          prior: context.prior,
          mode: context.mode,
          scopeApproval: context.scopeApproval,
        };
      },
    },
  });
}

type MCPAuthorizeEnqueue = EnqueueObject<MCPAuthorizeState, never, MCPAuthorizeEventPayloadMap>;

type MCPAuthorizeConfirmableState =
  | MCPAuthorizeWaitingState
  | MCPAuthorizeManualState
  | MCPAuthorizeExchangingState;

function confirmAuthorization(
  context: MCPAuthorizeConfirmableState,
  enqueue: MCPAuthorizeEnqueue,
  onConfirmed: (server: string) => void
): MCPAuthorizeState {
  enqueue.effect(() => onConfirmed(context.server));
  return context.dismissOnConfirmation
    ? { phase: "idle", attemptId: context.attemptId }
    : { ...context, phase: "confirmed" };
}

function enqueueBegin(
  begin: MCPAuthorizeBegin,
  enqueue: MCPAuthorizeEnqueue,
  attemptId: number,
  server: string,
  filter: SettingsMCPAuthFilter,
  mode: SettingsMCPAuthBeginMode,
  scopeApproval?: MCPAuthorizeScopeApproval
): void {
  enqueue.effect(async ({ trigger }) => {
    try {
      trigger.beginSucceeded({
        attemptId,
        begin: await begin({
          server,
          filter,
          mode,
          ...(scopeApproval ? { scopeApproval } : {}),
        }),
      });
    } catch (error) {
      trigger.beginFailed({
        attemptId,
        error: errorMessage(error, "Authorization could not be started."),
      });
    }
  });
}

function enqueueExchange(
  exchange: MCPAuthorizeExchange,
  enqueue: MCPAuthorizeEnqueue,
  context: MCPAuthorizeManualState | MCPAuthorizeCompletionFailedState,
  value: string,
  onConfirmed: (server: string) => void
): void {
  const attemptId = context.attemptId;
  const server = context.server;
  const filter = context.filter;
  const body = toExchangeBody(value);
  enqueue.effect(async ({ trigger }) => {
    try {
      trigger.exchangeSucceeded({
        attemptId,
        onConfirmed,
        status: await exchange({ server, filter, body }),
      });
    } catch (error) {
      trigger.exchangeFailed({
        attemptId,
        error: errorMessage(error, "Authorization could not be completed."),
      });
    }
  });
}

function isConfirmed(status: string, tokenPresent: boolean): boolean {
  return status === "authenticated" && tokenPresent;
}

function hasExplicitScopeApproval(approval: MCPAuthorizeScopeApproval): boolean {
  return approval.approveScopeEscalation && approval.approvedScopes.length > 0;
}

/** The daemon validates state, issuer, and remaining callback constraints. */
function toExchangeBody(value: string): SettingsMCPAuthExchangeRequest {
  return { redirect_url: normalizeMCPRedirectURL(value) };
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
