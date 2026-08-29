// Suite: agent-comms operator journeys (E2E-015..022, E2E-030)
// Invariant: every delegation surface renders daemon truth — states from the
// nine-word vocabulary, depths the daemon assigned, counts from
// `CallsResponse.total`, controls that map 1:1 to real operations — and every
// number the UI shows can be re-derived from the API in the same test.
// Owning layer: browser E2E against a live daemon. Canonical suite: this file.
//
// Two rules shape every case here:
//
//   - **Seed through the public routes.** Calls are created with the same
//     `POST /api/workspaces/{ws}/calls` the CLI and `compozy__agent_call` use,
//     and behaviour comes from acpmock fixtures. Nothing is injected behind the
//     daemon, so a journey that passes describes a runtime that works.
//   - **Every call route carries its workspace.** `callSurfaceScope` derives a
//     call's scope from the route it arrives on, so `/api/calls/{id}` resolves
//     `scope=global` and answers not-found for workspace work. The bare routes
//     are never used here, including for out-of-band setup.
import { fileURLToPath } from "node:url";
import path from "node:path";

import type { Page, Route } from "@playwright/test";

import { appWindow, sessionWindow } from "../fixtures/os-navigation";
import { type BrowserRuntime, type WorkspacePayload } from "../fixtures/runtime";
import { agentCommsSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

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
const callsFixture = path.join(fixtureRoot, "agent_calls_fixture.json");
const nestedFixture = path.join(fixtureRoot, "agent_calls_nested_fixture.json");
const blockedFixture = path.join(fixtureRoot, "agent_calls_blocked_fixture.json");

/**
 * The prompt-matched behaviours `agent_calls_fixture.json` provides.
 *
 * Each agent answers a substring, so a journey chooses an outcome by choosing
 * words — the fixture is the contract, not a mock of one.
 */
const REVIEWER = "reviewer";
const SILENT = "silent";
const BLOCKER = "blocker";
const MESSENGER = "messenger";
const BLOCKED_CHILD = "blocked-child";
const L1 = "delegator-l1";
const L2 = "delegator-l2";
const L3 = "leaf-l3";

function mockAgent(fixturePath: string, fixtureAgent: string) {
  return { fixturePath, fixtureAgent, agentName: fixtureAgent };
}

const callBehaviourAgents = [
  mockAgent(callsFixture, REVIEWER),
  mockAgent(callsFixture, SILENT),
  mockAgent(callsFixture, BLOCKER),
  mockAgent(callsFixture, MESSENGER),
  mockAgent(blockedFixture, BLOCKED_CHILD),
];

const nestedAgents = [
  mockAgent(nestedFixture, L1),
  mockAgent(nestedFixture, L2),
  mockAgent(nestedFixture, L3),
];

test.use({
  viewport: { width: 1440, height: 900 },
  runtimeOptions: { seed: { mockAgents: callBehaviourAgents } },
});

// --- daemon truth -----------------------------------------------------------

type Requester = { requestJSON<T>(pathname: string, init?: RequestInit): Promise<T> };

interface CallAcceptance {
  call_id: string;
  child_session_id?: string;
  state: string;
}

interface CallRow {
  call_id: string;
  state: string;
  depth: number;
  agent?: string;
  caller: { id: string; kind: string };
  child_session_id?: string;
  root_session_id: string;
}

interface CallsPage {
  items: CallRow[];
  total: number;
  next_cursor?: string;
}

interface SessionRow {
  id: string;
  badge: string;
  state: string;
  stop_detail?: string;
}

interface SessionEnvelope {
  session: SessionRow;
}

interface MessageRow {
  message_id: string;
  delivery: string;
  delivered_at?: string | null;
}

/** A contract no answer in the `reviewer` fixture can satisfy, first try or repair. */
const UNSATISFIABLE_CONTRACT = {
  type: "object",
  required: ["verdict"],
  properties: { verdict: { type: "string" } },
} as const;

interface UsageEnvelope {
  usage: { cost_status?: string | null; total_cost?: number | null };
}

type CallTarget = { agent: string } | { session_id: string };

/**
 * The project workspace this test's daemon was launched with.
 *
 * Mock agents are a launch-mode seed, so every journey here needs a daemon this
 * test started rather than one it attached to. Saying so once, loudly, beats a
 * non-null assertion at each call site that would fail as a null dereference.
 */
async function projectWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  if (!runtime.paths) throw new Error("agent-comms E2E requires a launch-mode runtime");
  return runtime.resolveWorkspace(runtime.paths.workspaceDir);
}

