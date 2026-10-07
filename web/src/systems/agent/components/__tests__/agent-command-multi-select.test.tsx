import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { AgentCommandMultiSelect } from "../agent-command-multi-select";
import type { AgentPayload } from "../../types";

function makeAgent(overrides: Partial<AgentPayload> & { name: string }): AgentPayload {
  return {
    provider: overrides.provider ?? "claude",
    prompt: overrides.prompt ?? `prompt for ${overrides.name}`,
    ...overrides,
  } as AgentPayload;
}

describe("AgentCommandMultiSelect", () => {
  it("Should call onToggle with the next selection set when an item is clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer" }), makeAgent({ name: "coder" })]}
          value={["writer"]}
          onToggle={onToggle}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    expect(screen.getByTestId("agent-command-item-writer")).toHaveAttribute("data-checked", "true");
    expect(screen.getByTestId("agent-command-item-coder")).toHaveAttribute("data-checked", "false");
    expect(screen.getByTestId("agent-command-provider-writer")).toHaveTextContent("claude");
    await user.click(screen.getByTestId("agent-command-item-coder"));
    expect(onToggle).toHaveBeenCalledWith(["writer", "coder"]);
    expect(screen.getByTestId("agent-command-input")).toBeInTheDocument();
  });

  it("Should remove an already selected agent on toggle", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer" }), makeAgent({ name: "coder" })]}
          value={["writer", "coder"]}
          onToggle={onToggle}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    await user.click(screen.getByTestId("agent-command-item-writer"));
    expect(onToggle).toHaveBeenCalledWith(["coder"]);
  });
});
