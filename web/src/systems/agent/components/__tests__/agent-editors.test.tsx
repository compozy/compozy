// Invariant: the three editors save real files through the digest guard, and tell the truth when a
// file can't be shown or a route doesn't answer yet.
// Owner: the agent-editors components, over HTTP; canonical suite for UT-077, UT-078 and UT-083.
// HTTP is the mocked boundary; the router blocker has no router here, so it is stubbed.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { ReactElement } from "react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { handlers as profileHandlers } from "@/systems/profiles/mocks";

import { primaryAgentFixture } from "../../mocks/fixtures";
import { handlers, resetAgentMockState } from "../../mocks/handlers";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { seedAgentInstanceMock } from "../../mocks/instance-handlers";
import { seedAgentMemoryMock } from "../../mocks/memory-handlers";
import type { AgentHeartbeatPayload, AgentPayload, AgentSoulPayload } from "../../types";
import { AgentCheckInsEditor } from "../agent-editors/agent-check-ins-editor";
import { AgentMemoryEditor } from "../agent-editors/agent-memory-editor";
import {
  AgentWhatItDoesEditor,
  type AgentEditorProps,
} from "../agent-editors/agent-what-it-does-editor";

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useBlocker: () => ({ status: "idle", proceed: undefined, reset: undefined }),
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

describe("What it does", () => {
  it("Should show the blank page with examples when there's no file", async () => {
    serveSoul(soulPayload({ present: false, body: "", frontmatter: {} }));
    renderEditor(<AgentWhatItDoesEditor {...props()} />);
    expect(await screen.findByText("Start from an example")).toBeTruthy();
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
    expect(screen.queryByRole("textbox", { name: "What it does" })).toBeNull();
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

describe("Check-ins", () => {
  it("Should show that checks need a place when the agent runs nowhere", async () => {
    serveHeartbeat(heartbeatPayload());
    const onAddToProject = vi.fn();
    const user = userEvent.setup();
    renderEditor(<AgentCheckInsEditor {...props({ onAddToProject })} />);
    expect(await screen.findByText("Nowhere to check in yet")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Add it to a project" }));
    expect(onAddToProject).toHaveBeenCalled();
  });

  it("Should leave an instance from before checks off until it's turned on", async () => {
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
    renderEditor(<AgentCheckInsEditor {...props()} />);
    await user.click(await screen.findByRole("button", { name: "Turn on check-ins" }));
    await waitFor(() =>
      expect(screen.getByRole("switch", { name: "Check-ins" }).getAttribute("aria-checked")).toBe(
        "true"
      )
    );
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
    await user.click(await screen.findByRole("button", { name: "Turn on check-ins" }));
    await waitFor(() =>
      expect(screen.getByRole("switch", { name: "Check-ins" }).getAttribute("aria-checked")).toBe(
        "true"
      )
    );
    expect(puts).toEqual([]);
  });

  it("Should map reasons from real check-ins and a dry run's digest", async () => {
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
    expect(await screen.findByText("Check-ins will show up here once they start.")).toBeTruthy();
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
