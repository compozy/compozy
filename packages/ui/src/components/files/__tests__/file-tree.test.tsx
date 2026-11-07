import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

import { DropdownMenuItem } from "../../dropdown-menu";
import { FileTree, type FileTreeProps } from "../file-tree";
import {
  buildRowDecoration,
  planPathSync,
  toGitStatusEntries,
  TREE_SPINNER_ICON,
  TREE_SPINNER_STATIC_ICON,
  TREE_SPRITE_SHEET,
  UNLOADED_CHANGES_ENTRY,
} from "../file-tree-model";

function treeRoot(container: HTMLElement): ShadowRoot {
  const host = container.querySelector("file-tree-container");
  if (!host?.shadowRoot) throw new Error("file tree host did not render");
  return host.shadowRoot;
}

function row(container: HTMLElement, path: string): HTMLElement {
  const element = treeRoot(container).querySelector<HTMLElement>(
    `[data-type="item"][data-item-path="${path}"]`
  );
  if (!element) throw new Error(`row ${path} is not rendered`);
  return element;
}

function LazyTree(props: Partial<FileTreeProps> & { onExpand: (dir: string) => void }) {
  const [paths, setPaths] = React.useState<string[]>(["src/", "README.md"]);
  return (
    <FileTree
      aria-label="Project files"
      paths={paths}
      {...props}
      onExpand={dir => {
        props.onExpand(dir);
        // The bridge inserts the children once the listing arrives.
        setPaths(current => [...current, `${dir}index.ts`, `${dir}lib/`]);
      }}
    />
  );
}

