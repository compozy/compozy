import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DescriptionCard } from "../description-card";
import { STREAMDOWN_SAFE_CONFIG } from "../markdown";
import { Markdown } from "../markdown";

interface XssCase {
  label: string;
  markdown: string;
  expectAbsent: ReadonlyArray<string>;
  expectPresent?: ReadonlyArray<string>;
}

const XSS_CORPUS: ReadonlyArray<XssCase> = [
  {
    label: "strips <script>alert(1)</script>",
    markdown: "<script>alert(1)</script>after",
    expectAbsent: ["<script", "alert("],
    expectPresent: ["after"],
  },
  {
    label: "strips <img onerror=...>",
    markdown: '<img src="x" onerror="alert(1)">after',
    expectAbsent: ["onerror", '<img src="x"'],
    expectPresent: ["after"],
  },
  {
    label: "strips [click](javascript:alert(1))",
    markdown: "[click](javascript:alert(1))",
    expectAbsent: ['href="javascript:', "javascript:alert"],
    expectPresent: ["click"],
  },
  {
    label: "strips <iframe>",
    markdown: '<iframe src="https://evil"></iframe>after',
    expectAbsent: ["iframe", "https://evil"],
    expectPresent: ["after"],
  },
  {
    label: 'strips <a href="data:...">',
    markdown: '<a href="data:text/html,<script>alert(1)</script>">click</a> tail',
    expectAbsent: ["data:text/html"],
    expectPresent: ["tail"],
  },
  {
    label: "strips <style>",
    markdown: "<style>body{display:none}</style>after",
    expectAbsent: ["<style"],
    expectPresent: ["after"],
  },
  {
    label: "strips <form><input>",
    markdown: '<form action="https://evil"><input></form>after',
    expectAbsent: ["<form", "<input"],
    expectPresent: ["after"],
  },
  {
    label: "strips <svg onload=...>",
    markdown: '<svg onload="alert(1)">after',
    expectAbsent: ["<svg", "onload"],
    expectPresent: ["after"],
  },
  {
    label: 'strips <a href="vbscript:...">',
    markdown: '<a href="vbscript:msgbox(1)">x</a> tail',
    expectAbsent: ["vbscript:"],
    expectPresent: ["tail"],
  },
  {
    label: 'blocks external <img src="https://...">',
    markdown: "![alt text](https://external/img.png)",
    expectAbsent: ['src="https://external/img.png"'],
    expectPresent: ["[image: alt text]"],
  },
];

describe("STREAMDOWN_SAFE_CONFIG", () => {
  it("Should disallow security-sensitive elements at the output stage", () => {
    const disallowed = STREAMDOWN_SAFE_CONFIG.disallowedElements;
    expect(disallowed).toEqual(
      expect.arrayContaining([
        "script",
        "iframe",
        "object",
        "embed",
        "form",
        "input",
        "button",
        "style",
        "link",
        "meta",
        "base",
        "svg",
        "math",
      ])
    );
  });

  it("Should set skipHtml to true so raw HTML markup is stripped at parse time", () => {
    expect(STREAMDOWN_SAFE_CONFIG.skipHtml).toBe(true);
  });

  it("Should disable the streamdown table/code controls and line numbers", () => {
    expect(STREAMDOWN_SAFE_CONFIG.controls).toBe(false);
    expect(STREAMDOWN_SAFE_CONFIG.lineNumbers).toBe(false);
  });

  it("Should mount a safe <img> override that blocks external URLs", () => {
    expect(STREAMDOWN_SAFE_CONFIG.components.img).toBeTypeOf("function");
  });
});

describe("DescriptionCard", () => {
  it("Should render safe-mode markdown headings, lists, code, fenced blocks, and tables", () => {
    const markdown = [
      "# Heading 1",
      "## Heading 2",
      "",
      "Paragraph with **bold** and `inline code`.",
      "",
      "- bullet a",
      "- bullet b",
      "",
      "1. ordered a",
      "2. ordered b",
      "",
      "```",
      "fenced code",
      "```",
      "",
      "| a | b |",
      "|---|---|",
      "| 1 | 2 |",
    ].join("\n");
    const { container } = render(<DescriptionCard>{markdown}</DescriptionCard>);

    expect(container.querySelector("h1")?.textContent).toBe("Heading 1");
    expect(container.querySelector("h2")?.textContent).toBe("Heading 2");
    expect(container.querySelector("ul li")?.textContent).toContain("bullet a");
    expect(container.querySelector("ol li")?.textContent).toContain("ordered a");
    const fencedCode = container.querySelector<HTMLElement>("code[data-block]");
    expect(fencedCode?.textContent).toContain("fenced code");
    expect(container.querySelector("table")).not.toBeNull();
    expect(container.querySelector("strong")?.textContent).toBe("bold");
    const inlineCode = Array.from(container.querySelectorAll<HTMLElement>("code")).find(
      el => el.textContent === "inline code"
    );
    expect(inlineCode).toBeDefined();
  });

  it("Should render a styled <img> for relative URLs", () => {
    const { container } = render(<DescriptionCard>{"![alt](./local.png)"}</DescriptionCard>);
    const img = container.querySelector<HTMLImageElement>('[data-slot="markdown-image"]');
    expect(img).not.toBeNull();
    expect(img?.getAttribute("src")).toBe("/local.png");
    expect(container.querySelector('[data-slot="markdown-image-fallback"]')).toBeNull();
  });

  it("Should rerender when forwarded Markdown container props change", () => {
    const { rerender } = render(
      <Markdown data-testid="markdown-props" aria-label="Initial markdown">
        hello
      </Markdown>
    );

    const initial = screen.getByTestId("markdown-props");
    expect(initial).toHaveAttribute("aria-label", "Initial markdown");

    rerender(
      <Markdown
        data-testid="markdown-props"
        aria-label="Updated markdown"
        style={{ color: "rgb(95, 191, 133)" }}
      >
        hello
      </Markdown>
    );

    const updated = screen.getByTestId("markdown-props");
    expect(updated).toHaveAttribute("aria-label", "Updated markdown");
    expect(updated).toHaveStyle({ color: "rgb(95, 191, 133)" });
  });

  it.each(XSS_CORPUS)("$label", ({ markdown, expectAbsent, expectPresent }) => {
    const { container } = render(<DescriptionCard>{markdown}</DescriptionCard>);
    const html = container.innerHTML;
    for (const fragment of expectAbsent) {
      expect(html.toLowerCase()).not.toContain(fragment.toLowerCase());
    }
    if (expectPresent) {
      for (const fragment of expectPresent) {
        expect(container.textContent).toContain(fragment);
      }
    }
    // Every payload MUST exit without inserting any disallowed root element.
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("iframe")).toBeNull();
    expect(container.querySelector("style")).toBeNull();
    expect(container.querySelector("form")).toBeNull();
    expect(container.querySelector("input")).toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    expect(container.querySelector("object")).toBeNull();
    expect(container.querySelector("embed")).toBeNull();
  });
});
