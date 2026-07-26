import type { Meta, StoryObj } from "@storybook/react-vite";

import { MonoId } from "../mono-id";

const meta: Meta<typeof MonoId> = {
  title: "components/custom/MonoId",
  component: MonoId,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Bare lowercase mono identifier (no Pill chrome) wave-2. Replaces 30+ inline `font-mono lowercase tracking-0` tuples; supersedes `<Pill mono>` for identifier contexts.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Default size — 10.5 px / `--faint`. */
export const Default: Story = {
  args: { value: "run_AbC123" },
};

/** Small size — 10 px. */
export const Small: Story = {
  args: { value: "run_AbC123", size: "sm" },
};

/** With the inline copy affordance. */
export const Copy: Story = {
  args: { value: "task_4QzPnzdNiF", copy: true },
};

/** Case-sensitive identifier (e.g. a Vault ref) — rendered and copied byte-for-byte. */
export const PreserveCase: Story = {
  args: {
    value: "vault:mcp/ws/ws-platform/github-local/env/QA_TYPED_TOKEN",
    preserveCase: true,
    copy: true,
  },
};

/** Inline with surrounding mono text. */
export const Inline: Story = {
  render: () => (
    <div className="flex items-center gap-2 text-[12px] text-muted">
      <span>run</span>
      <MonoId value="run_4qzpnzdnif" />
      <span>started 3m ago</span>
    </div>
  ),
};
