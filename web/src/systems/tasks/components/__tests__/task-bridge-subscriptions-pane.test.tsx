import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { taskBridgeNotificationSubscriptionFixture } from "../../mocks/fixtures";
import { TaskBridgeSubscriptionsPane } from "../task-bridge-subscriptions-pane";

describe("TaskBridgeSubscriptionsPane", () => {
  it("Should keep a load error visible beside stale subscription rows", () => {
    render(
      <TaskBridgeSubscriptionsPane
        errorMessage="Refresh failed"
        onCreate={vi.fn()}
        onDelete={vi.fn()}
        subscriptions={[taskBridgeNotificationSubscriptionFixture]}
        workspaceId="ws_alpha"
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Refresh failed");
    expect(
      screen.getByTestId(
        `tasks-bridges-row-${taskBridgeNotificationSubscriptionFixture.subscription_id}`
      )
    ).toBeInTheDocument();
  });

  it("Should open the create dialog and delegate its validated request", async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined);
    render(
      <TaskBridgeSubscriptionsPane
        onCreate={onCreate}
        onDelete={vi.fn()}
        subscriptions={[]}
        workspaceId="ws_alpha"
      />
    );

    fireEvent.click(screen.getByTestId("tasks-bridges-add"));
    fireEvent.change(screen.getByTestId("tasks-bridges-instance"), {
      target: { value: "bridge_primary" },
    });
    fireEvent.click(screen.getByTestId("tasks-bridges-create-submit"));

    await waitFor(() => expect(onCreate).toHaveBeenCalledTimes(1));
  });

  it("Should delete the selected subscription only after confirmation", async () => {
    const onDelete = vi.fn().mockResolvedValue(undefined);
    render(
      <TaskBridgeSubscriptionsPane
        onCreate={vi.fn()}
        onDelete={onDelete}
        subscriptions={[taskBridgeNotificationSubscriptionFixture]}
        workspaceId="ws_alpha"
      />
    );

    fireEvent.click(
      screen.getByRole("button", {
        name: `Remove subscription ${taskBridgeNotificationSubscriptionFixture.subscription_id}`,
      })
    );
    expect(onDelete).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Remove subscription" }));

    await waitFor(() =>
      expect(onDelete).toHaveBeenCalledWith(
        taskBridgeNotificationSubscriptionFixture.subscription_id
      )
    );
  });
});
