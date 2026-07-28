import { useEffect, useRef, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { permissionRequestFixture, primarySessionFixture } from "@/systems/session/mocks";

import { PermissionPrompt } from "../permission-prompt";

const meta: Meta<typeof PermissionPrompt> = {
  title: "systems/session/components/PermissionPrompt",
  component: PermissionPrompt,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

type AutoDecision = "allow-once" | "reject-once" | null;

function requireStoryWorkspaceId(): string {
  if (!primarySessionFixture.workspace_id) {
    throw new Error("PermissionPrompt stories require a workspace_id fixture");
  }
  return primarySessionFixture.workspace_id;
}

const storyWorkspaceId = requireStoryWorkspaceId();

function PermissionPromptHarness({
  autoDecision,
  permission = permissionRequestFixture,
}: {
  autoDecision: AutoDecision;
  permission?: typeof permissionRequestFixture;
}) {
  const [resolvedState, setResolvedState] = useState<AutoDecision>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!autoDecision) {
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      const testId =
        autoDecision === "allow-once"
          ? "[data-testid='permission-allow-once']"
          : "[data-testid='permission-reject-once']";
      const button = containerRef.current?.querySelector<HTMLButtonElement>(testId);
      button?.click();
    });

    return () => window.cancelAnimationFrame(frame);
  }, [autoDecision]);

  return (
    <CenteredSurface className="flex-col gap-4">
      {resolvedState ? (
        <div className="w-full max-w-xl rounded-xl border border-line bg-canvas-soft px-4 py-3 text-sm text-fg">
          {resolvedState === "allow-once"
            ? "Permission approved for this turn."
            : "Permission rejected for this turn."}
        </div>
      ) : null}
      <div ref={containerRef} className="w-full max-w-xl">
        {resolvedState ? null : (
          <PermissionPrompt
            onResolved={() => setResolvedState(autoDecision ?? "allow-once")}
            permission={permission}
            sessionId={primarySessionFixture.id}
            workspaceId={storyWorkspaceId}
          />
        )}
      </div>
    </CenteredSurface>
  );
}

/** Default high-stakes prompt (Bash) — danger tile + tint */
export const HighStakesDanger: Story = {
  render: () => <PermissionPromptHarness autoDecision={null} />,
};

/** Lower-stakes prompt (TodoWrite) — warning tile + tint. */
export const StandardWarning: Story = {
  render: () => (
    <PermissionPromptHarness
      autoDecision={null}
      permission={{
        ...permissionRequestFixture,
        toolName: "TodoWrite",
        action: "update",
        resource: "agent todo list",
        toolInput: { items: ["draft prd", "ship change"] },
      }}
    />
  ),
};

/** Operator allows the prompt (transitions to resolved acknowledgement). */
export const Accepted: Story = {
  render: () => <PermissionPromptHarness autoDecision="allow-once" />,
};

/** Operator rejects the prompt (transitions to resolved acknowledgement). */
export const Rejected: Story = {
  render: () => <PermissionPromptHarness autoDecision="reject-once" />,
};
