import { bypass, http, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type { FileContentResponse, FileEntry, FileRootPayload } from "../adapters/files-api";

import chimeMp3Url from "./fixtures/chime.mp3?url";
import demoWebmUrl from "./fixtures/demo.webm?url";
import skylinePngUrl from "./fixtures/skyline.png?url";

export const filesRootFixture: FileRootPayload = {
  kind: "worktree",
  workspace_id: "ws_alpha",
  worktree_id: "wt_review",
  name: "compozy",
  branch: "feat/file-explorer",
};

/** One directory level per listing path, as the daemon answers (directories first). */
export const fileListingsFixture: Record<string, FileEntry[]> = {
  "": [
    { name: ".github", path: ".github", kind: "directory" },
    { name: "docs", path: "docs", kind: "directory" },
    { name: "internal", path: "internal", kind: "directory", contains_changes: true },
    { name: "node_modules", path: "node_modules", kind: "directory", ignored: true },
    { name: "web", path: "web", kind: "directory", contains_changes: true },
    { name: ".env", path: ".env", kind: "file", size: 64, sensitive: true },
    { name: "go.mod", path: "go.mod", kind: "file", size: 2_140, git_status: "modified" },
    { name: "README.md", path: "README.md", kind: "file", size: 4_812 },
  ],
  docs: [
    { name: "empty", path: "docs/empty", kind: "directory" },
    { name: "guide.md", path: "docs/guide.md", kind: "file", size: 612 },
    { name: "people.csv", path: "docs/people.csv", kind: "file", size: 96 },
    { name: "metrics.csv", path: "docs/metrics.csv", kind: "file", size: 96_000 },
    { name: "demo.webm", path: "docs/demo.webm", kind: "file", size: 22_057 },
    { name: "chime.mp3", path: "docs/chime.mp3", kind: "file", size: 12_347 },
    { name: "skyline.png", path: "docs/skyline.png", kind: "file", size: 6_291_456 },
    { name: "report.pdf", path: "docs/report.pdf", kind: "file", size: 1_409 },
    { name: "page.html", path: "docs/page.html", kind: "file", size: 240 },
    { name: "analysis.ipynb", path: "docs/analysis.ipynb", kind: "file", size: 820 },
    { name: "logo.png", path: "docs/logo.png", kind: "file", size: 68 },
    { name: "diagram.bin", path: "docs/diagram.bin", kind: "file", size: 2_411_724 },
  ],
  "docs/empty": [],
  internal: [
    {
      name: "projectfiles",
      path: "internal/projectfiles",
      kind: "directory",
      contains_changes: true,
    },
  ],
  "internal/projectfiles": [
    {
      name: "reader.go",
      path: "internal/projectfiles/reader.go",
      kind: "file",
      size: 5_120,
      git_status: "modified",
    },
    {
      name: "status.go",
      path: "internal/projectfiles/status.go",
      kind: "file",
      size: 1_024,
      git_status: "untracked",
    },
  ],
  web: [{ name: "src", path: "web/src", kind: "directory" }],
  "web/src": [
    { name: "main.tsx", path: "web/src/main.tsx", kind: "file", size: 1_380, symlink: true },
  ],
};

const MAIN_TSX = Array.from(
  { length: 40 },
  (_, index) => `export const line${index + 1} = ${JSON.stringify(`value ${index + 1}`)};`
).join("\n");

/** Markdown with every preview feature #1605 names: headings, a table, Mermaid, relative refs. */
export const GUIDE_MD = [
  "---",
  "title: File explorer guide",
  "---",
  "# File explorer",
  "",
  "The viewer renders **Markdown**, tables and diagrams.",
  "",
  "## Formats",
  "",
  "| Format | Preview |",
  "| ------ | ------- |",
  "| `.md`  | Rendered prose |",
  "| `.csv` | Table |",
  "",
  "```mermaid",
  "graph LR; Source-->Preview; Preview-->Split",
  "```",
  "",
  "Read the [table fixture](./people.csv) next to the logo: ![Logo](./logo.png)",
].join("\n");

const PEOPLE_CSV =
  'id,name,role\n1,Ada Lovelace,Analyst\n2,Linus Torvalds,Kernel\n3,"Hopper, Grace",Compilers\n';

// 2,000 records, so the table preview has to virtualize its rows.
const METRICS_CSV = [
  "id,name,team,started,note",
  ...Array.from({ length: 2_000 }, (_, index) => {
    const id = index + 1;
    const team = ["Core", "Web", "Infra"][index % 3];
    const month = String((index % 12) + 1).padStart(2, "0");
    return `${id},Person ${id},${team},2026-${month}-01,"Row ${id}, quoted"`;
  }),
].join("\n");

// A two-page PDF (Helvetica, text near the top of each page) for the PDF preview.
const REPORT_PDF =
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

const PAGE_HTML =
  "<!doctype html><html><head><style>body{font:14px system-ui;padding:24px}h1{color:#0a7}</style></head>" +
  '<body><h1>Static page</h1><p>Rendered without scripts.</p><script>document.body.textContent="ran"</script></body></html>';

const ANALYSIS_IPYNB = JSON.stringify({
  nbformat: 4,
  nbformat_minor: 5,
  metadata: { language_info: { name: "python" } },
  cells: [
    { cell_type: "markdown", source: ["# Analysis\n", "Load the fixture and count rows."] },
    {
      cell_type: "code",
      execution_count: 1,
      source: ["import pandas as pd\n", "df = pd.read_csv('people.csv')\n", "len(df)"],
      outputs: [{ output_type: "execute_result", data: { "text/plain": ["3"] } }],
    },
  ],
});

const LOGO_PNG =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==";

function content(path: string, body: Partial<FileContentResponse>): FileContentResponse {
  return { root: filesRootFixture, path, kind: "text", size: 0, truncated: false, ...body };
}

/** Content per path; `reveal` returns the sensitive file's text. */
export function fileContentFixture(path: string, reveal: boolean): FileContentResponse | null {
  switch (path) {
    case "web/src/main.tsx":
      return content(path, { size: MAIN_TSX.length, text: MAIN_TSX, revision: "r1" });
    case "go.mod":
      return content(path, {
        size: 2_140,
        text: "module github.com/compozy/compozy\n\ngo 1.26\n\nrequire github.com/bluekeyes/go-gitdiff v0.8.1\n",
      });
    case "README.md":
      return content(path, {
        size: 14.2 * 1024 * 1024,
        text: "# CompozyOS\n\nThe first megabyte of a larger file.\n",
        truncated: true,
      });
    case ".env":
      return reveal
        ? content(path, { size: 64, text: "API_TOKEN=storybook-fixture\n", sensitive: true })
        : content(path, { kind: "sensitive", size: 64, sensitive: true });
    case "docs/logo.png":
      return content(path, {
        kind: "image",
        size: 68,
        image_mime: "image/png",
        image_base64: LOGO_PNG,
      });
    case "docs/guide.md":
      return content(path, { size: GUIDE_MD.length, text: GUIDE_MD, revision: "r-guide" });
    case "docs/people.csv":
      return content(path, { size: PEOPLE_CSV.length, text: PEOPLE_CSV, revision: "r-csv" });
    case "docs/page.html":
      return content(path, { size: PAGE_HTML.length, text: PAGE_HTML, revision: "r-html" });
    case "docs/analysis.ipynb":
      return content(path, { size: ANALYSIS_IPYNB.length, text: ANALYSIS_IPYNB, revision: "r-nb" });
    case "docs/metrics.csv":
      return content(path, { size: METRICS_CSV.length, text: METRICS_CSV, revision: "r-metrics" });
    case "docs/report.pdf":
      return content(path, { kind: "binary", size: 1_409 });
    case "docs/demo.webm":
      return content(path, { kind: "binary", size: 22_057 });
    case "docs/chime.mp3":
      return content(path, { kind: "binary", size: 12_347 });
    // Past the inline image cap, so the viewer streams it from the raw-bytes route.
    case "docs/skyline.png":
      return content(path, {
        kind: "image",
        size: 6_291_456,
        image_mime: "image/png",
        truncated: true,
      });
    case "docs/diagram.bin":
      return content(path, { kind: "binary", size: 2_411_724 });
    default:
      return null;
  }
}

// Media fixtures ship as files; the raw route reads them past MSW and serves their bytes.
const RAW_ASSETS: Record<string, { url: string; type: string }> = {
  "docs/demo.webm": { url: demoWebmUrl, type: "video/webm" },
  "docs/chime.mp3": { url: chimeMp3Url, type: "audio/mpeg" },
  "docs/skyline.png": { url: skylinePngUrl, type: "image/png" },
};

/** Serves a fixture's bytes for a raw-bytes URL of any root (project or session). */
async function rawFileResponse(request: Request) {
  const prefix = "/files/raw/";
  const pathname = new URL(request.url).pathname;
  const path = decodeURIComponent(pathname.slice(pathname.indexOf(prefix) + prefix.length));
  const asset = RAW_ASSETS[path];
  if (asset) {
    const bytes = await (await fetch(bypass(new URL(asset.url, request.url)))).arrayBuffer();
    return new HttpResponse(bytes, { headers: { "Content-Type": asset.type } });
  }
  if (path === "docs/report.pdf") {
    const bytes = Uint8Array.from(atob(REPORT_PDF), char => char.charCodeAt(0));
    return new HttpResponse(bytes, { headers: { "Content-Type": "application/pdf" } });
  }
  const body = fileContentFixture(path, false);
  if (body?.kind === "image" && body.image_base64) {
    const bytes = Uint8Array.from(atob(body.image_base64), char => char.charCodeAt(0));
    return new HttpResponse(bytes, { headers: { "Content-Type": body.image_mime ?? "" } });
  }
  if (body?.kind === "text") {
    return new HttpResponse(body.text ?? "", { headers: { "Content-Type": "text/plain" } });
  }
  return HttpResponse.json({ code: "file_not_found", error: "file not found" }, { status: 404 });
}

/**
 * The raw-bytes routes (#1605). The file path is a wildcard suffix with slashes, which the typed
 * OpenAPI mock cannot match, so they use plain MSW routes.
 */
const workspaceRawFileHandler = http.get(
  "*/api/workspaces/:workspaceId/files/raw/*",
  ({ request }) => rawFileResponse(request)
);
const sessionRawFileHandler = http.get(
  "*/api/workspaces/:workspaceId/sessions/:sessionId/files/raw/*",
  ({ request }) => rawFileResponse(request)
);

/** Session-root file routes over the fixtures above. */
export const sessionFilesHandlers = [
  sessionRawFileHandler,
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/files/entries",
    ({ request }) => {
      const path = new URL(request.url).searchParams.get("path") ?? "";
      const entries = fileListingsFixture[path];
      if (!entries) {
        return HttpResponse.json(
          { code: "file_not_found", error: "file not found" },
          { status: 404 }
        );
      }
      return HttpResponse.json({ root: filesRootFixture, path, entries, truncated: false });
    }
  ),
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/files/content",
    ({ request }) => {
      const params = new URL(request.url).searchParams;
      const body = fileContentFixture(params.get("path") ?? "", params.get("reveal") === "true");
      if (!body) {
        return HttpResponse.json(
          { code: "file_not_found", error: "file not found" },
          { status: 404 }
        );
      }
      return HttpResponse.json(body);
    }
  ),
];

/** Project-root file routes (the Files app) over the same fixtures. */
export const workspaceFilesHandlers = [
  workspaceRawFileHandler,
  compozyApiMock.get("/api/workspaces/{workspace_id}/files/entries", ({ request }) => {
    const path = new URL(request.url).searchParams.get("path") ?? "";
    const entries = fileListingsFixture[path];
    if (!entries) {
      return HttpResponse.json(
        { code: "file_not_found", error: "file not found" },
        { status: 404 }
      );
    }
    return HttpResponse.json({ root: filesRootFixture, path, entries, truncated: false });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/files/content", ({ request }) => {
    const params = new URL(request.url).searchParams;
    const body = fileContentFixture(params.get("path") ?? "", params.get("reveal") === "true");
    if (!body) {
      return HttpResponse.json(
        { code: "file_not_found", error: "file not found" },
        { status: 404 }
      );
    }
    return HttpResponse.json(body);
  }),
];

/** The bound worktree's directory is gone (US-001.EC-2). */
export const sessionFilesRootUnavailableHandlers = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/files/entries", () =>
    HttpResponse.json({ code: "root_unavailable", error: "root is unavailable" }, { status: 409 })
  ),
];
