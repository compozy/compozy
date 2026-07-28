import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ThinkingBlock } from "../thinking-block";

describe("ThinkingBlock", () => {
  it("Should auto-open inline while the turn is still reasoning", () => {
    render(
      <ThinkingBlock thinking="Checking tool output before answering." thinkingComplete={false} />
    );

    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).toHaveTextContent("Thinking");
    // Streaming reasoning is expanded inline without a user toggle.
    expect(screen.getByTestId("thinking-content")).toHaveTextContent(
      "Checking tool output before answering."
    );
    // The typing-dots live indicator marks the turn as still reasoning.
    expect(trigger.querySelector("[data-slot='typing-dots']")).toBeInTheDocument();
  });

  it("Should auto-collapse once reasoning settles", () => {
    render(<ThinkingBlock thinking="Checked the output." thinkingComplete />);

    const trigger = screen.getByTestId("thinking-trigger");
    expect(trigger).toHaveTextContent("Thought process");
    expect(screen.queryByTestId("thinking-content")).not.toBeInTheDocument();
    // No live indicator once the reasoning has settled.
    expect(trigger.querySelector("[data-slot='typing-dots']")).not.toBeInTheDocument();
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
    // is rendered THROUGH MessageMarkdown, not the deleted whitespace-pre-wrap box.
    expect(content.querySelectorAll("li")).toHaveLength(2);
    expect(content.querySelector('[data-slot="code-block"]')).toBeInTheDocument();
    expect(content.textContent).not.toContain("```");
    expect(content.textContent).not.toContain("- inspect the config");
  });

  it("Should align the reasoning body to the tool-row rail without the double indent", async () => {
    const user = userEvent.setup();
    render(<ThinkingBlock thinking="Reasoned." thinkingComplete />);

    const trigger = screen.getByTestId("thinking-trigger");
    await user.click(trigger);
    const content = screen.getByTestId("thinking-content");

    // Body sits on the shared tool-row rail (ml-7 border-l pl-3), aligned one
    // level in from the trigger — not the deleted double-indented bordered box.
    expect(content.className).toContain("ml-7");
    expect(content.className).toContain("border-l");
    expect(content.className).toContain("pl-3");
    expect(content.className).not.toContain("mx-4");
    expect(content.className).not.toContain("bg-canvas-soft");
    expect(content.className).not.toContain("rounded-lg");
    // The trigger drops the outer px-4 indent that stacked with the box margin.
    expect(trigger.className).not.toContain("px-4");
  });

  it("Should surface the grouped update count only when reasoning is grouped", () => {
    const { rerender } = render(
      <ThinkingBlock thinking="Reasoned three times." thinkingComplete updateCount={3} />
    );
    expect(screen.getByTestId("thinking-trigger")).toHaveTextContent("3 updates");

    rerender(<ThinkingBlock thinking="Reasoned once." thinkingComplete updateCount={1} />);
    expect(screen.getByTestId("thinking-trigger")).not.toHaveTextContent("updates");
  });
});
