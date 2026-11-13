import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { expect, waitFor } from "storybook/test";

import { createPdfWorker } from "../../../lib/pdf-worker";
import { FileHtmlPreview } from "../file-html-preview";
import { FileMediaPreview } from "../file-media-preview";
import { FileNotebookPreview } from "../file-notebook-preview";
import { FilePdfPreview } from "../file-pdf-preview";
import { FileTablePreview } from "../file-table-preview";

const CSV = [
  "id,name,team,started,note",
  ...Array.from(
    { length: 2_000 },
    (_, index) =>
      `${index + 1},Person ${index + 1},${["Core", "Web", "Infra"][index % 3]},2026-${String((index % 12) + 1).padStart(2, "0")}-01,"Row ${index + 1}, quoted"`
  ),
].join("\n");

const HTML =
  "<!doctype html><html><head><style>body{font:14px system-ui;padding:24px}</style></head>" +
  '<body><h1>Report</h1><p>Static markup only.</p><a href="https://example.com">External</a>' +
  '<script>document.body.innerHTML="script ran"</script><img src="x" onerror="document.title=1"></body></html>';

const NOTEBOOK = JSON.stringify({
  nbformat: 4,
  metadata: { language_info: { name: "python" } },
  cells: [
    { cell_type: "markdown", source: ["# Revenue check\n", "Totals per **quarter**."] },
    {
      cell_type: "code",
      execution_count: 1,
      source: "totals = {'Q1': 12, 'Q2': 18}\nprint(totals)",
      outputs: [{ output_type: "stream", name: "stdout", text: ["{'Q1': 12, 'Q2': 18}\n"] }],
    },
    {
      cell_type: "code",
      execution_count: 2,
      source: "totals['Q3']",
      outputs: [
        {
          output_type: "error",
          ename: "KeyError",
          evalue: "'Q3'",
          traceback: ["\u001b[0;31mKeyError\u001b[0m: 'Q3'"],
        },
      ],
    },
  ],
});

// The minimal "Hello, world!" PDF from the pdf.js examples.
const PDF_BASE64 =
  "JVBERi0xLjcKCjEgMCBvYmogICUgZW50cnkgcG9pbnQKPDwKICAvVHlwZSAvQ2F0YWxvZwog" +
  "IC9QYWdlcyAyIDAgUgo+PgplbmRvYmoKCjIgMCBvYmoKPDwKICAvVHlwZSAvUGFnZXMKICAv" +
  "TWVkaWFCb3ggWyAwIDAgMjAwIDIwMCBdCiAgL0NvdW50IDEKICAvS2lkcyBbIDMgMCBSIF0K" +
  "Pj4KZW5kb2JqCgozIDAgb2JqCjw8CiAgL1R5cGUgL1BhZ2UKICAvUGFyZW50IDIgMCBSCiAg" +
  "L1Jlc291cmNlcyA8PAogICAgL0ZvbnQgPDwKICAgICAgL0YxIDQgMCBSIAogICAgPj4KICA+" +
  "PgogIC9Db250ZW50cyA1IDAgUgo+PgplbmRvYmoKCjQgMCBvYmoKPDwKICAvVHlwZSAvRm9u" +
  "dAogIC9TdWJ0eXBlIC9UeXBlMQogIC9CYXNlRm9udCAvVGltZXMtUm9tYW4KPj4KZW5kb2Jq" +
  "Cgo1IDAgb2JqICAlIHBhZ2UgY29udGVudAo8PAogIC9MZW5ndGggNDQKPj4Kc3RyZWFtCkJU" +
  "CjcwIDUwIFRECi9GMSAxMiBUZgooSGVsbG8sIHdvcmxkISkgVGoKRVQKZW5kc3RyZWFtCmVu" +
  "ZG9iagoKeHJlZgowIDYKMDAwMDAwMDAwMCA2NTUzNSBmIAowMDAwMDAwMDEwIDAwMDAwIG4g" +
  "CjAwMDAwMDAwNzkgMDAwMDAgbiAKMDAwMDAwMDE3MyAwMDAwMCBuIAowMDAwMDAwMzAxIDAw" +
  "MDAwIG4gCjAwMDAwMDAzODAgMDAwMDAgbiAKdHJhaWxlcgo8PAogIC9TaXplIDYKICAvUm9v" +
  "dCAxIDAgUgo+PgpzdGFydHhyZWYKNDkyCiUlRU9G";

function Frame({ children }: { children: ReactNode }) {
  return <div className="flex h-[420px] flex-col">{children}</div>;
}

const meta: Meta = {
  title: "components/files/FilePreviews",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Rendered forms for the Files viewer's Preview mode: CSV/TSV tables, sandboxed HTML, Jupyter notebooks, PDF (pdf.js) and streamed video/audio.",
      },
    },
  },
  decorators: [
    Story => (
      <Frame>
        <Story />
      </Frame>
    ),
  ],
};

export default meta;
type Story = StoryObj;

export const Table: Story = {
  tags: ["play-fn"],
  render: () => <FileTablePreview text={CSV} delimiter="," />,
  play: async ({ canvasElement }) => {
    const table = canvasElement.querySelector('[data-slot="file-table-preview"]');
    await expect(table?.getAttribute("data-row-count")).toBe("2000");
    // Virtualized: only the rows near the viewport are in the DOM.
    await expect(
      canvasElement.querySelectorAll('[data-slot="file-table-preview-row"]').length
    ).toBeLessThan(100);
  },
};

export const Html: Story = {
  tags: ["play-fn"],
  render: () => <FileHtmlPreview html={HTML} title="report.html" />,
  play: async ({ canvasElement }) => {
    const frame = canvasElement.querySelector("iframe");
    await expect(frame?.getAttribute("sandbox")).toBe("allow-same-origin");
    await waitFor(() =>
      expect(frame?.contentDocument?.body.textContent).toContain("Static markup only.")
    );
    await expect(frame?.contentDocument?.body.textContent).not.toContain("script ran");
  },
};

export const Notebook: Story = {
  render: () => <FileNotebookPreview text={NOTEBOOK} />,
};

export const Pdf: Story = {
  tags: ["play-fn"],
  render: () => (
    <FilePdfPreview
      url={`data:application/pdf;base64,${PDF_BASE64}`}
      workerFactory={createPdfWorker}
      title="hello.pdf"
    />
  ),
  play: async ({ canvasElement }) => {
    await waitFor(
      () =>
        expect(
          canvasElement
            .querySelector('[data-slot="file-pdf-preview"]')
            ?.getAttribute("data-page-count")
        ).toBe("1"),
      { timeout: 10_000 }
    );
  },
};

export const Audio: Story = {
  render: () => (
    <FileMediaPreview
      kind="audio"
      src="data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAIA+AAACABAAZGF0YQAAAAA="
      title="silence.wav"
      meta="44 B"
    />
  ),
};
