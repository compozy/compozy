import { act, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CodeBlock, CopyIconButton } from "../code-block";

function installClipboard() {
  const writeText = vi.fn<(value: string) => Promise<void>>().mockResolvedValue(undefined);
  const descriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
  const restore = () => {
    if (descriptor) {
      Object.defineProperty(navigator, "clipboard", descriptor);
    } else {
      Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
    }
  };
  return { writeText, restore };
}

describe("CodeBlock", () => {
  let clipboard: ReturnType<typeof installClipboard>;

  beforeEach(() => {
    clipboard = installClipboard();
  });

  afterEach(() => {
    clipboard.restore();
    vi.useRealTimers();
  });

  it("Should render the provided code inside a <pre><code> wrapper", () => {
    const { container } = render(<CodeBlock code="agh start" />);
    const pre = container.querySelector<HTMLElement>('[data-slot="code-block-pre"]');
    const code = container.querySelector<HTMLElement>('[data-slot="code-block-code"]');
    expect(pre?.tagName).toBe("PRE");
    expect(code?.tagName).toBe("CODE");
    expect(code?.textContent).toContain("agh start");
  });

  it("Should render the `$ ` prompt when showPrompt is true", () => {
    const { container } = render(<CodeBlock code="agh start" />);
    const prompt = container.querySelector<HTMLElement>('[data-slot="code-block-prompt"]');
    expect(prompt).not.toBeNull();
    expect(prompt?.textContent).toBe("$ ");
    expect(prompt?.getAttribute("aria-hidden")).toBe("true");
  });

  it("Should suppress the prompt on continuation and comment lines", () => {
    const code = ["# comment", "agh network status", '    --body \'{"task":"go"}\'', ""].join("\n");
    const { container } = render(<CodeBlock code={code} />);
    const prompts = container.querySelectorAll('[data-slot="code-block-prompt"]');
    expect(prompts.length).toBe(1);
  });

  it("Should omit prompts entirely when showPrompt is false", () => {
    const { container } = render(
      <CodeBlock code={"const x = 1;\nconst y = 2;"} showPrompt={false} />
    );
    expect(container.querySelector('[data-slot="code-block-prompt"]')).toBeNull();
  });

  it("Should render the language eyebrow only when the language prop is provided", () => {
    const { container, rerender } = render(<CodeBlock code="agh start" />);
    expect(container.querySelector('[data-slot="code-block-language"]')).toBeNull();
    rerender(<CodeBlock code="agh start" language="not-a-language" />);
    const eyebrow = container.querySelector<HTMLElement>('[data-slot="code-block-language"]');
    expect(eyebrow?.textContent).toBe("not-a-language");
  });

  it("Should render a caption when it differs from the syntax language", () => {
    const { container } = render(
      <CodeBlock code="agh start" language="not-a-language" caption="agh shell" />
    );
    const eyebrow = container.querySelector<HTMLElement>('[data-slot="code-block-language"]');
    expect(eyebrow?.textContent).toBe("agh shell");
  });

  it("Should highlight supported languages with the Vitesse dark theme", async () => {
    const { container } = render(
      <CodeBlock
        code="const value: number = 1;"
        language="typescript"
        showPrompt={false}
        themeMode="dark"
      />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(root).toHaveAttribute("data-highlight-state", "loading");
    await waitFor(
      () => {
        expect(root).toHaveAttribute("data-highlight-state", "highlighted");
      },
      { timeout: 5_000 }
    );
    expect(root).toHaveAttribute("data-language", "typescript");
    expect(root).toHaveAttribute("data-theme", "vitesse-dark");
    expect(container.querySelectorAll('[data-slot="code-block-token"]').length).toBeGreaterThan(0);
  });

  it("Should normalize language aliases before highlighting", async () => {
    const { container } = render(
      <CodeBlock code="const value = 1;" language="ts" showPrompt={false} themeMode="light" />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    await waitFor(
      () => {
        expect(root).toHaveAttribute("data-highlight-state", "highlighted");
      },
      { timeout: 5_000 }
    );
    expect(root).toHaveAttribute("data-language", "typescript");
    expect(root).toHaveAttribute("data-theme", "vitesse-light");
  });

  it("Should render unsupported languages as escaped plain text", () => {
    const code = '<img src=x onerror="alert(1)">';
    const { container } = render(
      <CodeBlock code={code} language="not-a-language" showPrompt={false} />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    const codeNode = container.querySelector<HTMLElement>('[data-slot="code-block-code"]');
    expect(root).toHaveAttribute("data-highlight-state", "plain");
    expect(codeNode?.textContent).toContain(code);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector('[data-slot="code-block-token"]')).toBeNull();
  });

  it("Should hide the copy button when copyable is false", () => {
    const { container } = render(<CodeBlock code="agh start" copyable={false} />);
    expect(container.querySelector('[data-slot="code-block-copy"]')).toBeNull();
  });

  it("Should call navigator.clipboard.writeText with the code when copy is clicked", async () => {
    const { container } = render(<CodeBlock code="agh network status" />);
    const button = container.querySelector<HTMLButtonElement>('[data-slot="code-block-copy"]')!;
    fireEvent.click(button);
    await waitFor(() => {
      expect(clipboard.writeText).toHaveBeenCalledWith("agh network status");
    });
  });

  it("Should expose copy failure state when clipboard access fails", async () => {
    clipboard.writeText.mockRejectedValueOnce(new Error("blocked"));
    const { container } = render(<CodeBlock code="agh start" />);
    const button = container.querySelector<HTMLButtonElement>('[data-slot="code-block-copy"]')!;
    fireEvent.click(button);
    await waitFor(() => {
      expect(button).toHaveAttribute("data-copy-state", "failed");
    });
    expect(button.getAttribute("aria-label")).toBe("Copy failed");
    expect(button.querySelector("svg.lucide-triangle-alert")).not.toBeNull();
  });

  it("Should swap to the check icon for 1.5s on copy success, then revert", async () => {
    vi.useFakeTimers();
    const { container } = render(<CodeBlock code="agh start" />);
    const button = container.querySelector<HTMLButtonElement>('[data-slot="code-block-copy"]')!;
    expect(button.querySelector("svg.lucide-copy")).not.toBeNull();
    expect(button.querySelector("svg.lucide-check")).toBeNull();
    await act(async () => {
      fireEvent.click(button);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(button.getAttribute("data-copied")).toBe("true");
    expect(button.querySelector("svg.lucide-check")).not.toBeNull();
    expect(button.querySelector("svg.lucide-copy")).toBeNull();
    await act(async () => {
      vi.advanceTimersByTime(1500);
    });
    expect(button.getAttribute("data-copied")).toBeNull();
    expect(button.querySelector("svg.lucide-copy")).not.toBeNull();
  });

  it("Should restart the copy feedback timer when copy is clicked repeatedly", async () => {
    vi.useFakeTimers();
    const { container } = render(<CodeBlock code="agh start" />);
    const button = container.querySelector<HTMLButtonElement>('[data-slot="code-block-copy"]')!;

    await act(async () => {
      fireEvent.click(button);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(button.getAttribute("data-copied")).toBe("true");

    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    await act(async () => {
      fireEvent.click(button);
      await Promise.resolve();
      await Promise.resolve();
    });

    await act(async () => {
      vi.advanceTimersByTime(1499);
    });
    expect(button.getAttribute("data-copied")).toBe("true");

    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(button.getAttribute("data-copied")).toBeNull();
  });

  it("Should apply tone and line truncation attributes", () => {
    const { container } = render(
      <CodeBlock
        code={"one\ntwo\nthree\nfour"}
        showPrompt={false}
        tone="warning"
        truncateLines={2}
      />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    const pre = container.querySelector<HTMLElement>('[data-slot="code-block-pre"]');
    expect(root).toHaveAttribute("data-tone", "warning");
    expect(pre?.style.getPropertyValue("--code-block-lines")).toBe("2");
  });

  it("Should render optional line numbers and highlighted lines", () => {
    const { container } = render(
      <CodeBlock code={"one\ntwo\nthree"} showPrompt={false} showLineNumbers highlightLines={[2]} />
    );
    const lineNumbers = container.querySelectorAll('[data-slot="code-block-line-number"]');
    const highlightedLine = container.querySelector<HTMLElement>('[data-line-number="2"]');
    expect(lineNumbers).toHaveLength(3);
    expect(highlightedLine).toHaveAttribute("data-highlighted", "true");
  });

  it("Should export CopyIconButton as a standalone copy primitive", async () => {
    const { container } = render(<CopyIconButton value="copy me" />);
    const button = container.querySelector<HTMLButtonElement>('[data-slot="code-block-copy"]')!;
    fireEvent.click(button);
    await waitFor(() => {
      expect(clipboard.writeText).toHaveBeenCalledWith("copy me");
    });
  });
});
