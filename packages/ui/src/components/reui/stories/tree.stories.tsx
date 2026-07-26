import type { Meta, StoryObj } from "@storybook/react-vite";
import { selectionFeature, syncDataLoaderFeature } from "@headless-tree/core";
import { useTree } from "@headless-tree/react";

import { Tree, TreeItem, TreeItemLabel } from "../tree";

interface DemoTreeItem {
  kind: "root" | "folder" | "leaf";
  label: string;
}

const demoData: Record<string, DemoTreeItem> = {
  root: { kind: "root", label: "" },
  marketing: { kind: "folder", label: "Marketing" },
  sales: { kind: "folder", label: "Sales" },
  "growth-agent": { kind: "leaf", label: "growth-agent" },
  "sales-agent": { kind: "leaf", label: "sales-agent" },
};

const demoChildren: Record<string, string[]> = {
  root: ["marketing"],
  marketing: ["sales", "growth-agent"],
  sales: ["sales-agent"],
  "growth-agent": [],
  "sales-agent": [],
};

function TreeDemo() {
  const tree = useTree<DemoTreeItem>({
    rootItemId: "root",
    getItemName: item => item.getItemData().label,
    isItemFolder: item => item.getItemData().kind === "folder",
    initialState: { expandedItems: ["marketing", "sales"] },
    dataLoader: {
      getItem: id => demoData[id] ?? { kind: "leaf", label: "" },
      getChildren: id => demoChildren[id] ?? [],
    },
    features: [syncDataLoaderFeature, selectionFeature],
  });

  return (
    <div className="w-72 rounded-md border border-border bg-background p-2">
      <Tree tree={tree} aria-label="Agent categories" className="gap-0.5">
        {tree.getItems().map(item => {
          const data = item.getItemData();
          if (data.kind === "root") return null;
          return (
            <TreeItem key={item.getId()} item={item}>
              <TreeItemLabel item={item}>{data.label}</TreeItemLabel>
            </TreeItem>
          );
        })}
      </Tree>
    </div>
  );
}

const meta: Meta<typeof TreeDemo> = {
  title: "components/reui/Tree",
  component: TreeDemo,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Headless tree primitives for categorized navigation. Pair with `@headless-tree/react` `useTree`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Expanded folder hierarchy with leaf rows.
 */
export const Default: Story = {
  args: {},
  render: () => <TreeDemo />,
};
