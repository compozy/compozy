import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@agh/ui";

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
  it("Should mark currently selected items with data-checked=true", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer" }), makeAgent({ name: "coder" })]}
          value={["writer"]}
          onToggle={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    expect(screen.getByTestId("agent-command-item-writer")).toHaveAttribute("data-checked", "true");
    expect(screen.getByTestId("agent-command-item-coder")).toHaveAttribute("data-checked", "false");
  });

  it("Should call onToggle with the next selection set when an item is clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer" }), makeAgent({ name: "coder" })]}
          value={["writer"]}
          onToggle={onToggle}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    await user.click(screen.getByTestId("agent-command-item-coder"));
    expect(onToggle).toHaveBeenCalledWith(["writer", "coder"]);
  });

  it("Should remove an already selected agent on toggle", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(
      <UIProvider reducedMotion="always">
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

  it("Should remain open after a selection", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer" }), makeAgent({ name: "coder" })]}
          value={[]}
          onToggle={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    await user.click(screen.getByTestId("agent-command-item-writer"));
    expect(screen.getByTestId("agent-command-input")).toBeInTheDocument();
  });

  it("Should render provider metadata for each item", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer", provider: "claude" })]}
          value={[]}
          onToggle={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    expect(screen.getByTestId("agent-command-provider-writer")).toHaveTextContent("claude");
  });

  it("Should preserve custom item test IDs supplied by the consumer", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandMultiSelect
          agents={[makeAgent({ name: "writer" })]}
          value={[]}
          onToggle={() => undefined}
          triggerTestId="trigger"
          itemTestId={agent => `network-agent-option-${agent.name}`}
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    expect(screen.getByTestId("network-agent-option-writer")).toBeInTheDocument();
  });
});
