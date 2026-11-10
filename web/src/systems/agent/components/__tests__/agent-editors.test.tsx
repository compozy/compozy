// Invariant: the editors save real files through the digest guard, and tell the truth when a
// file can't be shown or a route doesn't answer yet; Permissions offers the five levels, marks Auto
// unavailable from provider data, and saves exceptions and approval reach with the definition.
// Owner: the agent-editors components, over HTTP; canonical suite for UT-077, UT-078, UT-083,
// UT-058 (agent editor half), UT-096 and the check-in source rows and Add source dialog
// (UT-070 to UT-076).
// HTTP is the mocked boundary; the router blocker has no router here, so it is stubbed.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { ReactElement } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import {
  settingsGeneralSectionFixture,
  settingsProvidersCollectionFixture,
} from "@/systems/settings/mocks";
import { handlers as statusHandlers } from "@/systems/status/mocks";

import { primarySessionFixture } from "@/systems/session/mocks/fixtures";

import { primaryAgentFixture } from "../../mocks/fixtures";
import { handlers, resetAgentMockState } from "../../mocks/handlers";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { seedAgentInstanceMock } from "../../mocks/instance-handlers";
import { seedAgentMemoryMock } from "../../mocks/memory-handlers";
import {
  sourceDraftFrom,
  sourceFromDraft,
  sourceValuePair,
  validateSourceDraft,
  type SourceParamValue,
} from "../../lib/agent-check-in-source-draft";
import type {
  AgentHeartbeatPayload,
  AgentHeartbeatSourceCatalog,
  AgentPayload,
  AgentSoulPayload,
} from "../../types";
import { AgentCheckInsEditor } from "../agent-editors/agent-check-ins-editor";
import { AgentMemoryEditor } from "../agent-editors/agent-memory-editor";
import { AgentPermissionsEditor } from "../agent-editors/agent-permissions-editor";
import {
  AgentWhatItDoesEditor,
  type AgentEditorProps,
} from "../agent-editors/agent-what-it-does-editor";

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useBlocker: () => ({ status: "idle", proceed: undefined, reset: undefined }),
  useNavigate: () => vi.fn(),
}));

// The editors also read shell data: projects, tools, sessions, providers, models and Settings.
// Serve all of it and fail any unhandled request in-process. MSW would otherwise pass it to a
// real socket, and the refused connection fails the mocked requests queued on it (#1445).
const shellHandlers = [
  compozyApiMock.get("/api/workspaces", () => HttpResponse.json({ workspaces: [] })),
  compozyApiMock.get("/api/tools", () => HttpResponse.json({ tools: [] })),
  compozyApiMock.get("/api/sessions", () =>
    HttpResponse.json({ sessions: [], page: { has_more: false, limit: 50, total: 0 } })
  ),
  compozyApiMock.get("/api/providers", () => HttpResponse.json({ providers: [] })),
  compozyApiMock.get("/api/settings/general", () =>
    HttpResponse.json(settingsGeneralSectionFixture)
  ),
  compozyApiMock.get("/api/settings/providers", () =>
    HttpResponse.json({ ...settingsProvidersCollectionFixture, providers: [] })
  ),
  compozyApiMock.get("/api/model-catalog/models", () => HttpResponse.json({ models: [] })),
  compozyApiMock.get("/api/model-catalog/sources/status", () => HttpResponse.json({ sources: [] })),
];
const server = setupServer(...profileHandlers, ...statusHandlers, ...handlers, ...shellHandlers);
const agent: AgentPayload = { ...primaryAgentFixture, label: "Triage Lead" };

function props(overrides: Partial<AgentEditorProps> = {}): AgentEditorProps {
  return { agent, workspace: null, state: "idle", readOnly: false, ...overrides };
}

function renderEditor(editor: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{editor}</QueryClientProvider>);
}

function soulPayload(overrides: Partial<AgentSoulPayload>): AgentSoulPayload {
  return {
    active: true,
    present: true,
    enabled: true,
    valid: true,
    validation_status: "valid",
    digest: "soul-1",
    body: "Keep issues moving.",
    frontmatter: { tone: ["direct"], constraints: ["Delete files"] },
    limits: { max_body_bytes: 65536 },
    config_provenance: {
      digest: "cfg",
      enabled: true,
      context_projection_bytes: 0,
      max_body_bytes: 65536,
    },
    agent_name: agent.name,
    ...overrides,
  } as AgentSoulPayload;
}

function serveSoul(payload: AgentSoulPayload) {
  server.use(http.get("*/api/agents/:name/soul", () => HttpResponse.json(payload)));
}

function heartbeatPayload(overrides: Partial<AgentHeartbeatPayload> = {}): AgentHeartbeatPayload {
  return {
    active: true,
    present: true,
    enabled: true,
    valid: true,
    validation_status: "valid",
    digest: "beat-1",
    schema_version: 1,
    agent_name: agent.name,
    guidance_markdown: "",
    frontmatter: {
      version: 1,
      enabled: true,
      preferences: {
        min_interval: "30m",
        active_hours: [{ timezone: "UTC", start: "09:00", end: "18:00" }],
      },
      context: { include: ["task"] },
    },
    limits: { max_body_bytes: 65536 },
    preferences: { min_interval: "30m", context: {} },
    prompt: {
      active: true,
      context: {},
      max_body_bytes: 65536,
      max_bytes: 65536,
      preferences: { context: {}, min_interval: "30m" },
      truncated: false,
    },
    config_provenance: {
      digest: "cfg",
      subset: {
        active_session_only: false,
        allow_active_hours_preferences: true,
        context_projection_bytes: 0,
        default_interval: "30m",
        enabled: true,
        max_body_bytes: 65536,
        max_wakes_per_cycle: 1,
        min_interval: "15m",
        session_health_hook_min_interval: "1m",
        session_health_stale_after: "10m",
        wake_cooldown: "1m",
        wake_event_retention: "24h",
      },
    },
    ...overrides,
  } as AgentHeartbeatPayload;
}

function serveHeartbeat(payload: AgentHeartbeatPayload) {
  server.use(http.get("*/api/agents/:name/heartbeat", () => HttpResponse.json(payload)));
}

const MEMORY = `- 2026-10-31 · compozy · Dispatch one executor per issue.
- 2020-01-02 · reloaddev · The old staging host. · until 2020-02-01
Prefer short status updates.
`;

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
beforeEach(() => resetAgentMockState());
afterEach(() => {
  cleanup();
  server.resetHandlers();
});
afterAll(() => server.close());

