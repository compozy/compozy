import { fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CodeHighlightPoolProvider } from "../code-highlight-pool";
import { CodeViewer } from "../code-viewer";
import { countLines, resolveTargetLine } from "../code-viewer-lines";
import { FileImagePreview } from "../file-image-preview";
import { imageDataUrl } from "../image-data-url";

function sourceText(lines: number): string {
  return Array.from({ length: lines }, (_, index) => `const line${index + 1} = ${index + 1};`).join(
    "\n"
  );
}

function renderedLine(container: HTMLElement, line: number): HTMLElement | null {
  const host = container.querySelector("diffs-container");
  return host?.shadowRoot?.querySelector<HTMLElement>(`[data-line="${line}"]`) ?? null;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("CodeViewer", () => {
  // UT-041
  it("Should scroll the target line into view and mark it highlighted", async () => {
    const scrollTo = vi.spyOn(Element.prototype, "scrollTo");
    const { container } = render(
      <CodeViewer path="internal/a.go" text={sourceText(312)} targetLine={42} />
    );

    const viewer = container.querySelector('[data-slot="code-viewer"]');
    expect(viewer).toHaveAttribute("data-highlighted-line", "42");
    await waitFor(() => expect(renderedLine(container, 42)).toHaveAttribute("data-selected-line"));
    expect(renderedLine(container, 41)).not.toHaveAttribute("data-selected-line");
    const scrolledTo = scrollTo.mock.calls.map(
      ([options]) => (options as ScrollToOptions).top ?? 0
    );
    expect(Math.max(...scrolledTo)).toBeGreaterThan(0);
    expect(container.querySelector('[data-slot="code-viewer-past-end"]')).toBeNull();
  });

  // UT-042
  it("Should show the past-the-end notice and highlight the last line", async () => {
    const { container, getByRole } = render(
      <CodeViewer path="internal/a.go" text={sourceText(312)} targetLine={420} />
    );

    expect(getByRole("status")).toHaveTextContent(
      "Line 420 is past the end of this file. Showing line 312."
    );
    expect(container.querySelector('[data-slot="code-viewer"]')).toHaveAttribute(
      "data-highlighted-line",
      "312"
    );
    await waitFor(() => expect(renderedLine(container, 312)).toHaveAttribute("data-selected-line"));
  });

  it("Should scroll the active find match into view without selecting its line", async () => {
    const scrollTo = vi.spyOn(Element.prototype, "scrollTo");
    const findMatches = [
      { line: 3, start: 6, end: 11 },
      { line: 280, start: 6, end: 13 },
    ];
    const { container } = render(
      <CodeViewer
        path="internal/a.go"
        text={sourceText(312)}
        findMatches={findMatches}
        activeFindMatch={1}
      />
    );
    const viewer = container.querySelector('[data-slot="code-viewer"]');
    expect(viewer).toHaveAttribute("data-find-matches", "2");
    expect(viewer).toHaveAttribute("data-find-active-line", "280");
    await waitFor(() => expect(renderedLine(container, 280)).not.toBeNull());
    expect(renderedLine(container, 280)).not.toHaveAttribute("data-selected-line");
    const scrolledTo = scrollTo.mock.calls.map(
      ([options]) => (options as ScrollToOptions).top ?? 0
    );
    expect(Math.max(...scrolledTo)).toBeGreaterThan(0);
  });

  // Invariant: the viewer is never blank while the highlighter loads (US-003): the lines paint
  // as plain text with line numbers first, and the highlighted view replaces them.
  it("Should paint plain numbered lines while the highlight pool is not ready", () => {
    // The worker is the I/O boundary: a stand-in that never answers keeps the pool loading.
    const inertWorker = () =>
      ({
        postMessage() {},
        addEventListener() {},
        removeEventListener() {},
        terminate() {},
      }) as unknown as Worker;
    const { container, unmount } = render(
      <CodeHighlightPoolProvider workerFactory={inertWorker}>
        <CodeViewer path="internal/a.go" text={sourceText(312)} targetLine={200} />
      </CodeHighlightPoolProvider>
    );
    const plain = container.querySelector('[data-slot="code-viewer-plain"]');
    expect(plain).not.toBeNull();
    const target = plain?.querySelector('[data-line="200"]');
    expect(target).toHaveTextContent("200const line200 = 200;");
    expect(target).toHaveAttribute("data-target-line", "true");
    expect(plain?.querySelector('[data-line="1"]')).toBeNull();
    expect(renderedLine(container, 200)).toBeNull();
    unmount();
  });

  it("Should replace the plain paint once the highlighted view renders", async () => {
    const { container } = render(<CodeViewer path="internal/a.go" text={sourceText(40)} />);
    await waitFor(() => expect(renderedLine(container, 1)).not.toBeNull());
    expect(container.querySelector('[data-slot="code-viewer-plain"]')).toBeNull();
  });

  it("Should count lines without a phantom trailing line and clamp targets", () => {
    expect(countLines("a\nb\n")).toBe(2);
    expect(countLines("")).toBe(1);
    expect(resolveTargetLine(0, 10)).toEqual({ line: 1, pastEnd: false, requested: 1 });
    expect(resolveTargetLine(null, 10)).toBeNull();
  });
});

describe("FileImagePreview", () => {
  // UT-043
  it("Should render an SVG with a script only through an img data URL", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script><rect/></svg>';
    const { container, getByRole } = render(
      <FileImagePreview data={btoa(svg)} mimeType="image/svg+xml" alt="logo.svg" />
    );

    const image = getByRole("img", { name: "logo.svg" });
    expect(image.getAttribute("src")).toMatch(/^data:image\/svg\+xml;base64,/);
    expect(container.querySelector("svg")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
  });

  // US-004.AC-1: the preview names its dimensions, read from the decoded image.
  it("Should caption the preview with the loaded dimensions and the size", () => {
    const { getByRole, container } = render(
      <FileImagePreview data={btoa("png")} mimeType="image/png" alt="shot.png" meta="245 KB" />
    );
    const caption = () =>
      container.querySelector('[data-slot="file-image-preview-caption"]')?.textContent;
    expect(caption()).toBe("245 KB");
    const image = getByRole("img", { name: "shot.png" });
    Object.defineProperty(image, "naturalWidth", { value: 1280 });
    Object.defineProperty(image, "naturalHeight", { value: 720 });
    fireEvent.load(image);
    expect(caption()).toBe("1280 × 720 · 245 KB");
  });

  it("Should render the fallback for a non-previewable type or a bad payload", () => {
    const { getByText } = render(
      <FileImagePreview data="AAAA" mimeType="text/html" alt="page" fallback={<p>No preview</p>} />
    );
    expect(getByText("No preview")).toBeInTheDocument();
    expect(imageDataUrl("image/png", "<svg>")).toBeNull();
  });
});
