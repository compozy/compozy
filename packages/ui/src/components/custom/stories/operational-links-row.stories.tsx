import type { Meta, StoryObj } from "@storybook/react-vite";
import { ExternalLinkIcon, FileTextIcon, TerminalIcon } from "lucide-react";

import { OperationalLinksRow } from "../operational-links-row";

const meta: Meta<typeof OperationalLinksRow> = {
  title: "components/custom/OperationalLinksRow",
  component: OperationalLinksRow,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Compact navigation row of operational links — runbooks, dashboards, repository links. Renders as a `<nav>` with optional `aria-label`. Each link uses ghost hover (`--hover` + `--fg`); no underlines.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[640px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Common operator surface — three links mixed with internal + external destinations.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <OperationalLinksRow
      ariaLabel="Session shortcuts"
      items={[
        { label: "View logs", href: "#", icon: TerminalIcon },
        { label: "Runbook", href: "#", icon: FileTextIcon },
        {
          label: "Open in Grafana",
          href: "https://grafana.example",
          target: "_blank",
          icon: ExternalLinkIcon,
        },
      ]}
    />
  ),
};

/**
 * Single-link variant — confirms the row collapses to a tight chip without spacing artefacts.
 */
export const SingleLink: Story = {
  args: {},
  render: () => (
    <OperationalLinksRow items={[{ label: "Open inspector", href: "#", icon: TerminalIcon }]} />
  ),
};
