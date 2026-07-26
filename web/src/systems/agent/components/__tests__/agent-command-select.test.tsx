import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@agh/ui";

import { AgentCommandSelect } from "../agent-command-select";
import type { AgentPayload } from "../../types";

function makeAgent(overrides: Partial<AgentPayload> & { name: string }): AgentPayload {
  return {
    provider: overrides.provider ?? "claude",
    prompt: overrides.prompt ?? `prompt for ${overrides.name}`,
    ...overrides,
  } as AgentPayload;
}

describe("AgentCommandSelect", () => {
  it("Should render the trigger with the selected agent's name and provider", () => {
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "writer", provider: "claude" })]}
          value="writer"
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    const trigger = screen.getByTestId("trigger");
    expect(trigger).toHaveTextContent("writer");
    expect(trigger).toHaveTextContent("claude");
  });

  it("Should display the formatted category label in the trigger when an agent has category_path", () => {
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "deals", category_path: ["Marketing", "Sales"] })]}
          value="deals"
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    expect(screen.getByTestId("agent-command-select-trigger-category")).toHaveTextContent(
      "Marketing / Sales"
    );
  });

  it("Should show placeholder text when no agent is selected", () => {
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "writer" })]}
          value={null}
          onChange={() => undefined}
          triggerTestId="trigger"
          placeholder="Pick agent"
        />
      </UIProvider>
    );
    expect(screen.getByTestId("trigger")).toHaveTextContent("Pick agent");
  });

  it("Should call onChange with the agent name and close the popover when an item is selected", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[
            makeAgent({ name: "writer" }),
            makeAgent({ name: "coder", category_path: ["Engineering"] }),
          ]}
          value={null}
          onChange={onChange}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    await user.click(screen.getByTestId("agent-command-item-coder"));
    expect(onChange).toHaveBeenCalledWith("coder");
    expect(screen.getByTestId("trigger")).toHaveAttribute("aria-expanded", "false");
  });

  it("Should group results by formatted category label and put root-level agents under Agents", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[
            makeAgent({ name: "deals", category_path: ["Marketing", "Sales"] }),
            makeAgent({ name: "writer" }),
          ]}
          value={null}
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    expect(screen.getByTestId("agent-command-group-agents:root")).toHaveTextContent("Agents");
    expect(screen.getByTestId("agent-command-group-category:Marketing/Sales")).toHaveTextContent(
      "Marketing / Sales"
    );
  });

  it("Should filter results via keyboard search", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "writer" }), makeAgent({ name: "coder" })]}
          value={null}
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    await user.type(screen.getByTestId("agent-command-input"), "wri");
    expect(screen.queryByTestId("agent-command-item-writer")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-command-item-coder")).not.toBeInTheDocument();
  });

  it("Should clear the selection through the leading clear item when clearLabel is given", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "writer" })]}
          value="writer"
          clearLabel="Role default"
          onChange={onChange}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    await user.click(screen.getByTestId("agent-command-item-clear"));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("Should show the clear label on the trigger when nothing is selected", () => {
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "writer" })]}
          value={null}
          clearLabel="Role default"
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    expect(screen.getByTestId("trigger")).toHaveTextContent("Role default");
  });

  it("Should keep an out-of-catalog value visible instead of collapsing it into the placeholder", () => {
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "writer" })]}
          value="ghost"
          clearLabel="Role default"
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    const trigger = screen.getByTestId("trigger");
    expect(trigger).toHaveTextContent("ghost");
    expect(trigger).toHaveTextContent("Not available");
  });

  it("Should use tokenized metadata classes for provider and category labels in the list", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <AgentCommandSelect
          agents={[makeAgent({ name: "deals", category_path: ["Marketing", "Sales"] })]}
          value={null}
          onChange={() => undefined}
          triggerTestId="trigger"
        />
      </UIProvider>
    );
    await user.click(screen.getByTestId("trigger"));
    expect(screen.getByTestId("agent-command-provider-deals")).toHaveClass("eyebrow");
    expect(screen.getByTestId("agent-command-category-deals")).toHaveClass("eyebrow");
  });
});
