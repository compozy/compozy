// Suite: window-manager client identity
// Invariant: one browser tab keeps one client id across storage failures and
// does not mint entropy when a persisted id already exists.
// Owning layer: unit — the identity helper.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  stableWindowManagerClientId,
  WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY,
  WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY,
} from "../window-manager-client-identity";

beforeEach(() => {
  Reflect.deleteProperty(globalThis, WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY);
  window.localStorage.clear();
});

afterEach(() => {
  Reflect.deleteProperty(globalThis, WINDOW_MANAGER_CLIENT_ID_MEMORY_KEY);
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("stableWindowManagerClientId", () => {
  it("Should reuse a persisted id without generating entropy", () => {
    const randomUUID = vi.fn(() => "generated");
    vi.stubGlobal("crypto", { randomUUID });
    window.localStorage.setItem(WINDOW_MANAGER_CLIENT_ID_STORAGE_KEY, "client:persisted");

    expect(stableWindowManagerClientId()).toBe("client:persisted");
    expect(stableWindowManagerClientId()).toBe("client:persisted");
    expect(randomUUID).not.toHaveBeenCalled();
  });

  it("Should keep one fallback id when storage throws", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("storage disabled");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("storage disabled");
    });

    const first = stableWindowManagerClientId();
    const second = stableWindowManagerClientId();
    expect(first).toMatch(/^web-/);
    expect(second).toBe(first);
  });
});
