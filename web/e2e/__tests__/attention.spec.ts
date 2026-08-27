import { randomUUID } from "node:crypto";
import { mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Page, Route } from "@playwright/test";

import { sessionWindow } from "../fixtures/os-navigation";
import {
  type BrowserRuntime,
  type WorkspacePayload,
  waitForSeedSessionActive,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

/**
 * Browser journeys for the attention program (E2E-009..E2E-015).
 *
 * Runtime truth comes from isolated acpmock sessions. Transport overrides are
 * limited to presentation boundaries that cannot be produced by an acpmock
 * turn: a frozen catalog source and an agent that reports an unknown badge.
 * E2E-020 (system notifications) belongs to the desktop lane.
 */

const fixtureRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata"
);
const attentionFixture = path.join(fixtureRoot, "attention_truth_fixture.json");
const faultFixture = path.join(fixtureRoot, "driver_fault_fixture.json");
const attentionAgent = "attention-agent";
const faultAgent = "attention-fault-agent";

const bell = {
  trigger: '[data-slot="os-menubar-bell"]',
  popover: "os-bell-popover",
  needsYou: "os-bell-needs-you",
  finished: "os-bell-finished",
  empty: "os-bell-empty",
  disconnected: "os-bell-disconnected",
} as const;

interface SessionPayload {
  id: string;
  agent_name: string;
  badge: string;
  state: string;
  workspace_id: string;
}

interface SessionEnvelope {
  session: SessionPayload;
}

interface SessionsEnvelope {
  sessions: SessionPayload[];
  page: {
    has_more: boolean;
    limit: number;
    next_cursor?: string | null;
    total: number;
  };
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: attentionAgent,
          fixtureAgent: "attention",
          fixturePath: attentionFixture,
        },
        {
          agentName: faultAgent,
          fixtureAgent: "faulty",
          fixturePath: faultFixture,
        },
      ],
    },
  },
});

test.beforeEach(async ({ appPage: page, runtime }) => {
  await ensureProjectWorkspace(page, runtime);
  await completeOnboardingIfPrompted(page);
});

test.describe("E2E-009 attention bell", () => {
  test("Should separate Needs you from Finished and count only needs-you", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const failed = await createFailedSession(runtime, workspace);
    const done = await createDoneSession(runtime, workspace);

    await page.locator(bell.trigger).click();
    const popover = page.getByTestId(bell.popover);
    await expect(popover).toBeVisible();
    await expect(popover.getByTestId(`os-attention-session-${failed.id}`)).toBeVisible();
    await expect(popover.getByTestId(`os-attention-session-${done.id}`)).toBeVisible();
    await expect(popover.getByTestId(bell.needsYou)).toBeVisible();
    await expect(popover.getByTestId(bell.finished)).toBeVisible();

    const badge = page.locator(`${bell.trigger} [class*="rounded-full"]`).first();
    await expect(badge).toHaveText("1");
  });

  test("Should offer no manual dismiss anywhere in the bell", async ({ appPage: page }) => {
    await page.locator(bell.trigger).click();
    const popover = page.getByTestId(bell.popover);
    await expect(popover.getByRole("button", { name: /mark .*seen/i })).toHaveCount(0);
    await expect(popover.getByRole("button", { name: /^dismiss/i })).toHaveCount(0);
  });

  test("Should land on the session a row names, switching workspace when needed", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await createWorkspace(runtime, "attention-jump");
    const failed = await createFailedSession(runtime, workspace);
    await reloadShell(page);

    await page.locator(bell.trigger).click();
    await page.getByTestId(`os-attention-session-${failed.id}`).click();

    await expect(page.getByTestId(bell.popover)).toBeHidden();
    await expect(sessionWindow(page, failed.id)).toBeVisible();
    await expect(page.locator('[data-slot="os-menubar-workspace"]')).toContainText(workspace.name);
  });

  test("Should render the quiet state rather than a blank popover at zero", async ({
    appPage: page,
  }) => {
    await page.locator(bell.trigger).click();
    await expect(page.getByTestId(bell.empty)).toContainText("All quiet");
  });

  test("Should state that a disconnected source is frozen and uncounted", async ({
    appPage: page,
  }) => {
    await page.route(/\/api\/sessions\/catalog-stream(?:\?.*)?$/, route => route.abort());
    await page.reload();
    await completeOnboardingIfPrompted(page);

    await page.locator(bell.trigger).click();
    await expect(page.getByTestId(bell.disconnected)).toContainText("Frozen rows do not count.");
  });
});

