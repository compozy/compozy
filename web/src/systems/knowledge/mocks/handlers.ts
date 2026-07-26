import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import {
  memoryDecisionsFixture,
  memoryDeleteFixture,
  memoryDreamTriggerFixture,
  memoryEditFixture,
  memoryHeadersFixture,
  memoryReadFixtures,
  memorySearchFixture,
  memoryWriteFixture,
} from "./fixtures";
import type {
  MemoryDecision,
  MemoryDecisionOp,
  MemoryDecisionsResponse,
  MemoryHeader,
  MemoryListPage,
} from "../types";

const DEFAULT_PAGE_LIMIT = 50;
const MAX_PAGE_LIMIT = 200;
const CURSOR_PREFIX = "memory:";

interface MemorySelector {
  scope?: string | null;
  workspaceId?: string | null;
  agentName?: string | null;
  agentTier?: string | null;
}

type MemorySort = "recent" | "name";

interface MemoryListRequest {
  selector: MemorySelector;
  type?: MemoryHeader["type"];
  sort?: MemorySort;
  includeSystem: boolean;
  offset: number;
  limit: number;
}

interface MemoryDecisionsRequest {
  selector: MemorySelector;
  op?: MemoryDecisionOp;
  filename?: string;
  since?: number;
  limit: number;
}

type QueryResult<T> = { ok: true; value: T } | { ok: false; message: string };

function readSelector(url: URL): MemorySelector {
  return {
    scope: url.searchParams.get("scope"),
    workspaceId: url.searchParams.get("workspace_id"),
    agentName: url.searchParams.get("agent_name"),
    agentTier: url.searchParams.get("agent_tier"),
  };
}

function matchesSelector(memory: MemoryHeader, selector: MemorySelector): boolean {
  if (selector.scope && memory.scope !== selector.scope) return false;
  if (selector.workspaceId && memory.workspace_id !== selector.workspaceId) return false;
  if (selector.agentName && memory.agent_name !== selector.agentName) return false;
  if (selector.agentTier && memory.agent_tier !== selector.agentTier) return false;
  return true;
}

function readMemoryType(value: string | null): QueryResult<MemoryHeader["type"] | undefined> {
  switch (value) {
    case null:
      return { ok: true, value: undefined };
    case "user":
    case "feedback":
    case "project":
    case "reference":
      return { ok: true, value };
    default:
      return { ok: false, message: `Invalid memory type: ${value}` };
  }
}

function readSort(value: string | null): QueryResult<MemorySort | undefined> {
  switch (value) {
    case null:
      return { ok: true, value: undefined };
    case "recent":
    case "name":
      return { ok: true, value };
    default:
      return { ok: false, message: `Invalid memory sort: ${value}` };
  }
}

function readIncludeSystem(value: string | null): QueryResult<boolean> {
  switch (value) {
    case null:
    case "false":
      return { ok: true, value: false };
    case "true":
      return { ok: true, value: true };
    default:
      return { ok: false, message: `Invalid include_system value: ${value}` };
  }
}

function readLimit(value: string | null): QueryResult<number> {
  if (value === null) {
    return { ok: true, value: DEFAULT_PAGE_LIMIT };
  }
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) {
    return { ok: false, message: `Invalid memory limit: ${value}` };
  }
  return { ok: true, value: Math.min(parsed, MAX_PAGE_LIMIT) };
}

function readOffset(value: string | null): QueryResult<number> {
  if (value === null) {
    return { ok: true, value: 0 };
  }
  if (!value.startsWith(CURSOR_PREFIX)) {
    return { ok: false, message: "Invalid memory cursor" };
  }
  const parsed = Number(value.slice(CURSOR_PREFIX.length));
  if (!Number.isInteger(parsed) || parsed < 0) {
    return { ok: false, message: "Invalid memory cursor" };
  }
  return { ok: true, value: parsed };
}

function readListRequest(url: URL): QueryResult<MemoryListRequest> {
  const type = readMemoryType(url.searchParams.get("type"));
  if (!type.ok) return type;

  const sort = readSort(url.searchParams.get("sort"));
  if (!sort.ok) return sort;

  const includeSystem = readIncludeSystem(url.searchParams.get("include_system"));
  if (!includeSystem.ok) return includeSystem;

  const limit = readLimit(url.searchParams.get("limit"));
  if (!limit.ok) return limit;

  const offset = readOffset(url.searchParams.get("cursor"));
  if (!offset.ok) return offset;

  return {
    ok: true,
    value: {
      selector: readSelector(url),
      type: type.value,
      sort: sort.value,
      includeSystem: includeSystem.value,
      offset: offset.value,
      limit: limit.value,
    },
  };
}

function readDecisionOp(value: string | null): QueryResult<MemoryDecisionOp | undefined> {
  switch (value) {
    case null:
      return { ok: true, value: undefined };
    case "noop":
    case "add":
    case "update":
    case "delete":
    case "reject":
      return { ok: true, value };
    default:
      return { ok: false, message: `Invalid memory decision op: ${value}` };
  }
}

function readDecisionSince(value: string | null): QueryResult<number | undefined> {
  if (value === null) {
    return { ok: true, value: undefined };
  }
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) {
    return { ok: false, message: `Invalid memory decision since: ${value}` };
  }
  return { ok: true, value: parsed };
}

