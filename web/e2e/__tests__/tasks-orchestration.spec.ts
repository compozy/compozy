import path from "node:path";
import { fileURLToPath } from "node:url";

import { tasksOperatorSelectors } from "../fixtures/selectors";
import { openAppWindow } from "../fixtures/os-navigation";
import { seedBrowserTasksOperatorFlow } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

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

const tasksSessionAgentName = "browser-lifecycle-agent";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: tasksSessionAgentName,
          fixtureAgent: tasksSessionAgentName,
          fixturePath: browserLifecycleFixture,
        },
      ],
    },
  },
});

test("operator uses the three-tab task detail, setup, inspect, and run review surfaces", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    sessionAgentName: tasksSessionAgentName,
  });

  await useGlobalWorkspaceIfPrompted(appPage);

  const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
  await expect(appPage).toHaveURL(/\/tasks$/);

  await appPage.goto(runtime.url(`/tasks/${seeded.referenceTask.id}?tab=activity&inspect=stream`), {
    waitUntil: "domcontentloaded",
  });
  await expect(tasksUI.detailContent).toBeVisible();
  await expect(tasksUI.detailTab("overview")).toBeVisible();
  await expect(tasksUI.detailTab("runs")).toBeVisible();
  await expect(tasksUI.detailTab("activity")).toHaveAttribute("aria-selected", "true");
  await expect(tasksUI.detailInspectDrawer).toBeVisible();
  await expect(tasksUI.detailInspectStream).toBeVisible();
  await expect(appPage).toHaveURL(/tab=activity.*inspect=stream|inspect=stream.*tab=activity/);

  await appPage.keyboard.press("Escape");
  await expect(tasksUI.detailInspectDrawer).not.toBeVisible();
  await expect(appPage).not.toHaveURL(/inspect=/);

  await tasksUI.detailSetupOpen.click();
  await expect(tasksUI.detailSetupSheet).toBeVisible();
  await tasksUI.detailSetupEdit.click();
  await expect(tasksUI.detailSetupForm).toBeVisible();
  await expect(tasksUI.detailSetupWorkerRuntime).toBeVisible();

  await appPage.keyboard.press("Escape");
  await appPage.goto(runtime.url(`/tasks/${seeded.runningTask.id}?tab=runs`), {
    waitUntil: "domcontentloaded",
  });
  await expect(tasksUI.detailContent).toBeVisible();
  await expect(tasksUI.runsRow(seeded.runningRun.id)).toBeVisible();
  await tasksUI
    .runsRow(seeded.runningRun.id)
    .getByRole("link", { exact: true, name: `Attempt ${seeded.runningRun.attempt}` })
    .click();
  await expect(tasksUI.runDetailContent).toBeVisible();
  await expect(tasksUI.runReviews).toHaveCount(0);

  await browserArtifacts.captureScreenshot("tasks-three-tab-detail", appPage);
});
