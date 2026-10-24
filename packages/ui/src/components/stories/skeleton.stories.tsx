import type { Meta, StoryObj } from "@storybook/react-vite";

import { Eyebrow } from "../custom/eyebrow";
import { Skeleton, SkeletonRegion, SkeletonRows, SkeletonText } from "../skeleton";

const meta: Meta<typeof Skeleton> = {
  title: "components/ui/Skeleton",
  component: Skeleton,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Placeholder block for content that is still loading. Motion is compositor-only (a swept highlight on `transform`, a fade on `opacity`) and stills under reduced motion. A block lays out at once but stays transparent for one `base` beat before it fades in, so a load that settles quickly never flashes a placeholder. Reach for the content-shaped primitives (`SkeletonText`, `ListingRowSkeleton`, …) before sizing bare blocks, and wrap the loading area in one `SkeletonRegion`, which carries the status semantics.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[360px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { className: "h-5 w-48" },
};

export const ListRow: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-3">
      <Skeleton className="size-10 rounded-full" />
      <div className="flex flex-col gap-2">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-3 w-24" />
      </div>
    </div>
  ),
};

export const Rows: Story = {
  args: {},
  render: () => (
    <SkeletonRows count={4} rowClassName="border-b border-border px-4 py-3">
      <Skeleton className="h-3.5 w-2/3" />
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-3/4" />
    </SkeletonRows>
  ),
};

/** One status region announces the whole area; the placeholders inside stay silent. */
export const Region: Story = {
  args: {},
  render: () => (
    <SkeletonRegion label="Loading activity">
      <SkeletonRows count={2} rowClassName="px-1 py-3" />
    </SkeletonRegion>
  ),
};

const SAMPLE = "Checks release notes, version policy and operator-facing docs before a tag.";

/**
 * Each placeholder line is one line box of the type it replaces, so its height equals the real
 * text's. Skeleton on the left, the text it stands in for on the right.
 */
export const Text: Story = {
  args: {},
  decorators: [
    Story => (
      <div className="w-[640px]">
        <Story />
      </div>
    ),
  ],
  render: () => (
    <div className="grid grid-cols-2 gap-x-6 gap-y-5">
      <Eyebrow>Skeleton</Eyebrow>
      <Eyebrow>Text</Eyebrow>
      <SkeletonText className="text-card-title" />
      <p className="text-card-title font-medium text-fg-strong">release-auditor</p>
      <SkeletonText className="text-small-body" lines={2} />
      <p className="text-small-body text-muted">{SAMPLE}</p>
      <SkeletonText className="text-eyebrow" />
      <p className="text-eyebrow text-faint">updated 2h ago</p>
    </div>
  ),
};
