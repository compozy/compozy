import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { AgentGroupedCatalogItem } from "../../types";
import { primaryAgentFixture } from "../../mocks";
import { AgentPicker, AgentScopePicker } from "../agent-add-pickers";

function item(name: string, label: string, provider = "claude"): AgentGroupedCatalogItem {
  return {
    name,
    label,
    kind: "stays_on",
    definition_digest: "d",
    scopes: [],
    active: 0,
    agent: { ...primaryAgentFixture, name, label, provider, description: `${label} helps` },
  };
}

const AGENTS = [item("triage", "Triage Lead"), item("scout", "Scout", "codex")];
const trigger = <button type="button">Add</button>;

function renderPicker(props: Partial<Parameters<typeof AgentPicker>[0]> = {}) {
  const handlers = { onPick: vi.fn(), onCreateNew: vi.fn() };
  render(
    <UIProvider reducedMotion="never" skipAnimations>
      <AgentPicker agents={AGENTS} name="compozy" open trigger={trigger} {...handlers} {...props} />
    </UIProvider>
  );
  return handlers;
}

describe("AgentPicker", () => {
  it("Should list the agents and add the one picked", async () => {
    const user = userEvent.setup();
    const { onPick } = renderPicker();

    expect(screen.getByText("Add to compozy")).toBeInTheDocument();
    await user.click(screen.getByText("Scout"));

    expect(onPick).toHaveBeenCalledWith(AGENTS[1]);
  });

  it("Should open the create screen from Create new", async () => {
    const user = userEvent.setup();
    const { onCreateNew } = renderPicker();

    await user.click(screen.getByText("Create new"));

    expect(onCreateNew).toHaveBeenCalledOnce();
  });

  it("Should block the rows and Create new at the limit and show it", async () => {
    const user = userEvent.setup();
    const { onPick, onCreateNew } = renderPicker({
      capacity: { limit: 10, active: 10, reached: true },
    });

    expect(screen.getByTestId("agent-add-limit")).toHaveTextContent(
      "10 of 10 agents are staying on."
    );
    await user.click(screen.getByText("Scout"));
    await user.click(screen.getByText("Create new"));

    expect(onPick).not.toHaveBeenCalled();
    expect(onCreateNew).not.toHaveBeenCalled();
  });

  it("Should follow the configured limit past ten", async () => {
    const user = userEvent.setup();
    const { onPick } = renderPicker({ capacity: { limit: 25, active: 10, reached: false } });

    expect(screen.queryByTestId("agent-add-limit")).toBeNull();
    await user.click(screen.getByText("Scout"));

    expect(onPick).toHaveBeenCalledOnce();
  });

  it("Should block only the agents whose provider isn't signed in", async () => {
    const user = userEvent.setup();
    const { onPick } = renderPicker({ signedOut: new Map([["codex", "Codex"]]) });

    expect(screen.getByText("Codex isn't signed in")).toBeInTheDocument();
    await user.click(screen.getByText("Scout"));
    expect(onPick).not.toHaveBeenCalled();

    await user.click(screen.getByText("Triage Lead"));
    expect(onPick).toHaveBeenCalledWith(AGENTS[0]);
  });

  it("Should say so when every agent already works here", async () => {
    const user = userEvent.setup();
    const { onCreateNew } = renderPicker({ agents: [] });

    expect(screen.getByText("All your stays-on agents already work here.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Create new" }));
    expect(onCreateNew).toHaveBeenCalledOnce();
  });
});

describe("AgentScopePicker", () => {
  const scopes = [
    { id: "@user", name: "Global" },
    { id: "ws-a", name: "alpha" },
  ];

  function renderScopes(props: Partial<Parameters<typeof AgentScopePicker>[0]> = {}) {
    const onPick = vi.fn();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <AgentScopePicker onPick={onPick} open scopes={scopes} trigger={trigger} {...props} />
      </UIProvider>
    );
    return onPick;
  }

  it("Should list only the valid scopes and pick one", async () => {
    const user = userEvent.setup();
    const onPick = renderScopes();

    await user.click(screen.getByText("alpha"));

    expect(onPick).toHaveBeenCalledWith("ws-a");
  });

  it("Should block the scopes at the limit or signed out", async () => {
    const user = userEvent.setup();
    const onPick = renderScopes({
      capacity: { limit: 10, active: 10, reached: true },
      signedOut: "Codex",
    });

    expect(screen.getByTestId("agent-add-limit")).toBeInTheDocument();
    expect(screen.getByText("Codex isn't signed in,")).toBeInTheDocument();
    await user.click(screen.getByText("alpha"));
    expect(onPick).not.toHaveBeenCalled();
  });

  it("Should say so when nothing is left", () => {
    renderScopes({ scopes: [] });
    expect(screen.getByText("This agent already works in all your projects.")).toBeInTheDocument();
  });
});
