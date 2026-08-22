import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, within } from "storybook/test";

import { Toaster } from "@compozy/ui";

import { notifyUser } from "@/lib/user-feedback";
import { CenteredSurface } from "@/storybook/story-layout";

import { AttentionToast, AttentionToastOverflowLedge } from "../attention-toast";
import type { AttentionDelivery, AttentionDeliveryTarget } from "../../lib/attention-delivery";

function target(overrides: Partial<AttentionDeliveryTarget> = {}): AttentionDeliveryTarget {
  return {
    id: "sess-refactor",
    title: "Refactor session store",
    agentName: "claude",
    workspaceId: "ws-compozy",
    workspaceLabel: "compozy",
    badge: "waiting-for-input",
    reason: "Should I drop the legacy column?",
    ...overrides,
  };
}

function Stack({
  deliveries,
  overflowCount = 0,
}: {
  deliveries: AttentionDelivery[];
  overflowCount?: number;
}) {
  return (
    <div className="flex w-90 flex-col gap-2.5">
      {deliveries.map(delivery => (
        <AttentionToast delivery={delivery} key={delivery.key} onActivate={fn()} />
      ))}
      {overflowCount > 0 ? (
        <AttentionToastOverflowLedge count={overflowCount} onActivate={fn()} />
      ) : null}
    </div>
  );
}

const meta: Meta<typeof Stack> = {
  title: "systems/os/components/AttentionToast",
  component: Stack,
  parameters: {
    docs: {
      description: {
        component:
          "Attention toast content. The whole toast is the jump, so needs-you toasts carry no buttons; only the coalesced completion has an action, because its landing is a list rather than a session.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof Stack>;

/** The three kinds side by side: a question, a coalesced group, an agent's message. */
export const Stacked: Story = {
  args: {
    deliveries: [
      { kind: "needs-you", key: "a", target: target() },
      {
        kind: "finished",
        key: "b",
        targets: [
          target({ id: "s1", badge: "done", workspaceLabel: "compozy" }),
          target({ id: "s2", badge: "done", workspaceLabel: "infra" }),
          target({ id: "s3", badge: "done", workspaceLabel: "infra" }),
        ],
      },
      {
        kind: "agent",
        key: "c",
        notification: {
          profile_id: "00000000000000000000000000",
          profile_name: "default",
          notification_id: "ntf-1",
          session_id: "sess-payments",
          workspace_id: "ws-compozy",
          title: "Staging deploy is live",
          body: "Ledger row confirmed; ready for the go/no-go call.",
          at: "2026-07-20T18:12:00Z",
        },
        target: target({ id: "sess-payments", title: "Fix payment retries" }),
      },
    ],
  },
};

/** One tone, three glyphs: the needs-you class never relies on colour alone. */
export const NeedsYouVariants: Story = {
  args: {
    deliveries: [
      { kind: "needs-you", key: "input", target: target() },
      {
        kind: "needs-you",
        key: "auth",
        target: target({
          id: "sess-chargeback",
          title: "Chargeback flow",
          badge: "waiting-for-auth",
          reason: "Wants to run stripe trigger payment_intent.succeeded",
          workspaceLabel: "payments",
        }),
      },
      {
        kind: "needs-you",
        key: "failed",
        target: target({
          id: "sess-tf",
          title: "TF upgrade",
          agentName: "codex",
          badge: "failed",
          reason: "Exited with code 1 during terraform plan",
          workspaceLabel: "infra",
        }),
      },
    ],
  },
};

/** Four newest actions stay visible; older needs-you work folds into the bell ledge. */
export const Overflow: Story = {
  args: {
    deliveries: [
      { kind: "needs-you", key: "newest", target: target() },
      {
        kind: "needs-you",
        key: "auth",
        target: target({
          id: "sess-chargeback",
          title: "Chargeback flow",
          badge: "waiting-for-auth",
          reason: "Wants to run stripe trigger",
          workspaceLabel: "payments",
        }),
      },
      {
        kind: "needs-you",
        key: "failed",
        target: target({
          id: "sess-sandbox",
          title: "Sandbox rebuild",
          agentName: "codex",
          badge: "failed",
          reason: "Exited with code 137",
          workspaceLabel: "sandbox",
        }),
      },
      {
        kind: "needs-you",
        key: "docs",
        target: target({
          id: "sess-docs",
          title: "Docs sweep",
          agentName: "hermes",
          reason: "Keep the deprecated aliases page?",
          workspaceLabel: "agh-docs",
        }),
      },
    ],
    overflowCount: 2,
  },
};

/** A group of one collapses to the single form rather than saying "1 session". */
export const SingleCompletion: Story = {
  args: {
    deliveries: [
      {
        kind: "finished",
        key: "single",
        targets: [
          target({
            id: "sess-notes",
            title: "Release notes draft",
            agentName: "hermes",
            badge: "done",
          }),
        ],
      },
    ],
  },
};

/** A stale click lands safely and explains that the attention already cleared. */
export const ResolvedBeforeClick: Story = {
  args: { deliveries: [] },
  render: () => <Toaster duration={60_000} position="top-right" />,
  play: async ({ canvasElement }) => {
    notifyUser({ message: "This session no longer needs attention.", tone: "info" });
    const notice = await within(canvasElement.ownerDocument.body).findByText(
      "This session no longer needs attention."
    );
    await expect(notice).toBeInTheDocument();
  },
};
