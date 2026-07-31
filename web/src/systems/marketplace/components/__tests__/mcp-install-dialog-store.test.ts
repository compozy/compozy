import { describe, expect, it, vi } from "vitest";

import { mcpInstallDialogLogic } from "../mcp-install-dialog-store";

describe("MCP install dialog store", () => {
  it("Should execute a typed install request when installation begins", async () => {
    const store = mcpInstallDialogLogic.createStore({ scope: "global", bindings: {} });
    const install = vi.fn().mockResolvedValue(undefined);

    store.trigger.installationRequested({ install });

    expect(install).toHaveBeenCalledWith({}, "global");
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("editing"));
  });

  it("Should ignore an obsolete installation completion after a newer request", () => {
    const store = mcpInstallDialogLogic.createStore({ scope: "global", bindings: {} });
    const install = vi.fn(() => new Promise<void>(() => undefined));
    let snapshot = store.getInitialSnapshot();

    [snapshot] = store.transition(snapshot, { type: "installationRequested", install });
    const firstRequestId = snapshot.context.requestId;
    [snapshot] = store.transition(snapshot, {
      type: "installationFailed",
      requestId: firstRequestId,
      error: "offline",
    });
    [snapshot] = store.transition(snapshot, { type: "installationRequested", install });
    [snapshot] = store.transition(snapshot, {
      type: "installationSucceeded",
      requestId: firstRequestId,
    });

    expect(snapshot.context.phase).toBe("installing");
  });

  it("Should execute the Vault request with its result fence", async () => {
    const store = mcpInstallDialogLogic.createStore({
      scope: "workspace",
      bindings: {
        API_TOKEN: {
          createError: undefined,
          createOpen: true,
          createValue: "secret-value",
          mode: "vault",
          touched: true,
          typedValue: "",
          vaultRef: "",
        },
      },
    });
    const createSecret = vi.fn().mockResolvedValue(undefined);

    store.trigger.secretCreationRequested({
      createSecret,
      name: "API_TOKEN",
      ref: "vault:mcp/ws/ws_alpha/server/inputs/API_TOKEN",
      value: "secret-value",
    });
    expect(createSecret).toHaveBeenCalledWith(
      "vault:mcp/ws/ws_alpha/server/inputs/API_TOKEN",
      "secret-value"
    );
    await vi.waitFor(() => expect(store.getSnapshot().context.phase).toBe("editing"));
    expect(store.getSnapshot().context.bindings.API_TOKEN?.vaultRef).toBe(
      "vault:mcp/ws/ws_alpha/server/inputs/API_TOKEN"
    );
  });

  it("Should preserve edits to another binding while a Vault secret is created", () => {
    const binding = {
      createOpen: false,
      createValue: "",
      mode: "typed" as const,
      touched: false,
      typedValue: "",
      vaultRef: "",
    };
    const store = mcpInstallDialogLogic.createStore({
      scope: "workspace",
      bindings: { API_TOKEN: binding, REGION: binding },
    });

    store.trigger.secretCreationRequested({
      createSecret: () => new Promise<void>(() => undefined),
      name: "API_TOKEN",
      ref: "vault:mcp/ws/ws_alpha/server/inputs/API_TOKEN",
      value: "secret-value",
    });
    store.trigger.bindingChanged({
      name: "REGION",
      binding: { ...binding, touched: true, typedValue: "us-east-1" },
    });

    expect(store.getSnapshot().context.phase).toBe("creating-secret");
    expect(store.getSnapshot().context.bindings.REGION?.typedValue).toBe("us-east-1");
  });
});