function callsPath(workspaceId: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/calls${suffix}`;
}

async function createCall(
  runtime: Requester,
  workspaceId: string,
  target: CallTarget,
  prompt: string,
  expectContract?: unknown
): Promise<CallAcceptance> {
  return runtime.requestJSON<CallAcceptance>(callsPath(workspaceId), {
    method: "POST",
    body: JSON.stringify({
      target,
      prompt,
      ...(expectContract === undefined ? {} : { expect: expectContract }),
    }),
  });
}

async function readCalls(runtime: Requester, workspaceId: string, query = ""): Promise<CallsPage> {
  const suffix = query === "" ? "" : `?${query}`;
  return runtime.requestJSON<CallsPage>(callsPath(workspaceId, suffix));
}

async function readCall(runtime: Requester, workspaceId: string, callId: string): Promise<CallRow> {
  return runtime.requestJSON<CallRow>(callsPath(workspaceId, `/${encodeURIComponent(callId)}`));
}

/** A call is asynchronous by contract; every assertion about one waits for it. */
async function waitForCallState(
  runtime: Requester,
  workspaceId: string,
  callId: string,
  states: readonly string[]
): Promise<CallRow> {
  await expect
    .poll(async () => (await readCall(runtime, workspaceId, callId)).state, { timeout: 60_000 })
    .toMatch(new RegExp(`^(${states.join("|")})$`));
  return readCall(runtime, workspaceId, callId);
}

/** The daemon's count for a filtered population, polled until it settles. */
async function waitForCallTotal(
  runtime: Requester,
  workspaceId: string,
  query: string,
  expected: number
): Promise<void> {
  await expect
    .poll(async () => (await readCalls(runtime, workspaceId, `${query}&limit=1`)).total, {
      timeout: 60_000,
    })
    .toBe(expected);
}

async function waitForSessionBadge(
  runtime: Requester,
  sessionId: string,
  badge: string
): Promise<void> {
  await expect
    .poll(
      async () => {
        const payload = await runtime.requestJSON<SessionEnvelope>(
          `/api/sessions/${encodeURIComponent(sessionId)}?include_health=true`
        );
        return payload.session.badge;
      },
      { timeout: 60_000 }
    )
    .toBe(badge);
}

async function readSession(runtime: Requester, sessionId: string): Promise<SessionRow> {
  const payload = await runtime.requestJSON<SessionEnvelope>(
    `/api/sessions/${encodeURIComponent(sessionId)}?include_health=true`
  );
  return payload.session;
}

/**
 * Wait for settlement to park a child.
 *
 * The daemon has no `parked` session state — `parkSettledChild` writes
 * `parked_at` and stops the child with this exact reason, and the stop detail is
 * the only part of that visible on the wire.
 */
async function waitForParkedChild(runtime: Requester, sessionId: string): Promise<void> {
  await expect
    .poll(
      async () => {
        const session = await readSession(runtime, sessionId);
        return `${session.state}/${session.stop_detail ?? ""}`;
      },
      { timeout: 60_000 }
    )
    .toBe("stopped/call child parked");
}

async function sendMessage(
  runtime: Requester,
  workspaceId: string,
  body: { to: CallTarget; text: string; call_id?: string }
): Promise<{ message_id: string; delivery: string }> {
  return runtime.requestJSON(`/api/workspaces/${encodeURIComponent(workspaceId)}/messages`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

async function waitForMessageDelivery(
  runtime: Requester,
  workspaceId: string,
  messageId: string,
  deliveries: readonly string[]
): Promise<MessageRow> {
  const path = `/api/workspaces/${encodeURIComponent(workspaceId)}/messages/${encodeURIComponent(messageId)}`;
  await expect
    .poll(async () => (await runtime.requestJSON<MessageRow>(path)).delivery, { timeout: 60_000 })
    .toMatch(new RegExp(`^(${deliveries.join("|")})$`));
  return runtime.requestJSON<MessageRow>(path);
}

// --- shell navigation -------------------------------------------------------

async function openAgents(page: Page, url: string) {
  await page.goto(url, { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(page);
  const win = appWindow(page, "agents");
  await expect(win).toBeVisible();
  return agentCommsSelectors(win);
}

async function openSession(page: Page, url: string, sessionId: string) {
  await page.goto(url, { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(page);
  const win = page.locator(
    `[data-slot="os-window-surface"][data-app="session"]` +
      `[data-instance-key="${sessionId}"]:visible`
  );
  await expect(win).toBeVisible();
  return agentCommsSelectors(win);
}

test.beforeEach(async ({ appPage, runtime }) => {
  await ensureProjectWorkspace(appPage, runtime);
});

// --- E2E-015 ----------------------------------------------------------------

test.describe("E2E-015 Activity tree", () => {
  // Depth is assigned by the daemon as delegation actually happens, so a nested
  // tree cannot be described by a flat fixture — these three agents delegate for
  // real, one level each, to the `max_depth = 3` wall.
  test.use({
    runtimeOptions: {
      seed: { mockAgents: [...nestedAgents, ...callBehaviourAgents] },
    },
  });

  test("Should render real depths 1-3 beside the state spectrum and open a record", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);

    // The four states the contract names, each produced by a real turn.
    await createCall(runtime, workspace.id, { agent: L1 }, "start nested delegation");
    const completed = await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path");
    const invalid = await createCall(
      runtime,
      workspace.id,
      { agent: REVIEWER },
      "repair result",
      UNSATISFIABLE_CONTRACT
    );
    await createCall(runtime, workspace.id, { agent: BLOCKER }, "keep working");

    // `invalid-result` is earned, not asserted: the fixture answers `{wrong:true}`,
    // the daemon runs the real repair round, and `{answer:9}` misses `verdict` too.
    await waitForCallState(runtime, workspace.id, invalid.call_id, ["invalid-result"]);
    const done = await waitForCallState(runtime, workspace.id, completed.call_id, ["completed"]);

    // The grandchild proves the chain: only a real delegation can produce it.
    await expect
      .poll(
        async () => {
          const page = await readCalls(runtime, workspace.id, "limit=100");
          return page.items.map(item => item.depth).sort();
        },
        { timeout: 60_000 }
      )
      .toEqual(expect.arrayContaining([1, 2, 3]));

    // Settlement parks the child. This read substantiates the pill asserted
    // below rather than standing in for it.
    await waitForParkedChild(runtime, done.child_session_id!);

    const calls = await openAgents(appPage, runtime.url("/agents/activity"));
    await expect(calls.activityTree).toBeVisible();
    await expect(calls.treeGroups.first()).toBeVisible();

    const tree = appWindow(appPage, "agents").getByTestId("agents-activity-tree");
    for (const depth of ["1", "2", "3"]) {
      await expect(tree.locator(`[data-depth="${depth}"]`).first()).toBeVisible();
    }

    // Every state in the contract, on the rows themselves.
    for (const state of ["running", "completed", "invalid-result"]) {
      await expect(tree.locator(`[data-state="${state}"]`).first()).toBeVisible();
    }
    // Parked is a fact about the *child*, so it rides its own pill.
    await expect(
      tree.locator(`[data-call-id="${done.call_id}"] [data-child-state="parked"]`)
    ).toBeVisible();

    // A folded tree keeps escalating: the header still carries the worst state
    // underneath it, so folding hides detail without hiding urgency. Collapse is
    // driven from the keyboard because clicking a row is the open-record action.
    const group = calls.treeGroups.first();
    await expect(group).toHaveAttribute("aria-expanded", "true");
    await group.focus();
    await appPage.keyboard.press("ArrowLeft");
    await expect(group).toHaveAttribute("aria-expanded", "false");
    await expect(group.locator("[data-state]").first()).toBeVisible();

    await appPage.keyboard.press("ArrowRight");
    await expect(group).toHaveAttribute("aria-expanded", "true");

    await calls.treeRows.first().click();
    await expect.poll(() => new URL(appPage.url()).pathname).toMatch(/^\/agents\/calls\/call-/);
    await expect(calls.callDetail).toBeVisible();

    // Parked is not gone, and the daemon draws that line at insert time: a
    // message to a parked child is accepted and delivered, while one to a child
    // that merely stopped is refused `target_stopped`.
    const parkedProbe = await sendMessage(runtime, workspace.id, {
      to: { session_id: done.child_session_id! },
      text: "still there?",
    });
    const delivered = await waitForMessageDelivery(runtime, workspace.id, parkedProbe.message_id, [
      "woke",
      "delivered-into-turn",
    ]);
    expect(delivered.delivery).not.toBe("failed");
  });
});

// --- E2E-016 ----------------------------------------------------------------

test.describe("E2E-016 call detail", () => {
  test("Should show contract, timeline, result and truthful cost, with no forbidden control", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path", {
      type: "object",
      required: ["answer"],
      properties: { answer: { type: "number" } },
    });
    const record = await waitForCallState(runtime, workspace.id, accepted.call_id, ["completed"]);

    const calls = await openAgents(appPage, runtime.url(`/agents/calls/${accepted.call_id}`));
    await expect(calls.callDetail).toBeVisible();
    await expect(calls.callContract).toBeVisible();
    await expect(calls.callTimeline).toBeVisible();
    await expect(calls.callResultRows).toBeVisible();

    // The bounded preview is not the answer. Fetching it is an explicit act,
    // and what comes back has to be on screen — otherwise the button is a
    // promise the surface never keeps.
    const win = appWindow(appPage, "agents");
    await expect(win.getByTestId("agent-call-result-full-payload")).toHaveCount(0);
    await win.getByTestId("agent-call-result-fetch").click();
    const payload = win.getByTestId("agent-call-result-full-payload");
    await expect(payload).toBeVisible();
    await expect(payload).toContainText("answer");

    // Terminal: cancel is absent, not disabled — the affordance goes with the
    // operation. Call again is what a settled call offers instead.
    await expect(calls.callCancel).toHaveCount(0);
    await expect(calls.callAgain).toBeVisible();

    // Cost comes from the child's books, and `describeCost` owns every word of
    // it. A provider that reported nothing must never render as a zero.
    const { usage } = await runtime.requestJSON<UsageEnvelope>(
      `/api/workspaces/${encodeURIComponent(workspace.id)}/sessions/${encodeURIComponent(
        record.child_session_id!
      )}/usage`
    );
    // `describeCost` normalizes an absent status to `"unknown"` rather than
    // inventing a fourth state, so that is what the panel must be stamped with.
    await expect(calls.callCost).toHaveAttribute(
      "data-cost-status",
      usage.cost_status ?? "unknown"
    );
    if (usage.total_cost === null || usage.total_cost === undefined) {
      await expect(calls.callCost).not.toContainText("$");
      await expect(calls.callCost).not.toContainText("0.00");
    }
  });
});

// --- E2E-017 ----------------------------------------------------------------

test.describe("E2E-017 in-context messages", () => {
  test("Should render an inbound message with provenance, inert body, and its receipt", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(
      runtime,
      workspace.id,
      { agent: MESSENGER },
      "message parent"
    );
    const record = await waitForCallState(runtime, workspace.id, accepted.call_id, ["completed"]);

    // The message landed in the caller's transcript, where it means something.
    const shell = await openSession(
      appPage,
      runtime.url(`/session/${record.caller.id}`),
      record.caller.id
    );

    const message = shell.syntheticTurn("message").first();
    await expect(message).toBeVisible();
    await expect(appPage.getByText(/not the operator/).first()).toBeVisible();
    // The body is characters on a screen, never a control.
    await expect(message.getByRole("button")).toHaveCount(0);
    await expect(message.getByRole("link")).toHaveCount(0);
    await expect(shell.messageDeliveries.first()).toBeVisible();
    // There is no read state in this feature, so there is nothing to mark.
    await expect(appPage.getByText(/unread/i)).toHaveCount(0);
  });

  test("Should deliver a message to the child from call detail and show the receipt", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path");
    const record = await waitForCallState(runtime, workspace.id, accepted.call_id, ["completed"]);
    await waitForParkedChild(runtime, record.child_session_id!);

    const calls = await openAgents(appPage, runtime.url(`/agents/calls/${accepted.call_id}`));
    // Messaging a parked child is one of the two ways to revive it.
    await expect(calls.callMessageChild).toBeVisible();
    await calls.callMessageChild.click();
    await expect(calls.callMessageCompose).toBeVisible();

    await calls.composeMessage.fill("Take another look at the retry path.");
    await calls.composeMessageSend.click();
    await expect(calls.composeMessageAccepted).toBeVisible();
    await expect(calls.composeMessageAccepted).toContainText("queued");

    // The banner only says the daemon accepted it. The receipt is on the record,
    // and it has to move: `queued` at insert, then `woke` once the dispatch tick
    // revives the parked child.
    const sent = await runtime.requestJSON<{ items: MessageRow[] }>(
      `/api/workspaces/${encodeURIComponent(workspace.id)}/messages?session=${encodeURIComponent(
        record.child_session_id!
      )}&limit=1`
    );
    const receipt = await waitForMessageDelivery(runtime, workspace.id, sent.items[0]!.message_id, [
      "woke",
      "delivered-into-turn",
    ]);
    expect(receipt.delivered_at).toBeTruthy();

    // Follow the same durable record into the owning transcript. The compose
    // showed its admission receipt (`queued`); this row must show the later
    // delivery receipt, so the transition is visible rather than API-only.
    const childWindow = sessionWindow(appPage, record.child_session_id!);
    await openSession(
      appPage,
      runtime.url(`/session/${record.child_session_id}`),
      record.child_session_id!
    );
    const message = childWindow.locator(
      `[data-testid="session-synthetic-turn"][data-synthetic-kind="message"]` +
        `[data-message-id="${sent.items[0]!.message_id}"]`
    );
    await expect(message).toBeVisible();
    await expect(message.getByTestId("agent-message-delivery")).toHaveText(receipt.delivery);
  });

  test("Should refuse a message to a child awaiting a decision with message_target_blocked", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(
      runtime,
      workspace.id,
      { agent: BLOCKED_CHILD },
      "hold for a decision"
    );
    const record = await waitForCallState(runtime, workspace.id, accepted.call_id, ["running"]);

    // `waiting-for-auth` is the public projection of `pending_permission_count > 0`
    // (`session/badge.go`), which is the exact condition `AcceptMessage` refuses
    // on. Waiting for it is what makes this deterministic rather than a race.
    await waitForSessionBadge(runtime, record.child_session_id!, "waiting-for-auth");

    const calls = await openAgents(appPage, runtime.url(`/agents/calls/${accepted.call_id}`));
    await calls.callMessageChild.click();
    await calls.composeMessage.fill("Never mind the permission, keep going.");
    await calls.composeMessageSend.click();

    await expect(calls.composeMessageError).toContainText("message_target_blocked");
    await expect(calls.composeMessageAccepted).toHaveCount(0);
  });
});

// --- E2E-018 ----------------------------------------------------------------

test.describe("E2E-018 liveness and stale actions", () => {
  test("Should state that the source is frozen, then resync when it returns", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path");

    const catalogStream = /\/api\/sessions\/catalog-stream(?:\?.*)?$/;
    let markStreamRejected!: () => void;
    const streamRejected = new Promise<void>(resolve => {
      markStreamRejected = resolve;
    });
    const rejectCatalogStream = async (route: Route) => {
      markStreamRejected();
      await route.abort();
    };
    await appPage.context().route(catalogStream, rejectCatalogStream);
    const stale = await openAgents(appPage, runtime.url("/agents/activity"));
    await streamRejected;
    // Out of date is stated, never silent.
    await expect(stale.activityStale).toBeVisible();

    await appPage.context().unroute(catalogStream, rejectCatalogStream);
    const live = await openAgents(appPage, runtime.url("/agents/activity"));
    await expect(live.activityStale).toHaveCount(0);
    await expect(live.activityTree).toBeVisible();
  });

  test("Should snap to daemon truth when the operator acts on a call that already settled", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(
      runtime,
      workspace.id,
      { agent: REVIEWER },
      "settle during stale action"
    );
    await waitForCallState(runtime, workspace.id, accepted.call_id, ["running"]);

    const calls = await openAgents(appPage, runtime.url(`/agents/calls/${accepted.call_id}`));
    await expect(calls.callCancel).toBeVisible();

    // Hold the public cancel request after the click has reached the daemon
    // boundary. The call can then settle out of band while the exact action the
    // operator already made remains in flight, without pinning stale reads or
    // racing a control that correctly disappears on a live update.
    const cancelRequest = "**/api/workspaces/*/calls/*/cancel*";
    let releaseCancel!: () => void;
    let markCancelReached!: () => void;
    const cancelReached = new Promise<void>(resolve => {
      markCancelReached = resolve;
    });
    const cancelRelease = new Promise<void>(resolve => {
      releaseCancel = resolve;
    });
    await appPage.route(cancelRequest, async route => {
      markCancelReached();
      await cancelRelease;
      await route.continue();
    });

    const cancelClick = calls.callCancel.click();
    try {
      await cancelReached;
      await waitForCallState(runtime, workspace.id, accepted.call_id, ["completed"]);
    } finally {
      releaseCancel();
    }
    await cancelClick;

    // The receipt comes from the mutation response, so it lands without a
    // re-read — and it names the state the daemon actually returned.
    const outcome = appWindow(appPage, "agents").getByTestId("agent-call-cancel-outcome");
    await expect(outcome).toBeVisible();
    await expect(outcome).toContainText("already settled");
    await expect(outcome).toContainText("completed");

    await appPage.unroute(cancelRequest);

    // Then the view reconciles: the control goes because the operation is gone.
    await expect(calls.callCancel).toHaveCount(0);
    await expect(calls.callAgain).toBeVisible();
  });
});

// --- E2E-019 ----------------------------------------------------------------

test.describe("E2E-019 session Calls panel and the wake row", () => {
  test("Should list both directions with daemon counts", async ({ appPage, runtime }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path");
    const record = await waitForCallState(runtime, workspace.id, accepted.call_id, ["completed"]);

    const calls = await openSession(
      appPage,
      runtime.url(`/session/${record.child_session_id}`),
      record.child_session_id!
    );

    await calls.inspectorToggle.click();
    await calls.inspectorTab.click();
    await expect(calls.inspectorPanel).toBeVisible();
    await expect(calls.panelReceived).toBeVisible();

    const received = await readCalls(
      runtime,
      workspace.id,
      `child_session_id=${encodeURIComponent(record.child_session_id!)}&limit=1`
    );
    // The panel shows a page; the count describes the population.
    await expect(calls.panelReceivedCount).toHaveText(String(received.total));

    // Both directions, and both against the daemon. The child made a call of its
    // own below, so Made is a real population here rather than a decoration.
    await createCall(
      runtime,
      workspace.id,
      { session_id: record.child_session_id! },
      "one more thing"
    );
    const made = await readCalls(
      runtime,
      workspace.id,
      `caller=${encodeURIComponent(record.caller.id)}&limit=1`
    );
    const callerPanel = await openSession(
      appPage,
      runtime.url(`/session/${record.caller.id}`),
      record.caller.id
    );
    await callerPanel.inspectorToggle.click();
    await callerPanel.inspectorTab.click();
    await expect(callerPanel.panelMade).toBeVisible();
    await expect(callerPanel.panelMadeCount).toHaveText(String(made.total));
  });

  test("Should explain why the caller woke, with the call id on the row", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);
    const accepted = await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path");
    const record = await waitForCallState(runtime, workspace.id, accepted.call_id, ["completed"]);

    // The caller is a real, browsable session: `ensureOperatorCallerSession`
    // creates a deterministic `ses_operator_*` session and binds it, so the
    // completion wake lands in a transcript the operator can actually read.
    const calls = await openSession(
      appPage,
      runtime.url(`/session/${record.caller.id}`),
      record.caller.id
    );

    const wake = calls.syntheticTurn("call-wake").first();
    await expect(wake).toBeVisible();
    await expect(wake).toHaveAttribute("data-call-id", accepted.call_id);
    // "Why did this wake" has to be readable, not just present in an attribute.
    // `RenderCompletionWake` writes both the id and a result preview into the
    // turn, and the row renders the daemon's sentence verbatim.
    await expect(wake).toContainText(accepted.call_id);
    await expect(wake).toContainText("Result:");
  });
});

// --- E2E-020 ----------------------------------------------------------------

test.describe("E2E-020 attention", () => {
  test("Should coalesce two causes into one bell row and clear both on real resolutions", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);

    // Two causes need two children, and that is a rule of the daemon rather than
    // a convenience: `attention` drops a cause once *the same child* is called or
    // messaged again, so a second failure on one child would already have
    // resolved the first. Both calls share the `ses_operator_*` root, which is
    // what coalesces them into a single tree row.
    const first = await createCall(
      runtime,
      workspace.id,
      { agent: REVIEWER },
      "repair result",
      UNSATISFIABLE_CONTRACT
    );
    const second = await createCall(
      runtime,
      workspace.id,
      { agent: REVIEWER },
      "repair result",
      UNSATISFIABLE_CONTRACT
    );
    const firstRecord = await waitForCallState(runtime, workspace.id, first.call_id, [
      "invalid-result",
    ]);
    const secondRecord = await waitForCallState(runtime, workspace.id, second.call_id, [
      "invalid-result",
    ]);
    expect(firstRecord.child_session_id).not.toBe(secondRecord.child_session_id);
    await waitForCallTotal(runtime, workspace.id, "attention=true", 2);

    const calls = await openAgents(appPage, runtime.url("/agents/activity"));
    await expect(calls.activityTree).toBeVisible();

    // The badge counts causes, not rows.
    const badge = agentCommsSelectors(appPage).dockBadge;
    await expect(badge).toHaveText("2");

    // One row stands for the whole tree, and says how many it speaks for.
    await appPage.locator('[data-slot="os-menubar-bell"]').click();
    const needsYou = appPage.getByTestId("os-bell-needs-you");
    await expect(needsYou).toBeVisible();
    const treeRow = needsYou.locator('[data-testid^="os-attention-call-"]').first();
    await expect(treeRow).toContainText("2 calls need your look in this tree");

    // Nothing here is dismissible: the badge is a fact about the work, not a
    // notification the operator can silence.
    await expect(
      appPage.getByRole("button", { name: /dismiss|clear all|snooze|mark.*read/i })
    ).toHaveCount(0);

    // Opening the row lands on Activity, where the causes are triaged.
    await treeRow.click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents/activity");
    await expect(calls.activityTree).toBeVisible();

    // Each cause is resolved by a real later call to *its own* child. The
    // uncontracted "one more thing" matches `reviewer`'s follow-up turn and
    // completes cleanly, so resolving raises no new cause of its own.
    await createCall(
      runtime,
      workspace.id,
      { session_id: firstRecord.child_session_id! },
      "one more thing"
    );
    await waitForCallTotal(runtime, workspace.id, "attention=true", 1);

    await createCall(
      runtime,
      workspace.id,
      { session_id: secondRecord.child_session_id! },
      "one more thing"
    );
    await waitForCallTotal(runtime, workspace.id, "attention=true", 0);

    await appPage.reload({ waitUntil: "domcontentloaded" });
    await completeOnboardingIfPrompted(appPage);
    await expect(agentCommsSelectors(appPage).dockBadge).toHaveCount(0);
    await appPage.locator('[data-slot="os-menubar-bell"]').click();
    await expect(
      appPage.getByTestId("os-bell-needs-you").locator('[data-testid^="os-attention-call-"]')
    ).toHaveCount(0);

    // The historical calls are resolved, not erased.
    await waitForCallTotal(runtime, workspace.id, "state=invalid-result", 2);
  });
});

// --- E2E-021 ----------------------------------------------------------------

test.describe("E2E-021 empty states", () => {
  // A workspace that has never delegated. Global built-ins keep the unfiltered
  // roster useful even when the workspace defines no custom agents.
  test.use({ runtimeOptions: { seed: { mockAgents: [] } } });

  test("Should teach the feature instead of showing an empty frame", async ({
    appPage,
    runtime,
  }) => {
    const calls = await openAgents(appPage, runtime.url("/agents/activity"));
    await expect(calls.activityEmpty).toBeVisible();
    await expect(
      appWindow(appPage, "agents").getByText("No agent is delegating work right now")
    ).toBeVisible();

    await openAgents(appPage, runtime.url("/agents"));
    const agents = appWindow(appPage, "agents");
    await expect(agents.getByTestId("agent-fleet-row-link-general")).toBeVisible();

    // Empty still has an honest, reachable state: a filter with no matches.
    await agents.getByTestId("agent-fleet-search").fill("no-agent-can-match-this-name");
    await expect(agents.getByTestId("agent-fleet-filtered-empty")).toBeVisible();
    await expect(agents.getByTestId("agent-fleet-clear-filters")).toBeVisible();
  });
});

// --- E2E-022 ----------------------------------------------------------------

test.describe("E2E-022 scale and keyboard", () => {
  /** The contract's number. Not a sample of it. */
  const TARGET_CALLS = 150;
  /** `calls.max_batch` is 8, checked before the loop runs. */
  const BATCH = 8;

  test("Should hold 150 daemon-counted calls and stay navigable by keyboard", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    const workspace = await projectWorkspace(runtime);

    // One agent-targeted call makes the child; every record after it is a
    // follow-up to that same child. Follow-ups never touch `calls.max_children`
    // — the cap is only counted when a call targets an agent — so one child
    // carries all 150 records without a second session ever existing.
    const first = await createCall(runtime, workspace.id, { agent: REVIEWER }, "golden path");
    const child = (await waitForCallState(runtime, workspace.id, first.call_id, ["completed"]))
      .child_session_id!;

    const childFilter = `child_session_id=${encodeURIComponent(child)}`;
    let created = 1;
    while (created < TARGET_CALLS) {
      const size = Math.min(BATCH, TARGET_CALLS - created);
      await runtime.requestJSON(callsPath(workspace.id), {
        method: "POST",
        body: JSON.stringify({
          tasks: Array.from({ length: size }, (_, index) => ({
            target: { session_id: child },
            prompt: `one more thing ${created + index}`,
          })),
        }),
      });
      created += size;
      // Drain before admitting more. `compozy__call_return` binds to the newest
      // running call on a child, so letting batches overlap would strand the
      // older ones unsettled forever. Waiting for the active population to reach
      // zero also keeps every follow-up on the parked-then-revive path.
      await waitForCallTotal(runtime, workspace.id, `${childFilter}&state=queued,running`, 0);
    }

    // The number the contract asks for, counted by the daemon.
    await waitForCallTotal(runtime, workspace.id, childFilter, TARGET_CALLS);
    const all = await readCalls(runtime, workspace.id, "limit=1");
    expect(all.total).toBe(TARGET_CALLS);

    const calls = await openAgents(appPage, runtime.url("/agents/activity"));
    await expect(calls.activityTree).toBeVisible();

    // Counts come from the summary projection, never from mounted rows: past the
    // threshold the tree windows, so only the visible slice is in the DOM. That
    // windowing invariant is owned by UT-129 and is not duplicated here.
    await expect(calls.activityTree).toHaveAttribute("data-virtualized", "true");
    const mounted = await calls.treeRows.count();
    expect(mounted).toBeLessThan(TARGET_CALLS);

    // Collapse buckets: folding the tree hides its rows and keeps its header.
    const group = calls.treeGroups.first();
    await group.focus();
    await appPage.keyboard.press("ArrowLeft");
    await expect(group).toHaveAttribute("aria-expanded", "false");
    await expect(calls.treeRows).toHaveCount(0);
    await appPage.keyboard.press("ArrowRight");
    await expect(group).toHaveAttribute("aria-expanded", "true");

    await calls.treeRows.first().focus();
    await appPage.keyboard.press("ArrowDown");
    await appPage.keyboard.press("ArrowUp");
    await appPage.keyboard.press("ArrowLeft");
    await appPage.keyboard.press("ArrowRight");
    await appPage.keyboard.press("Enter");
    await expect.poll(() => new URL(appPage.url()).pathname).toMatch(/^\/agents\/calls\/call-/);
  });
});

// --- E2E-030 ----------------------------------------------------------------

test.describe("E2E-030 roster journey", () => {
  /**
   * The agent whose detail page the journey opens.
   *
   * Seeded from `blocker` under its own registered name so it is genuinely
   * runnable — a live instance count only means something if the count belongs
   * to *this* agent, and only a real acpmock command can produce one. Its
   * description arrives afterwards over the public route, because the file
   * seeder writes no `description` and an HTTP-created definition has no
   * acpmock `command`.
   */
  const HELPER = "described-helper";
  const HELPER_DESCRIPTION = "Holds work open until an operator stops it.";

  /** Badge fixtures. Separate catalog rows, deliberately — they are never called. */
  const GLOBAL_ONLY = "global-only-agent";
  const SHADOWED = "shadowed-agent";

  test.use({
    runtimeOptions: {
      seed: {
        mockAgents: [
          { fixturePath: callsFixture, fixtureAgent: BLOCKER, agentName: HELPER },
          ...callBehaviourAgents,
        ],
      },
    },
  });

  test("Should describe agents, badge their scope, count live instances, and compose a real call", async ({
    appPage,
    runtime,
  }) => {
    const workspace = await projectWorkspace(runtime);

    // Describe the seeded agent without breaking it: read the definition the
    // seeder wrote, then write it back with a description and its digest, so
    // `command` and `provider` survive and the agent stays executable.
    const current = await runtime.requestJSON<{
      agent: Record<string, unknown> & { definition_digest: string };
    }>(`/api/agents/${encodeURIComponent(HELPER)}`);
    await runtime.requestJSON(`/api/agents/${encodeURIComponent(HELPER)}`, {
      method: "PUT",
      body: JSON.stringify({
        agent: {
          name: HELPER,
          description: HELPER_DESCRIPTION,
          provider: current.agent.provider,
          command: current.agent.command,
          model: current.agent.model,
          reasoning_effort: current.agent.reasoning_effort,
          tools: current.agent.tools,
          toolsets: current.agent.toolsets,
          deny_tools: current.agent.deny_tools,
          permissions: current.agent.permissions,
          category_path: current.agent.category_path,
          skills: current.agent.skills,
          prompt: current.agent.prompt,
        },
        expected_digest: current.agent.definition_digest,
      }),
    });

    // Scope badges come from their own rows. Nothing calls these.
    const badgeFixture = { provider: "acpmock", prompt: "You exist to carry a scope badge." };
    await runtime.requestJSON("/api/agents", {
      method: "POST",
      body: JSON.stringify({
        scope: "global",
        agent: { ...badgeFixture, name: GLOBAL_ONLY, description: "A global definition." },
      }),
    });
    for (const scope of ["global", "workspace"] as const) {
      await runtime.requestJSON("/api/agents", {
        method: "POST",
        body: JSON.stringify({
          scope,
          ...(scope === "workspace" ? { workspace: workspace.id } : {}),
          agent: { ...badgeFixture, name: SHADOWED, description: `The ${scope} copy.` },
        }),
      });
    }

    // One real instance of the selected agent, and only of that agent.
    await createCall(runtime, workspace.id, { agent: HELPER }, "keep working");
    await waitForCallTotal(runtime, workspace.id, `agent=${HELPER}&state=queued,running`, 1);

    await openAgents(appPage, runtime.url("/agents"));
    const agents = appWindow(appPage, "agents");

    await expect(agents.getByTestId(`agent-fleet-row-${HELPER}`)).toContainText(HELPER_DESCRIPTION);
    await expect(agents.getByTestId(`agent-fleet-origin-${GLOBAL_ONLY}`)).toHaveText("Global");
    await expect(agents.getByTestId(`agent-fleet-origin-${SHADOWED}`)).toHaveText("Workspace");
    await expect(agents.getByTestId(`agent-fleet-shadowed-${SHADOWED}`)).toHaveText("Shadowed");

    await agents.getByTestId(`agent-fleet-row-link-${HELPER}`).click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${HELPER}`);

    const calls = agentCommsSelectors(agents);
    await expect(calls.callCompose).toBeVisible();
    // The exact count, for this agent, cross-checked against the daemon before
    // the compose below adds a second.
    const live = await readCalls(
      runtime,
      workspace.id,
      `agent=${encodeURIComponent(HELPER)}&state=queued,running&limit=1`
    );
    expect(live.total).toBe(1);
    await expect(calls.detailActiveInstances).toHaveText("1 working");

    // A contract the browser is happy to parse and the daemon refuses: a
    // required secret-shaped field. Malformed JSON would never leave the page.
    const refusal = appPage.waitForResponse(
      response => response.url().includes("/calls") && response.request().method() === "POST"
    );
    await calls.callComposePrompt.fill("Review the retry path");
    await calls.callComposeExpect.fill('{"api_key":"string"}');
    await calls.callComposeSubmit.click();
    expect((await refusal).status()).toBe(422);
    await expect(calls.callComposeError).toContainText("call_expect_invalid");

    await calls.callComposeExpect.fill('{"verdict":""}');
    await calls.callComposeSubmit.click();
    await expect(calls.callComposeAccepted).toBeVisible();
    await waitForCallTotal(runtime, workspace.id, `agent=${encodeURIComponent(HELPER)}`, 2);

    const activity = await openAgents(appPage, runtime.url("/agents/activity"));
    await expect(activity.treeRows.first()).toBeVisible();
  });
});
