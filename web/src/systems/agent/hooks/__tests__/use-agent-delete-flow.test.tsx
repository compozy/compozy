import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";

const mockNavigate = vi.fn();
const mockMutate = vi.fn();
const mockToastSuccess = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => mockToastSuccess(...args),
    error: vi.fn(),
  },
}));

vi.mock("../use-agents", () => ({
  useDeleteAgent: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
}));

import { useAgentDeleteFlow } from "../use-agent-delete-flow";

function Harness({ origin = "workspace" }: { origin?: "workspace" | "global" }) {
  const flow = useAgentDeleteFlow({
    agent: { ...primaryAgentFixture, origin },
    workspaceId: "ws_test",
  });
  return (
    <div>
      <button type="button" data-testid="open-delete" onClick={flow.openDialog}>
        Open
      </button>
      {flow.confirmDialog}
    </div>
  );
}

function renderFlow(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

describe("useAgentDeleteFlow", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockMutate.mockReset();
    mockToastSuccess.mockReset();
  });

  it("Should require typed confirmation and disclose workspace un-shadow pre-copy", async () => {
    const user = userEvent.setup();
    renderFlow(<Harness origin="workspace" />);
    await user.click(screen.getByTestId("open-delete"));

    expect(screen.getByTestId("agent-delete-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("agent-delete-description")).toHaveTextContent(
      /global definition with this name exists/i
    );
    expect(screen.getByTestId("agent-delete-confirm")).toBeDisabled();

    await user.type(screen.getByTestId("agent-delete-confirm-typing"), primaryAgentFixture.name);
    expect(screen.getByTestId("agent-delete-confirm")).toBeEnabled();
  });

  it("Should toast with unshadowed_origin and navigate to /agents on success", async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_vars, opts) => {
      opts.onSuccess({
        name: primaryAgentFixture.name,
        origin: "workspace",
        unshadowed_origin: "global",
      });
    });
    renderFlow(<Harness />);
    await user.click(screen.getByTestId("open-delete"));
    await user.type(screen.getByTestId("agent-delete-confirm-typing"), primaryAgentFixture.name);
    await user.click(screen.getByTestId("agent-delete-confirm"));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Agent deleted",
        expect.objectContaining({
          description: expect.stringMatching(/global/i),
        })
      );
    });
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/agents" });
  });
});
