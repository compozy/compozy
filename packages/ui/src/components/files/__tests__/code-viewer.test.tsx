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
import { delimiterForPath, parseDelimited } from "../delimited-text";
import { FileHtmlPreview } from "../file-html-preview";
import { prepareHtmlPreview } from "../html-preview-document";
import { FileMediaPreview } from "../file-media-preview";
import { FileNotebookPreview } from "../file-notebook-preview";
import { FileTablePreview } from "../file-table-preview";
import { imageDataUrl } from "../image-data-url";
import { parseNotebook } from "../notebook-model";

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

  // #1605: an image past the inline cap streams from a URL; a load failure shows the fallback.
  it("Should load a URL source and fall back when it fails to decode", () => {
    const { getByRole, getByText } = render(
      <FileImagePreview src="/api/raw/huge.png" alt="huge.png" fallback={<p>Too large</p>} />
    );
    const image = getByRole("img", { name: "huge.png" });
    expect(image.getAttribute("src")).toBe("/api/raw/huge.png");
    fireEvent.error(image);
    expect(getByText("Too large")).toBeInTheDocument();
  });
});

// #1605 previews. Invariant: each rendered form shows the file's data faithfully (CSV records,
// notebook cells and outputs) and an untrusted document never runs script or reaches the app.
describe("delimited text", () => {
  it("Should parse quoted delimiters, doubled quotes, embedded newlines, CRLF and a BOM", () => {
    const table = parseDelimited(
      '\uFEFFname,note\r\n"a, b","say ""hi"""\r\n"multi\nline",x\n',
      ","
    );
    expect(table.header).toEqual(["name", "note"]);
    expect(table.rows).toEqual([
      ["a, b", 'say "hi"'],
      ["multi\nline", "x"],
    ]);
    // A quoted empty field is a record, at the end of the text or alone.
    expect(parseDelimited('h\n""', ",").rows).toEqual([[""]]);
    expect(parseDelimited('""\n', ",")).toEqual({ header: [""], rows: [], columnCount: 1 });
  });

  it("Should pad ragged rows to the widest record and pick the delimiter by extension", () => {
    const table = parseDelimited("a\tb\n1\t2\t3\n4", "\t");
    expect(table.columnCount).toBe(3);
    expect(table.rows).toEqual([["1", "2", "3"], ["4"]]);
    expect(delimiterForPath("data/out.TSV")).toBe("\t");
    expect(delimiterForPath("data/out.csv")).toBe(",");
  });

  it("Should render the header row and report every record, or the empty state", () => {
    const { container, getAllByRole, rerender, getByText } = render(
      <FileTablePreview text={"id,name\n1,ada\n2,linus\n"} delimiter="," />
    );
    // The first header is the row-number gutter.
    expect(getAllByRole("columnheader").map(cell => cell.textContent)).toEqual(["", "id", "name"]);
    const table = container.querySelector('[data-slot="file-table-preview"]');
    expect(table?.getAttribute("data-row-count")).toBe("2");
    expect(table?.querySelector("table")?.getAttribute("aria-rowcount")).toBe("3");
    rerender(<FileTablePreview text="" delimiter="," empty={<p>No rows</p>} />);
    expect(getByText("No rows")).toBeInTheDocument();
  });
});

describe("FileTablePreview virtualization", () => {
  // Invariant: under the React Compiler, the rows in view render once the scroll container
  // has a size; the virtualizer's output is never frozen at its first, unmeasured render.
  it("Should render the rows in view once the container is measured", async () => {
    // TanStack Virtual sizes the viewport from the scroll element's client box.
    vi.spyOn(Element.prototype, "clientHeight", "get").mockReturnValue(280);
    vi.spyOn(Element.prototype, "clientWidth", "get").mockReturnValue(800);
    const text = ["id,name", ...Array.from({ length: 500 }, (_, i) => `${i},row ${i}`)].join("\n");
    const { container } = render(<FileTablePreview text={text} delimiter="," />);
    await waitFor(() =>
      expect(
        container.querySelectorAll('[data-slot="file-table-preview-row"]').length
      ).toBeGreaterThan(0)
    );
    const rendered = container.querySelectorAll('[data-slot="file-table-preview-row"]').length;
    expect(rendered).toBeLessThan(500);
  });
});

