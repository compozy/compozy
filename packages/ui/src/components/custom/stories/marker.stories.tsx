import type { Meta, StoryObj } from "@storybook/react-vite";
import { AlertTriangleIcon, InfoIcon, RefreshCwIcon, TargetIcon } from "lucide-react";

import { Marker, MarkerMeta } from "../marker";

const meta: Meta<typeof Marker> = {
  title: "components/custom/Marker",
  component: Marker,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "One-line system event for the session transcript — replaces tinted Alert cards. A 12px tone glyph plus one muted sentence; tone lives in the glyph only, `<b>` carries the subject, `MarkerMeta` carries raw kind strings and ×N cluster counts in faint sans — wrap the child in `<MonoId>` when the reader has to match an identifier exactly.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    icon: <InfoIcon />,
    tone: "info",
    children: (
      <>
        <b>Session resumed</b> — transcript restored from the daemon.
      </>
    ),
  },
};

export const Tones: Story = {
  render: () => (
    <div className="flex max-w-xl flex-col gap-1">
      <Marker tone="neutral" icon={<RefreshCwIcon />}>
        <b>Compaction complete</b> — 41% of context reclaimed.
      </Marker>
      <Marker tone="info" icon={<TargetIcon />}>
        <b>Goal continuation</b> <MarkerMeta>generation 2 · turn 3</MarkerMeta>
      </Marker>
      <Marker tone="warning" icon={<AlertTriangleIcon />}>
        <b>Context 41% full</b> — no compaction needed yet.
      </Marker>
      <Marker tone="danger" icon={<AlertTriangleIcon />}>
        <b>Provider request failed</b> — the runtime is retrying.
      </Marker>
    </div>
  ),
};

export const Clustered: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Consecutive same-kind events merge into one marker; the ×N count rides the sentence as faint meta.",
      },
    },
  },
  args: {
    icon: <AlertTriangleIcon />,
    tone: "warning",
    children: (
      <>
        <b>Provider overloaded</b> — retried, run continuing. <MarkerMeta>×3</MarkerMeta>
      </>
    ),
  },
};

export const RawKindMeta: Story = {
  parameters: {
    docs: {
      description: {
        story: "Unknown runtime events keep their raw kind string as faint meta — never a pill.",
      },
    },
  },
  args: {
    icon: <InfoIcon />,
    children: (
      <>
        Runtime event <MarkerMeta>agent.sandbox.escalation</MarkerMeta>
      </>
    ),
  },
};