test.describe("E2E-010 needs-you toasts", () => {
  test("Should toast an unfocused session and land on it when activated", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const session = await createSession(runtime, workspace, faultAgent);
    await promptSession(runtime, session, "trigger crash mid-stream");
    await waitForBadge(runtime, session.id, "failed");

    const toast = page.getByTestId("os-attention-toast-needs-you").first();
    await expect(toast).toBeVisible();
    await toast.click();
    await expect(sessionWindow(page, session.id)).toBeVisible();
  });

  test("Should stay silent for the session already on screen", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const session = await createSession(runtime, workspace, faultAgent);
    await page.goto(runtime.url(sessionPath(faultAgent, session.id)), {
      waitUntil: "domcontentloaded",
    });
    await expect(sessionWindow(page, session.id)).toBeVisible();

    await promptSession(runtime, session, "trigger crash mid-stream");
    await waitForBadge(runtime, session.id, "failed");
    await expect(page.getByTestId("os-attention-toast-needs-you")).toHaveCount(0);
  });
});

test.describe("E2E-011 coalesced completions", () => {
  test("Should group near-simultaneous completions into one toast opening the bell", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const first = await createSession(runtime, workspace, attentionAgent);
    const second = await createSession(runtime, workspace, attentionAgent);

    await Promise.all([
      promptSession(runtime, first, "complete turn"),
      promptSession(runtime, second, "complete turn"),
    ]);
    await Promise.all([
      waitForBadge(runtime, first.id, "done"),
      waitForBadge(runtime, second.id, "done"),
    ]);

    const grouped = page.getByTestId("os-attention-toast-finished").first();
    await expect(grouped).toContainText("2 sessions finished");
    await grouped.getByRole("button", { name: "Review finished" }).click();
    await expect(page.getByTestId(bell.finished)).toBeVisible();
  });
});

test.describe("E2E-012 tab title count", () => {
  test("Should carry the cross-workspace needs-you total and clear at zero", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await createWorkspace(runtime, "attention-title");
    await createFailedSession(runtime, workspace);
    await expect(page).toHaveTitle(/^\(1\) /);

    await page.route("**/api/sessions/attention-summary", async route => {
      await route.fulfill({ json: { needs_you: 0, finished: 0, by_workspace: [] } });
    });
    await reloadShell(page);
    await expect(page).not.toHaveTitle(/^\(/);
  });

  test("Should survive route navigation", async ({ appPage: page, runtime }) => {
    const workspace = await globalWorkspace(runtime);
    await createFailedSession(runtime, workspace);
    await expect(page).toHaveTitle(/^\(1\) /);
    const before = await page.title();

    await page.goto(runtime.url("/settings/attention"));
    await expect(page).toHaveTitle(before);
  });
});

