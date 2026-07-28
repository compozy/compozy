import { Fragment } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { CpuIcon, DatabaseIcon, EyeIcon, MoreHorizontalIcon, ZapIcon } from "lucide-react";

import { Button } from "../button";
import { Pill } from "../custom/pill";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemSeparator,
  ItemSelectionIndicator,
  ItemTitle,
} from "../item";

const meta: Meta<typeof Item> = {
  title: "components/ui/Item",
  component: Item,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Compact list row with media, title, description, and actions slots. Wrap rows in ItemGroup for consistent spacing.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const agents = [
  {
    id: "claude",
    icon: CpuIcon,
    title: "Claude Code",
    description: "Primary orchestrator for long-running sessions.",
    badge: "active",
  },
  {
    id: "codex",
    icon: ZapIcon,
    title: "Codex CLI",
    description: "Fast fallback for single-shot refactors.",
    badge: "ready",
  },
  {
    id: "gemini",
    icon: DatabaseIcon,
    title: "Gemini CLI",
    description: "Bound to the knowledge retrieval workspace.",
    badge: "idle",
  },
];

export const InGroup: Story = {
  args: {},
  render: () => (
    <div className="w-lg">
      <ItemGroup>
        {agents.map((agent, index) => (
          <Fragment key={agent.id}>
            {index > 0 && <ItemSeparator />}
            <Item variant="outline">
              <ItemMedia variant="icon">
                <agent.icon />
              </ItemMedia>
              <ItemContent>
                <ItemTitle>
                  {agent.title}
                  <Pill tone="neutral">{agent.badge}</Pill>
                </ItemTitle>
                <ItemDescription>{agent.description}</ItemDescription>
              </ItemContent>
              <ItemActions>
                <Button variant="ghost" size="icon-sm" aria-label="Preview">
                  <EyeIcon />
                </Button>
                <Button variant="ghost" size="icon-sm" aria-label="More">
                  <MoreHorizontalIcon />
                </Button>
              </ItemActions>
            </Item>
          </Fragment>
        ))}
      </ItemGroup>
    </div>
  ),
};

export const Muted: Story = {
  args: {},
  render: () => (
    <div className="w-lg">
      <Item variant="muted">
        <ItemMedia variant="icon">
          <CpuIcon />
        </ItemMedia>
        <ItemContent>
          <ItemTitle>Claude Code</ItemTitle>
          <ItemDescription>Currently running the incident triage playbook.</ItemDescription>
        </ItemContent>
      </Item>
    </div>
  ),
};

export const Compact: Story = {
  args: {},
  render: () => (
    <div className="w-md">
      <ItemGroup>
        {agents.slice(0, 2).map(agent => (
          <Item key={agent.id} size="xs" variant="outline">
            <ItemMedia variant="icon">
              <agent.icon />
            </ItemMedia>
            <ItemContent>
              <ItemTitle>{agent.title}</ItemTitle>
            </ItemContent>
          </Item>
        ))}
      </ItemGroup>
    </div>
  ),
};

export const SelectableRail: Story = {
  args: {},
  render: () => (
    <div className="w-lg">
      <ItemGroup>
        {agents.slice(0, 2).map((agent, index) => (
          <Item
            key={agent.id}
            as="button"
            selectable
            selected={index === 0}
            indicator={index === 0 ? "rail" : "none"}
            className="rounded-none border-x-0 border-t-0 border-b border-line px-4 py-3"
          >
            <ItemMedia variant="icon">
              <agent.icon />
            </ItemMedia>
            <ItemContent>
              <ItemTitle>{agent.title}</ItemTitle>
              <ItemDescription>{agent.description}</ItemDescription>
            </ItemContent>
            <ItemActions>
              <Pill tone="neutral">{agent.badge}</Pill>
            </ItemActions>
          </Item>
        ))}
      </ItemGroup>
    </div>
  ),
};

export const SelectableDot: Story = {
  args: {},
  render: () => (
    <div className="w-lg">
      <Item as="button" selectable selected indicator="dot" variant="outline">
        <ItemContent>
          <ItemTitle>Queued review</ItemTitle>
          <ItemDescription>Dot indicator keeps the selection signal inline.</ItemDescription>
        </ItemContent>
        <ItemActions>
          <Pill tone="neutral">selected</Pill>
        </ItemActions>
      </Item>
    </div>
  ),
};

export const SelectedActive: Story = {
  args: {},
  render: () => (
    <div className="w-lg">
      <Item as="button" selectable selected variant="outline">
        <ItemSelectionIndicator kind="rail" />
        <ItemMedia variant="icon">
          <CpuIcon />
        </ItemMedia>
        <ItemContent>
          <ItemTitle>Claude Code</ItemTitle>
          <ItemDescription>
            Selected row with an explicitly composed indicator slot.
          </ItemDescription>
        </ItemContent>
      </Item>
    </div>
  ),
};
