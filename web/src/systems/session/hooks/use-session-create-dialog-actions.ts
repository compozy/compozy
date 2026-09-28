import { useSelector } from "@xstate/store-react";

import type { EntityMode } from "@compozy/ui";

import {
  clearPendingTerminalQuote,
  restorePendingTerminalQuoteAfterFailedCreate,
} from "../lib/session-terminal-quote";
import type { SessionCreateStore } from "../stores/session-create-store";
import type { SessionEnvironmentModel } from "./use-session-environment";

export function useSessionCreateDialogActions({
  environment,
  runtimeWorkspaceId,
  store,
}: {
  environment: SessionEnvironmentModel;
  runtimeWorkspaceId: string | null;
  store: SessionCreateStore;
}) {
  const flow = useSelector(store, snapshot => snapshot.context);
  return {
    onCancelEnvironment: () => {
      restorePendingTerminalQuoteAfterFailedCreate(flow.pendingSubmit?.terminalQuote ?? null);
      const restoreTarget = flow.pendingSubmit?.previousEnvironment ?? { kind: "root" as const };
      environment.cancelPending(restoreTarget);
      store.trigger.environmentRestored({ environment: restoreTarget });
    },
    onOpenChange: (open: boolean) => {
      if (!open) environment.cancelPending();
      store.trigger.dialogOpenChanged({ open });
      if (!open && store.getSnapshot().context.operation.status !== "submitting") {
        clearPendingTerminalQuote();
      }
    },
    onModeChange: (mode: EntityMode) => store.trigger.modeSelected({ mode }),
    onAgentChange: (agentName: string) =>
      store.trigger.agentSelected({ agentName, workspaceId: runtimeWorkspaceId ?? "" }),
    onSessionNameChange: (sessionName: string) => store.trigger.sessionNameChanged({ sessionName }),
  };
}
