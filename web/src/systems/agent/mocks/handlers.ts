import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { sessionFixtures } from "@/systems/session/mocks";

import { agentFixtures, FIXTURE_AGENT_DEFINITION_DIGEST } from "./fixtures";
import type { AgentHeartbeatPayload, AgentPayload, AgentSoulPayload } from "../types";

function cloneAgents(): AgentPayload[] {
  return structuredClone(agentFixtures);
}

let agentsState: AgentPayload[] = cloneAgents();
const soulBodies = new Map<string, { body: string; digest: string }>();
const heartbeatBodies = new Map<string, { body: string; digest: string }>();

export function resetAgentMockState(): void {
  agentsState = cloneAgents();
  soulBodies.clear();
  heartbeatBodies.clear();
}

function agentByName(name: string): AgentPayload | undefined {
  return agentsState.find(agent => agent.name === name);
}

function activeAgentNamesFromSessions(): string[] {
  return [
    ...new Set(
      sessionFixtures
        .filter(session => session.state === "active" && Boolean(session.agent_name))
        .map(session => session.agent_name as string)
    ),
  ];
}

interface AgentCatalogMockOptions {
  activeAgents?: readonly string[];
  sessionsAvailable?: boolean;
}

export function agentCatalogMockResponse(
  request: Request,
  agents: readonly AgentPayload[] = agentsState,
  options: AgentCatalogMockOptions = {}
) {
  const url = new URL(request.url);
  const exactName = url.searchParams.get("name")?.trim() ?? "";
  const search = url.searchParams.get("q")?.trim().toLowerCase() ?? "";
  const category = url.searchParams.get("category")?.trim() ?? "";
  const status = url.searchParams.get("status")?.trim() ?? "";
  const limit = Number(url.searchParams.get("limit") ?? "50");
  const start = Number(url.searchParams.get("cursor") ?? "0");
  const activeAgents = new Set(options.activeAgents ?? activeAgentNamesFromSessions());
  const sessionsAvailable = options.sessionsAvailable ?? true;
  const filtered = agents.filter(agent => {
    if (exactName && agent.name !== exactName) return false;
    const agentCategory = agent.category_path?.join(" / ") ?? "";
    if (
      search &&
      !agent.name.toLowerCase().includes(search) &&
      !agentCategory.toLowerCase().includes(search)
    ) {
      return false;
    }
    if (category && agentCategory !== category) return false;
    if (sessionsAvailable && status === "active" && !activeAgents.has(agent.name)) return false;
    if (sessionsAvailable && status === "idle" && activeAgents.has(agent.name)) return false;
    return true;
  });
  const pageAgents = filtered.slice(start, start + limit);
  const next = start + pageAgents.length;
  const categories = [
    ...new Set(agents.flatMap(agent => agent.category_path?.join(" / ") ?? [])),
  ].sort((left, right) => left.localeCompare(right));

  return {
    agents: pageAgents.map(agent => ({
      agent,
      ...(sessionsAvailable
        ? {
            sessions: activeAgents.has(agent.name)
              ? {
                  active: 1,
                  failed: 0,
                  runtime_seconds: 0,
                  total: 1,
                  last_activity_at: "2026-04-17T18:10:00Z",
                }
              : {
                  active: 0,
                  failed: 0,
                  runtime_seconds: 0,
                  total: 0,
                },
          }
        : {}),
    })),
    facets: {
      active: sessionsAvailable ? agents.filter(agent => activeAgents.has(agent.name)).length : 0,
      categories,
      idle: sessionsAvailable ? agents.filter(agent => !activeAgents.has(agent.name)).length : 0,
      total: agents.length,
    },
    page: {
      has_more: next < filtered.length,
      limit,
      total: filtered.length,
      ...(next < filtered.length ? { next_cursor: String(next) } : {}),
    },
    sessions_available: sessionsAvailable,
  };
}

function nextDigest(seed: string): string {
  const hex = Array.from(seed)
    .map(char => char.charCodeAt(0).toString(16).padStart(2, "0"))
    .join("")
    .padEnd(64, "a")
    .slice(0, 64);
  return hex;
}

