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
    expect(folderPersistKey({ $id: "loops", name: "Loops" })).toBe("loops");
    expect(
      folderPersistKey({
        name: "Loops",
        index: { url: "/docs/loops" },
      })
    ).toBe("/docs/loops");
    expect(folderPersistKey({ name: "Loops" })).toBe("Loops");
  });

  it("Should round-trip open state through localStorage and notify subscribers", () => {
    const onChange = vi.fn();
    const unsubscribe = subscribeFolderOpen("loops", onChange);
    writeFolderOpen("loops", true);
    expect(readFolderOpen("loops")).toBe(true);
    expect(getFolderOpenSnapshot("loops", false)).toBe(true);
    expect(onChange).toHaveBeenCalledTimes(1);
    writeFolderOpen("loops", false);
    expect(readFolderOpen("loops")).toBe(false);
    expect(readFolderOpen("missing")).toBeNull();
    expect(getFolderOpenSnapshot("missing", true)).toBe(true);
    unsubscribe();
  });
});
