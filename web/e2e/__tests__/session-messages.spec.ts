import { randomUUID } from "node:crypto";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
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

test("E2E-002: a queued session message shows its sender with Remove only, and Remove drops it", async ({
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
  // Agent-authored: no edit and no steer, only Remove.
  await expect(row.getByRole("button", { name: "Edit queued message" })).toHaveCount(0);
  await expect(row.getByRole("button")).toHaveCount(1);
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
