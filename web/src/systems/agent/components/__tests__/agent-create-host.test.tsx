import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentCreateHostProvider } from "../agent-create-host";
import { useAgentCreateHost } from "../../hooks/use-agent-create-host";

function CreateHostProbe() {
  const { openDialog } = useAgentCreateHost();
  return (
    <button data-testid="probe-open" onClick={openDialog} type="button">
      Open
    </button>
  );
}

describe("AgentCreateHost", () => {
  it("Should expose openDialog to consumers inside the provider", async () => {
    const user = userEvent.setup();
    const openDialog = vi.fn();
    const openForDuplicate = vi.fn();
    render(
      <AgentCreateHostProvider openDialog={openDialog} openForDuplicate={openForDuplicate}>
        <CreateHostProbe />
      </AgentCreateHostProvider>
    );
    await user.click(screen.getByTestId("probe-open"));
    expect(openDialog).toHaveBeenCalledOnce();
  });

  it("Should throw when used outside the provider", () => {
    expect(() => render(<CreateHostProbe />)).toThrow(
      /useAgentCreateHost must be used within AgentCreateHostProvider/
    );
  });
});