function authoredMarkdown(source: string): string {
  if (!source.startsWith("---\n")) return source;

  const closingDelimiter = source.indexOf("\n---", 4);
  if (closingDelimiter < 0) return source;

  return source.slice(closingDelimiter + 4).replace(/^\r?\n/, "");
}

const HEARTBEAT_SUBSET = {
  active_session_only: false,
  allow_active_hours_preferences: true,
  context_projection_bytes: 0,
  default_interval: "30m",
  enabled: true,
  max_body_bytes: 65536,
  max_wakes_per_cycle: 1,
  min_interval: "5m",
  session_health_hook_min_interval: "1m",
  session_health_stale_after: "10m",
  wake_cooldown: "1m",
  wake_event_retention: "24h",
} as const;

function emptySoulPrompt(active: boolean): AgentHeartbeatPayload["prompt"] {
  return {
    active,
    context: {},
    max_body_bytes: 65536,
    max_bytes: 65536,
    preferences: { context: {}, min_interval: "30m" },
    truncated: false,
  };
}

function buildSoulPayload(
  name: string,
  stored: { body: string; digest: string } | undefined
): AgentSoulPayload {
  const active = Boolean(stored);
  const digest = stored?.digest ?? FIXTURE_AGENT_DEFINITION_DIGEST;
  return {
    active,
    present: active,
    enabled: true,
    body: stored ? authoredMarkdown(stored.body) : "",
    digest,
    valid: true,
    validation_status: active ? "valid" : "missing",
    frontmatter: {},
    limits: { max_body_bytes: 65536 },
    config_provenance: {
      digest,
      enabled: true,
      context_projection_bytes: 0,
      max_body_bytes: 65536,
    },
    agent_name: name,
  };
}

function buildHeartbeatPayload(
  name: string,
  stored: { body: string; digest: string } | undefined,
  overrides?: {
    active?: boolean;
    enabled?: boolean;
    validation_status?: AgentHeartbeatPayload["validation_status"];
  }
): AgentHeartbeatPayload {
  const active = overrides?.active ?? Boolean(stored);
  const enabled = overrides?.enabled ?? active;
  const digest = stored?.digest ?? FIXTURE_AGENT_DEFINITION_DIGEST;
  return {
    active,
    present: Boolean(stored) || active,
    enabled,
    valid: true,
    validation_status: overrides?.validation_status ?? (active ? "valid" : "missing"),
    digest,
    schema_version: 1,
    agent_name: name,
    frontmatter: { enabled, version: 1, context: {}, preferences: {} },
    guidance_markdown: stored ? authoredMarkdown(stored.body) : "",
    limits: { max_body_bytes: 65536 },
    preferences: { min_interval: "30m", context: {} },
    prompt: emptySoulPrompt(active),
    config_provenance: {
      digest,
      subset: { ...HEARTBEAT_SUBSET, enabled },
    },
  };
}

function soulRevision(name: string, action: "put" | "delete" | "rollback", digest?: string) {
  return {
    id: `rev-${action}-${name}`,
    action,
    actor: { kind: "user" },
    agent_name: name,
    created_at: new Date().toISOString(),
    source_path: `${name}/SOUL.md`,
    ...(digest ? { new_digest: digest } : {}),
  };
}