describe("Instructions", () => {
  it("Should show the blank page with examples when there's no file", async () => {
    serveSoul(soulPayload({ present: false, body: "", frontmatter: {} }));
    renderEditor(<AgentWhatItDoesEditor {...props()} />);
    expect(await screen.findByText("Start from an example")).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Instructions" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Keep issues moving" })).toBeTruthy();
  });

  it("Should offer Use theirs and Keep mine on a conflict, and keep mine on the new digest", async () => {
    serveSoul(soulPayload({}));
    const puts: { expected_digest: string; body: string }[] = [];
    server.use(
      http.put("*/api/agents/:name/soul", async ({ request }) => {
        const body = (await request.json()) as { expected_digest: string; body: string };
        puts.push(body);
        if (body.expected_digest === "soul-1") {
          serveSoul(
            soulPayload({ digest: "soul-2", body: "Keep issues moving.\n\nFlag old reviews." })
          );
          return HttpResponse.json({ error: "conflict" }, { status: 409 });
        }
        return HttpResponse.json({ soul: soulPayload({ digest: "soul-3", body: body.body }) });
      })
    );
    const user = userEvent.setup();
    renderEditor(<AgentWhatItDoesEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Friendly" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Triage Lead changed this while you were editing")).toBeTruthy();
    expect(screen.getByText('Adds "Flag old reviews."')).toBeTruthy();
    expect(screen.getByRole("button", { name: "Use theirs" })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Keep mine" }));
    await waitFor(() => expect(puts).toHaveLength(2));
    expect(puts[1]?.expected_digest).toBe("soul-2");
    expect(puts[1]?.body).toContain('- "friendly"');
    await waitFor(() =>
      expect(screen.queryByText("Triage Lead changed this while you were editing")).toBeNull()
    );
  });

  it("Should adopt their version on Use theirs", async () => {
    serveSoul(soulPayload({}));
    server.use(
      http.put("*/api/agents/:name/soul", () => {
        serveSoul(soulPayload({ digest: "soul-2", frontmatter: { tone: ["formal"] } }));
        return HttpResponse.json({ error: "conflict" }, { status: 409 });
      })
    );
    const user = userEvent.setup();
    renderEditor(<AgentWhatItDoesEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Friendly" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await user.click(await screen.findByRole("button", { name: "Use theirs" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Formal" }).getAttribute("aria-pressed")).toBe(
        "true"
      )
    );
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("Should open a file with a table in Advanced only, with the reason", async () => {
    serveSoul(soulPayload({ body: "| Step | Who |\n| --- | --- |\n| Tag | Ana |" }));
    renderEditor(<AgentWhatItDoesEditor {...props()} />);
    expect(
      await screen.findByText(
        "This file has a table the editor can't show, so it opens here. Nothing gets lost."
      )
    ).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "Instructions" })).toBeNull();
    expect(screen.queryByText("How it should sound")).toBeNull();
    expect(
      (screen.getByRole("textbox", { name: "The file behind this page" }) as HTMLTextAreaElement)
        .value
    ).toContain("| Step | Who |");
  });

  it("Should show validation errors and not save invalid raw content", async () => {
    serveSoul(soulPayload({ body: "| a | b |\n| --- | --- |" }));
    const put = vi.fn();
    server.use(
      http.post("*/api/agents/:name/soul/validate", () =>
        HttpResponse.json(
          soulPayload({
            valid: false,
            validation_status: "invalid",
            diagnostics: [
              {
                code: "malformed_frontmatter",
                message: "SOUL.md frontmatter is not valid YAML",
                severity: "error",
                line: 2,
              },
            ],
          })
        )
      ),
      http.put("*/api/agents/:name/soul", () => {
        put();
        return HttpResponse.json({});
      })
    );
    const user = userEvent.setup();
    renderEditor(<AgentWhatItDoesEditor {...props()} />);
    const file = await screen.findByRole("textbox", { name: "The file behind this page" });
    fireEvent.change(file, { target: { value: "---\ntone: [direct\n---\n" } });
    await user.click(await screen.findByRole("button", { name: "Save" }));
    expect(await screen.findByText("Line 2: This file is not valid YAML")).toBeTruthy();
    expect(screen.getByText("Fix the file first")).toBeTruthy();
    expect(put).not.toHaveBeenCalled();
  });

  it("Should be read-only and name the owner when an extension owns the agent", async () => {
    serveSoul(soulPayload({}));
    renderEditor(
      <AgentWhatItDoesEditor
        {...props({ agent: { ...agent, package_owned: true }, readOnly: true })}
      />
    );
    expect(
      await screen.findByText(
        "An extension owns this agent, so you can read this page but not change it."
      )
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Remove Delete files" })).toBeNull();
    expect(screen.queryByRole("textbox", { name: "Add something it must never do" })).toBeNull();
  });
});