function readDecisionLimit(value: string | null): QueryResult<number> {
  if (value === null) {
    return { ok: true, value: MAX_PAGE_LIMIT };
  }
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) {
    return { ok: false, message: `Invalid memory decision limit: ${value}` };
  }
  return { ok: true, value: Math.min(parsed, MAX_PAGE_LIMIT) };
}

function readDecisionRequest(url: URL): QueryResult<MemoryDecisionsRequest> {
  const op = readDecisionOp(url.searchParams.get("op"));
  if (!op.ok) return op;

  const since = readDecisionSince(url.searchParams.get("since"));
  if (!since.ok) return since;

  const limit = readDecisionLimit(url.searchParams.get("limit"));
  if (!limit.ok) return limit;

  const filename = url.searchParams.get("filename")?.trim();
  return {
    ok: true,
    value: {
      selector: readSelector(url),
      op: op.value,
      filename: filename || undefined,
      since: since.value,
      limit: limit.value,
    },
  };
}

function compareMemoryNames(left: MemoryHeader, right: MemoryHeader): number {
  const leftName = left.name.normalize("NFKD").toLowerCase();
  const rightName = right.name.normalize("NFKD").toLowerCase();
  if (leftName < rightName) return -1;
  if (leftName > rightName) return 1;
  return 0;
}

function sortMemories(memories: MemoryHeader[], sort: MemorySort | undefined): MemoryHeader[] {
  const ordered = [...memories];
  if (sort === "recent") {
    return ordered.sort((left, right) => right.mod_time.localeCompare(left.mod_time));
  }
  if (sort === "name") {
    return ordered.sort(compareMemoryNames);
  }
  return ordered;
}

function listMemories(request: MemoryListRequest): MemoryListPage {
  const matching = memoryHeadersFixture.filter(memory => {
    if (!matchesSelector(memory, request.selector)) return false;
    if (request.type && memory.type !== request.type) return false;
    if (!request.includeSystem && memory.system_managed) return false;
    return true;
  });
  const ordered = sortMemories(matching, request.sort);
  const memories = ordered.slice(request.offset, request.offset + request.limit);
  const nextOffset = request.offset + memories.length;
  const hasMore = nextOffset < ordered.length;
  const page = hasMore
    ? {
        has_more: true,
        limit: request.limit,
        next_cursor: `${CURSOR_PREFIX}${nextOffset}`,
        total: ordered.length,
      }
    : {
        has_more: false,
        limit: request.limit,
        total: ordered.length,
      };

  return { memories, page };
}

function listMemoryDecisions(request: MemoryDecisionsRequest): MemoryDecisionsResponse {
  const decisions = memoryDecisionsFixture.decisions
    .filter(decision => matchesDecisionSelector(decision, request.selector))
    .filter(decision => (request.op ? decision.op === request.op : true))
    .filter(decision => (request.filename ? decision.target_filename === request.filename : true))
    .filter(decision =>
      request.since === undefined ? true : Date.parse(decision.decided_at) >= request.since
    )
    .sort(
      (left, right) =>
        right.decided_at.localeCompare(left.decided_at) || right.id.localeCompare(left.id)
    )
    .slice(0, request.limit);
  return { decisions };
}

function matchesDecisionSelector(decision: MemoryDecision, selector: MemorySelector): boolean {
  if (selector.scope && decision.scope !== selector.scope) return false;
  if (selector.workspaceId && decision.workspace_id !== selector.workspaceId) return false;
  if (selector.agentName && decision.agent_name !== selector.agentName) return false;
  if (selector.agentTier && decision.agent_tier !== selector.agentTier) return false;
  return true;
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/memory", ({ request }) => {
    const query = readListRequest(new URL(request.url));
    if (!query.ok) {
      return HttpResponse.json(
        { code: "memory.invalid_filter", message: query.message },
        { status: 400 }
      );
    }
    return HttpResponse.json(listMemories(query.value));
  }),
  aghApiMock.get("/api/memory/decisions", ({ request }) => {
    const query = readDecisionRequest(new URL(request.url));
    if (!query.ok) {
      return HttpResponse.json(
        { code: "memory.invalid_filter", message: query.message },
        { status: 400 }
      );
    }
    return HttpResponse.json(listMemoryDecisions(query.value));
  }),
  aghApiMock.get("/api/memory/{filename}", ({ params }) => {
    const filename = decodeURIComponent(String(params.filename));
    const memory = memoryReadFixtures[filename];
    if (!memory) {
      return HttpResponse.json(
        { code: "memory.not_found", message: `Memory not found: ${filename}` },
        { status: 404 }
      );
    }
    return HttpResponse.json(memory);
  }),
  aghApiMock.post("/api/memory", () => HttpResponse.json(memoryWriteFixture)),
  aghApiMock.patch("/api/memory/{filename}", () => HttpResponse.json(memoryEditFixture)),
  aghApiMock.delete("/api/memory/{filename}", () => HttpResponse.json(memoryDeleteFixture)),
  aghApiMock.post("/api/memory/search", () => HttpResponse.json(memorySearchFixture)),
  aghApiMock.post("/api/memory/dreams/trigger", () => HttpResponse.json(memoryDreamTriggerFixture)),
];