function heartbeatRevision(
  name: string,
  operation: "write" | "delete" | "rollback",
  digest?: string
) {
  return {
    id: `rev-hb-${operation}-${name}`,
    operation,
    actor: { kind: "user" as const },
    agent_name: name,
    created_at: new Date().toISOString(),
    source_path: `${name}/HEARTBEAT.md`,
    ...(digest ? { new_digest: digest } : {}),
  };
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/agents", () => HttpResponse.json({ agents: agentsState })),
  aghApiMock.get("/api/agents/catalog", ({ request }) =>
    HttpResponse.json(agentCatalogMockResponse(request))
  ),
  aghApiMock.get("/api/agents/{name}", ({ params }) => {
    const name = String(params.name);
    const agent = agentByName(name);
    if (!agent) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json({ agent });
  }),
  aghApiMock.put("/api/agents/{name}", async ({ params, request }) => {
    const name = String(params.name);
    const body = (await request.json()) as {
      expected_digest: string;
      agent: AgentPayload;
      workspace?: string;
    };
    const existing = agentByName(name);
    if (!existing) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    if (body.expected_digest !== existing.definition_digest) {
      return HttpResponse.json({ error: "definition digest conflict" }, { status: 409 });
    }
    const updated: AgentPayload = {
      ...existing,
      ...body.agent,
      name,
      definition_digest: nextDigest(`${name}:${body.agent.prompt}:${Date.now()}`),
    };
    agentsState = agentsState.map(agent => (agent.name === name ? updated : agent));
    return HttpResponse.json({ agent: updated });
  }),
  aghApiMock.delete("/api/agents/{name}", ({ params }) => {
    const name = String(params.name);
    const existing = agentByName(name);
    if (!existing) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    agentsState = agentsState.filter(agent => agent.name !== name);
    const unshadowed =
      existing.origin === "workspace" ? { unshadowed_origin: "global" as const } : {};
    return HttpResponse.json({
      name,
      origin: existing.origin,
      ...unshadowed,
    });
  }),
  aghApiMock.post("/api/agents/{name}/duplicate", async ({ params, request }) => {
    const sourceName = String(params.name);
    const source = agentByName(sourceName);
    if (!source) {
      return HttpResponse.json({ error: `Agent not found: ${sourceName}` }, { status: 404 });
    }
    const body = (await request.json()) as {
      name: string;
      scope?: "workspace" | "global";
      workspace?: string;
      overrides?: Partial<AgentPayload> & {
        skills?: { disabled?: string[] } | null;
        deny_tools?: string[];
        reasoning_effort?: AgentPayload["reasoning_effort"];
      };
    };
    if (agentByName(body.name)) {
      return HttpResponse.json({ error: "agent definition already exists" }, { status: 409 });
    }
    const copy: AgentPayload = {
      ...source,
      ...body.overrides,
      name: body.name,
      origin: body.scope ?? source.origin,
      definition_digest: nextDigest(`dup:${body.name}`),
      ...(body.overrides?.skills ? { skills: body.overrides.skills } : {}),
      ...(body.overrides?.deny_tools ? { deny_tools: body.overrides.deny_tools } : {}),
    };
    agentsState = [...agentsState, copy];
    return HttpResponse.json({ agent: copy }, { status: 201 });
  }),
  aghApiMock.get("/api/agents/{name}/soul", ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json(buildSoulPayload(name, soulBodies.get(name)));
  }),
  aghApiMock.put("/api/agents/{name}/soul", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { body: string; expected_digest: string };
    const stored = soulBodies.get(name);
    const currentDigest = stored?.digest ?? FIXTURE_AGENT_DEFINITION_DIGEST;
    if (body.expected_digest !== currentDigest) {
      return HttpResponse.json({ error: "soul digest conflict" }, { status: 409 });
    }
    const digest = nextDigest(`soul:${name}:${body.body}`);
    soulBodies.set(name, { body: body.body, digest });
    const soul = buildSoulPayload(name, { body: body.body, digest });
    return HttpResponse.json({
      soul,
      revision: soulRevision(name, "put", digest),
    });
  }),
  aghApiMock.delete("/api/agents/{name}/soul", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { expected_digest: string };
    const stored = soulBodies.get(name);
    const currentDigest = stored?.digest ?? FIXTURE_AGENT_DEFINITION_DIGEST;
    if (body.expected_digest !== currentDigest) {
      return HttpResponse.json({ error: "soul digest conflict" }, { status: 409 });
    }
    soulBodies.delete(name);
    return HttpResponse.json({
      soul: buildSoulPayload(name, undefined),
      revision: soulRevision(name, "delete"),
    });
  }),
  aghApiMock.post("/api/agents/{name}/soul/validate", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { body: string };
    const valid = body.body.trim().length > 0;
    return HttpResponse.json({
      ...buildSoulPayload(name, {
        body: body.body,
        digest: nextDigest(`soul-validate:${name}`),
      }),
      active: valid,
      present: valid,
      valid,
      validation_status: valid ? "valid" : "invalid",
      diagnostics: [],
    });
  }),
  aghApiMock.get("/api/agents/{name}/soul/history", ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json({ revisions: [] });
  }),
  aghApiMock.post("/api/agents/{name}/soul/rollback", async ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const stored = soulBodies.get(name) ?? {
      body: "rolled back",
      digest: nextDigest(`soul-rb:${name}`),
    };
    soulBodies.set(name, stored);
    return HttpResponse.json({
      soul: buildSoulPayload(name, stored),
      revision: soulRevision(name, "rollback", stored.digest),
    });
  }),
  aghApiMock.get("/api/agents/{name}/heartbeat", ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json(buildHeartbeatPayload(name, heartbeatBodies.get(name)));
  }),
  aghApiMock.put("/api/agents/{name}/heartbeat", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { body: string; expected_digest: string };
    const stored = heartbeatBodies.get(name);
    const currentDigest = stored?.digest ?? FIXTURE_AGENT_DEFINITION_DIGEST;
    if (body.expected_digest !== currentDigest) {
      return HttpResponse.json({ error: "heartbeat digest conflict" }, { status: 409 });
    }
    const digest = nextDigest(`hb:${name}:${body.body}`);
    heartbeatBodies.set(name, { body: body.body, digest });
    return HttpResponse.json({
      heartbeat: buildHeartbeatPayload(
        name,
        { body: body.body, digest },
        { active: true, enabled: true }
      ),
      revision: heartbeatRevision(name, "write", digest),
    });
  }),
  aghApiMock.delete("/api/agents/{name}/heartbeat", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { expected_digest: string };
    const stored = heartbeatBodies.get(name);
    const currentDigest = stored?.digest ?? FIXTURE_AGENT_DEFINITION_DIGEST;
    if (body.expected_digest !== currentDigest) {
      return HttpResponse.json({ error: "heartbeat digest conflict" }, { status: 409 });
    }
    heartbeatBodies.delete(name);
    return HttpResponse.json({
      heartbeat: buildHeartbeatPayload(name, undefined, {
        active: false,
        enabled: false,
        validation_status: "missing",
      }),
      revision: heartbeatRevision(name, "delete"),
    });
  }),
  aghApiMock.post("/api/agents/{name}/heartbeat/validate", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { body: string };
    const valid = body.body.trim().length > 0;
    return HttpResponse.json({
      ...buildHeartbeatPayload(name, {
        body: body.body,
        digest: nextDigest(`hb-validate:${name}`),
      }),
      active: valid,
      present: valid,
      enabled: valid,
      valid,
      validation_status: valid ? "valid" : "invalid",
      diagnostics: [],
    });
  }),
  aghApiMock.get("/api/agents/{name}/heartbeat/history", ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json({ revisions: [] });
  }),
  aghApiMock.post("/api/agents/{name}/heartbeat/rollback", async ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const stored = heartbeatBodies.get(name) ?? {
      body: "rolled back",
      digest: nextDigest(`hb-rb:${name}`),
    };
    heartbeatBodies.set(name, stored);
    return HttpResponse.json({
      heartbeat: buildHeartbeatPayload(name, stored, { active: true, enabled: true }),
      revision: heartbeatRevision(name, "rollback", stored.digest),
    });
  }),
  aghApiMock.get("/api/agents/{name}/heartbeat/status", ({ params }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const stored = heartbeatBodies.get(name);
    return HttpResponse.json({
      agent_name: name,
      active: Boolean(stored),
      present: Boolean(stored),
      enabled: Boolean(stored),
      valid: true,
      validation_status: stored ? "valid" : "missing",
      preferences: { min_interval: "30m", context: {} },
    });
  }),
  aghApiMock.post("/api/agents/{name}/heartbeat/wake", async ({ params, request }) => {
    const name = String(params.name);
    if (!agentByName(name)) {
      return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { session_id: string };
    return HttpResponse.json({
      decision: {
        result: "sent",
        reason: "wake_sent",
        wake_event_id: `wake-${body.session_id}`,
      },
    });
  }),
];