describe("FileTree", () => {
  // UT-040: lazy expansion fires once, children render, selection re-fires.
  it("Should call onExpand once per lazy directory and render inserted children", async () => {
    const onExpand = vi.fn();
    const { container } = render(<LazyTree onExpand={onExpand} />);
    await waitFor(() => row(container, "src/"));

    act(() => row(container, "src/").click());
    await waitFor(() => expect(onExpand).toHaveBeenCalledTimes(1));
    expect(onExpand).toHaveBeenCalledWith("src/");
    await waitFor(() => row(container, "src/index.ts"));

    act(() => row(container, "src/").click());
    act(() => row(container, "src/").click());
    await waitFor(() => row(container, "src/index.ts"));
    expect(onExpand).toHaveBeenCalledTimes(1);
  });

  it("Should fire onExpand again when a still-childless directory is re-expanded", async () => {
    const onExpand = vi.fn();
    const { container } = render(
      <FileTree aria-label="Project files" paths={["src/", "README.md"]} onExpand={onExpand} />
    );
    await waitFor(() => row(container, "src/"));

    act(() => row(container, "src/").click());
    await waitFor(() => expect(onExpand).toHaveBeenCalledTimes(1));
    act(() => row(container, "src/").click());
    act(() => row(container, "src/").click());
    await waitFor(() => expect(onExpand).toHaveBeenCalledTimes(2));
    expect(onExpand).toHaveBeenLastCalledWith("src/");
  });

  it("Should fire onSelect for every file click, including the selected row", async () => {
    const onSelect = vi.fn();
    const { container } = render(
      <FileTree
        aria-label="Project files"
        paths={["README.md", "go.mod"]}
        selectedPath="README.md"
        onSelect={onSelect}
      />
    );
    await waitFor(() => row(container, "README.md"));

    act(() => row(container, "README.md").click());
    act(() => row(container, "README.md").click());
    expect(onSelect).toHaveBeenCalledTimes(2);
    expect(onSelect).toHaveBeenLastCalledWith("README.md");
  });

  it("Should reveal a selected directory open with its children", async () => {
    const { container } = render(
      <FileTree
        aria-label="Project files"
        paths={["web/", "web/src/", "web/src/main.tsx", "README.md"]}
        selectedPath="web/src/"
      />
    );
    await waitFor(() =>
      expect(row(container, "web/src/")).toHaveAttribute("aria-selected", "true")
    );
    await waitFor(() => row(container, "web/src/main.tsx"));
  });

  it("Should open the row menu at a right-clicked row and close it after an action", async () => {
    const onCopy = vi.fn();
    const { container } = render(
      <FileTree
        aria-label="Project files"
        paths={["web/", "README.md"]}
        rowMenu={(item, { close }) =>
          item.path === "README.md" ? null : (
            <DropdownMenuItem
              onClick={() => {
                onCopy(item);
                close();
              }}
            >
              Copy path
            </DropdownMenuItem>
          )
        }
      />
    );
    await waitFor(() => row(container, "web/"));

    fireEvent.contextMenu(row(container, "web/"));
    const menu = await screen.findByRole("menu", { name: "Actions for web" });
    expect(menu).toHaveAttribute("data-file-tree-context-menu-root", "true");
    fireEvent.click(screen.getByRole("menuitem", { name: "Copy path" }));
    expect(onCopy).toHaveBeenCalledWith({ kind: "directory", name: "web", path: "web/" });
    await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());

    fireEvent.contextMenu(row(container, "README.md"));
    expect(screen.queryByRole("menu")).toBeNull();
  });

  // Greptile regression on the perf batch. Invariant: a user's collapse of the selected file's
  // ancestor sticks when the path set changes (a listing arrives, a folder collapses); only a
  // new reveal request (another path or revealKey) expands ancestors again.
  it("Should keep a collapsed ancestor of the selection collapsed until a new reveal", async () => {
    const paths = ["web/", "web/src/", "web/src/main.tsx", "web/src/app.tsx", "README.md"];
    const view = (props: Partial<FileTreeProps>) => (
      <FileTree
        aria-label="Project files"
        paths={paths}
        selectedPath="web/src/main.tsx"
        revealKey="open-1"
        {...props}
      />
    );
    const { container, rerender } = render(view({}));
    await waitFor(() => expect(row(container, "web/")).toHaveAttribute("aria-expanded", "true"));
    await waitFor(() => row(container, "web/src/main.tsx"));

    act(() => row(container, "web/").click());
    await waitFor(() => expect(row(container, "web/")).toHaveAttribute("aria-expanded", "false"));
    // A path change (a listing arriving) must not undo the collapse.
    rerender(view({ paths: [...paths, "docs/"] }));
    await waitFor(() => row(container, "docs/"));
    expect(row(container, "web/")).toHaveAttribute("aria-expanded", "false");

    // Selecting another deep file is a new reveal request.
    rerender(
      view({ paths: [...paths, "docs/"], selectedPath: "web/src/app.tsx", revealKey: "open-2" })
    );
    await waitFor(() => expect(row(container, "web/")).toHaveAttribute("aria-expanded", "true"));
    await waitFor(() =>
      expect(row(container, "web/src/app.tsx")).toHaveAttribute("aria-selected", "true")
    );

    // So is "Reveal in tree" for the same file, after another collapse.
    act(() => row(container, "web/").click());
    await waitFor(() => expect(row(container, "web/")).toHaveAttribute("aria-expanded", "false"));
    rerender(
      view({
        paths: [...paths, "docs/"],
        selectedPath: "web/src/app.tsx",
        revealKey: "open-2:reveal",
      })
    );
    await waitFor(() => expect(row(container, "web/")).toHaveAttribute("aria-expanded", "true"));
  });

  it("Should render the empty slot when there are no paths", () => {
    const { getByText } = render(
      <FileTree aria-label="Project files" paths={[]} empty={<p>This folder is empty</p>} />
    );
    expect(getByText("This folder is empty")).toBeInTheDocument();
  });

  it("Should plan parents before children and collapse subtree removals", () => {
    expect(
      planPathSync(new Set(["a/", "a/b.ts", "a/c/", "z.ts"]), ["z.ts", "n/m.ts", "n/"])
    ).toEqual([
      { type: "remove", path: "a/", recursive: true },
      { type: "add", path: "n/" },
      { type: "add", path: "n/m.ts" },
    ]);
  });

  // US-001.AC-4: an unloaded changed directory gets Pierre's own dot through one placeholder.
  it("Should mark an unloaded changed directory with a placeholder descendant status", () => {
    expect(
      toGitStatusEntries({ "web/": { containsChanges: true }, "go.mod": { status: "modified" } })
    ).toEqual([
      { path: `web/${UNLOADED_CHANGES_ENTRY}`, status: "modified" },
      { path: "go.mod", status: "modified" },
    ]);
  });

  // Reduced motion: the loading mark keeps its shape but never spins (UI guide, motion).
  it("Should show a still loading mark under reduced motion", () => {
    expect(buildRowDecoration(undefined, true, true)).toEqual({
      icon: TREE_SPINNER_STATIC_ICON,
      title: "Loading",
    });
    expect(TREE_SPRITE_SHEET).toContain(`<symbol id="${TREE_SPINNER_STATIC_ICON}"`);
    const still = TREE_SPRITE_SHEET.slice(TREE_SPRITE_SHEET.indexOf(TREE_SPINNER_STATIC_ICON));
    expect(still.slice(0, still.indexOf("</symbol>"))).not.toContain("animate");
  });

  it("Should prefer the loading spinner, then counts, and never guess an unknown status", () => {
    expect(buildRowDecoration({ additions: 3 }, true)).toMatchObject({
      title: "Loading",
      icon: TREE_SPINNER_ICON,
    });
    expect(buildRowDecoration({ additions: 1200, deletions: 4 }, false)).toMatchObject({
      text: "+1,200 −4",
    });
    expect(buildRowDecoration({ status: "conflicted" }, false)).toMatchObject({ text: "C" });
    expect(buildRowDecoration({}, false)).toBeNull();
  });
});
