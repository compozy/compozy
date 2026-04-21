import { beforeEach, describe, expect, it } from "vitest";

import {
  readActiveWorkspaceId,
  writeActiveWorkspaceId,
  WORKSPACE_STORAGE_KEY,
} from "./session-storage";

beforeEach(() => {
  window.sessionStorage.clear();
});

describe("session-storage workspace helpers", () => {
  it("Should return null when the storage key is missing", () => {
    expect(readActiveWorkspaceId()).toBeNull();
  });

  it("Should round-trip a workspace id through sessionStorage", () => {
    writeActiveWorkspaceId("ws-1");
    expect(window.sessionStorage.getItem(WORKSPACE_STORAGE_KEY)).toBe("ws-1");
    expect(readActiveWorkspaceId()).toBe("ws-1");
  });

  it("Should clear the storage key when the workspace id is null", () => {
    window.sessionStorage.setItem(WORKSPACE_STORAGE_KEY, "ws-1");
    writeActiveWorkspaceId(null);
    expect(window.sessionStorage.getItem(WORKSPACE_STORAGE_KEY)).toBeNull();
  });

  it("Should treat a whitespace-only value as missing", () => {
    window.sessionStorage.setItem(WORKSPACE_STORAGE_KEY, "   ");
    expect(readActiveWorkspaceId()).toBeNull();
  });
});
