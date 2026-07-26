import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

vi.mock("../../adapters/session-api", () => ({
  approveSession: vi.fn(),
}));

import { toast } from "sonner";

import { approveSession } from "../../adapters/session-api";
import type { AghPermissionData, PermissionRequest } from "../../types";
import { PermissionDataPart, PermissionPrompt } from "../permission-prompt";

const mockPermission: PermissionRequest = {
  requestId: "req-123",
  toolName: "Bash",
  action: "execute",
  resource: "rm -rf /tmp/test",
  toolInput: { command: "rm -rf /tmp/test" },
};

const mockPermissionData: AghPermissionData = {
  type: "permission",
  session_id: "sess-001",
  turn_id: "turn-001",
  request_id: "req-123",
  title: "Bash",
  action: "execute",
  resource: "rm -rf /tmp/test",
  raw: { command: "rm -rf /tmp/test" },
};

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

describe("PermissionPrompt — inline sticky anatomy", () => {
  beforeEach(() => {
    vi.mocked(approveSession).mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should mark the inline prompt as sticky-scroll so it stays in viewport", () => {
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    const root = screen.getByTestId("permission-prompt");
    expect(root.getAttribute("data-sticky")).toBe("true");
  });

  it("Should render a tone tile with danger signal for high-stakes filesystem/network tools", () => {
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    const root = screen.getByTestId("permission-prompt");
    expect(root.getAttribute("data-tone")).toBe("danger");
    const tile = screen.getByTestId("permission-prompt-tile");
    expect(tile.getAttribute("data-tone")).toBe("danger");
  });

  it("Should fall back to warning tone for non-high-stakes tools", () => {
    const safePermission: PermissionRequest = {
      ...mockPermission,
      toolName: "TodoWrite",
      resource: "agent todo list",
    };
    render(
      <PermissionPrompt
        permission={safePermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.getByTestId("permission-prompt").getAttribute("data-tone")).toBe("warning");
    expect(screen.getByTestId("permission-prompt-tile").getAttribute("data-tone")).toBe("warning");
  });

  it("Should render tool name, action, and resource in the meta row", () => {
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.getByText("Bash")).toBeInTheDocument();
    expect(screen.getByText("execute")).toBeInTheDocument();
    expect(screen.getByText("rm -rf /tmp/test")).toBeInTheDocument();
  });

  it("Should expose the four canonical decision buttons", () => {
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.getByTestId("permission-allow-always")).toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-always")).toBeInTheDocument();
  });

  it("Should hide persistent decisions when the provider did not offer them", () => {
    render(
      <PermissionPrompt
        permission={{ ...mockPermission, supportedDecisions: ["allow-once", "reject-once"] }}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-allow-always")).not.toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-reject-always")).not.toBeInTheDocument();
  });

  it("Should fall back to the canonical decisions when supported decisions are unknown", () => {
    render(
      <PermissionPrompt
        permission={{ ...mockPermission, supportedDecisions: ["defer", "ignore"] as never[] }}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.getByTestId("permission-allow-always")).toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-always")).toBeInTheDocument();
  });

  it("Should call approveSession with allow-once on Allow Once click and resolve", async () => {
    const onResolved = vi.fn();
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={onResolved}
      />
    );

    fireEvent.click(screen.getByTestId("permission-allow-once"));

    await waitFor(() => {
      expect(approveSession).toHaveBeenCalledWith(WORKSPACE_ID, SESSION_ID, {
        request_id: "req-123",
        turn_id: "",
        decision: "allow-once",
      });
    });
    expect(onResolved).toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByTestId("permission-prompt")).not.toBeInTheDocument();
    });
  });

  it("Should submit only one permission decision while the first request is pending", async () => {
    let resolveRequest: (() => void) | undefined;
    const approveSessionMock = vi.mocked(approveSession);
    approveSessionMock.mockClear();
    approveSessionMock.mockImplementation(
      () =>
        new Promise<void>(resolve => {
          resolveRequest = resolve;
        })
    );
    const onResolved = vi.fn();
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={onResolved}
      />
    );

    const allowOnce = screen.getByTestId("permission-allow-once");
    fireEvent.click(allowOnce);
    expect(allowOnce).toBeDisabled();

    expect(approveSession).toHaveBeenCalledOnce();
    await act(async () => resolveRequest?.());
    await waitFor(() => expect(onResolved).toHaveBeenCalledOnce());
  });

  it("Should call approveSession with allow-always on Allow Always click", async () => {
    const onResolved = vi.fn();
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={onResolved}
      />
    );

    fireEvent.click(screen.getByTestId("permission-allow-always"));

    await waitFor(() => {
      expect(approveSession).toHaveBeenCalledWith(WORKSPACE_ID, SESSION_ID, {
        request_id: "req-123",
        turn_id: "",
        decision: "allow-always",
      });
    });
    expect(onResolved).toHaveBeenCalled();
  });

  it("Should call approveSession with reject-once on Reject Once click", async () => {
    const onResolved = vi.fn();
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={onResolved}
      />
    );

    fireEvent.click(screen.getByTestId("permission-reject-once"));

    await waitFor(() => {
      expect(approveSession).toHaveBeenCalledWith(WORKSPACE_ID, SESSION_ID, {
        request_id: "req-123",
        turn_id: "",
        decision: "reject-once",
      });
    });
    expect(onResolved).toHaveBeenCalled();
  });

  it("Should call approveSession with reject-always on Reject Always click", async () => {
    const onResolved = vi.fn();
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={onResolved}
      />
    );

    fireEvent.click(screen.getByTestId("permission-reject-always"));

    await waitFor(() => {
      expect(approveSession).toHaveBeenCalledWith(WORKSPACE_ID, SESSION_ID, {
        request_id: "req-123",
        turn_id: "",
        decision: "reject-always",
      });
    });
    expect(onResolved).toHaveBeenCalled();
  });

  it("Should surface a toast and stay open on API failure", async () => {
    const error = new Error("Network error");
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    vi.mocked(approveSession).mockRejectedValue(error);
    const onResolved = vi.fn();

    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={onResolved}
      />
    );

    fireEvent.click(screen.getByTestId("permission-allow-once"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "Failed to send permission response. The agent may continue waiting."
      );
    });

    expect(onResolved).not.toHaveBeenCalled();
    expect(consoleError).toHaveBeenCalledWith("Failed to submit permission decision", error);
    expect(screen.getByTestId("permission-prompt")).toBeInTheDocument();
    expect(screen.getByTestId("permission-allow-once")).not.toBeDisabled();
  });

  it("Should render the tool input JSON when keys exist and hide when empty", () => {
    const { rerender } = render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    const inputEl = screen.getByTestId("permission-tool-input");
    expect(inputEl).toBeInTheDocument();
    expect(inputEl.textContent).toContain("rm -rf /tmp/test");

    rerender(
      <PermissionPrompt
        permission={{ ...mockPermission, toolInput: {} }}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.queryByTestId("permission-tool-input")).not.toBeInTheDocument();
  });

  it("Should render the Permission Required eyebrow", () => {
    render(
      <PermissionPrompt
        permission={mockPermission}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        onResolved={vi.fn()}
      />
    );

    expect(screen.getByTestId("permission-prompt-eyebrow")).toHaveTextContent(
      "Permission Required"
    );
  });
});

