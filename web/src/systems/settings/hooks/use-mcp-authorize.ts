import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";

import {
  createMCPAuthorizeLogic,
  type MCPAuthorizeBegin,
  type MCPAuthorizeExchange,
  type MCPAuthorizePriorStatus,
  type MCPAuthorizeState,
} from "../stores/mcp-authorize-store";
import { mcpScopeEscalationScopes, normalizeMCPAuthScopes } from "../lib/mcp-authorize-model";
import { mcpPanelAnchor, type MCPPanelAnchor } from "../lib/mcp-panel-anchor";
import { openMCPSignInWindow, type MCPSignInWindow } from "../lib/mcp-sign-in-window";
import type { SettingsMCPAuthFilter, SettingsMCPServerEntry } from "../types";
import { useBeginMCPAuth, useExchangeMCPAuth } from "./use-settings-mutations";

export type { MCPAuthorizePriorStatus, MCPAuthorizeState };
export type MCPAuthorizePhase = MCPAuthorizeState["phase"];

export function isMCPAuthorizeAwaiting(phase: MCPAuthorizePhase): boolean {
  return phase === "waiting" || phase === "manual" || phase === "exchanging";
}

export function isMCPAuthorizePending(phase: MCPAuthorizePhase): boolean {
  return phase === "reviewing_scopes" || phase === "beginning" || isMCPAuthorizeAwaiting(phase);
}

const mcpAuthorizeLogic = createMCPAuthorizeLogic();

interface MCPAuthorizeOptions {
  dismissOnConfirmation?: boolean;
  onConfirmed?: (server: string, filter: SettingsMCPAuthFilter) => void;
}

export function useMCPAuthorize(options: MCPAuthorizeOptions = {}) {
  const beginMutation = useBeginMCPAuth();
  const exchangeMutation = useExchangeMCPAuth();
  const store = useStore(mcpAuthorizeLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const [anchor, setAnchor] = useState<MCPPanelAnchor | null>(null);
  // The browser tab reserved by the click that starts a sign-in; the begin's answer navigates it.
  const signInWindow = useRef<MCPSignInWindow | null>(null);
  const reserveSignInWindow = () => {
    signInWindow.current?.close();
    signInWindow.current = openMCPSignInWindow();
  };
  const releaseSignInWindow = () => {
    signInWindow.current?.close();
    signInWindow.current = null;
  };
  // A host that unmounts mid-sign-in gives back a tab nobody will navigate and drops its reference.
  useEffect(
    () => () => {
      signInWindow.current?.close();
      signInWindow.current = null;
    },
    []
  );
  const onConfirmed = (server: string) => {
    if (state.phase !== "idle") options.onConfirmed?.(server, state.filter);
  };
  const beginExecution: MCPAuthorizeBegin = input => {
    const scopeApproval = input.scopeApproval;
    const scopes = normalizeMCPAuthScopes(scopeApproval?.approvedScopes ?? []);
    const explicitScopeApproval =
      scopeApproval?.approveScopeEscalation === true && scopes.length > 0;
    // A manual sign-in happens on another computer: nothing opens here.
    const reserved = input.mode === "manual" ? null : signInWindow.current;
    const settle = () => {
      if (signInWindow.current === reserved) signInWindow.current = null;
    };
    const request = beginMutation.mutateAsync({
      name: input.server,
      filter: input.filter,
      body: {
        mode: input.mode,
        ...(explicitScopeApproval
          ? { approve_scope_escalation: true, approved_scopes: scopes }
          : {}),
      },
    });
    if (!reserved) return request;
    return request.then(
      response => {
        if (signInWindow.current === reserved) reserved.navigate(response.authorization_url);
        settle();
        return response;
      },
      (error: unknown) => {
        reserved.close();
        settle();
        throw error;
      }
    );
  };
  const exchangeExecution: MCPAuthorizeExchange = input =>
    exchangeMutation.mutateAsync({ name: input.server, filter: input.filter, body: input.body });

  /**
   * Starts a sign-in from a click. `trigger` is the control the panel opens beside; the browser
   * opens on its own unless the server first asks for more access.
   */
  const requestAuthorize = (
    filter: SettingsMCPAuthFilter,
    server: SettingsMCPServerEntry,
    trigger: HTMLElement
  ) => {
    // The live phase, not this render's: a second press in the same tick must not open a tab.
    if (store.getSnapshot().context.phase !== "idle") return;
    setAnchor(mcpPanelAnchor(trigger));
    const prior: MCPAuthorizePriorStatus = {
      status: server.auth_status?.status ?? "needs_login",
      tokenPresent: Boolean(server.auth_status?.token_present),
    };
    const approvedScopes = mcpScopeEscalationScopes(server);
    if (approvedScopes.length > 0) {
      store.trigger.scopeEscalationRequested({
        approvedScopes,
        dismissOnConfirmation: options.dismissOnConfirmation ?? false,
        server: server.name,
        prior,
        filter,
      });
      return;
    }
    reserveSignInWindow();
    store.trigger.authorizeRequested({
      begin: beginExecution,
      dismissOnConfirmation: options.dismissOnConfirmation ?? false,
      server: server.name,
      prior,
      filter,
      mode: "automatic",
    });
  };

  const activeState = state.phase === "idle" ? null : state;
  const beginResponse = "begin" in state ? state.begin : null;
  const error = state.phase === "failed" ? state.error : null;

  return {
    phase: state.phase,
    server: activeState?.server ?? null,
    filter: activeState?.filter ?? null,
    begin: beginResponse,
    error,
    prior: activeState?.prior ?? null,
    mode: activeState?.mode ?? null,
    approvedScopes: activeState?.scopeApproval?.approvedScopes ?? [],
    /** Where the sign-in panel opens: beside the control that started it. */
    anchor: activeState ? anchor : null,
    requestAuthorize,
    confirmScopeEscalation: () => {
      if (store.getSnapshot().context.phase !== "reviewing_scopes") return;
      reserveSignInWindow();
      store.trigger.scopeEscalationConfirmed({ begin: beginExecution });
    },
    retryBegin: () => {
      const current = store.getSnapshot().context;
      if (current.phase !== "failed") return;
      if (current.mode !== "manual") reserveSignInWindow();
      store.trigger.beginRetried({ begin: beginExecution });
    },
    enterManual: () => {
      releaseSignInWindow();
      store.trigger.manualAuthorizationRequested({ begin: beginExecution });
    },
    submitManual: (value: string) => {
      if (state.phase === "failed" && state.stage === "completion") {
        store.trigger.manualCompletionRetried({ exchange: exchangeExecution, onConfirmed, value });
        return;
      }
      store.trigger.manualRedirectSubmitted({ exchange: exchangeExecution, onConfirmed, value });
    },
    acknowledgeStatus: (status: string, tokenPresent: boolean) =>
      store.trigger.statusObserved({ onConfirmed, status, tokenPresent }),
    cancel: () => {
      releaseSignInWindow();
      store.trigger.authorizationCancelled();
    },
  };
}

export type UseMCPAuthorizeReturn = ReturnType<typeof useMCPAuthorize>;
