import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AgentIcon } from "../agent-icon";

describe("AgentIcon", () => {
  it('maps "claude" provider to BrainCircuit icon', () => {
    render(<AgentIcon provider="claude" data-testid="icon" />);
    const icon = screen.getByTestId("icon");
    expect(icon).toBeInTheDocument();
    expect(icon.tagName.toLowerCase()).toBe("span");
    expect(icon.querySelector("svg")).toBeInTheDocument();
    expect(icon).toHaveAttribute("data-slot", "agent-icon");
    expect(icon).toHaveAttribute("data-provider", "claude");
  });

  it("returns fallback icon for unknown provider", () => {
    render(<AgentIcon provider="unknown-provider" data-testid="icon" />);
    expect(screen.getByTestId("icon")).toBeInTheDocument();
  });

  it("is case-insensitive for provider matching", () => {
    render(<AgentIcon provider="Claude" data-testid="icon" />);
    const icon = screen.getByTestId("icon");
    expect(icon).toHaveAttribute("data-provider", "claude");
  });
});
