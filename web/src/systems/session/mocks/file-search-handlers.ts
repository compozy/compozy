import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type { SessionFileEntry } from "../adapters/session-files-api";

/** A small working directory for the `@` picker: real repo-style paths. */
export const sessionFileEntriesFixture: SessionFileEntry[] = [
  { path: "README.md", name: "README.md", kind: "file" },
  { path: "go.mod", name: "go.mod", kind: "file" },
  { path: "docs/", name: "docs", kind: "folder" },
  { path: "internal/", name: "internal", kind: "folder" },
  { path: "web/", name: "web", kind: "folder" },
  { path: "docs/My Notes.md", name: "My Notes.md", kind: "file" },
  { path: "internal/acp/", name: "acp", kind: "folder" },
  { path: "web/src/", name: "src", kind: "folder" },
  { path: "internal/acp/client.go", name: "client.go", kind: "file" },
  { path: "internal/acp/client_prompt.go", name: "client_prompt.go", kind: "file" },
  { path: "internal/acp/client_steer.go", name: "client_steer.go", kind: "file" },
];

/** Name matches before path-only matches, shallower first — a stand-in for the daemon ranking. */
export function searchSessionFileEntriesFixture(
  query: string,
  limit: number,
  entries: readonly SessionFileEntry[] = sessionFileEntriesFixture
): SessionFileEntry[] {
  const needle = query.trim().toLowerCase();
  const depth = (entry: SessionFileEntry) => entry.path.replace(/\/$/, "").split("/").length;
  const tier = (entry: SessionFileEntry) =>
    entry.name.toLowerCase().includes(needle)
      ? 0
      : entry.path.toLowerCase().includes(needle)
        ? 1
        : 2;
  return entries
    .filter(entry => needle === "" || tier(entry) < 2)
    .sort(
      (a, b) =>
        (needle === "" ? 0 : tier(a) - tier(b)) ||
        depth(a) - depth(b) ||
        a.path.localeCompare(b.path)
    )
    .slice(0, limit);
}

export const sessionFileSearchHandlers = [
  compozyApiMock.get(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/files",
    ({ request }) => {
      const url = new URL(request.url);
      const limit = Number(url.searchParams.get("limit") ?? 50);
      return HttpResponse.json({
        entries: searchSessionFileEntriesFixture(url.searchParams.get("q") ?? "", limit),
        truncated: false,
      });
    }
  ),
];
