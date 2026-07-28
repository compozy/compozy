import type { HTMLAttributes } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { syncDataLoaderFeature } from "@headless-tree/core";
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

const testData: Record<string, TestTreeItem> = {
  root: { kind: "root", label: "" },
  folder: { kind: "folder", label: "Marketing" },
  leaf: { kind: "leaf", label: "Sales agent" },
};

const testChildren: Record<string, string[]> = {
  root: ["folder"],
  folder: ["leaf"],
  leaf: [],
};

function TreeHarness({
  treeProps,
  itemProps,
}: {
  treeProps?: Partial<TreeProps<TestTreeItem>>;
  itemProps?: Partial<TreeItemProps<TestTreeItem>>;
} = {}) {
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
});
