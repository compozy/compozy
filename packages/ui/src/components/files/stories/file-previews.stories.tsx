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

// A two-page PDF (Helvetica, text near the top of each page).
const PDF_BASE64 =
  "JVBERi0xLjcKJeLjz9MKMSAwIG9iago8PCAvVHlwZSAvQ2F0YWxvZyAvUGFnZXMgMiAwIFIg" +
  "Pj4KZW5kb2JqCjIgMCBvYmoKPDwgL1R5cGUgL1BhZ2VzIC9LaWRzIFszIDAgUiA0IDAgUl0g" +
  "L0NvdW50IDIgPj4KZW5kb2JqCjMgMCBvYmoKPDwgL1R5cGUgL1BhZ2UgL1BhcmVudCAyIDAg" +
  "UiAvTWVkaWFCb3ggWzAgMCA2MTIgNzkyXSAvUmVzb3VyY2VzIDw8IC9Gb250IDw8IC9GMSA1" +
  "IDAgUiAvRjIgNiAwIFIgPj4gPj4gL0NvbnRlbnRzIDcgMCBSID4+CmVuZG9iago0IDAgb2Jq" +
  "Cjw8IC9UeXBlIC9QYWdlIC9QYXJlbnQgMiAwIFIgL01lZGlhQm94IFswIDAgNjEyIDc5Ml0g" +
  "L1Jlc291cmNlcyA8PCAvRm9udCA8PCAvRjEgNSAwIFIgL0YyIDYgMCBSID4+ID4+IC9Db250" +
  "ZW50cyA4IDAgUiA+PgplbmRvYmoKNSAwIG9iago8PCAvVHlwZSAvRm9udCAvU3VidHlwZSAv" +
  "VHlwZTEgL0Jhc2VGb250IC9IZWx2ZXRpY2EgPj4KZW5kb2JqCjYgMCBvYmoKPDwgL1R5cGUg" +
  "L0ZvbnQgL1N1YnR5cGUgL1R5cGUxIC9CYXNlRm9udCAvSGVsdmV0aWNhLUJvbGQgPj4KZW5k" +
  "b2JqCjcgMCBvYmoKPDwgL0xlbmd0aCAzMjggL0ZpbHRlciAvRmxhdGVEZWNvZGUgPj4Kc3Ry" +
  "ZWFtCnicfZJBU8IwEIXv/op31JmKbYVSPaKiB5lhaMd7SrYQCE0nSan66w1pYXQUDzubtq/f" +
  "vreTSY6baYx4iLzEOMb4NkTOcbkgScwQNNVK2yvkGzzlF5ODOkIU9eok7dRTIcmg1rQX1N4j" +
  "U41eUoB59wKs4shqKSxKpTFjestVWwV4yN6Cs+hk6NEv+ew1QKUsFUptjYM+TgPsBScVgDVc" +
  "KI+XTK8IYsdWZAY/mDGiY7gkTjumWK2lK2vOTg+76dcnt24TFSdt0Aq7dg+SWbEnSFFtjXfw" +
  "1+xvxFGa9kSX2v+Qu65qqsAcwCXStmFSfBKHZYWk8yRXHemwm5Mv4UAwDlyod29SNRZmqUVt" +
  "/3E1SnqWW6vpWahdEhQfXfd5a14ONr8wd6dtecj8II+gSsRHYThIcazFs/vY+ksWhthhNAz9" +
  "SSL7Ap3ctcQKZW5kc3RyZWFtCmVuZG9iago4IDAgb2JqCjw8IC9MZW5ndGggMTQ5IC9GaWx0" +
  "ZXIgL0ZsYXRlRGVjb2RlID4+CnN0cmVhbQp4nF2OQQ6CMBBF957iL3UhthUFl5KoW4O9QA1F" +
  "apA2pVi9vRXsxsX8/Jm8vEzBsToysBS8RsaQrQl4hfneGNlV6rUAv+PAZ8WXo6D0x23zieON" +
  "hBV+eX072cPqwUn00j7DUqs2pFeuQSm6W7gPxmjrkj/nLion41kElEHXYBEkSY445QkUfnyV" +
  "EDywScnYWlw+ayY19wplbmRzdHJlYW0KZW5kb2JqCnhyZWYKMCA5CjAwMDAwMDAwMDAgNjU1" +
  "MzUgZiAKMDAwMDAwMDAxNSAwMDAwMCBuIAowMDAwMDAwMDY0IDAwMDAwIG4gCjAwMDAwMDAx" +
  "MjcgMDAwMDAgbiAKMDAwMDAwMDI2MyAwMDAwMCBuIAowMDAwMDAwMzk5IDAwMDAwIG4gCjAw" +
  "MDAwMDA0NjkgMDAwMDAgbiAKMDAwMDAwMDU0NCAwMDAwMCBuIAowMDAwMDAwOTQ0IDAwMDAw" +
  "IG4gCnRyYWlsZXIKPDwgL1NpemUgOSAvUm9vdCAxIDAgUiA+PgpzdGFydHhyZWYKMTE2NQol" +
  "JUVPRgo=";

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
      title="report.pdf"
    />
  ),
  play: async ({ canvasElement }) => {
    await waitFor(
      () =>
        expect(
          canvasElement
            .querySelector('[data-slot="file-pdf-preview"]')
            ?.getAttribute("data-page-count")
        ).toBe("2"),
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
