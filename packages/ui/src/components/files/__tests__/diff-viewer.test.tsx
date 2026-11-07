import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { DiffViewer, type DiffViewerItem } from "../diff-viewer";
import { parseItemPatch } from "../diff-viewer-items";

function patchFor(path: string, added: string): string {
  return [
    `diff --git a/${path} b/${path}`,
    "index 1111111..2222222 100644",
    `--- a/${path}`,
    `+++ b/${path}`,
    "@@ -1,2 +1,3 @@",
    " package main",
    `+${added}`,
    " func main() {}",
    "",
  ].join("\n");
}

const items: DiffViewerItem[] = [
  { id: "a.go", version: "r1", path: "a.go" },
  { id: "logo.png", version: "r1", path: "logo.png", loadable: false },
];

function header(item: DiffViewerItem, state: { status: string }) {
  return (
    <div data-testid={`header-${item.id}`}>
      {item.path} · {state.status}
    </div>
  );
}

describe("DiffViewer", () => {
  it("Should load patches for rendered files and never fetch unloadable ones", async () => {
    const loadPatch = vi.fn(async (item: DiffViewerItem) => patchFor(item.path, "var x = 1"));
    const onVisible = vi.fn();
    render(
      <DiffViewer
        items={items}
        loadPatch={loadPatch}
        fileHeader={header}
        onVisibleItemsChange={onVisible}
      />
    );

    await waitFor(() => expect(screen.getByTestId("header-a.go")).toHaveTextContent("ready"));
    expect(loadPatch).toHaveBeenCalledTimes(1);
    expect(loadPatch).toHaveBeenCalledWith(items[0]);
    expect(screen.getByTestId("header-logo.png")).toHaveTextContent("unavailable");
    expect(onVisible).toHaveBeenCalledWith(["a.go", "logo.png"]);
  });

  it("Should refetch when an item's version changes and mark failed loads", async () => {
    const loadPatch = vi
      .fn<(item: DiffViewerItem) => Promise<string | null>>()
      .mockImplementation(async item => patchFor(item.path, `var v = "${String(item.version)}"`));
    const { rerender } = render(
      <DiffViewer items={[items[0]!]} loadPatch={loadPatch} fileHeader={header} />
    );
    await waitFor(() => expect(screen.getByTestId("header-a.go")).toHaveTextContent("ready"));

    loadPatch.mockRejectedValueOnce(new Error("changes_stale"));
    rerender(
      <DiffViewer
        items={[{ ...items[0]!, version: "r2" }]}
        loadPatch={loadPatch}
        fileHeader={header}
      />
    );
    await waitFor(() => expect(screen.getByTestId("header-a.go")).toHaveTextContent("error"));
    expect(loadPatch).toHaveBeenCalledTimes(2);
  });

  it("Should key parsed patches by item version", () => {
    const fileDiff = parseItemPatch(items[0]!, patchFor("a.go", "var x = 1"));
    expect(fileDiff?.cacheKey).toBe("a.go:r1");
    expect(fileDiff?.hunks).toHaveLength(1);
    expect(parseItemPatch(items[0]!, "not a patch")).toBeNull();
  });
});
