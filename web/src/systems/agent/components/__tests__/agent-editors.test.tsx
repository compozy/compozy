// Invariant: the editors save real files through the digest guard, and tell the truth when a
// file can't be shown or a route doesn't answer yet; Permissions offers the five levels, marks Auto
// unavailable from provider data, and saves exceptions and approval reach with the definition.
// Owner: the agent-editors components, over HTTP; canonical suite for UT-077, UT-078, UT-083,
// UT-058 (agent editor half) and UT-096.
// HTTP is the mocked boundary; the router blocker has no router here, so it is stubbed.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { ReactElement } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { handlers as profileHandlers } from "@/systems/profiles/mocks";

import { primarySessionFixture } from "@/systems/session/mocks/fixtures";

import { primaryAgentFixture } from "../../mocks/fixtures";
import { handlers, resetAgentMockState } from "../../mocks/handlers";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { seedAgentInstanceMock } from "../../mocks/instance-handlers";
import { seedAgentMemoryMock } from "../../mocks/memory-handlers";
import type { AgentHeartbeatPayload, AgentPayload, AgentSoulPayload } from "../../types";
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

const server = setupServer(...profileHandlers, ...handlers);
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

beforeAll(() => server.listen());
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
      [{ id: "c1", result: "failed", reason: "triage_timeout", created_at: now.toISOString() }],
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

  it("Should open a custom schedule in Advanced only", async () => {
    serveHeartbeat(
      heartbeatPayload({
        frontmatter: {
          version: 1,
          enabled: true,
          preferences: { min_interval: "45m" },
          context: {},
        },
      })
    );
    renderEditor(<AgentCheckInsEditor {...props()} />);
    expect(await screen.findByText(/has settings the editor can't show/)).toBeTruthy();
    expect(screen.queryByText("How often")).toBeNull();
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

  it("Should explain that Full access on Codex keeps Codex's own reviewer", async () => {
    const codex: AgentPayload = { ...alwaysOn, provider: "codex", permissions: "approve-all" };
    serveAgent(codex);
    serveProvider("codex", "agent");
    renderEditor(<AgentPermissionsEditor {...props({ agent: codex })} />);
    expect(await screen.findByText("Codex keeps its own reviewer here")).toBeTruthy();
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

  it("Should explain the Codex reviewer for an agent that inherits Codex", async () => {
    const inherits: AgentPayload = {
      ...alwaysOn,
      provider: "",
      permissions: "approve-all",
      effective_runtime: {
        ...agent.effective_runtime,
        provider: "codex",
      } as AgentPayload["effective_runtime"],
    };
    serveAgent(inherits);
    serveProvider("codex", "agent");
    renderEditor(<AgentPermissionsEditor {...props({ agent: inherits })} />);
    expect(await screen.findByText("Codex keeps its own reviewer here")).toBeTruthy();
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
