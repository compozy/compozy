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
          'Empty state with an icon well, title, and optional description, hint, cause, actions, and starter next steps. `illustration` replaces the icon well when a state earns real art. A raw `cause` stays collapsed behind a "Details" disclosure so an error reads as a sentence first. `framed` is the bordered card variant for routed empty/error states.',
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
  parameters: {
    docs: {
      description: {
        story:
          "The raw cause ships collapsed. The reader gets a plain sentence; the machine string is one keyboard-reachable step deeper.",
      },
    },
  },
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

export const WithHintAndNextSteps: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "First-run shape: one heading, one line of orientation, a primary action, and starter actions that already exist.",
      },
    },
  },
  render: () => (
    <div className="w-[480px]">
      <Empty
        icon={InboxIcon}
        title="Nothing running yet"
        description="Agents you start show up here."
        hint="Sessions keep running after you close the tab."
        action={
          <Button size="sm" type="button">
            <PlusIcon className="size-3" />
            Start a session
          </Button>
        }
        nextSteps={
          <>
            <Button size="sm" type="button" variant="ghost">
              Create a task
            </Button>
            <Button size="sm" type="button" variant="ghost">
              Browse the marketplace
            </Button>
          </>
        }
      />
    </div>
  ),
};

export const WithIllustration: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "`illustration` is an art slot above the icon well for states that earn real art; the icon well stays. Keep the art geometric.",
      },
    },
  },
  render: () => (
    <div className="w-[420px]">
      <Empty
        illustration={
          <svg width="96" height="64" viewBox="0 0 96 64" fill="none">
            <rect
              x="0.5"
              y="0.5"
              width="95"
              height="63"
              rx="11.5"
              className="stroke-line"
              fill="var(--color-canvas-soft)"
            />
            <rect x="16" y="20" width="64" height="4" rx="2" className="fill-line-strong" />
            <rect x="16" y="32" width="44" height="4" rx="2" className="fill-line" />
            <rect x="16" y="44" width="28" height="4" rx="2" className="fill-line" />
          </svg>
        }
        title="No history yet"
        description="Everything you and your agents do gets saved here."
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
