import { fileURLToPath } from "node:url";
import path from "node:path";

import { openAppWindow, sessionWindow } from "../fixtures/os-navigation";
import { waitForSeedSessionActive } from "../fixtures/runtime";
import { sessionLifecycleSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

const browserLifecycleFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_lifecycle_fixture.json"
);

const browserLifecycleAgent = "browser-lifecycle-agent";
const browserLifecyclePrompt = "run browser lifecycle flow";

function browserLifecycleSessionPath(agentName: string, sessionId: string): string {
  return `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(sessionId)}`;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: browserLifecycleFixture,
          fixtureAgent: browserLifecycleAgent,
        },
      ],
    },
  },
});

test("operator can onboard, create a session, submit work, approve a permission request, reload transcript continuity, and resume controls", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const ui = sessionLifecycleSelectors(appPage);

  await completeOnboardingIfPrompted(ui);
  await expect(ui.osDesktop).toBeVisible();
  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const fleet = sessionLifecycleSelectors(agentsWin);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
  await expect(fleet.agentRow(browserLifecycleAgent)).toBeVisible();

  await fleet.agentRow(browserLifecycleAgent).click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${browserLifecycleAgent}`);
  await expect(fleet.agentPageNewSession).toBeVisible();
  await fleet.agentPageNewSession.click();

  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();
  await expect(appPage.getByTestId("session-create-agent-select")).toContainText(
    browserLifecycleAgent
  );

  const createResponsePromise = appPage.waitForResponse(
    response => response.request().method() === "POST" && response.url().endsWith("/api/sessions")
  );
  await appPage.getByTestId("session-create-submit").click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();
  const createPayload = (await createResponse.json()) as {
    session?: { id?: string; workspace_id?: string };
  };
  const sessionId = createPayload.session?.id ?? "";
  const workspaceId = createPayload.session?.workspace_id ?? "";
  expect(sessionId).not.toBe("");
  expect(workspaceId).not.toBe("");
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(browserLifecycleSessionPath(browserLifecycleAgent, sessionId));
  const sessionWin = sessionWindow(appPage, sessionId);
  const sessionUi = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await expect(sessionUi.composerTextarea).toBeVisible();

  // Runtime selection belongs to the first-message composer after the durable,
  // unbound session has been accepted.
  const runtimeTrigger = sessionWin.getByTestId("session-prompt-runtime-select");
  await runtimeTrigger.click();
  await expect(appPage.getByTestId("runtime-selector-search")).toBeVisible();
  await appPage.getByTestId("runtime-selector-search").fill("qa-browser-model");
  await appPage.getByTestId("runtime-selector-custom").click();
  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("runtime-selector-popup")).toHaveCount(0);
  await expect(runtimeTrigger).toContainText("qa-browser-model");

  await sessionUi.composerTextarea.fill(browserLifecyclePrompt);
  await sessionUi.composerTextarea.press("Enter");
  await waitForSeedSessionActive(runtime, sessionId);

  await expect(sessionUi.permissionPrompt).toBeVisible();
  await expect(sessionWin.getByTestId("permission-reject-once")).toBeVisible();
  await expect(sessionUi.chatView).toContainText("Streaming response started.");

  const approvalResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(sessionAPIPath(workspaceId, sessionId, "/approve"))
    );
  });
  await sessionUi.permissionAllowOnce.click();
  const approvalResponse = await approvalResponsePromise;
  expect(approvalResponse.ok()).toBeTruthy();

  await expect(sessionUi.chatView).toContainText("Streaming response started.");
  await expect(sessionUi.chatView).toContainText(
    "Approval granted.Session continued after approval."
  );

  const sessionPath = new URL(appPage.url()).pathname;

  await appPage.reload({ waitUntil: "domcontentloaded" });

  await expect.poll(() => new URL(appPage.url()).pathname).toBe(sessionPath);
  await expect(sessionWin).toBeVisible();
  await expect(sessionUi.chatView).toContainText(browserLifecyclePrompt);
  // Rehydration keeps every authored response segment visible across the
  // permission boundary instead of hiding the pre-permission text as work.
  await expect(sessionUi.chatView).toContainText("Streaming response started.");
  await expect(sessionUi.chatView).toContainText(
    "Approval granted.Session continued after approval."
  );
  const workedFold = sessionWin.getByRole("button", { name: /^Worked/ });
  await expect(workedFold).toHaveCount(0);
  await expect(sessionUi.resumeButton).toBeVisible();

  await sessionUi.resumeButton.click();
  await expect(sessionUi.stopButton).toBeVisible();

  await sessionUi.stopButton.click();
  await expect(sessionUi.resumeButton).not.toBeVisible();

  await browserArtifacts.captureScreenshot("session-onboarding-hydrated", appPage);
});
