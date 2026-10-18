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

  // Live reasoning rests as one breathing line of what it thinks now; the body
  // opens only on the reader's toggle (or inside a disclosed work group).
  it("Should show live reasoning as its latest paragraph and open the body only on toggle", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<ThinkingBlock thinking="" thinkingComplete={false} />);
    expect(screen.getByTestId("thinking-trigger")).toHaveTextContent("Thinking…");
    expect(
      screen.getByTestId("thinking-trigger").querySelector(".session-shimmer")
    ).toBeInTheDocument();

    rerender(
      <ThinkingBlock
        thinking={"Read the **config** first.\n\nNow checking the tests."}
        thinkingComplete={false}
      />
    );
    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).toHaveTextContent("Now checking the tests.");
    expect(trigger.querySelector(".session-thinking-pulse")).toBeInTheDocument();
    expect(screen.queryByTestId("thinking-content")).not.toBeInTheDocument();

    await user.click(trigger);
    expect(screen.getByTestId("thinking-content")).toHaveTextContent("Read the config first.");
  });

  it("Should keep code fences, open or split by blank lines, out of the live line", () => {
    const { rerender } = render(
      <ThinkingBlock
        thinking={"Checking the parser:\n```ts\nconst token = next("}
        thinkingComplete={false}
      />
    );
    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).toHaveTextContent("Checking the parser:");
    expect(trigger).not.toHaveTextContent("const token");

    rerender(
      <ThinkingBlock
        thinking={"Checking:\n```ts\nconst a = 1;\n\nconst b = 2;\n```\nNow checking tests."}
        thinkingComplete={false}
      />
    );
    expect(trigger).toHaveTextContent("Now checking tests.");
    expect(trigger).not.toHaveTextContent("const b");
  });

  it("Should open live reasoning inside a disclosed work group", () => {
    render(
      <ThinkingBlock thinking="Reasoning in progress." thinkingComplete={false} defaultOpen />
    );
    expect(screen.getByTestId("thinking-content")).toHaveTextContent("Reasoning in progress.");
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
