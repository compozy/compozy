import { createStoreLogic } from "@xstate/store";

import type { MCPFieldBinding } from "./mcp-install-model";

type Scope = "global" | "workspace";
type Phase = "editing" | "creating-secret" | "installing" | "failed";

export interface MCPInstallDialogState {
  phase: Phase;
  requestId: number;
  bindings: Record<string, MCPFieldBinding>;
  installError: string | null;
}

export interface MCPInstallDialogInput {
  bindings: Record<string, MCPFieldBinding>;
}

type Events = {
  bindingChanged: { name: string; binding: MCPFieldBinding };
  secretCreationRequested: {
    createSecret: (ref: string, value: string) => Promise<void>;
    name: string;
    ref: string;
    value: string;
  };
  secretCreationSucceeded: { requestId: number; name: string; ref: string };
  secretCreationFailed: { requestId: number; name: string; error: string };
  installationRequested: {
    /** Landing scope is menubar-derived at request time, never stored dialog state. */
    scope: Scope;
    install: (bindings: Record<string, MCPFieldBinding>, scope: Scope) => Promise<void>;
  };
  installationSucceeded: { requestId: number };
  installationFailed: { requestId: number; error: string };
};

type Emitted = {
  installed: {};
};

export const mcpInstallDialogLogic = createStoreLogic<
  MCPInstallDialogState,
  Events,
  Emitted,
  MCPInstallDialogInput
>({
  context: input => ({ ...input, phase: "editing", requestId: 0, installError: null }),
  on: {
    bindingChanged: (context, event) => {
      if (context.phase === "installing") return;
      return {
        ...context,
        installError: null,
        bindings: { ...context.bindings, [event.name]: event.binding },
      };
    },
    secretCreationRequested: (context, event, enqueue) => {
      if (context.phase === "installing" || context.phase === "creating-secret") return;
      const requestId = context.requestId + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.createSecret(event.ref, event.value);
          trigger.secretCreationSucceeded({
            name: event.name,
            ref: event.ref,
            requestId,
          });
        } catch (error) {
          trigger.secretCreationFailed({
            error: errorMessage(error, "Failed to create the Vault secret"),
            name: event.name,
            requestId,
          });
        }
      });
      return { ...context, phase: "creating-secret", requestId, installError: null };
    },
    secretCreationSucceeded: (context, event) => {
      if (context.phase !== "creating-secret" || context.requestId !== event.requestId) return;
      const binding = context.bindings[event.name];
      if (!binding) return { ...context, phase: "editing" };
      return {
        ...context,
        phase: "editing",
        bindings: {
          ...context.bindings,
          [event.name]: {
            ...binding,
            createError: undefined,
            createOpen: false,
            createValue: "",
            mode: "vault",
            touched: true,
            vaultRef: event.ref,
          },
        },
      };
    },
    secretCreationFailed: (context, event) => {
      if (context.phase !== "creating-secret" || context.requestId !== event.requestId) return;
      const binding = context.bindings[event.name];
      return {
        ...context,
        phase: "editing",
        bindings: binding
          ? { ...context.bindings, [event.name]: { ...binding, createError: event.error } }
          : context.bindings,
      };
    },
    installationRequested: (context, event, enqueue) => {
      if (context.phase === "installing" || context.phase === "creating-secret") return;
      const requestId = context.requestId + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.install(context.bindings, event.scope);
          trigger.installationSucceeded({ requestId });
        } catch (error) {
          trigger.installationFailed({
            error: errorMessage(error, "Failed to install the MCP server"),
            requestId,
          });
        }
      });
      return { ...context, phase: "installing", requestId, installError: null };
    },
    installationSucceeded: (context, event, enqueue) => {
      if (context.phase !== "installing" || context.requestId !== event.requestId) return;
      enqueue.emit.installed({});
      return { ...context, phase: "editing" };
    },
    installationFailed: (context, event) => {
      if (context.phase !== "installing" || context.requestId !== event.requestId) return;
      return { ...context, phase: "failed", installError: event.error };
    },
  },
});

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