describe("Heartbeat", () => {
  it("Should show that checks need a place when the agent runs nowhere", async () => {
    serveHeartbeat(heartbeatPayload());
    const onAddToProject = vi.fn();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props({ onAddToProject })} />);
    expect(await screen.findByText("Nowhere to check in yet")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Add it to a project" }));
    expect(onAddToProject).toHaveBeenCalled();
  });

  it.each([null, "@user"])(
    "Should enable its project checks from definition scope %s",
    async workspace => {
      serveHeartbeat(heartbeatPayload());
      seedAgentInstanceMock([
        agentInstanceFixture({
          agent_name: agent.name,
          checks_enabled: false,
          last_wake_at: "",
          next_wake_at: "",
        }),
      ]);
      const user = userEvent.setup();
      renderEditor(<AgentCheckInsEditor {...props({ workspace })} />);
      await user.click(await screen.findByRole("button", { name: "Turn on heartbeat" }));
      await waitFor(() =>
        expect(screen.getByRole("switch", { name: "Heartbeat" }).getAttribute("aria-checked")).toBe(
          "true"
        )
      );
    }
  );

  it("Should say the heartbeat is off once checks were turned off", async () => {
    serveHeartbeat(heartbeatPayload());
    seedAgentInstanceMock([
      agentInstanceFixture({
        agent_name: agent.name,
        checks_enabled: false,
        last_wake_at: new Date().toISOString(),
        next_wake_at: "",
      }),
    ]);
    renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findByText("Heartbeat is off")).toBeTruthy();
    expect(screen.queryByText("Next heartbeat")).toBeNull();
  });

  it("Should not claim a report for a real run that delivered nothing", async () => {
    serveHeartbeat(heartbeatPayload());
    seedAgentInstanceMock([agentInstanceFixture({ agent_name: agent.name })]);
    server.use(
      http.get("*/api/agent-instances/:id/occurrences/:occurrence", () =>
        HttpResponse.json({ state: "finished", result: "skipped", reason: "duplicate_suppressed" })
      )
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    await user.click(await screen.findByRole("button", { name: "Try it now" }));
    await user.click(await screen.findByRole("button", { name: "Run it for real" }));

    expect(await screen.findByText("It already told you this recently.")).toBeTruthy();
    expect(screen.queryByText("Done. It told you in its main chat.")).toBeNull();
  });

  it("Should turn checks on without writing a file the operator never chose", async () => {
    serveHeartbeat(heartbeatPayload({ present: false }));
    const puts: unknown[] = [];
    server.use(
      http.put("*/api/agents/:name/heartbeat", async ({ request }) => {
        puts.push(await request.json());
        return HttpResponse.json({});
      })
    );
    seedAgentInstanceMock([
      agentInstanceFixture({ agent_name: agent.name, checks_enabled: false, last_wake_at: "" }),
    ]);
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Turn on heartbeat" }));
    await waitFor(() =>
      expect(screen.getByRole("switch", { name: "Heartbeat" }).getAttribute("aria-checked")).toBe(
        "true"
      )
    );
    expect(puts).toEqual([]);
  });

  it("Should map reasons from real heartbeats and a dry run's digest", async () => {
    serveHeartbeat(heartbeatPayload());
    const now = new Date();
    seedAgentInstanceMock(
      [agentInstanceFixture({ agent_name: agent.name, last_wake_at: now.toISOString() })],
      [
        {
          id: "c1",
          occurrence_id: "occ-1",
          proposals: { auto_accepted: 0, waiting: 0 },
          result: "failed",
          reason: "triage_timeout",
          created_at: now.toISOString(),
          tool_calls: [],
          tools_status: "enabled",
        },
      ],
      [{ key: "task:1", summary: "Migrate tokens is stuck" }]
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findByText("It ran out of time.")).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Heartbeat" })).toBeTruthy();
    expect(screen.getByText("Next heartbeat")).toBeTruthy();
    expect(screen.getByRole("list", { name: "Recent heartbeats" })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Try it now" }));
    expect(await screen.findByText("It would look into 1 thing")).toBeTruthy();
    expect(screen.getByText("Migrate tokens is stuck")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Run it for real" }));
    expect(await screen.findByText("Done. It told you in its main chat.")).toBeTruthy();
  });

  it("Should say so plainly when the routes don't answer yet", async () => {
    serveHeartbeat(heartbeatPayload());
    seedAgentInstanceMock([
      agentInstanceFixture({ agent_name: agent.name, last_wake_at: new Date().toISOString() }),
    ]);
    server.use(
      http.get("*/api/agent-instances/:id/check-ins", () =>
        HttpResponse.json({ code: "not_implemented" }, { status: 501 })
      ),
      http.post("*/api/agent-instances/:id/check", () =>
        HttpResponse.json({ code: "not_found" }, { status: 404 })
      )
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findByText("Heartbeats will show up here once they start.")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Try it now" }));
    expect(await screen.findByText("This can't run here yet.")).toBeTruthy();
  });

  it("Should offer Use theirs and Keep mine on a conflict", async () => {
    serveHeartbeat(heartbeatPayload());
    seedAgentInstanceMock([
      agentInstanceFixture({ agent_name: agent.name, last_wake_at: new Date().toISOString() }),
    ]);
    server.use(
      http.put("*/api/agents/:name/heartbeat", () =>
        HttpResponse.json({ error: "conflict" }, { status: 409 })
      )
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("switch", { name: "Tasks that got stuck" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("button", { name: "Keep mine" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Use theirs" })).toBeTruthy();
  });

  const SOURCES = [
    {
      id: "new-issues",
      kind: "forge-github/issues",
      with: { repo: "compozy/compozy", exclude_labels: ["wontfix", "duplicate"], backfill: "0s" },
    },
    { id: "deploys", script: "failed-deploys", with: { env: "production", retry: { max: 3 } } },
  ];

  function sourcesPayload(sources: readonly object[] = SOURCES, digest = "beat-1") {
    const base = heartbeatPayload();
    return heartbeatPayload({
      digest,
      frontmatter: {
        ...base.frontmatter,
        context: { include: ["task"], sources },
      } as AgentHeartbeatPayload["frontmatter"],
    });
  }

  function serveSourceCatalog(overrides: Record<string, unknown> = {}) {
    server.use(
      http.get("*/api/agents/:name/heartbeat/sources", () =>
        HttpResponse.json({
          agent: agent.name,
          scripts_enabled: true,
          limits: { max_per_agent: 16, max_items: 20 },
          kinds: [
            { kind: "forge-github/issues", extension: "forge-github", title: "New GitHub issues" },
          ],
          scripts: [],
          ...overrides,
        })
      )
    );
  }

  it("Should open a file with sources in the form and keep them through a save [UT-070]", async () => {
    serveHeartbeat(sourcesPayload());
    serveSourceCatalog();
    seedAgentInstanceMock([
      agentInstanceFixture({ agent_name: agent.name, last_wake_at: new Date().toISOString() }),
    ]);
    const bodies: string[] = [];
    server.use(
      http.put("*/api/agents/:name/heartbeat", async ({ request }) => {
        const { body } = (await request.json()) as { body: string };
        bodies.push(body);
        return HttpResponse.json({ heartbeat: sourcesPayload(SOURCES, "beat-2") });
      })
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    expect(await screen.findByText("How often")).toBeTruthy();
    expect(screen.queryByText(/has settings the editor can't show/)).toBeNull();
    expect(await screen.findByText("New GitHub issues")).toBeTruthy();
    expect(screen.getByText("failed-deploys")).toBeTruthy();

    await user.click(screen.getByRole("switch", { name: "Tasks that got stuck" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toContain(
      [
        "context:",
        "  include: []",
        "  sources:",
        '    - id: "new-issues"',
        '      kind: "forge-github/issues"',
        "      with:",
        '        repo: "compozy/compozy"',
        '        exclude_labels: ["wontfix","duplicate"]',
        '        backfill: "0s"',
        '    - id: "deploys"',
        '      script: "failed-deploys"',
        "      with:",
        '        env: "production"',
        '        retry: {"max":3}',
      ].join("\n")
    );
  });

  it("Should say how each source last went, never OK before its first read [UT-071]", async () => {
    const minutesAgo = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();
    const declared = [
      { id: "fresh", kind: "forge-github/issues" },
      { id: "issues", kind: "forge-github/issues", with: { repo: "compozy/compozy" } },
      { id: "broken", kind: "forge-github/issues" },
      { id: "deploys", script: "failed-deploys" },
      { id: "off", kind: "forge-github/pulls" },
    ];
    serveHeartbeat(sourcesPayload(declared));
    serveSourceCatalog();
    const status = { failure_streak: 0, last_item_count: 0 };
    seedAgentInstanceMock([
      agentInstanceFixture({
        agent_name: agent.name,
        last_wake_at: new Date().toISOString(),
        sources: [
          { ...status, id: "fresh", status: "pending_first_poll" },
          {
            ...status,
            id: "issues",
            status: "ok",
            last_item_count: 1,
            last_polled_at: minutesAgo(2.5),
            last_success_at: minutesAgo(2.5),
          },
          {
            ...status,
            id: "broken",
            status: "failing",
            failure_streak: 3,
            error: { code: "source_credential_expired", detail: "GitHub sign-in expired" },
          },
          {
            ...status,
            id: "deploys",
            status: "disabled",
            error: { code: "source_scripts_disabled", detail: "" },
          },
          {
            ...status,
            id: "off",
            status: "unavailable",
            error: { code: "source_kind_unavailable", detail: "" },
          },
        ],
      }),
    ]);
    renderEditor(<AgentCheckInsEditor {...props()} />);

    const rows = await screen.findAllByTestId("check-in-source-row");
    expect(rows.map(row => row.textContent)).toEqual([
      expect.stringContaining("Not checked yet"),
      expect.stringContaining("Checked 2 min ago · 1 new"),
      expect.stringContaining("Couldn't check: GitHub sign-in expired"),
      expect.stringContaining("Scripts are turned off"),
      expect.stringContaining("Extension forge-github is off"),
    ]);
    expect(rows[1]).toHaveTextContent("compozy/compozy");
    expect(rows[0]).not.toHaveTextContent("Checked");
  });

  it("Should list each source's outcome above what it found, and say a check is running [UT-075]", async () => {
    serveHeartbeat(sourcesPayload());
    serveSourceCatalog();
    seedAgentInstanceMock([
      agentInstanceFixture({ agent_name: agent.name, last_wake_at: new Date().toISOString() }),
    ]);
    server.use(
      http.post("*/api/agent-instances/:id/check", async ({ request }) => {
        const { dry_run } = (await request.json()) as { dry_run: boolean };
        if (!dry_run) return HttpResponse.json({ gate: "source_poll_in_flight" });
        return HttpResponse.json({
          gate: "would_run",
          digest: [{ key: "source:new-issues:compozy/compozy#1201", summary: "#1201 No owner" }],
          sources: [
            { id: "new-issues", result: "ok", items: 1, more: false, duration_ms: 412 },
            {
              id: "deploys",
              result: "failing",
              items: 0,
              more: false,
              duration_ms: 9,
              error: { code: "source_script_failed", detail: "exit status 2" },
            },
          ],
        });
      })
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Try it now" }));

    const outcomes = await screen.findByRole("list", { name: "Sources" });
    expect(outcomes.textContent).toContain("1 new");
    expect(outcomes.textContent).toContain("Couldn't check: exit status 2");
    const item = screen.getByText("#1201 No owner");
    expect(outcomes.compareDocumentPosition(item) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Run it for real" }));
    expect(await screen.findByText("A check is already running.")).toBeTruthy();
  });

  it("Should still say how each source went when a try finds nothing new", async () => {
    serveHeartbeat(sourcesPayload());
    serveSourceCatalog();
    seedCheckingInstance();
    server.use(
      http.post("*/api/agent-instances/:id/check", () =>
        HttpResponse.json({
          gate: "digest_empty",
          sources: [
            {
              id: "deploys",
              result: "failing",
              items: 0,
              more: false,
              duration_ms: 9,
              error: { code: "source_script_failed", detail: "exit status 2" },
            },
          ],
        })
      )
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Try it now" }));

    expect(await screen.findByText("Nothing new, so it didn't bother you.")).toBeTruthy();
    const outcomes = screen.getByRole("list", { name: "Sources" });
    expect(outcomes.textContent).toContain("Couldn't check: exit status 2");
    expect(screen.queryByRole("button", { name: "Run it for real" })).toBeNull();
  });

  const ISSUES_KIND = {
    kind: "forge-github/issues",
    extension: "forge-github",
    title: "New GitHub issues",
    params_schema: {
      type: "object",
      properties: {
        repo: { type: "string" },
        labels: { type: "array", items: { type: "string" } },
        backfill: { type: "string", format: "duration" },
      },
    },
  };
  const SCRIPT_SHA = "sha256:7f3a91c0d2b4e6f8a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5e2";

  /**
   * Records each save and, as the daemon would, answers it and every later read with the sources
   * that save kept; reads before the first save fall through to the served file.
   */
  function captureHeartbeatWrites() {
    const bodies: string[] = [];
    let written: AgentHeartbeatPayload | undefined;
    server.use(
      http.get("*/api/agents/:name/heartbeat", () =>
        written ? HttpResponse.json(written) : undefined
      ),
      http.put("*/api/agents/:name/heartbeat", async ({ request }) => {
        const { body } = (await request.json()) as { body: string };
        bodies.push(body);
        const kept = SOURCES.filter(source => body.includes(`- id: "${source.id}"`));
        written = sourcesPayload(kept, `beat-${bodies.length + 1}`);
        return HttpResponse.json({ heartbeat: written });
      })
    );
    return bodies;
  }

  function seedCheckingInstance() {
    seedAgentInstanceMock([
      agentInstanceFixture({ agent_name: agent.name, last_wake_at: new Date().toISOString() }),
    ]);
  }

  async function pickReads(user: ReturnType<typeof userEvent.setup>, option: string) {
    await user.click(await screen.findByRole("combobox", { name: "Reads" }));
    await user.click(await screen.findByRole("option", { name: option }));
  }

  it("Should add a kind with its string, list and duration settings, checked before saving [UT-072]", async () => {
    serveHeartbeat(sourcesPayload([]));
    serveSourceCatalog({ kinds: [ISSUES_KIND] });
    seedCheckingInstance();
    const bodies = captureHeartbeatWrites();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    await user.click(await screen.findByRole("button", { name: "Add source" }));
    const dialog = await screen.findByRole("dialog");
    await pickReads(user, "New GitHub issues");
    expect(within(dialog).getByLabelText("Name")).toHaveValue("issues");
    await user.type(within(dialog).getByLabelText("Repo"), "compozy/compozy");
    await user.type(within(dialog).getByLabelText("Labels"), "bug{Enter}needs-triage{Enter}");
    await user.type(within(dialog).getByLabelText("Backfill"), "soon");
    await user.clear(within(dialog).getByLabelText("Name"));
    await user.type(within(dialog).getByLabelText("Name"), "New Issues");
    await user.click(within(dialog).getByRole("button", { name: "Add source" }));

    expect(within(dialog).getByText("Use a length of time, like 30m or 24h.")).toBeTruthy();
    expect(
      within(dialog).getByText("Use lowercase letters, numbers and dashes, starting with a letter.")
    ).toBeTruthy();
    expect(bodies).toHaveLength(0);

    await user.clear(within(dialog).getByLabelText("Backfill"));
    await user.type(within(dialog).getByLabelText("Backfill"), "24h");
    await user.clear(within(dialog).getByLabelText("Name"));
    await user.type(within(dialog).getByLabelText("Name"), "new-issues");
    await user.click(within(dialog).getByRole("button", { name: "Add source" }));

    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toContain(
      [
        "  sources:",
        '    - id: "new-issues"',
        '      kind: "forge-github/issues"',
        "      with:",
        '        repo: "compozy/compozy"',
        '        labels: ["bug","needs-triage"]',
        '        backfill: "24h"',
      ].join("\n")
    );
  });

  it("Should hide Add at the catalog's limit, and offer it below the limit [UT-074]", async () => {
    const declared = (count: number) =>
      Array.from({ length: count }, (_, index) => ({
        id: `source-${index + 1}`,
        kind: "forge-github/issues",
      }));
    serveHeartbeat(sourcesPayload(declared(3)));
    serveSourceCatalog({ limits: { max_per_agent: 3, max_items: 20 } });
    seedCheckingInstance();
    const first = renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findAllByTestId("check-in-source-row")).toHaveLength(3);
    await waitFor(() => expect(screen.queryByRole("button", { name: "Add source" })).toBeNull());
    first.unmount();

    serveHeartbeat(sourcesPayload(declared(9)));
    serveSourceCatalog({ limits: { max_per_agent: 16, max_items: 20 } });
    renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findAllByTestId("check-in-source-row")).toHaveLength(9);
    expect(await screen.findByRole("button", { name: "Add source" })).toBeTruthy();
  });

  it("Should add a script with text values it reads by name [UT-076]", async () => {
    serveHeartbeat(sourcesPayload([]));
    serveSourceCatalog({
      kinds: [],
      scripts: [
        {
          name: "failed-deploys",
          executable: true,
          sha256: SCRIPT_SHA,
          approved: false,
          approved_sha256: null,
        },
      ],
    });
    seedCheckingInstance();
    const bodies = captureHeartbeatWrites();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    await user.click(await screen.findByRole("button", { name: "Add source" }));
    const dialog = await screen.findByRole("dialog");
    await pickReads(user, "failed-deploys");
    await user.click(within(dialog).getByRole("button", { name: "Add a value" }));
    await user.type(within(dialog).getByLabelText("Name 1"), "env");
    await user.type(within(dialog).getByLabelText("Value 1"), "production");
    await user.click(within(dialog).getByRole("button", { name: "Add source" }));

    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toContain(
      [
        '    - id: "failed-deploys"',
        '      script: "failed-deploys"',
        "      with:",
        '        env: "production"',
      ].join("\n")
    );
  });

  it("Should say why the dialog has nothing to offer, and hide scripts when they're off", async () => {
    serveHeartbeat(sourcesPayload([]));
    serveSourceCatalog({ kinds: [], scripts: [] });
    seedCheckingInstance();
    const user = userEvent.setup();
    const view = renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Add source" }));
    const empty = await screen.findByRole("dialog");
    expect(empty).toHaveTextContent(
      "No sources installed. Add a script to sources/ or install an extension."
    );
    expect(within(empty).getByRole("button", { name: "Add source" })).toBeDisabled();
    view.unmount();

    serveSourceCatalog({
      kinds: [ISSUES_KIND],
      scripts_enabled: false,
      scripts: [
        {
          name: "failed-deploys",
          executable: true,
          sha256: SCRIPT_SHA,
          approved: false,
          approved_sha256: null,
        },
      ],
    });
    renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Add source" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent(
      "Scripts are turned off in your settings, so none are listed."
    );
    await user.click(await screen.findByRole("combobox", { name: "Reads" }));
    expect(await screen.findByRole("option", { name: "New GitHub issues" })).toBeTruthy();
    expect(screen.queryByRole("option", { name: "failed-deploys" })).toBeNull();
  });

  it("Should edit a source keeping what the form can't show, and remove one until saved", async () => {
    serveHeartbeat(sourcesPayload());
    serveSourceCatalog({ kinds: [ISSUES_KIND] });
    seedCheckingInstance();
    const bodies = captureHeartbeatWrites();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    await user.click(await screen.findByRole("button", { name: "More for failed-deploys" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Value 1")).toHaveValue("production");
    expect(dialog).toHaveTextContent("Some of its settings can only be changed in the file.");
    await user.clear(within(dialog).getByLabelText("Value 1"));
    await user.type(within(dialog).getByLabelText("Value 1"), "staging");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toContain(
      [
        '      script: "failed-deploys"',
        "      with:",
        '        env: "staging"',
        '        retry: {"max":3}',
      ].join("\n")
    );

    await user.click(await screen.findByRole("button", { name: "More for New GitHub issues" }));
    await user.click(await screen.findByRole("menuitem", { name: "Remove" }));
    await waitFor(() => expect(screen.getAllByTestId("check-in-source-row")).toHaveLength(1));
    expect(bodies).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(bodies).toHaveLength(2));
    expect(bodies[1]).not.toContain("new-issues");
    // Once the save settles, the editor shows the file the daemon answered with.
    await waitFor(() => expect(screen.queryByRole("button", { name: "Save" })).toBeNull());
    expect(screen.getAllByTestId("check-in-source-row")).toHaveLength(1);
    expect(screen.queryByRole("button", { name: "More for New GitHub issues" })).toBeNull();
  });

  it("Should keep an existing source editable when nothing is offered to add", async () => {
    serveHeartbeat(sourcesPayload());
    serveSourceCatalog({ kinds: [], scripts: [] });
    seedCheckingInstance();
    const bodies = captureHeartbeatWrites();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    await user.click(await screen.findByRole("button", { name: "More for failed-deploys" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).not.toHaveTextContent("No sources installed.");
    expect(within(dialog).getByLabelText("Name")).toHaveValue("deploys");
    await user.clear(within(dialog).getByLabelText("Value 1"));
    await user.type(within(dialog).getByLabelText("Value 1"), "staging");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toContain(
      ['      script: "failed-deploys"', "      with:", '        env: "staging"'].join("\n")
    );
  });

  const WATCH_KIND = {
    kind: "x/watch",
    extension: "x",
    title: "Watch",
    params_schema: {
      type: "object",
      required: ["strict"],
      properties: {
        repo: { type: "string" },
        labels: { type: "array", items: { type: "string" } },
        since: { type: "string", format: "duration" },
        enabled: { type: "boolean", default: true },
        drafts: { type: "boolean", default: true },
        bots: { type: "boolean" },
        strict: { type: "boolean", default: false },
      },
    },
  };
  const WATCH_CATALOG = {
    kinds: [WATCH_KIND],
  } as unknown as AgentHeartbeatSourceCatalog;

  it.each([
    ["an explicit false over a true default", { enabled: false, strict: false }],
    ["an explicit true at its default", { enabled: true, strict: false }],
    [
      "empty text, an empty list and padded text",
      { repo: "", labels: [], since: " 24h ", strict: true },
    ],
    [
      "values outside the form",
      { strict: false, repo: 7, labels: "bug", limit: 20, nested: { a: [1, null] }, on: null },
    ],
  ])("Should save %s back exactly as the file wrote it", (_name, values) => {
    const source = { id: "watch", kind: "x/watch", with: values };
    const saved = sourceFromDraft(sourceDraftFrom(source, WATCH_CATALOG), WATCH_CATALOG);
    expect(saved).toEqual(source);
  });

  it("Should keep an empty with, and a script's values, exactly as written", () => {
    const empty = { id: "issues", kind: "forge-github/issues", with: {} };
    const script = { id: "deploys", script: "failed-deploys", with: { env: "", n: 2, ok: false } };
    for (const source of [empty, script]) {
      expect(sourceFromDraft(sourceDraftFrom(source, WATCH_CATALOG), WATCH_CATALOG)).toEqual(
        source
      );
    }
  });

  it("Should keep settings named like built-in object properties, and refuse a script value that would replace one", () => {
    const source = {
      id: "deploys",
      script: "failed-deploys",
      with: JSON.parse('{"env":"prod","constructor":{"a":1},"__proto__":{"b":2}}') as Record<
        string,
        unknown
      >,
    };
    const draft = sourceDraftFrom(source, WATCH_CATALOG);
    const saved = sourceFromDraft(draft, WATCH_CATALOG).with as Record<string, unknown>;
    expect(Object.entries(saved)).toEqual([
      ["env", "prod"],
      ["constructor", { a: 1 }],
      ["__proto__", { b: 2 }],
    ]);

    const clash = { ...draft, pairs: [...draft.pairs, sourceValuePair("constructor", "x")] };
    expect(validateSourceDraft(clash, WATCH_CATALOG, []).pairs).toBe("Each name can be used once.");
    expect(validateSourceDraft(draft, WATCH_CATALOG, []).pairs).toBeUndefined();
  });

  it("Should write only the settings the operator changed into a file that left them out", () => {
    const source = { id: "watch", kind: "x/watch", with: { repo: "a/b", strict: false } };
    const draft = sourceDraftFrom(source, WATCH_CATALOG);
    const save = (params: Record<string, SourceParamValue>) =>
      sourceFromDraft({ ...draft, params: { ...draft.params, ...params } }, WATCH_CATALOG).with;

    expect(save({})).toEqual({ repo: "a/b", strict: false });
    // A switch turned off over a true default, or toggled back to its default, is written.
    expect(save({ drafts: false, bots: false })).toEqual({
      repo: "a/b",
      strict: false,
      drafts: false,
      bots: false,
    });
    // Cleared text removes the key; new text is trimmed.
    expect(save({ repo: "  ", since: " 1h " })).toEqual({ strict: false, since: "1h" });
    // A required switch the file left out is written as it shows.
    const bare = sourceDraftFrom({ id: "watch", kind: "x/watch" }, WATCH_CATALOG);
    expect(sourceFromDraft(bare, WATCH_CATALOG).with).toEqual({ strict: false });
  });

  it("Should keep an explicit false through Edit and Save, and write a switch turned off", async () => {
    serveHeartbeat(
      sourcesPayload([{ id: "watch", kind: "x/watch", with: { repo: "a/b", enabled: false } }])
    );
    serveSourceCatalog({ kinds: [WATCH_KIND] });
    seedCheckingInstance();
    const bodies = captureHeartbeatWrites();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    await user.click(await screen.findByRole("button", { name: "More for Watch" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("switch", { name: "Enabled" })).not.toBeChecked();
    expect(within(dialog).getByRole("switch", { name: "Drafts" })).toBeChecked();
    await user.click(within(dialog).getByRole("switch", { name: "Drafts" }));
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0]).toContain(
      [
        '      kind: "x/watch"',
        "      with:",
        '        repo: "a/b"',
        "        enabled: false",
        "        drafts: false",
        "        strict: false",
      ].join("\n")
    );
  });

  it("Should approve the hash it showed, and read the script again when it changed", async () => {
    serveHeartbeat(sourcesPayload([{ id: "deploys", script: "failed-deploys" }]));
    let catalogReads = 0;
    server.use(
      http.get("*/api/agents/:name/heartbeat/sources", () => {
        catalogReads += 1;
        return HttpResponse.json({
          agent: agent.name,
          scripts_enabled: true,
          limits: { max_per_agent: 16, max_items: 20 },
          kinds: [],
          scripts: [
            {
              name: "failed-deploys",
              executable: true,
              sha256: SCRIPT_SHA,
              approved: false,
              // Approved once, then edited: the notice asks for the new version.
              approved_sha256:
                "sha256:0000000000000000000000000000000000000000000000000000000000000000",
              line_count: 42,
            },
          ],
        });
      })
    );
    seedAgentInstanceMock([
      agentInstanceFixture({
        agent_name: agent.name,
        last_wake_at: new Date().toISOString(),
        sources: [{ id: "deploys", status: "unapproved", failure_streak: 0, last_item_count: 0 }],
      }),
    ]);
    const approvals: string[] = [];
    server.use(
      http.post("*/api/agents/:name/heartbeat/scripts/:script/approve", async ({ request }) => {
        approvals.push(((await request.json()) as { sha256: string }).sha256);
        return HttpResponse.json(
          { error: "script contents changed", code: "source_script_changed" },
          { status: 409 }
        );
      })
    );
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    const notice = await screen.findByTestId("check-in-script-approval");
    expect(notice).toHaveTextContent("Approve the new version of failed-deploys");
    expect(notice).toHaveTextContent("42 lines");
    expect(screen.getByTestId("check-in-source-row")).toHaveTextContent("Needs your approval");
    await user.click(within(notice).getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(approvals).toEqual([SCRIPT_SHA]));
    await waitFor(() => expect(catalogReads).toBe(2));
  });

  it("Should ask for approval above Sources before the first check, even on a paused place", async () => {
    serveHeartbeat(
      sourcesPayload([
        { id: "deploys", script: "failed-deploys" },
        { id: "draft", script: "draft-deploys" },
      ])
    );
    let approved = false;
    server.use(
      http.get("*/api/agents/:name/heartbeat/sources", () =>
        HttpResponse.json({
          agent: agent.name,
          scripts_enabled: true,
          limits: { max_per_agent: 16, max_items: 20 },
          kinds: [],
          scripts: [
            {
              name: "failed-deploys",
              executable: true,
              sha256: SCRIPT_SHA,
              approved,
              approved_sha256: approved ? SCRIPT_SHA : null,
            },
            // Without the exec bit it can't run, so there is nothing to approve yet.
            {
              name: "draft-deploys",
              executable: false,
              sha256: SCRIPT_SHA,
              approved: false,
              approved_sha256: null,
            },
          ],
        })
      ),
      http.post("*/api/agents/:name/heartbeat/scripts/:script/approve", () => {
        approved = true;
        return HttpResponse.json({
          script: "failed-deploys",
          sha256: SCRIPT_SHA,
          approved_at: new Date().toISOString(),
        });
      })
    );
    const pending = { status: "pending_first_poll", failure_streak: 0, last_item_count: 0 };
    seedAgentInstanceMock([
      agentInstanceFixture({
        agent_name: agent.name,
        state: "paused",
        pause_reason: "manual",
        sources: [
          { ...pending, id: "deploys" },
          { ...pending, id: "draft" },
        ],
      }),
    ]);
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props()} />);

    // One notice, for the script that can run; the paused place can still approve it.
    const notice = await screen.findByTestId("check-in-script-approval");
    expect(notice).toHaveTextContent("Approve failed-deploys to let it run");
    expect(notice).toHaveTextContent("Approve it now and it runs as soon as you resume");
    expect(screen.getAllByTestId("check-in-script-approval")).toHaveLength(1);
    expect(screen.getByText("1 needs your approval")).toBeInTheDocument();
    const [row, draft] = screen.getAllByTestId("check-in-source-row");
    expect(row).toHaveTextContent("Needs your approval");
    // Without its exec bit it can't run, so it says that rather than asking for approval.
    expect(draft).toHaveTextContent("Can't run: the file isn't executable");
    // Rows only report; the notice owns Approve.
    expect(within(row!).queryByRole("button", { name: "Approve" })).toBeNull();

    await user.click(within(notice).getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(row).toHaveTextContent("Approved · runs at the next check-in"));
    expect(screen.queryByTestId("check-in-script-approval")).toBeNull();
  });

  it.each([
    {
      name: "scripts are off",
      status: "pending_first_poll",
      scriptsEnabled: false,
      script: { executable: true, approved: false, approved_sha256: null },
      label: "Scripts are turned off",
    },
    {
      name: "scripts are off after an unapproved check",
      status: "unapproved",
      scriptsEnabled: false,
      script: { executable: true, approved: false, approved_sha256: null },
      label: "Scripts are turned off",
    },
    {
      name: "an approved script lost its exec bit",
      status: "pending_first_poll",
      scriptsEnabled: true,
      script: { executable: false, approved: true, approved_sha256: SCRIPT_SHA },
      label: "Can't run: the file isn't executable",
    },
    {
      name: "an unapproved check read a file without its exec bit",
      status: "unapproved",
      scriptsEnabled: true,
      script: { executable: false, approved: false, approved_sha256: null },
      label: "Can't run: the file isn't executable",
    },
  ])(
    "Should promise nothing that can't run when $name",
    async ({ status, scriptsEnabled, script, label }) => {
      serveHeartbeat(sourcesPayload([{ id: "deploys", script: "failed-deploys" }]));
      server.use(
        http.get("*/api/agents/:name/heartbeat/sources", () =>
          HttpResponse.json({
            agent: agent.name,
            scripts_enabled: scriptsEnabled,
            limits: { max_per_agent: 16, max_items: 20 },
            kinds: [],
            scripts: [{ name: "failed-deploys", sha256: SCRIPT_SHA, ...script }],
          })
        )
      );
      seedAgentInstanceMock([
        agentInstanceFixture({
          agent_name: agent.name,
          sources: [{ id: "deploys", status, failure_streak: 0, last_item_count: 0 }],
        }),
      ]);
      renderEditor(<AgentCheckInsEditor {...props()} />);

      const row = await screen.findByTestId("check-in-source-row");
      await waitFor(() => expect(row).toHaveTextContent(label));
      expect(screen.queryByTestId("check-in-script-approval")).toBeNull();
      expect(screen.queryByText(/needs your approval/i)).toBeNull();
    }
  );

  it("Should ask again when scripts were turned back on after a disabled check", async () => {
    serveHeartbeat(sourcesPayload([{ id: "deploys", script: "failed-deploys" }]));
    server.use(
      http.get("*/api/agents/:name/heartbeat/sources", () =>
        HttpResponse.json({
          agent: agent.name,
          scripts_enabled: true,
          limits: { max_per_agent: 16, max_items: 20 },
          kinds: [],
          scripts: [
            {
              name: "failed-deploys",
              executable: true,
              sha256: SCRIPT_SHA,
              approved: false,
              approved_sha256: null,
            },
          ],
        })
      )
    );
    // The last check ran while scripts were off; the stored status still says so.
    seedAgentInstanceMock([
      agentInstanceFixture({
        agent_name: agent.name,
        sources: [{ id: "deploys", status: "disabled", failure_streak: 0, last_item_count: 0 }],
      }),
    ]);
    renderEditor(<AgentCheckInsEditor {...props()} />);

    expect(await screen.findByTestId("check-in-script-approval")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId("check-in-source-row")).toHaveTextContent("Needs your approval")
    );
  });

  it("Should open a custom schedule in Advanced only", async () => {
    serveHeartbeat(
      heartbeatPayload({
        frontmatter: {
          version: 1,
          enabled: true,
          preferences: { min_interval: "45m" },
          context: {},
          check_ins: {},
          actions: [],
        },
      })
    );
    renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findByText(/has settings the editor can't show/)).toBeTruthy();
    expect(screen.queryByText("How often")).toBeNull();
  });

  // Invariant (U6/U7): the form edits what check-ins may change and their actions, and a save that
  // changes that block approves it only after the operator saw it and confirmed.
  describe("check-in changes and authority", () => {
    const OVERRIDES = { github: ["get_item", "search_items"], linear: ["list_issues"] };
    function expectOverrides(container: HTMLElement) {
      const view = within(container);
      for (const [server, tools] of Object.entries(OVERRIDES)) {
        for (const tool of tools) expect(view.getByText(`${server}: ${tool}`)).toBeTruthy();
      }
      expect(view.getByText("Treated as read-only")).toBeTruthy();
      expect(
        view.getByText(
          "These tools can run in check-ins without being listed under What it can change."
        )
      ).toBeTruthy();
      expect(view.queryByText("Nothing, reads only")).toBeNull();
    }

    const NONE = { digest: "", status: "none", check_ins: {}, actions: [] };

    function serveTools() {
      const tool = (tool_id: string, title: string, kind: string, server?: string) => ({
        descriptor: {
          tool_id,
          display_title: title,
          access: "write",
          check_in_writable: true,
          backend: { kind, mcp_server: server },
        },
      });
      server.use(
        http.get("*/api/tools", () =>
          HttpResponse.json({
            tools: [
              tool("compozy__notify", "Notify", "native_go"),
              tool("mcp__github__add_label", "Add a label", "mcp", "github"),
            ],
          })
        )
      );
    }

    function serveValidate(authority: AgentHeartbeatPayload["authority"]) {
      server.use(
        http.post("*/api/agents/:name/heartbeat/validate", () =>
          HttpResponse.json(heartbeatPayload({ authority }))
        )
      );
    }

    function captureWrites() {
      const writes: { body: string; approve_authority_sha256?: string }[] = [];
      server.use(
        http.put("*/api/agents/:name/heartbeat", async ({ request }) => {
          writes.push((await request.json()) as (typeof writes)[number]);
          return HttpResponse.json({ heartbeat: heartbeatPayload({ authority: NONE }) });
        })
      );
      return writes;
    }

    beforeEach(() => {
      seedAgentInstanceMock([
        agentInstanceFixture({ agent_name: agent.name, last_wake_at: new Date().toISOString() }),
      ]);
      serveTools();
    });

    it.each([
      ["Approve and save", "sha-new"],
      ["Save without approving", undefined],
    ])("Should show a changed block before saving, then %s [UT-043]", async (choice, approved) => {
      serveHeartbeat(heartbeatPayload({ authority: NONE }));
      serveValidate({
        digest: "sha-new",
        status: "unapproved",
        check_ins: { allow_writes: ["mcp__github__add_label"] },
        read_only_tools: OVERRIDES,
        actions: [],
      });
      const writes = captureWrites();
      const user = userEvent.setup();
      renderEditor(<AgentCheckInsEditor {...props()} />);

      expect(await screen.findByText("Nothing, reads only")).toBeTruthy();
      await user.click(await screen.findByRole("switch", { name: "Add a label · github" }));
      await user.click(screen.getByRole("button", { name: "Save" }));

      const dialog = await screen.findByRole("dialog", { name: "Approve these check-in changes?" });
      expect(within(dialog).getByText("mcp__github__add_label")).toBeTruthy();
      expectOverrides(dialog);
      expect(writes).toHaveLength(0);
      await user.click(within(dialog).getByRole("button", { name: choice }));

      await waitFor(() => expect(writes).toHaveLength(1));
      expect(writes[0]?.body).toContain('allow_writes:\n    - "mcp__github__add_label"');
      expect(writes[0]?.approve_authority_sha256).toBe(approved);
    });

    it("Should save actions from the form, the auto-accept switch included [UT-043]", async () => {
      serveHeartbeat(heartbeatPayload({ authority: NONE }));
      serveValidate({ ...NONE, digest: "sha-ok", status: "approved" });
      const writes = captureWrites();
      const user = userEvent.setup();
      renderEditor(<AgentCheckInsEditor {...props()} />);

      await user.click(await screen.findByRole("button", { name: "Add action" }));
      await user.type(screen.getByLabelText("Title"), "Triage new issues");
      await user.type(screen.getByLabelText("Instructions"), "Label each new issue.");
      await user.click(screen.getByRole("switch", { name: "Do it without asking me" }));
      await user.click(screen.getByRole("button", { name: "Save" }));

      await waitFor(() => expect(writes).toHaveLength(1));
      expect(writes[0]?.body).toContain(
        [
          "actions:",
          '  - id: "action"',
          '    title: "Triage new issues"',
          '    instructions: "Label each new issue."',
          "    auto_accept: true",
        ].join("\n")
      );
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    it.each([{ allowWrites: [] }, { allowWrites: ["compozy__notify"] }])(
      "Should ask for approval of a waiting block and approve its exact digest [UT-043]",
      async ({ allowWrites }) => {
        serveHeartbeat(
          heartbeatPayload({
            authority: {
              digest: "sha-old",
              status: "unapproved",
              check_ins: { allow_writes: allowWrites },
              read_only_tools: OVERRIDES,
              actions: [
                { id: "triage", title: "Triage", instructions: "Label them.", auto_accept: true },
              ],
            },
          })
        );
        const approvals: unknown[] = [];
        server.use(
          http.post("*/api/agents/:name/heartbeat/authority/approve", async ({ request }) => {
            approvals.push(await request.json());
            return new HttpResponse(null, { status: 204 });
          })
        );
        const user = userEvent.setup();
        renderEditor(<AgentCheckInsEditor {...props()} />);

        const notice = await screen.findByRole("region", {
          name: "Check-in changes need your approval",
        });
        expectOverrides(notice);
        if (allowWrites.length > 0)
          expect(within(notice).getByText("compozy__notify")).toBeTruthy();
        expect(within(notice).getByText("Label them.", { exact: false })).toBeTruthy();
        await user.click(within(notice).getByRole("button", { name: "Approve" }));

        await waitFor(() => expect(approvals).toEqual([{ sha256: "sha-old" }]));
      }
    );
  });
});

describe("Memory", () => {
  it("Should show where each note came from, and how full memory is", async () => {
    seedAgentMemoryMock("older\n", MEMORY);
    renderEditor(<AgentMemoryEditor {...props()} />);
    expect(await screen.findByText(/Learned in compozy/)).toBeTruthy();
    expect(screen.getByText("You wrote this")).toBeTruthy();
    expect(screen.getByText(/Stopped using this on/)).toBeTruthy();
    expect(screen.getByRole("img", { name: "Memory is 2% full" })).toBeTruthy();
    expect(screen.getByText("2 learned")).toBeTruthy();
    expect(screen.getByText("1 yours")).toBeTruthy();
  });

  it("Should label history as versions and roll back through the route", async () => {
    seedAgentMemoryMock("Old note\n", MEMORY);
    const user = userEvent.setup();
    renderEditor(<AgentMemoryEditor {...props()} />);
    expect(await screen.findByText("Current version")).toBeTruthy();
    expect(screen.getByText("Version 2")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Go back to Version 1" }));
    expect(await screen.findByText("Old note")).toBeTruthy();
    await waitFor(() => expect(screen.queryByText(/Learned in compozy/)).toBeNull());
  });

  it("Should show the conflict with the reread content, and keep mine on the new digest", async () => {
    seedAgentMemoryMock(MEMORY);
    const user = userEvent.setup();
    renderEditor(<AgentMemoryEditor {...props()} />);
    await screen.findByText(/Learned in compozy/);
    seedAgentMemoryMock(MEMORY, `${MEMORY}Their new note\n`);

    await user.click(screen.getAllByRole("button", { name: "Remove note" })[0]!);
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Triage Lead added a note while you were editing")).toBeTruthy();
    expect(screen.getByText('Adds "Their new note"')).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Keep mine" }));
    await waitFor(() =>
      expect(screen.queryByText("Triage Lead added a note while you were editing")).toBeNull()
    );
    expect(screen.queryByText(/Dispatch one executor/)).toBeNull();
  });

  it("Should say near budget at 90% and offer starters when empty", async () => {
    seedAgentMemoryMock(Array.from({ length: 182 }, (_, i) => `Note ${i}`).join("\n"));
    const near = renderEditor(<AgentMemoryEditor {...props()} />);
    expect(await screen.findByText("91%")).toBeTruthy();
    near.unmount();

    seedAgentMemoryMock();
    renderEditor(<AgentMemoryEditor {...props()} />);
    expect(await screen.findByText("Triage Lead hasn't learned anything yet")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Keep updates short" })).toBeTruthy();
  });

  it("Should stay editable when an extension owns the agent", async () => {
    seedAgentMemoryMock(MEMORY);
    renderEditor(<AgentMemoryEditor {...props({ readOnly: true })} />);
    expect(await screen.findByRole("button", { name: "Add a note" })).toBeTruthy();
  });
});

describe("Permissions", () => {
  const alwaysOn: AgentPayload = { ...agent, kind: "always_on", provider: "claude" };

  function serveAgent(served: AgentPayload, onPut?: (body: unknown) => void) {
    server.use(
      http.get("*/api/agents/:name", () => HttpResponse.json({ agent: served })),
      http.put("*/api/agents/:name", async ({ request }) => {
        const body = (await request.json()) as { agent: Partial<AgentPayload> };
        onPut?.(body);
        return HttpResponse.json({ agent: { ...served, ...body.agent } });
      })
    );
  }

  function serveProvider(provider: string, reviewerMode?: string) {
    server.use(
      http.get("*/api/providers", () =>
        HttpResponse.json({
          providers: [
            {
              name: provider,
              display_name: provider === "codex" ? "Codex" : "Claude Code",
              default: true,
              auth_status: { state: "ok" },
              ...(reviewerMode === undefined ? {} : { reviewer_mode: reviewerMode }),
            },
          ],
        })
      )
    );
  }

  it("Should show five levels with unset as the always-on default, and no Follow Settings", async () => {
    serveAgent(alwaysOn);
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    const levels = await screen.findByRole("radiogroup", { name: "Permission level" });
    const cards = levels.querySelectorAll('[role="radio"]');
    expect(Array.from(cards, card => card.textContent)).toEqual([
      expect.stringContaining("Ask before every action"),
      expect.stringContaining("Ask before changes"),
      expect.stringContaining("Accept edits"),
      expect.stringContaining("Auto-review"),
      expect.stringContaining("Full access"),
    ]);
    const edits = screen.getByTestId("agent-permissions-level-approve-edits");
    expect(edits).toHaveAttribute("aria-checked", "true");
    expect(edits).toHaveTextContent("Default");
    expect(screen.queryByText("Follow Settings")).toBeNull();
    expect(screen.getByRole("radiogroup", { name: "Approval reach" })).toBeTruthy();
  });

  it.each([
    ["approve-edits", "Uses your Settings. Right now: Accept edits."],
    ["approve-all", "Uses your Settings. Right now: Full access."],
  ])(
    "Should let an on-demand agent follow Settings at %s and name the current level",
    async (mode, text) => {
      const onDemand: AgentPayload = { ...agent, kind: "on_demand", permissions: undefined };
      serveAgent(onDemand);
      server.use(
        http.get("*/api/settings/general", () =>
          HttpResponse.json({ config: { permissions: { mode } } })
        )
      );
      renderEditor(<AgentPermissionsEditor {...props({ agent: onDemand })} />);
      const follow = await screen.findByTestId("agent-permissions-level-settings");
      expect(follow).toHaveAttribute("aria-checked", "true");
      await waitFor(() => expect(follow).toHaveTextContent(text));
      expect(screen.queryByRole("radiogroup", { name: "Approval reach" })).toBeNull();
    }
  );

  it("Should mark Auto unavailable from the provider's empty reviewer_mode", async () => {
    const user = userEvent.setup();
    serveAgent(alwaysOn);
    serveProvider("claude", "");
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    const auto = await screen.findByTestId("agent-permissions-level-auto");
    await waitFor(() => expect(auto).toHaveTextContent("Not available"));
    // As in the draft, the named alert explains the choice only once it is the selected level.
    expect(screen.queryByTestId("agent-permissions-no-reviewer")).toBeNull();
    await user.click(auto);
    expect(await screen.findByText("Claude Code has no AI reviewer")).toBeTruthy();
  });

  it("Should name the provider with no reviewer for an agent saved at Auto-review", async () => {
    const reviewer: AgentPayload = { ...alwaysOn, permissions: "auto" };
    serveAgent(reviewer);
    serveProvider("claude", "");
    renderEditor(<AgentPermissionsEditor {...props({ agent: reviewer })} />);
    expect(await screen.findByText("Claude Code has no AI reviewer")).toBeTruthy();
    expect(screen.getByTestId("agent-permissions-no-reviewer")).toHaveTextContent(
      "This agent will use Accept edits instead."
    );
  });

  it("Should not warn about the reviewer when an older daemon sends no reviewer_mode", async () => {
    const user = userEvent.setup();
    serveAgent({ ...alwaysOn, permissions: "approve-reads" });
    serveProvider("claude");
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    await user.click(await screen.findByTestId("agent-permissions-level-auto"));
    expect(screen.queryByTestId("agent-permissions-no-reviewer")).toBeNull();
  });

  it("Should list exceptions with the coverage note and block a path outside the project", async () => {
    const user = userEvent.setup();
    const put = vi.fn();
    const withRules: AgentPayload = {
      ...alwaysOn,
      permission_rules: [
        { kind: "edit", path: "docs/**", decision: "allow" },
        { kind: "delete", decision: "deny" },
      ],
    };
    serveAgent(withRules, put);
    renderEditor(<AgentPermissionsEditor {...props({ agent: withRules })} />);
    expect(await screen.findAllByTestId("agent-permission-rule")).toHaveLength(2);
    expect(screen.getByTestId("agent-permission-rules-coverage")).toHaveTextContent(
      "They can't see inside commands"
    );
    await user.click(screen.getByRole("button", { name: "Add an exception" }));
    const paths = screen.getAllByRole("textbox", { name: "Folder or file" });
    fireEvent.change(paths[2]!, { target: { value: "../secrets" } });
    expect(await screen.findByText("Fix the exception first")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(put).not.toHaveBeenCalled();
  });

  it("Should save the level, exceptions and approval reach with the definition", async () => {
    const user = userEvent.setup();
    const put = vi.fn();
    serveAgent(alwaysOn, put);
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    expect(screen.queryByTestId("agent-permission-rules-coverage")).toBeNull();
    await user.click(await screen.findByRole("radio", { name: /Full access/ }));
    await user.click(screen.getByTestId("agent-permissions-reach-workspace"));
    await user.click(screen.getByRole("button", { name: "Add an exception" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Folder or file" }), {
      target: { value: "infra/**" },
    });
    expect(screen.getByText("3 unsaved changes")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(put).toHaveBeenCalledTimes(1));
    expect(put.mock.calls[0]![0]).toMatchObject({
      agent: {
        permissions: "approve-all",
        session_approval: "workspace",
        permission_rules: [{ kind: "edit", path: "infra/**", decision: "allow" }],
      },
    });
  });

  it("Should say the reviewer fell back only for a running session", async () => {
    const reviewer: AgentPayload = { ...alwaysOn, permissions: "auto" };
    serveAgent(reviewer);
    serveProvider("claude", "auto");
    const serveSessions = (state: string) =>
      server.use(
        http.get("*/api/sessions", () =>
          HttpResponse.json({
            sessions: [
              {
                ...primarySessionFixture,
                agent_name: agent.name,
                state,
                permission_mode_degraded: true,
              },
            ],
            page: { has_more: false, limit: 50, total: 1 },
          })
        )
      );
    serveSessions("stopped");
    const view = renderEditor(<AgentPermissionsEditor {...props({ agent: reviewer })} />);
    await screen.findByTestId("agent-permissions-level-auto");
    await waitFor(() => expect(screen.queryByTestId("agent-permissions-degraded")).toBeNull());
    view.unmount();

    serveSessions("active");
    renderEditor(<AgentPermissionsEditor {...props({ agent: reviewer })} />);
    expect(await screen.findByText("The reviewer isn't available right now")).toBeTruthy();
  });

  it("Should treat picking the default reach as no change, and round-trip a wider one", async () => {
    const user = userEvent.setup();
    const put = vi.fn();
    serveAgent(alwaysOn, put);
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    const children = await screen.findByTestId("agent-permissions-reach-children");
    expect(children).toHaveAttribute("aria-checked", "true");
    await user.click(children);
    expect(screen.queryByText(/unsaved change/u)).toBeNull();

    await user.click(screen.getByTestId("agent-permissions-reach-workspace"));
    await user.click(screen.getByTestId("agent-permissions-reach-children"));
    expect(screen.queryByText(/unsaved change/u)).toBeNull();

    await user.click(screen.getByTestId("agent-permissions-reach-workspace"));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(put).toHaveBeenCalledTimes(1));
    expect(put.mock.calls[0]![0]).toMatchObject({ agent: { session_approval: "workspace" } });
  });

  it("Should move between levels with the arrow keys from one tab stop", async () => {
    const user = userEvent.setup();
    serveAgent(alwaysOn);
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    const edits = await screen.findByTestId("agent-permissions-level-approve-edits");
    const levels = screen.getByRole("radiogroup", { name: "Permission level" });
    expect(levels.querySelectorAll('[role="radio"][tabindex="0"]')).toHaveLength(1);
    edits.focus();
    await user.keyboard("{ArrowDown}");
    const auto = screen.getByTestId("agent-permissions-level-auto");
    expect(auto).toHaveAttribute("aria-checked", "true");
    expect(auto).toHaveFocus();
  });

  it("Should follow the daemon's path rule, check glob syntax and tie the message to its row", async () => {
    const user = userEvent.setup();
    serveAgent(alwaysOn);
    renderEditor(<AgentPermissionsEditor {...props({ agent: alwaysOn })} />);
    await user.click(await screen.findByRole("button", { name: "Add an exception" }));
    const path = screen.getByRole("textbox", { name: "Folder or file" });
    fireEvent.change(path, { target: { value: "docs/../src/**" } });
    expect(path).not.toHaveAttribute("aria-invalid");
    fireEvent.change(path, { target: { value: "docs/[" } });
    const message = await screen.findByText(/Close every \[ and \{/u);
    expect(path).toHaveAttribute("aria-describedby", message.id);
    fireEvent.change(path, { target: { value: "docs/../../secrets" } });
    expect(await screen.findByText(/Use a path inside the project/u)).toBeTruthy();
  });

  it("Should be read-only for an extension-owned agent", async () => {
    const owned: AgentPayload = { ...alwaysOn, package_owned: true };
    serveAgent(owned);
    renderEditor(<AgentPermissionsEditor {...props({ agent: owned, readOnly: true })} />);
    expect(await screen.findByTestId("agent-permissions-read-only")).toBeTruthy();
    expect(screen.getByTestId("agent-permissions-level-deny-all")).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Add an exception" })).toBeNull();
  });
});
