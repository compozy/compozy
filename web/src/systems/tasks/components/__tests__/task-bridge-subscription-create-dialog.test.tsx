import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TaskBridgeSubscriptionCreateDialog } from "../task-bridge-subscription-create-dialog";

describe("TaskBridgeSubscriptionCreateDialog", () => {
  it("Should submit every opaque identity byte exactly", async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined);

    render(
      <TaskBridgeSubscriptionCreateDialog
        isPending={false}
        onCreate={onCreate}
        onOpenChange={vi.fn()}
        open
        workspaceId=" ws_authoritative "
      />
    );

    expect(screen.getByLabelText("Workspace id")).toHaveValue(" ws_authoritative ");
    expect(screen.getByLabelText("Workspace id")).toHaveAttribute("readonly");

    fireEvent.change(screen.getByTestId("tasks-bridges-instance"), {
      target: { value: " bridge_primary " },
    });
    fireEvent.change(screen.getByLabelText("Peer id (optional)"), {
      target: { value: " peer_primary " },
    });
    fireEvent.change(screen.getByLabelText("Group id (optional)"), {
      target: { value: " group_primary " },
    });
    fireEvent.change(screen.getByLabelText("Thread id (optional)"), {
      target: { value: " thread_primary " },
    });
    fireEvent.change(screen.getByLabelText("Subscription id (optional)"), {
      target: { value: " subscription_primary " },
    });
    fireEvent.click(screen.getByTestId("tasks-bridges-create-submit"));

    await waitFor(() =>
      expect(onCreate).toHaveBeenCalledWith({
        bridge_instance_id: " bridge_primary ",
        delivery_mode: "direct-send",
        scope: "workspace",
        workspace_id: " ws_authoritative ",
        peer_id: " peer_primary ",
        group_id: " group_primary ",
        thread_id: " thread_primary ",
        subscription_id: " subscription_primary ",
      })
    );
  });
});
