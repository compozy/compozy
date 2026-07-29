import { afterEach, describe, expect, it, vi } from "vitest";
import {
  folderPersistKey,
  getFolderOpenSnapshot,
  readFolderOpen,
  subscribeFolderOpen,
  writeFolderOpen,
} from "../sidebar-folder-persist";

describe("sidebar folder persist", () => {
  afterEach(() => {
    window.localStorage.clear();
  });

  it("Should prefer $id, then index url, then string name for the persist key", () => {
    expect(folderPersistKey({ $id: "core/loops", name: "Loops" })).toBe("core/loops");
    expect(
      folderPersistKey({
        name: "Loops",
        index: { url: "/runtime/core/loops" },
      })
    ).toBe("/runtime/core/loops");
    expect(folderPersistKey({ name: "Loops" })).toBe("Loops");
  });

  it("Should round-trip open state through localStorage and notify subscribers", () => {
    const onChange = vi.fn();
    const unsubscribe = subscribeFolderOpen("core/loops", onChange);
    writeFolderOpen("core/loops", true);
    expect(readFolderOpen("core/loops")).toBe(true);
    expect(getFolderOpenSnapshot("core/loops", false)).toBe(true);
    expect(onChange).toHaveBeenCalledTimes(1);
    writeFolderOpen("core/loops", false);
    expect(readFolderOpen("core/loops")).toBe(false);
    expect(readFolderOpen("missing")).toBeNull();
    expect(getFolderOpenSnapshot("missing", true)).toBe(true);
    unsubscribe();
  });
});
