import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

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
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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

  it("Should lead the composer trigger with the full agent name and leave the category path to the list", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <AgentCommandSelect
          agents={[
            makeAgent({
              name: "product-launch-manager-agent",
              category_path: ["Operations", "Launch week"],
            }),
          ]}
          value="product-launch-manager-agent"
          onChange={() => undefined}
          triggerTestId="trigger"
          variant="composer"
        />
      </UIProvider>
    );
    const trigger = screen.getByTestId("trigger");
    expect(trigger).toHaveTextContent("product-launch-manager-agent");
    expect(trigger).toHaveTextContent("claude");
    expect(screen.queryByTestId("agent-command-select-trigger-category")).not.toBeInTheDocument();

    await user.click(trigger);
    expect((await screen.findAllByText("Operations / Launch week")).length).toBeGreaterThan(0);
  });

  it("Should show placeholder text when no agent is selected", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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
      <UIProvider reducedMotion="never" skipAnimations>
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
});

// Invariant: a picker whose catalog has not answered holds its place with shapes — a text bar
// named for assistive technology in the trigger, row-shaped placeholders in the popup — and never
// a "Loading…" sentence, an unavailable claim about a configured name, or an empty-result claim.
// Agents already in hand stay listed and selectable, and loaded agents replace the placeholders.
describe("AgentCommandSelect loading", () => {
  const writer = makeAgent({ name: "writer" });
  const coder = makeAgent({ name: "coder", provider: "codex" });
  const select = (props: Partial<React.ComponentProps<typeof AgentCommandSelect>>) => (
    <UIProvider reducedMotion="never" skipAnimations>
      <AgentCommandSelect
        agents={[]}
        value={null}
        onChange={() => undefined}
        triggerTestId="trigger"
        {...props}
      />
    </UIProvider>
  );

  it("Should hold the trigger's place with a text bar that still names the control", () => {
    const { rerender } = render(select({ loading: true, placeholder: "Choose an agent" }));

    const trigger = screen.getByRole("button", { name: "Loading agents" });
    expect(trigger).toBe(screen.getByTestId("trigger"));
    expect(trigger).toBeDisabled();
    expect(trigger).toHaveAttribute("aria-busy", "true");
    expect(trigger.querySelector('[data-slot="skeleton-text"]')).not.toBeNull();
    expect(trigger).not.toHaveTextContent("Choose an agent");
    expect(screen.queryByText(/Loading agents…/)).not.toBeInTheDocument();

    rerender(select({ agents: [writer], placeholder: "Choose an agent" }));

    expect(trigger).toBeEnabled();
    expect(trigger).toHaveTextContent("Choose an agent");
    expect(trigger.querySelector('[data-slot="skeleton-text"]')).toBeNull();
  });

  it("Should keep a configured name readable instead of calling it unavailable while loading", () => {
    const { rerender } = render(select({ agents: [], value: "writer", loading: true }));

    const trigger = screen.getByTestId("trigger");
    expect(trigger).toHaveTextContent("writer");
    expect(trigger).not.toHaveTextContent("Not available");
    expect(trigger.querySelector('[data-slot="skeleton-text"]')).not.toBeNull();

    rerender(select({ agents: [writer], value: "writer" }));
    expect(trigger).toHaveTextContent("claude");
    expect(trigger.querySelector('[data-slot="skeleton-text"]')).toBeNull();

    // Once the catalog has answered without it, the claim is true.
    rerender(select({ agents: [coder], value: "writer" }));
    expect(trigger).toHaveTextContent("Not available");
  });

  it("Should fill an open popup with row placeholders, not a sentence, until the catalog lands", async () => {
    const user = userEvent.setup();
    const clearLabel = "Role default";
    const { rerender } = render(select({ agents: [writer], clearLabel }));
    await user.click(screen.getByTestId("trigger"));
    expect(await screen.findByTestId("agent-command-item-writer")).toBeInTheDocument();

    rerender(select({ agents: [], clearLabel, loading: true }));

    const region = await screen.findByTestId("agent-command-loading");
    expect(region).toHaveAttribute("role", "status");
    expect(region).toHaveTextContent("Loading agents");
    expect(
      region.querySelectorAll('[data-slot="agent-command-row-skeleton"]').length
    ).toBeGreaterThan(3);
    // The leading choice stays real, and a listbox holds only options, so the region sits beside it.
    expect(screen.getByRole("listbox")).toContainElement(
      screen.getByTestId("agent-command-item-clear")
    );
    expect(screen.getByRole("listbox")).not.toContainElement(region);
    expect(screen.queryByTestId("agent-command-empty")).not.toBeInTheDocument();
    expect(screen.queryByText(/Loading agents…/)).not.toBeInTheDocument();

    rerender(select({ agents: [writer, coder], clearLabel }));

    expect(screen.queryByTestId("agent-command-loading")).not.toBeInTheDocument();
    expect(await screen.findByTestId("agent-command-item-coder")).toBeInTheDocument();
  });

  it("Should keep agents in hand selectable while the catalog is read again", async () => {
    const user = userEvent.setup();
    render(select({ agents: [writer, coder], loading: true, placeholder: "Choose an agent" }));

    const trigger = screen.getByTestId("trigger");
    expect(trigger).toBeEnabled();
    expect(trigger).toHaveTextContent("Choose an agent");
    expect(trigger.querySelector('[data-slot="skeleton-text"]')).toBeNull();

    await user.click(trigger);
    expect(await screen.findByTestId("agent-command-item-coder")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-command-loading")).not.toBeInTheDocument();
  });

  it("Should let a failed read replace the placeholders with its message", () => {
    render(select({ loading: true, error: "Catalog offline" }));

    const trigger = screen.getByTestId("trigger");
    expect(trigger).toHaveTextContent("Unable to load agents");
    expect(trigger.querySelector('[data-slot="skeleton-text"]')).toBeNull();
  });
});