describe("FileHtmlPreview", () => {
  it("Should strip scripts, handlers, frames and forms and pin a no-script CSP and base", () => {
    const html =
      '<html><head><meta http-equiv="refresh" content="0;url=https://x.test"><base href="https://evil.test/"></head>' +
      '<body><script>alert(1)</script><img src="a.png" onerror="alert(2)"><iframe src="x"></iframe>' +
      '<form action="/api"><input></form><p>Hello</p></body></html>';
    const prepared = prepareHtmlPreview(
      html,
      "http://127.0.0.1:2323/api/workspaces/ws/files/raw/docs/"
    );
    const document = new DOMParser().parseFromString(prepared, "text/html");
    expect(document.querySelector("script, iframe, form, meta[http-equiv='refresh']")).toBeNull();
    expect(document.querySelector("img")?.hasAttribute("onerror")).toBe(false);
    expect(document.querySelectorAll("base")).toHaveLength(1);
    expect(document.querySelector("base")?.getAttribute("href")).toBe(
      "http://127.0.0.1:2323/api/workspaces/ws/files/raw/docs/"
    );
    const policy = document.head.firstElementChild;
    expect(policy?.getAttribute("http-equiv")).toBe("Content-Security-Policy");
    expect(policy?.getAttribute("content")).toContain("script-src 'none'");
    expect(policy?.getAttribute("content")).toContain("img-src data: http://127.0.0.1:2323");
    expect(document.body.textContent).toContain("Hello");
  });

  it("Should route element and CSS asset URLs through the resolver, keeping data URLs", () => {
    const base = "http://127.0.0.1:2323/api/workspaces/ws/files/raw/docs/";
    const prepared = prepareHtmlPreview(
      '<html><head><style>body { background: url("img/bg.png") } i { background: url(data:image/png;base64,AA) }</style></head>' +
        '<body><img src="a.png" srcset="a@2x.png 2x"><p style="background-image: url(../b.png)">x</p></body></html>',
      base,
      url => `${url}?scoped`
    );
    const document = new DOMParser().parseFromString(prepared, "text/html");
    expect(document.querySelector("img")?.getAttribute("src")).toBe(`${base}a.png?scoped`);
    expect(document.querySelector("img")?.hasAttribute("srcset")).toBe(false);
    expect(document.querySelector("style")?.textContent).toContain(
      `url("${base}img/bg.png?scoped")`
    );
    expect(document.querySelector("style")?.textContent).toContain("url(data:image/png;base64,AA)");
    expect(document.querySelector("p")?.getAttribute("style")).toContain(
      'url("http://127.0.0.1:2323/api/workspaces/ws/files/raw/b.png?scoped")'
    );
  });

  it("Should quote resolved CSS URLs whose path holds apostrophes and parentheses", () => {
    const base = "http://127.0.0.1:2323/raw/O'Reilly (1)/";
    const prepared = prepareHtmlPreview(
      "<html><head><style>a { background: url('img/a(2).png') } b { background: url( c.png ) }</style></head><body></body></html>",
      base,
      url => url
    );
    const css = new DOMParser().parseFromString(prepared, "text/html").querySelector("style");
    expect(css?.textContent).toContain(
      `url("http://127.0.0.1:2323/raw/O'Reilly%20(1)/img/a(2).png")`
    );
    expect(css?.textContent).toContain(`url("http://127.0.0.1:2323/raw/O'Reilly%20(1)/c.png")`);
  });

  it("Should frame the document with a sandbox that never allows scripts", () => {
    const { getByTitle } = render(<FileHtmlPreview html="<p>Hi</p>" title="index.html" />);
    const frame = getByTitle("index.html");
    expect(frame.getAttribute("sandbox")).toBe("allow-same-origin");
    expect(frame.getAttribute("srcdoc")).toContain("<p>Hi</p>");
  });
});

