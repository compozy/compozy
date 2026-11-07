import { describe, expect, it } from "vitest";

import type { FileEntry } from "../../adapters/files-api";
import { entriesDecorations } from "../file-tree-paths";

// Suite: tree decorations from listings (file-explorer US-001.AC-4).
// Invariant: a directory the daemon marks `contains_changes` shows the dot while none of its
// changed descendants is loaded; once one is, the tree's own dot takes over, so there is no
// second dot. Owning layer: the bridge's decoration mapping.
function entry(path: string, extra: Partial<FileEntry> = {}): FileEntry {
  return { name: path.split("/").at(-1) ?? path, path, kind: "file", ...extra } as FileEntry;
}

describe("entriesDecorations", () => {
  it("Should mark an unloaded changed directory and hand off once a changed child loads", () => {
    const web = entry("web", { kind: "directory", contains_changes: true });
    const docs = entry("docs", { kind: "directory" });
    expect(entriesDecorations([web, docs])).toEqual({ "web/": { containsChanges: true } });

    const src = entry("web/src", { kind: "directory", contains_changes: true });
    const readme = entry("web/README.md", { git_status: "modified" });
    expect(entriesDecorations([web, src, readme])).toEqual({
      "web/src/": { containsChanges: true },
      "web/README.md": { status: "modified" },
    });
  });
});
