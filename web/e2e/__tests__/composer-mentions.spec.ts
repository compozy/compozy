import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@playwright/test";

import { sessionWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// Composer mentions (`.compozy/tasks/composer-mentions/_tests.md` E2E-001…E2E-003): the
// `@` picker reads the real `searchSessionFiles` route and session catalog, the daemon
// resolves the sent tokens, and the transcript renders only what it verified.
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_composer_mentions_fixture.json"
);
const agentName = "composer-mentions-agent";
const promptPath = "internal/acp/client_prompt.go";

test.use({
  runtimeOptions: {
    seed: { mockAgents: [{ fixturePath, fixtureAgent: agentName }] },
  },
});

interface Scene {
  page: Page;
  runtime: BrowserRuntime;
  workspaceId: string;
}

async function prepare(appPage: Page, runtime: BrowserRuntime): Promise<Scene> {
  if (!runtime.paths) throw new Error("composer mention E2E requires an isolated runtime");
  const workspaceDir = runtime.paths.workspaceDir;
  await mkdir(path.join(workspaceDir, "internal", "acp"), { recursive: true });
  await writeFile(path.join(workspaceDir, promptPath), "package acp\n", "utf8");
  await writeFile(path.join(workspaceDir, "README.md"), "# Mentions\n", "utf8");
  const workspace = await runtime.resolveWorkspace(workspaceDir);
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(appPage);
  return { page: appPage, runtime, workspaceId: workspace.id };
}

async function createSession(scene: Scene, name: string): Promise<string> {
  const created = await scene.runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, name, workspace: scene.workspaceId }),
  });
  return created.session.id;
}

async function openSession(scene: Scene, sessionId: string): Promise<Locator> {
  await scene.page.goto(scene.runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const window = sessionWindow(scene.page, sessionId);
  await expect(window).toBeVisible();
  return window;
}

test("E2E-001: picking a file from the @ picker sends its token and the transcript keeps the chip", async ({
  appPage,
  runtime,
}) => {
  const scene = await prepare(appPage, runtime);
  const sessionId = await createSession(scene, "Review the prompt builder");
  const window = await openSession(scene, sessionId);
  const ui = sessionWindowSelectors(window, appPage);

  await ui.composerTextarea.click();
  await ui.composerTextarea.pressSequentially("Review @clie");
  const picker = window.getByRole("listbox", { name: "Mentions" });
  const row = picker.getByRole("option", { name: /client_prompt\.go/ });
  await expect(row).toBeVisible();
  await expect(row).toContainText("internal/acp");
  await ui.composerTextarea.press("Enter");

  const composerChip = window.getByTestId("composer-input").getByTestId("session-mention-chip");
  await expect(composerChip).toHaveAttribute("data-mention-target", promptPath);
  await expect(composerChip).toHaveText("client_prompt.go");
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

  // The daemon delivered the file to the agent as a link to its absolute path.
  await expect
    .poll(async () =>
      JSON.stringify(
        await runtime.requestJSON(
          `/api/workspaces/${encodeURIComponent(scene.workspaceId)}/sessions/${encodeURIComponent(
            sessionId
          )}/events`
        )
      )
    )
    .toContain(`/${promptPath}`);
});

test("E2E-002: a session picked from the @ picker renders as its title and opens that session", async ({
  appPage,
  runtime,
}) => {
  const scene = await prepare(appPage, runtime);
  const referencedId = await createSession(scene, "Fix login bug");
  const sessionId = await createSession(scene, "Pick up the login fix");
  const window = await openSession(scene, sessionId);
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
  await chip.click();
  await expect(sessionWindow(appPage, referencedId)).toBeVisible();
});

test("E2E-003: a typed token without the picker still resolves to a verified chip", async ({
  appPage,
  runtime,
}) => {
  const scene = await prepare(appPage, runtime);
  const sessionId = await createSession(scene, "Typed mention");
  const window = await openSession(scene, sessionId);
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
