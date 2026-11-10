import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CodeFrame } from "../code-frame";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("blog CodeFrame", () => {
  it("sets fenced code in the shared CodeBlock with its language and a copy button", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });

    const { container } = render(
      <CodeFrame data-language="bash">
        <code data-language="bash">
          <span data-line="">compozy daemon start</span>
          {"\n"}
          <span data-line="">compozy session list</span>
          {"\n"}
        </code>
      </CodeFrame>
    );

    const block = container.querySelector('[data-slot="code-block"]');
    expect(block?.getAttribute("data-language")).toBe("bash");
    expect(screen.getByText("bash").closest("pre")).toBeNull();
    expect(block?.querySelector("pre")?.textContent).toContain("compozy daemon start");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Copy code" }));
      await Promise.resolve();
    });

    expect(writeText).toHaveBeenCalledWith("compozy daemon start\ncompozy session list");
  });

  it("leaves the label empty for plain text blocks", () => {
    const { container } = render(
      <CodeFrame data-language="text">
        <code data-language="text">plain output</code>
      </CodeFrame>
    );

    expect(screen.queryByText("text")).toBeNull();
    expect(container.querySelector('[data-slot="code-block"]')?.hasAttribute("data-language")).toBe(
      false
    );
    expect(container.querySelector("pre")?.textContent).toBe("plain output");
  });
});
