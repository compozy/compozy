import path from "node:path";
import { fileURLToPath } from "node:url";

import { openSessionsCatalog } from "../fixtures/os-navigation";
import { waitForSeedSessionActive } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

const agentName = "browser-lifecycle-agent";
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_lifecycle_fixture.json"
);

test.use({ runtimeOptions: { seed: { mockAgents: [{ fixturePath, fixtureAgent: agentName }] } } });

// Invariant: bulk deletion removes selected active/stopped sessions while preserving unselected catalog members.
// Owner: public Sessions modal + daemon; reuse the existing browser lifecycle runtime seed.
test("operator deletes selected active and stopped sessions and preserves their neighbor", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  await ensureProjectWorkspace(appPage, runtime);
  const workspace =
    runtime.seeded.workspace ??
    (runtime.paths?.workspaceDir
      ? await runtime.resolveWorkspace(runtime.paths.workspaceDir)
      : null);
  if (!workspace) throw new Error("Bulk session walk requires a seeded workspace");
  const ids: string[] = [];
  for (let index = 0; index < 4; index++) {
    const { session } = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ agent_name: agentName, workspace: workspace.id }),
    });
    await waitForSeedSessionActive(runtime, session.id);
    ids.push(session.id);
  }
  const neighborID = ids.pop()!;
  for (const id of ids.slice(0, 2)) {
    const base = `/api/workspaces/${workspace.id}/sessions/${id}`;
    const stopResponse = await appPage.request.post(runtime.url(`${base}/stop`));
    expect(stopResponse.status(), await stopResponse.text()).toBe(204);
    await expect
      .poll(async () => {
        const payload = await runtime.requestJSON<{ session: { state: string } }>(base);
        return payload.session.state;
      })
      .toBe("stopped");
  }
  const catalog = await openSessionsCatalog(appPage);
  const filter = catalog.getByRole("searchbox", { name: "Filter sessions" });
  const filterBefore = await filter.boundingBox();
  for (const id of ids) {
    await catalog
      .getByTestId(`os-sessions-modal-session-${id}`)
      .click({ modifiers: ["ControlOrMeta"] });
  }
  await expect(catalog.getByTestId("os-sessions-modal-selection-count")).toHaveText("3 selected");
  // The selection bar preserves the host toolbar height: filtering stays in place.
  const filterAfter = await filter.boundingBox();
  expect(filterAfter?.y).toBe(filterBefore?.y);
  await browserArtifacts.captureScreenshot("bulk-selection", appPage);
  await catalog.getByTestId("os-sessions-modal-selection-delete").click();
  await expect(appPage.getByRole("heading", { name: "Delete 3 sessions" })).toBeVisible();
  await browserArtifacts.captureScreenshot("bulk-delete-confirm", appPage);
  await appPage.getByTestId("delete-dialog-confirm").click();
  await expect(appPage.getByTestId("delete-dialog")).toBeHidden();
  await expect(appPage.getByText("3 sessions deleted", { exact: true })).toBeVisible();
  for (const id of ids) {
    await expect(catalog.getByTestId(`os-sessions-modal-session-${id}`)).toHaveCount(0);
  }
  await expect(catalog).toBeVisible();
  await expect(catalog.getByTestId("os-sessions-modal-selection-bar")).toHaveCount(0);
  const remaining = await runtime.requestJSON<{ sessions: Array<{ id: string }> }>(
    `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}&archive=include`
  );
  expect(remaining.sessions.filter(session => ids.includes(session.id))).toEqual([]);
  expect(remaining.sessions.some(session => session.id === neighborID)).toBe(true);
  await expect(catalog.getByTestId(`os-sessions-modal-session-${neighborID}`)).toBeVisible();
  await browserArtifacts.captureScreenshot("bulk-delete-complete", appPage);
});
