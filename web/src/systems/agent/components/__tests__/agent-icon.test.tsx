import { render, screen } from "@testing-library/react";
import { providerKindIconRegistry } from "@agh/ui";
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

  it('maps "codex" provider to Code icon', () => {
    render(<AgentIcon provider="codex" data-testid="icon" />);
    expect(screen.getByTestId("icon")).toBeInTheDocument();
  });

  it('maps "gemini" provider to Sparkles icon', () => {
    render(<AgentIcon provider="gemini" data-testid="icon" />);
    expect(screen.getByTestId("icon")).toBeInTheDocument();
  });

  it('maps "openai" provider to Bot icon', () => {
    render(<AgentIcon provider="openai" data-testid="icon" />);
    expect(screen.getByTestId("icon")).toBeInTheDocument();
  });

  it('maps "ollama" provider to Terminal icon', () => {
    render(<AgentIcon provider="ollama" data-testid="icon" />);
    expect(screen.getByTestId("icon")).toBeInTheDocument();
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

  it("has known providers in the shared provider icon registry", () => {
    const expected = [
      "blackbox",
      "claude",
      "cline",
      "codex",
      "gemini",
      "goose",
      "hermes",
      "junie",
      "kimi-cli",
      "openai",
      "openclaw",
      "openhands",
      "ollama",
      "qoder",
      "qwen-code",
    ];

    for (const provider of expected) {
      expect(providerKindIconRegistry).toHaveProperty(provider);
    }
  });

  it("applies custom className on top of the tone class", () => {
    render(<AgentIcon provider="claude" tone="accent" data-testid="icon" className="size-8" />);
    const icon = screen.getByTestId("icon");
    expect(icon.getAttribute("class")).toContain("size-8");
    expect(icon.getAttribute("class")).toContain("text-accent");
  });
});