describe("PermissionDataPart", () => {
  beforeEach(() => {
    vi.mocked(approveSession).mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should render an actionable prompt for pending permission data", () => {
    render(
      <PermissionDataPart
        data={mockPermissionData}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      />
    );

    expect(screen.getByTestId("permission-prompt")).toBeInTheDocument();
    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
  });

  it("Should derive actionable decisions from the raw provider option list", () => {
    render(
      <PermissionDataPart
        data={{
          ...mockPermissionData,
          raw: {
            options: [
              { decision: "allow-once", option_id: "allow-once" },
              { decision: "reject-once", option_id: "reject-once" },
            ],
            tool_input: { command: "touch blocked.txt" },
          },
        }}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      />
    );

    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-allow-always")).not.toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-reject-always")).not.toBeInTheDocument();
    expect(screen.getByTestId("permission-tool-input").textContent).toContain("touch blocked.txt");
  });

  it("Should render nothing for allowed resolved permission data", () => {
    const { container } = render(
      <PermissionDataPart
        data={{
          ...mockPermissionData,
          decision: "allow-once",
        }}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      />
    );

    expect(screen.queryByTestId("permission-prompt")).not.toBeInTheDocument();
    expect(screen.queryByTestId("permission-rejected-notice")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
  });

  it("Should render a passive notice for rejected resolved permission data", () => {
    render(
      <PermissionDataPart
        data={{
          ...mockPermissionData,
          decision: "reject-once",
        }}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      />
    );

    expect(screen.getByTestId("permission-rejected-notice")).toBeInTheDocument();
    expect(screen.getByText("Permission Rejected")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-prompt")).not.toBeInTheDocument();
  });
});
