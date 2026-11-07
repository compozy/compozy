import { fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CodeHighlightPoolProvider } from "../code-highlight-pool";
import { CodeViewer } from "../code-viewer";
import { countLines, resolveTargetLine } from "../code-viewer-lines";
import { FileImagePreview } from "../file-image-preview";
import {
  FIND_DIRECT_PAINT_LIMIT,
  FIND_PAINT_BUDGET,
  FIND_WINDOW_MARGIN,
  matchesInWindow,
  visibleCharWindow,
} from "../find-highlight-window";
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

// Perf audit P-4. Invariant: Find never registers a highlight range per match on a pathological
// line, and never measures ranges deep inside it. A minified line's 10,000 matches paint only the
// ones in the viewport's character window, capped by the paint budget.
describe("find highlight window", () => {
  const view = { top: 0, bottom: 600, left: 0, right: 800 };
  const starts = Array.from({ length: 10_000 }, (_, index) => index * 100);

  it("Should map the viewport to the visible characters of a scrolled long line", () => {
    // 8 px glyphs, scrolled 40,000 px: characters 5,000..5,100 are on screen.
    const line = { top: 100, bottom: 120, left: -40_000, right: 8_000_000 };
    const window = visibleCharWindow({ line, view, charWidth: 8, length: 1_000_000, wrap: false });
    expect(window).toEqual([5_000 - FIND_WINDOW_MARGIN, 5_100 + FIND_WINDOW_MARGIN]);
  });

  it("Should map visible rows of a wrapped long line to characters", () => {
    // 100 glyphs per row, 20 px rows, the viewport shows rows 50..80.
    const line = { top: -1_000, bottom: 199_000, left: 0, right: 800 };
    const [from, to] = visibleCharWindow({
      line,
      view,
      charWidth: 8,
      length: 1_000_000,
      wrap: true,
    });
    expect(from).toBe(5_000 - FIND_WINDOW_MARGIN);
    expect(to).toBe(8_100 + FIND_WINDOW_MARGIN);
  });

  it("Should paint only the window's matches of a 10,000-match line, within the budget", () => {
    const painted = matchesInWindow(starts, [500_000, 510_000], FIND_PAINT_BUDGET);
    expect(painted[0]).toBe(5_000);
    expect(painted.at(-1)).toBe(5_099);
    expect(matchesInWindow(starts, [0, 1_000_000], FIND_PAINT_BUDGET)).toHaveLength(
      FIND_PAINT_BUDGET
    );
    expect(matchesInWindow(starts, [0, 1_000_000], 0)).toEqual([]);
  });

  it("Should paint an ordinary line whole", () => {
    const few = starts.slice(0, FIND_DIRECT_PAINT_LIMIT);
    expect(matchesInWindow(few, [0, 1], FIND_PAINT_BUDGET)).toHaveLength(FIND_DIRECT_PAINT_LIMIT);
  });
});