test.describe("E2E-013 Settings → Attention", () => {
  test("Should round-trip the delivery toggles without a save bar", async ({
    appPage: page,
    runtime,
  }) => {
    await page.goto(runtime.url("/settings/attention"));
    const sound = page.getByTestId("settings-attention-sound");
    await expect(sound).toBeVisible();
    await expect(page.getByTestId("settings-page-attention-save-bar")).toHaveCount(0);

    await sound.click();
    await page.reload();
    await expect(page.getByTestId("settings-attention-sound")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });

  test("Should render the system channel's real platform state", async ({
    appPage: page,
    runtime,
  }) => {
    await page.goto(runtime.url("/settings/attention"));
    await expect(
      page.getByTestId(/^settings-attention-system-(default|denied|unsupported)$/)
    ).toBeVisible();
    await expect(page.getByTestId("settings-attention-system")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });

  test("Should keep a muted workspace's bell rows while silencing delivery", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await createWorkspace(runtime, "attention-muted");
    const failed = await createFailedSession(runtime, workspace);
    await reloadShell(page);
    await page.goto(runtime.url("/settings/attention"));
    await page.getByTestId("settings-attention-mute-picker").selectOption(workspace.id);
    await expect(page.getByTestId(`settings-attention-unmute-${workspace.id}`)).toBeVisible();

    await page.locator(bell.trigger).click();
    await expect(page.getByTestId(`os-attention-session-${failed.id}`)).toHaveAttribute(
      "data-muted",
      "true"
    );
  });
});

test.describe("E2E-014 All workspaces breadth", () => {
  test("Should widen to every workspace, grouped and labelled", async ({
    appPage: page,
    runtime,
  }) => {
    const first = await createWorkspace(runtime, "attention-groups-first");
    const second = await createWorkspace(runtime, "attention-groups-second");
    await createSession(runtime, first, attentionAgent);
    await createSession(runtime, second, attentionAgent);
    await reloadShell(page);

    const modal = await openSessionsModal(page);
    await modal.getByTestId("os-sessions-modal-scope").click();
    await expect(modal.getByTestId(`os-sessions-modal-workspace-${first.id}`)).toBeVisible();
    await expect(modal.getByTestId(`os-sessions-modal-workspace-${second.id}`)).toBeVisible();
  });

  test("Should isolate a failing workspace instead of blanking the list", async ({
    appPage: page,
    runtime,
  }) => {
    const healthy = await createWorkspace(runtime, "attention-healthy");
    const project = await createWorkspace(runtime, "attention-unreachable");
    await createSession(runtime, healthy, attentionAgent);
    await createSession(runtime, project, attentionAgent);
    await reloadShell(page);
    await failWorkspaceSessionCatalog(page, project.id);

    const modal = await openSessionsModal(page);
    await modal.getByTestId("os-sessions-modal-scope").click();
    await expect(modal.getByText("Couldn’t load sessions")).toBeVisible();
    await expect(
      modal.getByTestId(`os-sessions-modal-workspace-${project.id}-retry`)
    ).toBeVisible();
    await expect(modal.getByTestId(`os-sessions-modal-workspace-${healthy.id}`)).toBeVisible();
  });

  test("Should persist the breadth across a reload", async ({ appPage: page }) => {
    let modal = await openSessionsModal(page);
    await modal.getByTestId("os-sessions-modal-scope").click();
    await expect(modal.getByTestId("os-sessions-modal-scope")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await reloadShell(page);

    modal = await openSessionsModal(page);
    await expect(modal.getByTestId("os-sessions-modal-scope")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    // Narrow again so the daemon-persisted breadth returns to its default for
    // the suites that follow.
    await modal.getByTestId("os-sessions-modal-scope").click();
    await expect(modal.getByTestId("os-sessions-modal-scope")).toHaveAttribute(
      "aria-pressed",
      "false"
    );
  });
});

test.describe("E2E-015 sidebar badges and sort", () => {
  test("Should render each badge with a distinct glyph and an accessible label", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const failed = await createFailedSession(runtime, workspace);
    const done = await createDoneSession(runtime, workspace);

    const modal = await openSessionsModal(page);
    await expect(
      modal.getByTestId(`os-sessions-modal-session-${failed.id}`).getByRole("img")
    ).toHaveAttribute("aria-label", "Session badge: failed");
    await expect(
      modal.getByTestId(`os-sessions-modal-session-${done.id}`).getByRole("img")
    ).toHaveAttribute("aria-label", "Session badge: done");
  });

  test("Should float needs-you sessions when attention-first is chosen", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const done = await createDoneSession(runtime, workspace);
    const failed = await createFailedSession(runtime, workspace);

    const modal = await openSessionsModal(page);
    await modal.getByTestId("os-sessions-modal-sort-trigger").click();
    await page.getByTestId("os-sessions-modal-sort-attention").click();

    const first = modal.getByTestId(/^os-sessions-modal-session-/).first();
    await expect(first).toHaveAttribute("data-status", "failed");
    await expect(first).toHaveAttribute("data-testid", `os-sessions-modal-session-${failed.id}`);
    await expect(modal.getByTestId(`os-sessions-modal-session-${done.id}`)).toBeVisible();
  });

  test("Should render an unreporting session honestly as unknown", async ({
    appPage: page,
    runtime,
  }) => {
    const workspace = await globalWorkspace(runtime);
    const session = await createSession(runtime, workspace, attentionAgent);
    await overrideSessionBadge(page, workspace.id, session.id, "provider-state-unreported");
    try {
      await reloadShell(page);

      const modal = await openSessionsModal(page);
      const row = modal.getByTestId(`os-sessions-modal-session-${session.id}`);
      await expect(row.getByRole("img")).toHaveAttribute("aria-label", "Session badge: unknown");
      await expect(row.getByRole("img")).toHaveAttribute("data-badge", "unknown");
    } finally {
      await page.unrouteAll({ behavior: "ignoreErrors" });
    }
  });
});

async function globalWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  if (!runtime.paths?.homeDir) {
    throw new Error("attention E2E requires a launch-mode runtime");
  }
  return await runtime.resolveWorkspace(runtime.paths.workspaceDir);
}

async function createWorkspace(runtime: BrowserRuntime, label: string): Promise<WorkspacePayload> {
  if (!runtime.paths?.homeDir) {
    throw new Error("attention E2E requires a launch-mode runtime");
  }
  // Project workspaces must sit outside COMPOZY_HOME. Directories below the
  // runtime home are internal state and are intentionally absent from the
  // operator's workspace catalog.
  const rootDir = `${runtime.paths.homeDir}-${label}-${randomUUID()}`;
  await mkdir(rootDir, { recursive: true });
  return await runtime.resolveWorkspace(rootDir);
}

