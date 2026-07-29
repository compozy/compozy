import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ThinkingBlock } from "../thinking-block";

describe("ThinkingBlock", () => {
  it("Should auto-open inline while the turn is still reasoning, behind the shimmer label", () => {
    render(
      <ThinkingBlock thinking="Checking tool output before answering." thinkingComplete={false} />
    );

    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).toHaveTextContent("Thinking…");
    // Live thinking is a shimmering text label only — no icon well, no dots.
    expect(trigger.querySelector(".session-shimmer")).toBeInTheDocument();
    // Streaming reasoning is expanded inline without a user toggle.
    expect(screen.getByTestId("thinking-content")).toHaveTextContent(
      "Checking tool output before answering."
    );
  });

  it("Should auto-collapse to the Thought row once reasoning settles", () => {
    render(<ThinkingBlock thinking="Checked the output." thinkingComplete />);

    const trigger = screen.getByTestId("thinking-trigger");
    // Settled reasoning is a tool-row line: "Thought" verb + first-line preview.
    expect(trigger).toHaveTextContent("Thought");
    expect(trigger).toHaveTextContent("Checked the output.");
    expect(trigger.querySelector(".session-shimmer")).not.toBeInTheDocument();
    expect(screen.queryByTestId("thinking-content")).not.toBeInTheDocument();
  });

  it("Should let a user toggle override the settled auto-collapse", async () => {
    const user = userEvent.setup();
    render(<ThinkingBlock thinking="Reasoned about the fix." thinkingComplete />);

    expect(screen.queryByTestId("thinking-content")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("thinking-trigger"));
    expect(screen.getByTestId("thinking-content")).toBeInTheDocument();
  });

  it("Should let a user toggle override the streaming auto-open", async () => {
    const user = userEvent.setup();
    render(<ThinkingBlock thinking="Reasoning in progress." thinkingComplete={false} />);

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

    // Bullets parse into real list items instead of literal "- " lines, and the
    // fenced block routes through the shared CodeBlock primitive — the grammar
    // itself is owned by message-markdown.test.tsx; here we prove the reasoning
    // is rendered THROUGH MessageMarkdown, not a whitespace-pre-wrap box.
    expect(content.querySelectorAll("li")).toHaveLength(2);
    expect(content.querySelector('[data-slot="code-block"]')).toBeInTheDocument();
    expect(content.textContent).not.toContain("```");
    expect(content.textContent).not.toContain("- inspect the config");
  });

  it("Should align the reasoning body to the detail rail with a single indent", async () => {
    const user = userEvent.setup();
    render(<ThinkingBlock thinking="Reasoned." thinkingComplete />);

    const trigger = screen.getByTestId("thinking-trigger");
    await user.click(trigger);
    const content = screen.getByTestId("thinking-content");

    // Body sits on the shared 25px detail rail (`.trow__detail` alignment) —
    // a hairline rule, no bordered box, no background fill.
    expect(content.className).toContain("ml-[25px]");
    expect(content.className).toContain("border-l");
    expect(content.className).toContain("pl-[11px]");
    expect(content.className).not.toContain("bg-canvas-soft");
    expect(content.className).not.toContain("rounded-lg");
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
});
