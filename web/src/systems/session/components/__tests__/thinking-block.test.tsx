import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ThinkingBlock } from "../thinking-block";

describe("ThinkingBlock", () => {
  it("Should let a user toggle override the settled auto-collapse", async () => {
    const user = userEvent.setup();
    render(<ThinkingBlock thinking="Reasoned about the fix." thinkingComplete />);
    expect(screen.getByTestId("thinking-trigger")).toHaveTextContent("Thought");
    expect(screen.getByTestId("thinking-trigger")).toHaveTextContent("Reasoned about the fix.");
    expect(
      screen.getByTestId("thinking-trigger").querySelector(".session-shimmer")
    ).not.toBeInTheDocument();

    expect(screen.queryByTestId("thinking-content")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("thinking-trigger"));
    expect(screen.getByTestId("thinking-content")).toBeInTheDocument();
  });

  it("Should let a user toggle override the streaming auto-open", async () => {
    const user = userEvent.setup();
    render(<ThinkingBlock thinking="Reasoning in progress." thinkingComplete={false} />);
    expect(screen.getByTestId("thinking-trigger")).toHaveTextContent("Thinking…");
    expect(
      screen.getByTestId("thinking-trigger").querySelector(".session-shimmer")
    ).toBeInTheDocument();
    expect(screen.getByTestId("thinking-content")).toHaveTextContent("Reasoning in progress.");

    // Auto-open while streaming, then a user collapse pins it closed even though
    // the turn is still in flight.
    expect(screen.getByTestId("thinking-content")).toBeInTheDocument();
    await user.click(screen.getByTestId("thinking-trigger"));
    expect(screen.queryByTestId("thinking-content")).not.toBeInTheDocument();
  });

  it("Should render reasoning as markdown rather than raw pre-wrapped text", async () => {
    const user = userEvent.setup();
    const markdown = [
      "Plan of attack:",
      "",
      "- inspect the config",
      "- run the tests",
      "",
      "```ts",
      "const answer = 42;",
      "```",
    ].join("\n");
    render(<ThinkingBlock thinking={markdown} thinkingComplete />);

    await user.click(screen.getByTestId("thinking-trigger"));
    const content = screen.getByTestId("thinking-content");
    expect(content).toHaveAttribute("role", "region");
    expect(content).toHaveAttribute("aria-label", "Reasoning");
    expect(content).toHaveAttribute("tabindex", "0");

    // Bullets parse into real list items instead of literal "- " lines, and the
    // fenced block routes through the shared CodeBlock primitive — the grammar
    // itself is owned by message-markdown.test.tsx; here we prove the reasoning
    // is rendered THROUGH MessageMarkdown, not a whitespace-pre-wrap box.
    expect(content.querySelectorAll("li")).toHaveLength(2);
    expect(content.querySelector('[data-slot="code-block"]')).toBeInTheDocument();
    expect(content.textContent).not.toContain("```");
    expect(content.textContent).not.toContain("- inspect the config");
  });

  it("Should preview the first reasoning line on the settled row without an updates count", () => {
    render(
      <ThinkingBlock
        thinking={"mapped the loud components first\n\nthen retoned them"}
        thinkingComplete
      />
    );

    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).toHaveTextContent("mapped the loud components first");
    // The "N updates" eyebrow is gone — grouping stays a derivation concern.
    expect(trigger).not.toHaveTextContent("updates");
  });

  // Invariant: only the collapsed preview is bounded; keyboard disclosure preserves reasoning.
  // Owner: reasoning disclosure; canonical suite: thinking-block.
  it("Should expose the full long reasoning after keyboard expansion", async () => {
    const user = userEvent.setup();
    const thinking = "Reviewing " + "ação 👩🏽‍💻 ".repeat(90) + "reasoning-tail";
    render(<ThinkingBlock thinking={thinking} thinkingComplete />);
    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).not.toHaveTextContent("reasoning-tail");
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("region", { name: "Reasoning" })).toHaveTextContent(thinking);
  });
});
