import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { DirectoryBrowser } from "../directory-browser";
import type { FSEntry } from "../../types";

const meta: Meta<typeof DirectoryBrowser> = {
  title: "systems/onboarding/components/DirectoryBrowser",
  component: DirectoryBrowser,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Filesystem root picker. Presentational by contract: it reports a pick and never persists one, so a dialog can gate registration behind its own primary action.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const entries: FSEntry[] = [
  { name: "agh-extensions", path: "/Users/you/Dev/agh-extensions", is_dir: true },
  { name: "billing", path: "/Users/you/Dev/billing", is_dir: true },
  { name: "checkout-platform", path: "/Users/you/Dev/checkout-platform", is_dir: true },
  { name: "design-tokens", path: "/Users/you/Dev/design-tokens", is_dir: true },
  { name: "growth-experiments", path: "/Users/you/Dev/growth-experiments", is_dir: true },
  { name: "shared-libs", path: "/Users/you/Dev/shared-libs", is_dir: true },
];

const base = {
  currentPath: "/Users/you/Dev",
  parentPath: "/Users/you",
  homePath: "/Users/you",
  entries,
  isBrowsing: false,
  browseError: null,
  onNavigate: () => undefined,
  onGoParent: () => undefined,
  onGoHome: () => undefined,
  onPick: () => undefined,
  isPicked: () => false,
  testIdPrefix: "directory-browser",
} satisfies React.ComponentProps<typeof DirectoryBrowser>;

function harness(props: React.ComponentProps<typeof DirectoryBrowser>) {
  return (
    <StorySurface className="p-10">
      <div className="flex h-96 w-[520px] flex-col">
        <DirectoryBrowser {...props} />
      </div>
    </StorySurface>
  );
}

/** Populated: sub-folders listed, current path in the mono toolbar. */
export const Populated: Story = {
  args: {},
  render: () => harness(base),
};

/** Reading: the daemon browse is in flight. */
export const Loading: Story = {
  args: {},
  render: () => harness({ ...base, entries: [], isBrowsing: true }),
};

/** Empty: the folder has no sub-folders, but it can still be chosen. */
export const Empty: Story = {
  args: {},
  render: () => harness({ ...base, entries: [] }),
};

/** Error: the daemon refused the browse; the row list is replaced by the reason. */
export const ErrorState: Story = {
  args: {},
  render: () =>
    harness({
      ...base,
      entries: [],
      browseError: "permission denied: /Users/you/Dev/private",
    }),
};

/** Already picked: the current folder is the selected root, so its pick is disabled. */
export const CurrentFolderPicked: Story = {
  args: {},
  render: () => harness({ ...base, isPicked: path => path === "/Users/you/Dev" }),
};
