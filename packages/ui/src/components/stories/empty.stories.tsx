import type { Meta, StoryObj } from "@storybook/react-vite";
import { InboxIcon, PlusIcon, SearchIcon } from "lucide-react";

import { Button } from "../button";
import { Empty } from "../empty";

const meta: Meta<typeof Empty> = {
  title: "components/ui/Empty",
  component: Empty,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Empty state with an icon container, muted title, optional description, cause, and actions. `framed` is the bordered card variant for routed empty/error states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const TitleOnly: Story = {
  render: () => (
    <div className="w-[420px]">
      <Empty title="Nothing here yet" />
    </div>
  ),
};

export const WithDescription: Story = {
  render: () => (
    <div className="w-[420px]">
      <Empty
        icon={SearchIcon}
        title="Nothing matches"
        description="Adjust the search or clear filters to see more results."
      />
    </div>
  ),
};

export const WithAction: Story = {
  render: () => (
    <div className="w-[420px]">
      <Empty
        icon={InboxIcon}
        title="Your inbox is empty"
        description="Approval requests, failed runs, and blockers appear here."
        action={
          <Button size="sm" type="button">
            <PlusIcon className="size-3" />
            New task
          </Button>
        }
      />
    </div>
  ),
};

export const Framed: Story = {
  render: () => (
    <div className="w-[420px]">
      <Empty
        framed
        titleAs="h2"
        icon={SearchIcon}
        title="Unable to load this view"
        description="The source did not respond. Retry, or come back in a moment."
        action={
          <Button size="sm" type="button">
            Retry
          </Button>
        }
      />
    </div>
  ),
};

export const WithCause: Story = {
  render: () => (
    <div className="w-[420px]">
      <Empty
        framed
        titleAs="h2"
        icon={SearchIcon}
        title="The catalog is incomplete"
        description="Installed status is unavailable until the catalog can be checked."
        cause="marketplace: continuation token expired (409)"
      />
    </div>
  ),
};

/** Empty with default fill inside a tall flex column — mirrors ListingPage remaining height. */
export const FillRemaining: Story = {
  parameters: { layout: "fullscreen" },
  render: () => (
    <div className="flex h-screen w-full flex-col">
      <div className="shrink-0 border-b border-line px-9 py-4 text-sm text-muted">
        Page head + toolbar
      </div>
      <div className="flex min-h-0 flex-1 flex-col px-9 pb-20 pt-7">
        <Empty
          action={
            <Button size="sm" type="button">
              <PlusIcon className="size-3" />
              Create job
            </Button>
          }
          description="Create your first job to run a configured target on a schedule."
          icon={InboxIcon}
          title="No jobs yet"
        />
      </div>
    </div>
  ),
};
