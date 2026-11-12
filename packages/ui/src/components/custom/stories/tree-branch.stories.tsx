import type { Meta, StoryObj } from "@storybook/react-vite";

import { FolderIcon, GitBranchIcon } from "../../../icons";
import { Command, CommandGroup, CommandItem, CommandList } from "../../command";
import { TreeBranch, TreeBranchNode } from "../tree-branch";

const meta: Meta<typeof TreeBranch> = {
  title: "components/custom/TreeBranch",
  component: TreeBranch,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "The branched tree connector for nested rows: `TreeBranch` is the rail under a parent's leading mark, each `TreeBranchNode` draws its elbow into that rail (├) and the last rendered node closes it (└). Child sessions, subagents, project folders, and the terminal picker's worktrees all hang from it.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const ROW = "flex min-h-8 items-center gap-2 rounded-md px-1.5 py-1.5 text-small-body text-fg";

export const Default: Story = {
  render: () => (
    <div className="flex w-72 flex-col">
      <div className={ROW}>
        <FolderIcon aria-hidden="true" className="size-4 text-subtle" />
        atlas
      </div>
      <TreeBranch>
        {["feature-login", "fix-retry", "docs-refresh"].map(name => (
          <TreeBranchNode className={ROW} key={name}>
            <GitBranchIcon aria-hidden="true" className="size-4 text-subtle" />
            {name}
          </TreeBranchNode>
        ))}
      </TreeBranch>
    </div>
  ),
};

export const HeaderArm: Story = {
  parameters: {
    docs: {
      description: {
        story:
          '`arm="header"` lands the elbow on a 28 px header row, such as a project inside a project folder.',
      },
    },
  },
  render: () => (
    <div className="flex w-72 flex-col">
      <div className="flex h-7 items-center px-1.5 text-small-body text-fg">Client work</div>
      <TreeBranch>
        {["atlas", "beacon"].map(name => (
          <TreeBranchNode
            arm="header"
            className="flex h-7 items-center px-1.5 text-small-body text-fg"
            key={name}
          >
            {name}
          </TreeBranchNode>
        ))}
      </TreeBranch>
    </div>
  ),
};

export const OnCommandItems: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "`render` draws the connector on the command item itself, so a filtered item takes its elbow with it and the branch still closes on the last visible row.",
      },
    },
  },
  render: () => (
    <Command className="w-80 border border-line">
      <CommandList>
        <CommandGroup heading="New terminal">
          <CommandItem value="atlas">
            <FolderIcon aria-hidden="true" />
            atlas
          </CommandItem>
          <TreeBranch>
            {["feature-login", "fix-retry"].map(name => (
              <TreeBranchNode key={name} render={<CommandItem value={`atlas/${name}`} />}>
                <GitBranchIcon aria-hidden="true" />
                {name}
              </TreeBranchNode>
            ))}
          </TreeBranch>
        </CommandGroup>
      </CommandList>
    </Command>
  ),
};
