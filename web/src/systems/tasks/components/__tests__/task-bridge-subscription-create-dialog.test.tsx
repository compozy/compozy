import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TaskBridgeSubscriptionCreateDialog } from "../task-bridge-subscription-create-dialog";

describe("TaskBridgeSubscriptionCreateDialog", () => {
  it("Should submit the authoritative task workspace id for workspace subscriptions", async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined);

    render(
      <TaskBridgeSubscriptionCreateDialog
        isPending={false}
        onCreate={onCreate}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws_authoritative"
      />
    );

    expect(screen.getByLabelText("Workspace id")).toHaveValue("ws_authoritative");
    expect(screen.getByLabelText("Workspace id")).toHaveAttribute("readonly");

    fireEvent.change(screen.getByTestId("tasks-bridges-instance"), {
      target: { value: "bridge_primary" },
    });
    fireEvent.click(screen.getByTestId("tasks-bridges-create-submit"));

    await waitFor(() =>
      expect(onCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          bridge_instance_id: "bridge_primary",
          scope: "workspace",
          workspace_id: "ws_authoritative",
        })
      )
    );
  });
});
