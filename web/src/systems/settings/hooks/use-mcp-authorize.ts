import { useSelector, useStore } from "@xstate/store-react";

import {
  createMCPAuthorizeLogic,
  type MCPAuthorizeBegin,
  type MCPAuthorizeExchange,
  type MCPAuthorizePriorStatus,
  type MCPAuthorizeState,
} from "../stores/mcp-authorize-store";
import type { SettingsMCPAuthFilter } from "../types";
import { useBeginMCPAuth, useExchangeMCPAuth } from "./use-settings-mutations";

export type { MCPAuthorizePriorStatus, MCPAuthorizeState };
export type MCPAuthorizePhase = MCPAuthorizeState["phase"];

export function isMCPAuthorizeAwaiting(phase: MCPAuthorizePhase): boolean {
  return phase === "waiting" || phase === "manual" || phase === "exchanging";
}

export function isMCPAuthorizePending(phase: MCPAuthorizePhase): boolean {
  return phase === "beginning" || isMCPAuthorizeAwaiting(phase);
}

const mcpAuthorizeLogic = createMCPAuthorizeLogic();

interface MCPAuthorizeOptions {
  dismissOnConfirmation?: boolean;
  onConfirmed?: (server: string) => void;
}

export function useMCPAuthorize(options: MCPAuthorizeOptions = {}) {
  const beginMutation = useBeginMCPAuth();
  const exchangeMutation = useExchangeMCPAuth();
  const store = useStore(mcpAuthorizeLogic);
  const onConfirmed = (server: string) => options.onConfirmed?.(server);

  const state = useSelector(store, snapshot => snapshot.context);
  const beginExecution: MCPAuthorizeBegin = input =>
    beginMutation.mutateAsync({ name: input.server, filter: input.filter, mode: input.mode });
  const exchangeExecution: MCPAuthorizeExchange = input =>
    exchangeMutation.mutateAsync({ name: input.server, filter: input.filter, body: input.body });

  const beginAuthorize = (
    filter: SettingsMCPAuthFilter,
    server: string,
    prior: MCPAuthorizePriorStatus
  ) => {
    store.trigger.authorizeRequested({
      begin: beginExecution,
      dismissOnConfirmation: options.dismissOnConfirmation ?? false,
      server,
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
    begin: beginResponse,
    error,
    prior: activeState?.prior ?? null,
    mode: activeState?.mode ?? null,
    beginAuthorize,
    retryBegin: () => store.trigger.beginRetried({ begin: beginExecution }),
    enterManual: () => store.trigger.manualAuthorizationRequested({ begin: beginExecution }),
    submitManual: (value: string) => {
      if (state.phase === "failed" && state.stage === "completion") {
        store.trigger.manualCompletionRetried({ exchange: exchangeExecution, onConfirmed, value });
        return;
      }
      store.trigger.manualCodeSubmitted({ exchange: exchangeExecution, onConfirmed, value });
    },
    acknowledgeStatus: (status: string, tokenPresent: boolean) =>
      store.trigger.statusObserved({ onConfirmed, status, tokenPresent }),
    cancel: () => store.trigger.authorizationCancelled(),
  };
}

export type UseMCPAuthorizeReturn = ReturnType<typeof useMCPAuthorize>;
