import type { HTMLAttributes } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { hotkeysCoreFeature, selectionFeature, syncDataLoaderFeature } from "@headless-tree/core";
import { useTree } from "@headless-tree/react";
import { describe, expect, expectTypeOf, it, vi } from "vitest";

import type { ItemInstance } from "@headless-tree/core";

import {
  Tree,
  TreeItem,
  TreeItemLabel,
  type TreeDragLineProps,
  type TreeItemLabelProps,
  type TreeItemProps,
  type TreeProps,
} from "../../../index";

interface TestTreeItem {
  kind: "root" | "folder" | "leaf";
  label: string;
}

/**
 * Three levels, because the primitive's whole job at depth is the indent: a
 * two-level fixture cannot tell a per-level offset from a flat one.
 */
const testData: Record<string, TestTreeItem> = {
  root: { kind: "root", label: "" },
  folder: { kind: "folder", label: "Marketing" },
  subfolder: { kind: "folder", label: "Campaigns" },
  leaf: { kind: "leaf", label: "Sales agent" },
  deepLeaf: { kind: "leaf", label: "Launch agent" },
};

const testChildren: Record<string, string[]> = {
  root: ["folder"],
  folder: ["subfolder", "leaf"],
  subfolder: ["deepLeaf"],
  leaf: [],
  deepLeaf: [],
};

function TreeHarness({
  treeProps,
  itemProps,
  features = [syncDataLoaderFeature],
  expandedItems = ["folder", "subfolder"],
  itemChildren = testChildren,
}: {
  treeProps?: Partial<TreeProps<TestTreeItem>>;
  itemProps?: Partial<TreeItemProps<TestTreeItem>>;
  features?: Parameters<typeof useTree<TestTreeItem>>[0]["features"];
  expandedItems?: string[];
  itemChildren?: Record<string, string[]>;
} = {}) {
  const tree = useTree<TestTreeItem>({
    rootItemId: "root",
    getItemName: item => item.getItemData().label,
    isItemFolder: item => item.getItemData().kind === "folder",
    initialState: { expandedItems },
    dataLoader: {
      getItem: id => testData[id] ?? { kind: "leaf", label: "" },
      getChildren: id => itemChildren[id] ?? [],
    },
    features,
  });

  return (
    <Tree tree={tree} aria-label="Tree test" {...treeProps}>
      {tree.getItems().map(item => {
        const data = item.getItemData();
        if (data.kind === "root") return null;
        return (
          <TreeItem
            key={item.getId()}
            item={item}
            data-testid={`tree-item-${item.getId()}`}
            {...itemProps}
          >
            <TreeItemLabel item={item}>{data.label}</TreeItemLabel>
          </TreeItem>
        );
      })}
    </Tree>
  );
}