describe("notebooks", () => {
  const NOTEBOOK = JSON.stringify({
    nbformat: 4,
    metadata: { language_info: { name: "python" } },
    cells: [
      { cell_type: "markdown", source: ["# Title\n", "Some *text*"] },
      {
        cell_type: "code",
        execution_count: 3,
        source: "print('hi')",
        outputs: [
          { output_type: "stream", name: "stdout", text: ["hi\n"] },
          {
            output_type: "display_data",
            data: { "image/png": "iVBORw0KGgo=\n", "text/plain": "<Figure>" },
          },
          { output_type: "execute_result", data: { "text/plain": ["42"] } },
          {
            output_type: "error",
            ename: "ValueError",
            evalue: "bad",
            traceback: ["\u001b[31mValueError\u001b[0m: bad"],
          },
        ],
      },
    ],
  });

  it("Should read cells and reduce each output to its richest safe form", () => {
    const notebook = parseNotebook(NOTEBOOK);
    expect(notebook?.language).toBe("python");
    expect(notebook?.cells[0]).toEqual({
      id: "cell-0",
      kind: "markdown",
      source: "# Title\nSome *text*",
    });
    const code = notebook?.cells[1];
    expect(code?.kind === "code" && code.executionCount).toBe(3);
    expect(code?.kind === "code" && code.outputs).toEqual([
      { id: "cell-1-output-0", kind: "text", text: "hi\n", stream: "stdout" },
      { id: "cell-1-output-1", kind: "image", mimeType: "image/png", data: "iVBORw0KGgo=" },
      { id: "cell-1-output-2", kind: "text", text: "42" },
      { id: "cell-1-output-3", kind: "error", name: "ValueError: bad", text: "ValueError: bad" },
    ]);
    // A rich HTML repr (a DataFrame) wins over its plain-text summary.
    const frame = parseNotebook(
      JSON.stringify({
        nbformat: 4,
        cells: [
          {
            cell_type: "code",
            source: "df",
            outputs: [
              {
                output_type: "execute_result",
                data: { "text/plain": "   a\n0  1", "text/html": "<table><td>1</td></table>" },
              },
            ],
          },
        ],
      })
    )?.cells[0];
    expect(frame?.kind === "code" && frame.outputs[0]).toMatchObject({ kind: "html" });
    expect(parseNotebook('{"cells": [')).toBeNull();
    expect(parseNotebook('{"nbformat": 4}')).toBeNull();
  });

  it("Should render the cells with their prompts, or the fallback for unreadable JSON", () => {
    const { container, getByText, rerender } = render(<FileNotebookPreview text={NOTEBOOK} />);
    expect(container.querySelectorAll('[data-slot="file-notebook-cell"]')).toHaveLength(2);
    expect(getByText("[3]:")).toBeInTheDocument();
    expect(container.querySelector('[data-error="true"]')?.textContent).toContain(
      "ValueError: bad"
    );
    rerender(<FileNotebookPreview text="not json" fallback={<p>Unreadable</p>} />);
    expect(getByText("Unreadable")).toBeInTheDocument();
  });
});

describe("FileMediaPreview", () => {
  it("Should stream the source with native controls and fall back when it cannot play", () => {
    const { container, getByText } = render(
      <FileMediaPreview
        kind="video"
        src="/api/raw/demo.mp4"
        title="demo.mp4"
        fallback={<p>Binary</p>}
      />
    );
    const video = container.querySelector("video");
    expect(video?.getAttribute("src")).toBe("/api/raw/demo.mp4");
    expect(video?.hasAttribute("controls")).toBe(true);
    fireEvent.error(video as HTMLVideoElement);
    expect(getByText("Binary")).toBeInTheDocument();
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
