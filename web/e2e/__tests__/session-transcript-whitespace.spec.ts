import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow } from "../fixtures/os-navigation";
import { sessionWindowSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

const fixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_transcript_whitespace_fixture.json"
);
const agentName = "transcript-whitespace-agent";

test.use({
  runtimeOptions: {
    seed: { mockAgents: [{ fixturePath: fixture, fixtureAgent: agentName }] },
  },
});

test("split Mermaid and Markdown chunks render as separate elements after reload", async ({
  appPage,
  runtime,
}) => {
  if (!runtime.paths) throw new Error("transcript E2E requires an isolated runtime");
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await completeOnboardingIfPrompted(appPage);
  const created = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspace.id }),
  });
  const sessionId = created.session.id;
  await appPage.goto(runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const window = sessionWindow(appPage, sessionId);
  const ui = sessionWindowSelectors(window, appPage);
  await ui.composerTextarea.fill("show the diagram");
  await ui.composerTextarea.press("Enter");

  const markdown = window.getByTestId("message-markdown");
  await expect(markdown.getByRole("heading", { name: "After the diagram" })).toBeVisible();
  await expect(markdown.getByText("The paragraph is outside the code block.")).toBeVisible();
  expect(await markdown.locator('[data-slot="code-block-line"]').allTextContents()).toEqual(
    expect.arrayContaining(["flowchart LR", "A --> B"])
  );
  await expect(markdown.locator("pre")).not.toContainText("After the diagram");

  await appPage.reload();
  await expect(markdown.getByRole("heading", { name: "After the diagram" })).toBeVisible();
  await expect(markdown.getByText("The paragraph is outside the code block.")).toBeVisible();
  expect(await markdown.locator('[data-slot="code-block-line"]').allTextContents()).toEqual(
    expect.arrayContaining(["flowchart LR", "A --> B"])
  );
  await expect(markdown.locator("pre")).not.toContainText("After the diagram");
});
