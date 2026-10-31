import { randomUUID } from "node:crypto";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { sessionWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// Agent collaboration (`.compozy/tasks/agent-collaboration/_tests.md` E2E-001…E2E-003).
// The sender agent calls `compozy__session_prompt` for real through its hosted MCP server
// (acpmock `native_tool_call`), so the daemon stamps the sender, queues or dispatches the
// message, arms the reply watch and wakes the sender exactly as for a provider. A native call
// needs the receiver's session id, which exists only at run time: each worker copies the
// fixture to a temp file the seeded agent reads, and every test rewrites that copy with its
// receiver's id before it creates the sender (the driver reads its fixture when the sender's
// session starts).
const sourceFixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_messages_fixture.json"
);
const TARGET_PLACEHOLDER = "__TARGET_SESSION_ID__";
const fixtureTemplate = readFileSync(sourceFixturePath, "utf8");
const fixturePath = path.join(
  mkdtempSync(path.join(os.tmpdir(), "compozy-session-messages-fixture-")),
  "browser_session_messages_fixture.json"
);
writeFileSync(fixturePath, fixtureTemplate, "utf8");

const senderAgent = "session-message-sender";
const receiverAgent = "session-message-receiver";
const senderTitle = "Refactor billing";
const receiverTitle = "Billing reviewer";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath, fixtureAgent: senderAgent },
        { fixturePath, fixtureAgent: receiverAgent },
      ],
    },
  },
});

interface Party {
  sessionId: string;
  window: Locator;
}

interface Scene {
  page: Page;
  runtime: BrowserRuntime;
  workspaceId: string;
}

