import { describe, expect, it } from "vitest";

import { vaultPageLogic } from "../vault-page-logic";
import type { VaultDraft } from "../use-vault-page";

const DRAFT: VaultDraft = {
  kind: "api_key",
  overwriteConfirmed: false,
  ref: "vault:sessions/example",
  secretValue: "secret",
};

describe("vault page logic", () => {
  it("Should reject editor dismissal until an accepted put settles", () => {
    const store = vaultPageLogic.createStore();
    store.trigger.createOpened({ draft: DRAFT });
    store.trigger.putRequested({ execute: () => new Promise<never>(() => undefined) });
    const attempt = store.getSnapshot().context.pendingPutAttempt;
    store.trigger.editorDismissed();
    expect(store.getSnapshot().context.editor).toEqual({ draft: DRAFT, mode: "create" });
    store.trigger.putSucceeded({ attempt: attempt ?? 0, ref: DRAFT.ref });

    expect(store.getSnapshot().context.editor).toEqual({ mode: "closed" });
    expect(store.getSnapshot().context.lastAction).toEqual({ kind: "saved", ref: DRAFT.ref });
  });

  it("Should reject a put without an open create editor or selected secret", () => {
    const store = vaultPageLogic.createStore();
    const execute = () => Promise.resolve({ ref: DRAFT.ref });

    store.trigger.putRequested({ execute });

    expect(store.getSnapshot().context.pendingPutAttempt).toBeNull();
    expect(store.getSnapshot().context.lastAction).toBeNull();
  });

  it("Should reject another surface opening until an accepted put settles", () => {
    const store = vaultPageLogic.createStore();
    store.trigger.createOpened({ draft: DRAFT });
    store.trigger.putRequested({ execute: () => new Promise<never>(() => undefined) });
    const attempt = store.getSnapshot().context.pendingPutAttempt;

    store.trigger.inspectOpened({ ref: "vault:providers/other" });
    store.trigger.createOpened({ draft: { ...DRAFT, ref: "vault:providers/replacement" } });

    expect(store.getSnapshot().context.editor).toEqual({ draft: DRAFT, mode: "create" });
    expect(store.getSnapshot().context.pendingPutAttempt).toBe(attempt);

    store.trigger.putSucceeded({ attempt: attempt ?? 0, ref: DRAFT.ref });
    expect(store.getSnapshot().context.lastAction).toEqual({ kind: "saved", ref: DRAFT.ref });
  });
});
