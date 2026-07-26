import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import { MessageMarkdown } from "../message-markdown";

describe("MessageMarkdown", () => {
  it("renders fenced code through the shared CodeBlock primitive", async () => {
    const { container } = render(
      <MessageMarkdown content={"```typescript\nconst value = 1;\n```"} />
    );

    const codeBlock = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlock).toBeInTheDocument();
    expect(codeBlock).toHaveAttribute("data-language", "typescript");

    await waitFor(
      () => {
        expect(codeBlock).toHaveAttribute("data-highlight-state", "highlighted");
      },
      { timeout: 5_000 }
    );
    await waitFor(() => {
      expect(codeBlock).toHaveAttribute("data-theme", "vitesse-light");
    });

    expect(screen.getByRole("button", { name: "Copy to clipboard" })).toBeInTheDocument();
  });

  it("renders fenced code without a language as a plain shared CodeBlock", () => {
    const { container } = render(
      <MessageMarkdown content={["```", "agh start", "```"].join("\n")} />
    );

    const codeBlock = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlock).toBeInTheDocument();
    expect(codeBlock).toHaveAttribute("data-highlight-state", "plain");
    expect(codeBlock).not.toHaveAttribute("data-language");
    expect(codeBlock?.textContent).toContain("agh start");
  });

  it("keeps inline code as compact inline prose", () => {
    const { container } = render(<MessageMarkdown content={"Use `agh start` from the shell."} />);

    expect(container.querySelector('[data-slot="code-block"]')).toBeNull();
    expect(container.querySelector("code")?.textContent).toBe("agh start");
  });

  it("renders incomplete fenced code while streaming", async () => {
    const { container } = render(
      <MessageMarkdown content={"Plan:\n\n```ts\nconst value = 1"} streaming />
    );

    const codeBlock = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlock).toBeInTheDocument();
    expect(codeBlock).toHaveTextContent("const value = 1");
    await waitFor(
      () => {
        expect(codeBlock).toHaveAttribute("data-highlight-state", "highlighted");
      },
      { timeout: 5_000 }
    );
  });

  it("renders GFM tables without raw HTML elements", () => {
    const { container } = render(
      <MessageMarkdown
        content={[
          "| Area | Status |",
          "| --- | --- |",
          "| Stream | Fixed |",
          "",
          "<script>alert('xss')</script>",
        ].join("\n")}
      />
    );

    expect(container.querySelector("table")).toBeInTheDocument();
    expect(screen.getByText("Stream")).toBeInTheDocument();
    expect(screen.getByText("Fixed")).toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
  });
});