function sessionAPIPath(workspaceId: string, sessionId: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(
    sessionId
  )}${suffix}`;
}

async function prepare(appPage: Page, runtime: BrowserRuntime): Promise<Scene> {
  if (!runtime.paths) throw new Error("session message E2E requires an isolated runtime");
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(appPage);
  return { page: appPage, runtime, workspaceId: workspace.id };
}

async function createSession(scene: Scene, agentName: string, name: string): Promise<string> {
  const created = await scene.runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, name, workspace: scene.workspaceId }),
  });
  return created.session.id;
}

async function openSession(scene: Scene, agentName: string, sessionId: string): Promise<Party> {
  await scene.page.goto(scene.runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const window = sessionWindow(scene.page, sessionId);
  await expect(window).toBeVisible();
  return { sessionId, window };
}

/** Points the sender's native `session_prompt` calls at this test's receiver, then creates it. */
async function createSender(scene: Scene, receiverId: string): Promise<string> {
  writeFileSync(fixturePath, fixtureTemplate.replaceAll(TARGET_PLACEHOLDER, receiverId), "utf8");
  return createSession(scene, senderAgent, senderTitle);
}

// The prompt route answers with an event stream; drain it so the sender's turn has ended.
async function promptSession(scene: Scene, sessionId: string, message: string): Promise<void> {
  const response = await fetch(
    scene.runtime.url(sessionAPIPath(scene.workspaceId, sessionId, "/prompt")),
    {
      headers: { "content-type": "application/json" },
      method: "POST",
      body: JSON.stringify({ idempotency_key: randomUUID(), message, message_id: randomUUID() }),
    }
  );
  if (!response.ok) {
    throw new Error(`prompt failed with ${response.status}: ${await response.text()}`);
  }
  await response.text();
}

async function sendFromComposer(scene: Scene, party: Party, message: string): Promise<void> {
  const ui = sessionWindowSelectors(party.window, scene.page);
  await ui.composerTextarea.fill(message);
  await ui.composerTextarea.press("Enter");
}

test("E2E-001: a session message renders as a left-aligned card from its sender, not an operator bubble", async ({
  appPage,
  runtime,
}) => {
  const scene = await prepare(appPage, runtime);
  const receiverId = await createSession(scene, receiverAgent, receiverTitle);
  const receiver = await openSession(scene, receiverAgent, receiverId);
  await sendFromComposer(scene, receiver, "hello reviewer");
  await expect(receiver.window.getByText("Ready to review.")).toBeVisible();

  const senderId = await createSender(scene, receiverId);
  await promptSession(scene, senderId, "send the reviewer a question");

  const message = receiver.window.getByRole("article", { name: `Message from ${senderTitle}` });
  await expect(message).toBeVisible();
  await expect(message).toContainText(
    "Is the retry budget in billing.toml per request or per job?"
  );
  await expect(
    receiver.window.getByText("Per job. The retry budget caps attempts across a job's requests.")
  ).toBeVisible();
  // Only the operator's own prompt is a right-aligned bubble.
  const bubbles = receiver.window.getByTestId("user-message-bubble");
  await expect(bubbles).toHaveCount(1);
  await expect(bubbles).toContainText("hello reviewer");
  const messageBox = (await message.boundingBox())!;
  const bubbleBox = (await bubbles.boundingBox())!;
  expect(messageBox.x).toBeLessThan(bubbleBox.x);
  expect(bubbleBox.x + bubbleBox.width).toBeGreaterThan(messageBox.x + messageBox.width);

  await message.getByRole("button", { name: senderTitle }).click();
  await expect(sessionWindow(appPage, senderId)).toBeVisible();
});

test("E2E-002: a queued session message shows its sender with Steer and Remove, no Edit, and Remove drops it", async ({
  appPage,
  runtime,
}) => {
  const scene = await prepare(appPage, runtime);
  const receiverId = await createSession(scene, receiverAgent, receiverTitle);
  const receiver = await openSession(scene, receiverAgent, receiverId);
  await sendFromComposer(scene, receiver, "hold on a long review");
  await expect(receiver.window.getByText("Reviewing the whole module.")).toBeVisible();

  const senderId = await createSender(scene, receiverId);
  await promptSession(scene, senderId, "queue a note for the reviewer");

  const sender = receiver.window.getByTestId("composer-queued-sender");
  await expect(sender).toContainText(`From${senderTitle}`);
  const row = receiver.window
    .getByTestId("composer-queued-prompt-row")
    .filter({ has: appPage.getByTestId("composer-queued-sender") });
  // Agent-authored (#760): the operator may Steer or Remove it, never edit its text.
  await expect(row.getByRole("button", { name: "Edit queued message" })).toHaveCount(0);
  await expect(
    row.getByRole("button", { name: `Steer message from ${senderTitle}` })
  ).toBeVisible();
  await expect(row.getByRole("button")).toHaveCount(2);
  await row.getByRole("button", { name: `Remove message from ${senderTitle}` }).click();
  await expect(sender).toHaveCount(0);
  await expect
    .poll(async () => {
      const queue = await runtime.requestJSON<{ inputs: unknown[] }>(
        sessionAPIPath(scene.workspaceId, receiverId, "/prompt/queue")
      );
      return queue.inputs.length;
    })
    .toBe(0);
});

test("E2E-003: the sender sees Sent to, Waiting for reply, then the Reply from card and Replied", async ({
  appPage,
  runtime,
}) => {
  const scene = await prepare(appPage, runtime);
  const receiverId = await createSession(scene, receiverAgent, receiverTitle);
  const senderId = await createSender(scene, receiverId);
  const sender = await openSession(scene, senderAgent, senderId);
  await sendFromComposer(scene, sender, "ask the reviewer and wait");

  const sent = sender.window.getByTestId("session-sent-card");
  await expect(sent).toHaveAccessibleName(`Sent to ${receiverTitle}, waiting for reply`);
  await expect(sent).toContainText("Waiting for reply");

  const reply = sender.window.getByRole("article", {
    name: `Reply from ${receiverTitle}, completed`,
  });
  await expect(reply).toBeVisible({ timeout: 30_000 });
  await expect(reply).toContainText(
    "Per job. The retry budget caps attempts across a job's requests."
  );
  await expect(sent).toHaveAccessibleName(`Sent to ${receiverTitle}, replied`);
  await expect(sent).toContainText("Replied");
  // The reply wake started a turn on the sender with the answer.
  await expect(sender.window.getByText("The reviewer answered: per job.")).toBeVisible();
});

interface HeaderGeometry {
  title: string | null;
  ellipsized: boolean;
  titleRight: number;
  trailingLeft: number;
  trailingRight: number;
  cardRight: number;
}

/** Where a card's linked title ends relative to the header items after it and the card edge. */
async function headerGeometry(card: Locator): Promise<HeaderGeometry> {
  return card.evaluate(element => {
    const link = element.querySelector<HTMLElement>('[data-testid="session-message-party-link"]');
    const text = link?.querySelector<HTMLElement>("span") ?? link;
    if (!link || !text) throw new Error("card has no linked party title");
    const trailing = [
      ...element.querySelectorAll<HTMLElement>(
        '[data-testid="session-message-chip"], [data-testid="session-sent-reply-state"], [data-testid="session-reply-outcome"], [data-testid="session-message-time"]'
      ),
    ].map(item => item.getBoundingClientRect());
    if (trailing.length === 0) throw new Error("card has no trailing header items");
    return {
      title: link.getAttribute("title"),
      ellipsized: text.scrollWidth > text.clientWidth,
      titleRight: link.getBoundingClientRect().right,
      trailingLeft: Math.min(...trailing.map(rect => rect.left)),
      trailingRight: Math.max(...trailing.map(rect => rect.right)),
      cardRight: element.getBoundingClientRect().right,
    };
  });
}

test("E2E-004: a long session title ellipsizes on the Sent to and Reply from cards without covering the reply state or time (#1097)", async ({
  appPage,
  runtime,
}) => {
  const longTitle = "Preparar BnUI para migração do AdvOS (tema AdvOS + base-ui 1.8 + ícones)";
  const scene = await prepare(appPage, runtime);
  const receiverId = await createSession(scene, receiverAgent, longTitle);
  const senderId = await createSender(scene, receiverId);
  const sender = await openSession(scene, senderAgent, senderId);
  await sendFromComposer(scene, sender, "ask the reviewer and wait");

  const sent = sender.window.getByTestId("session-sent-card");
  const reply = sender.window.getByRole("article", { name: `Reply from ${longTitle}, completed` });
  await expect(reply).toBeVisible({ timeout: 30_000 });
  await expect(sent).toHaveAccessibleName(`Sent to ${longTitle}, replied`);

  // Invariant: the title shrinks first and clips with an ellipsis inside its column; the
  // header items after it stay whole, to its right, and inside the card.
  for (const card of [sent, reply]) {
    await expect(card.getByRole("button", { name: longTitle })).toBeVisible();
    const geometry = await headerGeometry(card);
    expect(geometry.title).toBe(longTitle);
    expect(geometry.ellipsized).toBe(true);
    expect(geometry.titleRight).toBeLessThanOrEqual(geometry.trailingLeft);
    expect(geometry.trailingRight).toBeLessThanOrEqual(geometry.cardRight);
  }
});

// Composer mentions (`.compozy/tasks/composer-mentions/_tests.md` E2E-001…E2E-003, titled
// CM-E2E-001…CM-E2E-003 so they stay distinct from the agent-collaboration journeys above): the
// `@` picker reads the real `searchSessionFiles` route and session catalog, the daemon resolves
// the sent tokens, and the transcript renders only what it verified. The daemon persists
// no absolute path and no references text — the prompt delivery manifest records what each
// turn delivered (`mentions.links`, `mentions.sessions`, `mentions.references_form`).
test.describe("composer mentions", () => {
  const mentionFixturePath = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "../../../internal/testutil/acpmock/testdata/browser_composer_mentions_fixture.json"
  );
  const mentionAgent = "composer-mentions-agent";
  const promptPath = "internal/acp/client_prompt.go";

  test.use({
    runtimeOptions: {
      seed: { mockAgents: [{ fixturePath: mentionFixturePath, fixtureAgent: mentionAgent }] },
    },
  });

  interface DeliveredMentions {
    links: number;
    sessions: number;
    references_form?: string;
  }

  interface RecordedMention {
    kind: string;
    target: string;
  }

  /** Every value under `key` anywhere in the session's event envelope. */
  function valuesUnder(value: unknown, key: string, found: unknown[] = []): unknown[] {
    if (Array.isArray(value)) {
      for (const item of value) valuesUnder(item, key, found);
    } else if (value !== null && typeof value === "object") {
      for (const [entryKey, entry] of Object.entries(value)) {
        if (entryKey === key) found.push(entry);
        valuesUnder(entry, key, found);
      }
    }
    return found;
  }

  async function sessionEvents(scene: Scene, sessionId: string): Promise<unknown> {
    return scene.runtime.requestJSON<unknown>(
      sessionAPIPath(scene.workspaceId, sessionId, "/events")
    );
  }

  /** The prompt delivery manifests' mention counts for this session. */
  async function deliveredMentions(scene: Scene, sessionId: string): Promise<DeliveredMentions[]> {
    return valuesUnder(await sessionEvents(scene, sessionId), "mentions").filter(
      (value): value is DeliveredMentions =>
        value !== null && typeof value === "object" && !Array.isArray(value) && "links" in value
    );
  }

  /** The daemon-verified mentions recorded on this session's input events. */
  async function recordedMentions(scene: Scene, sessionId: string): Promise<RecordedMention[]> {
    return valuesUnder(await sessionEvents(scene, sessionId), "mentions")
      .filter(Array.isArray)
      .flat()
      .filter(
        (value): value is RecordedMention =>
          value !== null && typeof value === "object" && "kind" in value && "target" in value
      );
  }

  async function prepareMentions(appPage: Page, runtime: BrowserRuntime): Promise<Scene> {
    if (!runtime.paths) throw new Error("composer mention E2E requires an isolated runtime");
    const workspaceDir = runtime.paths.workspaceDir;
    await mkdir(path.join(workspaceDir, "internal", "acp"), { recursive: true });
    await writeFile(path.join(workspaceDir, promptPath), "package acp\n", "utf8");
    await writeFile(path.join(workspaceDir, "README.md"), "# Mentions\n", "utf8");
    return prepare(appPage, runtime);
  }

  test("CM-E2E-001: picking a file from the @ picker sends its token and the transcript keeps the chip", async ({
    appPage,
    runtime,
  }) => {
    const scene = await prepareMentions(appPage, runtime);
    const sessionId = await createSession(scene, mentionAgent, "Review the prompt builder");
    const { window } = await openSession(scene, mentionAgent, sessionId);
    const ui = sessionWindowSelectors(window, appPage);

    await ui.composerTextarea.click();
    await ui.composerTextarea.pressSequentially("Review @clie");
    const picker = window.getByRole("listbox", { name: "Mentions" });
    const row = picker.getByRole("option", { name: /client_prompt\.go/ });
    await expect(row).toBeVisible();
    await expect(row).toContainText("internal/acp");
    // Rows from an earlier query stay visible but the keyboard never picks them; Enter commits
    // only once the typed query's own rows have landed and the target is highlighted.
    await expect(picker).not.toHaveAttribute("aria-busy", "true");
    await expect(row).toHaveAttribute("aria-selected", "true");
    await ui.composerTextarea.press("Enter");

    const composerChip = window.getByTestId("composer-input").getByTestId("session-mention-chip");
    await expect(composerChip).toHaveAttribute("data-mention-target", promptPath);
    await expect(composerChip).toContainText("client_prompt.go");
    await expect(picker).toHaveCount(0);
    await ui.composerTextarea.press("Enter");

    const bubble = window.getByTestId("user-message-bubble");
    await expect(bubble).toContainText("Review client_prompt.go");
    await expect(window.getByText("Read the mentioned context.")).toBeVisible();

    await appPage.reload();
    const historyChip = window
      .getByTestId("user-message-bubble")
      .getByTestId("session-mention-chip")
      .filter({ hasText: "client_prompt.go" });
    await expect(historyChip).toHaveAttribute("data-mention-target", promptPath);

    // The input event recorded the verified file, and the turn delivered one resource_link.
    await expect
      .poll(async () => (await recordedMentions(scene, sessionId)).map(m => m.target))
      .toContain(promptPath);
    await expect
      .poll(async () => (await deliveredMentions(scene, sessionId)).map(m => m.links))
      .toContain(1);
  });

  test("CM-E2E-002: a session picked from the @ picker renders as its title, opens that session, and reaches the agent as a reference", async ({
    appPage,
    runtime,
  }) => {
    const scene = await prepareMentions(appPage, runtime);
    const referencedId = await createSession(scene, mentionAgent, "Fix login bug");
    const sessionId = await createSession(scene, mentionAgent, "Pick up the login fix");
    const { window } = await openSession(scene, mentionAgent, sessionId);
    const ui = sessionWindowSelectors(window, appPage);

    await ui.composerTextarea.click();
    await ui.composerTextarea.pressSequentially("Continue from @Fix");
    const sessions = window.getByTestId("composer-mention-group-sessions");
    await expect(sessions.getByRole("option", { name: /Fix login bug/ })).toBeVisible();
    // The session being prompted is never offered.
    await expect(sessions.getByRole("option", { name: /Pick up the login fix/ })).toHaveCount(0);
    await sessions.getByRole("option", { name: /Fix login bug/ }).click();
    await ui.composerTextarea.press("Enter");

    const chip = window
      .getByTestId("user-message-bubble")
      .getByRole("button", { name: `Session Fix login bug (${referencedId})` });
    await expect(chip).toBeVisible();

    // B's turn carried the <compozy_session_references> block for A: the input event
    // recorded the verified session, and the delivery manifest counts one reference in
    // the tool-reading form.
    await expect
      .poll(async () =>
        (await recordedMentions(scene, sessionId))
          .filter(m => m.kind === "session")
          .map(m => m.target)
      )
      .toContain(referencedId);
    await expect
      .poll(async () =>
        (await deliveredMentions(scene, sessionId)).map(
          m => `${m.sessions}:${m.references_form ?? ""}`
        )
      )
      .toContain("1:reference");

    await chip.click();
    await expect(sessionWindow(appPage, referencedId)).toBeVisible();
  });

  test("CM-E2E-003: a typed token without the picker still resolves to a verified chip", async ({
    appPage,
    runtime,
  }) => {
    const scene = await prepareMentions(appPage, runtime);
    const sessionId = await createSession(scene, mentionAgent, "Typed mention");
    const { window } = await openSession(scene, mentionAgent, sessionId);
    const ui = sessionWindowSelectors(window, appPage);

    // The trailing space ends the token, so the picker is closed when Enter sends.
    await ui.composerTextarea.fill("Check @README.md ");
    await ui.composerTextarea.press("Enter");

    const chip = window
      .getByTestId("user-message-bubble")
      .getByTestId("session-mention-chip")
      .filter({ hasText: "README.md" });
    await expect(chip).toHaveAttribute("data-mention-kind", "file");
    await expect(window.getByTestId("composer-input")).not.toContainText("README.md");
  });
});
