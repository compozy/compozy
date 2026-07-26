import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { NetworkCoordinationInvitation } from "../coordination-invitation";
import { TaskRunCoordinationInvitationHost } from "../task-run-coordination-invitation-host";

const coordinationHooks = vi.hoisted(() => ({
  acceptMutate: vi.fn(),
  dismissMutate: vi.fn(),
  useAccept: vi.fn(),
  useCoordination: vi.fn(),
  useDismiss: vi.fn(),
}));

vi.mock("../../hooks/use-network-coordination", () => ({
  useAcceptNetworkCoordinationInvitation: coordinationHooks.useAccept,
  useDismissNetworkCoordinationInvitation: coordinationHooks.useDismiss,
  useNetworkCoordination: coordinationHooks.useCoordination,
}));

describe("NetworkCoordinationInvitation interactions", () => {
  it("Should submit only once when two accept clicks happen before React rerenders", () => {
    coordinationHooks.acceptMutate.mockReset();
    coordinationHooks.dismissMutate.mockReset();
    coordinationHooks.useCoordination.mockReturnValue({
      data: { eligibility: { eligible: true }, revision: 7 },
    });
    coordinationHooks.useAccept.mockReturnValue({
      error: null,
      isPending: false,
      mutate: coordinationHooks.acceptMutate,
    });
    coordinationHooks.useDismiss.mockReturnValue({
      error: null,
      isPending: false,
      mutate: coordinationHooks.dismissMutate,
    });

    render(
      <TaskRunCoordinationInvitationHost runId="run-1" taskId="task-1" workspaceId="ws-run-owner" />
    );
    const accept = screen.getByTestId("network-coordination-invitation-accept");
    fireEvent.click(accept);
    fireEvent.click(accept);

    expect(coordinationHooks.acceptMutate).toHaveBeenCalledTimes(1);
    expect(coordinationHooks.acceptMutate).toHaveBeenCalledWith(7, {
      onSettled: expect.any(Function),
    });
    expect(coordinationHooks.useCoordination).toHaveBeenCalledWith("ws-run-owner", {
      scope: "task",
      taskId: "task-1",
      runId: "run-1",
    });
    expect(coordinationHooks.useAccept).toHaveBeenCalledWith("ws-run-owner", {
      scope: "task",
      taskId: "task-1",
      runId: "run-1",
    });
    expect(coordinationHooks.useDismiss).toHaveBeenCalledWith("ws-run-owner", {
      scope: "task",
      taskId: "task-1",
      runId: "run-1",
    });
    fireEvent.click(screen.getByTestId("network-coordination-invitation-accept"));
    expect(coordinationHooks.acceptMutate).toHaveBeenCalledTimes(1);
    expect(
      screen.getByText(/Turn on coordination conversations for future runs/i)
    ).toBeInTheDocument();
  });

  it("Should expose mutation failures with retry guidance", () => {
    render(
      <NetworkCoordinationInvitation
        errorMessage="Revision conflict."
        onAccept={() => undefined}
        onDismiss={() => undefined}
        visible
      />
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Revision conflict. You can try again.");
  });

  it("Should hide when not visible so terminal runs can drop the invite gracefully", () => {
    const { container } = render(
      <NetworkCoordinationInvitation
        onAccept={() => undefined}
        onDismiss={() => undefined}
        visible={false}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });
});
