import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import type { AgentMemoryDocument } from "../types";

const MAX_LINES = 200;
const MAX_BYTES = 24 * 1024;

let revision = 0;
let contentState = "";
let digestState = "sha256:empty";
let historyState: { id: string; digest: string; content: string }[] = [];

function lineCount(content: string) {
  return content === "" ? 0 : content.replace(/\n$/, "").split("\n").length;
}

function memoryDocument(): AgentMemoryDocument {
  return {
    content: contentState,
    digest: digestState,
    budget: {
      lines: lineCount(contentState),
      max_lines: MAX_LINES,
      bytes: new TextEncoder().encode(contentState).length,
      max_bytes: MAX_BYTES,
    },
  };
}

function store(content: string) {
  revision += 1;
  contentState = content;
  digestState = `sha256:memory-${revision}`;
  const id = `${String(revision).padStart(20, "0")}-${digestState}.md`;
  historyState = [...historyState, { id, digest: digestState, content }];
}

export function resetAgentMemoryMock(): void {
  revision = 0;
  contentState = "";
  digestState = "sha256:empty";
  historyState = [];
}

/** Each entry is one saved version, oldest first; the last one is the file. */
export function seedAgentMemoryMock(...versions: readonly string[]): void {
  resetAgentMemoryMock();
  versions.forEach(store);
}

/** The memory routes, on one shared file; `_dx.md` Memory. */
export const memoryHandlers: HttpHandler[] = [
  compozyApiMock.get("/api/agents/{name}/memory", () => HttpResponse.json(memoryDocument())),
  compozyApiMock.put("/api/agents/{name}/memory", async ({ request }) => {
    const { content, expected_digest } = (await request.json()) as {
      content: string;
      expected_digest: string;
    };
    if (expected_digest !== digestState) {
      return HttpResponse.json(
        { code: "agent_memory_conflict", error: "Memory changed since you opened it." },
        { status: 409 }
      );
    }
    store(content);
    return HttpResponse.json(memoryDocument());
  }),
  compozyApiMock.post("/api/agents/{name}/memory/validate", () =>
    HttpResponse.json({ valid: true, diagnostics: [] })
  ),
  compozyApiMock.get("/api/agents/{name}/memory/history", () =>
    HttpResponse.json({ revisions: historyState.map(({ id, digest }) => ({ id, digest })) })
  ),
  compozyApiMock.post("/api/agents/{name}/memory/rollback", async ({ request }) => {
    const body = (await request.json()) as { revision: string };
    const target = historyState.find(item => item.id === body.revision);
    if (!target) return HttpResponse.json({ error: "Revision not found" }, { status: 404 });
    store(target.content);
    return HttpResponse.json(memoryDocument());
  }),
];
