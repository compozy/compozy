import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type { FileContentResponse, FileEntry, FileRootPayload } from "../adapters/files-api";

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
    case "docs/diagram.bin":
      return content(path, { kind: "binary", size: 2_411_724 });
    default:
      return null;
  }
}

/** Session-root file routes over the fixtures above. */
export const sessionFilesHandlers = [
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

/** The bound worktree's directory is gone (US-001.EC-2). */
export const sessionFilesRootUnavailableHandlers = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/files/entries", () =>
    HttpResponse.json({ code: "root_unavailable", error: "root is unavailable" }, { status: 409 })
  ),
];