describe("Tree", () => {
  it("Should expose tree prop types from the public entrypoint", () => {
    expectTypeOf<TreeProps<TestTreeItem>>().toMatchTypeOf<{ tree: object }>();
    expectTypeOf<TreeItemProps<TestTreeItem>>().toMatchTypeOf<{
      item: ItemInstance<TestTreeItem>;
    }>();
    expectTypeOf<TreeItemLabelProps<TestTreeItem>>().toMatchTypeOf<{
      item?: ItemInstance<TestTreeItem>;
    }>();
    expectTypeOf<TreeDragLineProps<TestTreeItem>>().toMatchTypeOf<HTMLAttributes<HTMLDivElement>>();
  });

  it("Should emit aria-expanded for folders only", () => {
    render(<TreeHarness />);

    expect(screen.getByTestId("tree-item-folder")).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByTestId("tree-item-leaf")).not.toHaveAttribute("aria-expanded");
  });

  it("Should ignore missing optional tree features without warning", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    try {
      render(<TreeHarness />);
      expect(warn).not.toHaveBeenCalled();
    } finally {
      warn.mockRestore();
    }
  });

  it("Should preserve caller click handlers on tree items", () => {
    const onClick = vi.fn();
    render(<TreeHarness itemProps={{ onClick }} />);

    fireEvent.click(screen.getByTestId("tree-item-folder"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("Should default tree items to type=button", () => {
    render(<TreeHarness />);

    expect(screen.getByTestId("tree-item-folder")).toHaveAttribute("type", "button");
  });

  it("Should not leak button-only attributes onto a custom item element", () => {
    render(<TreeHarness itemProps={{ render: <div /> }} />);

    const folder = screen.getByTestId("tree-item-folder");
    expect(folder.tagName).toBe("DIV");
    expect(folder).not.toHaveAttribute("type");
  });

  it("Should offset each level by the indent so depth is visible, not implied", () => {
    render(<TreeHarness />);

    // The indent rides a CSS variable rather than a class, so it is the only
    // thing a consumer can rely on to show delegation depth.
    expect(screen.getByTestId("tree-item-folder")).toHaveStyle({ "--tree-padding": "0px" });
    expect(screen.getByTestId("tree-item-subfolder")).toHaveStyle({ "--tree-padding": "20px" });
    expect(screen.getByTestId("tree-item-deepLeaf")).toHaveStyle({ "--tree-padding": "40px" });
  });

  it("Should honour a caller's indent for every level", () => {
    render(<TreeHarness treeProps={{ indent: 12 }} />);

    expect(screen.getByTestId("tree-item-subfolder")).toHaveStyle({ "--tree-padding": "12px" });
    expect(screen.getByTestId("tree-item-deepLeaf")).toHaveStyle({ "--tree-padding": "24px" });
  });

  it("Should hide a collapsed folder's descendants from the rendered list", () => {
    render(<TreeHarness expandedItems={["folder"]} />);

    expect(screen.getByTestId("tree-item-subfolder")).toHaveAttribute("aria-expanded", "false");
    // Collapsed means genuinely absent, not visually hidden — assistive tech and
    // find-in-page both agree with the eye.
    expect(screen.queryByTestId("tree-item-deepLeaf")).not.toBeInTheDocument();
  });

  it("Should expose a roving tabindex so the tree is one tab stop", () => {
    render(
      <TreeHarness features={[syncDataLoaderFeature, selectionFeature, hotkeysCoreFeature]} />
    );

    const items = [
      screen.getByTestId("tree-item-folder"),
      screen.getByTestId("tree-item-subfolder"),
      screen.getByTestId("tree-item-leaf"),
      screen.getByTestId("tree-item-deepLeaf"),
    ];
    const focusable = items.filter(item => item.getAttribute("tabindex") !== "-1");
    expect(focusable).toHaveLength(1);
  });

  it("Should expose no tree-item tab stop when the tree is empty", () => {
    render(<TreeHarness itemChildren={{ ...testChildren, root: [] }} />);

    expect(screen.queryAllByRole("treeitem")).toHaveLength(0);
  });

  it("Should mark the selected item so a consumer can style it without local state", () => {
    render(<TreeHarness features={[syncDataLoaderFeature, selectionFeature]} />);

    const item = screen.getByTestId("tree-item-leaf");
    expect(item).toHaveAttribute("data-selected", "false");
    fireEvent.click(item);
    expect(screen.getByTestId("tree-item-leaf")).toHaveAttribute("data-selected", "true");
  });

  it("Should name the tree for assistive tech", () => {
    render(<TreeHarness />);

    expect(screen.getByLabelText("Tree test")).toBeInTheDocument();
  });

  it("Should fall back to the item name when no label children are given", () => {
    render(<LabelFallbackHarness />);

    expect(screen.getByText("Marketing")).toBeInTheDocument();
  });
});

/** A consumer that relies on `getItemName` instead of passing label children. */
function LabelFallbackHarness() {
  const tree = useTree<TestTreeItem>({
    rootItemId: "root",
    getItemName: item => item.getItemData().label,
    isItemFolder: item => item.getItemData().kind === "folder",
    initialState: { expandedItems: ["folder"] },
    dataLoader: {
      getItem: id => testData[id] ?? { kind: "leaf", label: "" },
      getChildren: id => testChildren[id] ?? [],
    },
    features: [syncDataLoaderFeature],
  });

  return (
    <Tree tree={tree} aria-label="Fallback tree">
      {tree.getItems().map(item =>
        item.getItemData().kind === "root" ? null : (
          <TreeItem key={item.getId()} item={item}>
            <TreeItemLabel item={item} />
          </TreeItem>
        )
      )}
    </Tree>
  );
}