async function createSession(
  runtime: BrowserRuntime,
  workspace: WorkspacePayload,
  agentName: string
): Promise<SessionPayload> {
  const payload = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspace.id }),
  });
  await waitForSeedSessionActive(runtime, payload.session.id);
  return payload.session;
}

async function createFailedSession(
  runtime: BrowserRuntime,
  workspace: WorkspacePayload
): Promise<SessionPayload> {
  const session = await createSession(runtime, workspace, faultAgent);
  await promptSession(runtime, session, "trigger crash mid-stream");
  return await waitForBadge(runtime, session.id, "failed");
}

async function createDoneSession(
  runtime: BrowserRuntime,
  workspace: WorkspacePayload
): Promise<SessionPayload> {
  const session = await createSession(runtime, workspace, attentionAgent);
  await promptSession(runtime, session, "complete turn");
  return await waitForBadge(runtime, session.id, "done");
}

async function promptSession(
  runtime: BrowserRuntime,
  session: SessionPayload,
  message: string
): Promise<void> {
  const response = await fetch(
    runtime.url(sessionAPIPath(session.workspace_id, session.id, "/prompt")),
    {
      headers: { "content-type": "application/json" },
      method: "POST",
      body: JSON.stringify({
        idempotency_key: randomUUID(),
        message,
        message_id: randomUUID(),
      }),
    }
  );
  if (!response.ok) {
    throw new Error(`prompt failed with ${response.status}: ${await response.text()}`);
  }
  expect(response.headers.get("content-type")).toContain("text/event-stream");
  await response.text();
}

async function waitForBadge(
  runtime: BrowserRuntime,
  sessionID: string,
  badge: string
): Promise<SessionPayload> {
  let current: SessionPayload | undefined;
  await expect
    .poll(
      async () => {
        const payload = await runtime.requestJSON<SessionEnvelope>(
          `/api/sessions/${encodeURIComponent(sessionID)}?include_health=true`
        );
        current = payload.session;
        return payload.session.badge;
      },
      { timeout: 30_000 }
    )
    .toBe(badge);
  if (!current) throw new Error(`session ${sessionID} never produced badge ${badge}`);
  return current;
}

async function openSessionsModal(page: Page) {
  await page.getByRole("menuitem", { name: "Session", exact: true }).click();
  await expect(page.getByTestId("os-menu-session")).toBeVisible();
  await page.getByTestId("os-menubar-command-shell.sessions.toggle").click();
  const modal = page.getByTestId("os-sessions-modal");
  await expect(modal).toBeVisible();
  return modal;
}

async function reloadShell(page: Page): Promise<void> {
  await page.reload({ waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(page);
}

async function failWorkspaceSessionCatalog(page: Page, workspaceID: string): Promise<void> {
  await page.route("**/api/sessions?*", async route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("workspace_id") === workspaceID) {
      await route.fulfill({ status: 500, json: { error: "workspace unreachable" } });
      return;
    }
    await route.continue();
  });
}

async function overrideSessionBadge(
  page: Page,
  workspaceID: string,
  sessionID: string,
  badge: string
): Promise<void> {
  await page.route("**/api/sessions?*", async route => {
    const url = new URL(route.request().url());
    if (
      url.searchParams.get("workspace_id") !== workspaceID ||
      url.searchParams.has("attention") ||
      url.searchParams.has("archive") ||
      url.searchParams.has("badge")
    ) {
      await route.continue();
      return;
    }
    const response = await fetchRouteAcrossDaemonTransition(route);
    const payload = (await response.json()) as SessionsEnvelope;
    await route.fulfill({
      response,
      json: {
        ...payload,
        sessions: payload.sessions.map(session =>
          session.id === sessionID ? { ...session, badge } : session
        ),
      },
    });
  });
}

async function fetchRouteAcrossDaemonTransition(route: Route) {
  for (let attempt = 0; attempt < 4; attempt += 1) {
    try {
      return await route.fetch({ maxRetries: 3 });
    } catch (error) {
      const retryable = String(error).includes("ECONNREFUSED");
      if (!retryable || attempt === 3) throw error;
      await new Promise(resolve => setTimeout(resolve, 250 * 2 ** attempt));
    }
  }
  throw new Error("unreachable route fetch retry state");
}

function sessionPath(agentName: string, sessionID: string): string {
  return `/agents/${agentName}/sessions/${sessionID}`;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}
