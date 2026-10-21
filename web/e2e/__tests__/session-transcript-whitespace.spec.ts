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
  browserArtifacts,
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
  // Invariant: authored soft breaks and lists keep their visual structure through send/reload.
  // Owner: session transcript browser rendering; canonical suite: this file.
  const prompt = "show the diagram\non a separate line\n\n- first item\n- second item";
  await ui.composerTextarea.fill(prompt);
  await ui.composerTextarea.press("Enter");

  const bubble = window.getByTestId("user-message-bubble");
  const assertAuthoredLayout = async () => {
    await expect(bubble.getByRole("listitem")).toHaveText(["first item", "second item"]);
    await expect
      .poll(() =>
        bubble
          .locator("p")
          .first()
          .evaluate(element => {
            const text = element.firstChild!;
            const source = text.textContent ?? "";
            const nextLine = source.indexOf("on a separate line");
            if (nextLine < 0) return false;
            const first = document.createRange();
            first.setStart(text, 0);
            first.setEnd(text, 1);
            const second = document.createRange();
            second.setStart(text, nextLine);
            second.setEnd(text, nextLine + 1);
            return second.getBoundingClientRect().top > first.getBoundingClientRect().top;
          })
      )
      .toBe(true);
    await appPage.mouse.move(0, 0);
    await expect(window.getByTestId("user-message-actions")).toHaveCSS("opacity", "1");
    await expect(window.getByTestId("assistant-message-actions")).toHaveCSS("opacity", "1");
  };
  const markdown = window.getByTestId("message-markdown");
  await expect(markdown.getByRole("heading", { name: "After the diagram" })).toBeVisible();
  await expect(markdown.getByText("The paragraph is outside the code block.")).toBeVisible();
  await expect(markdown.getByRole("img", { name: "Mermaid diagram" })).toBeVisible();
  await markdown.getByRole("button", { name: "View source", exact: true }).click();
  await expect
    .poll(() => markdown.locator('[data-slot="code-block-line"]').allTextContents())
    .toEqual(expect.arrayContaining(["flowchart LR", "A --> B"]));
  await expect(markdown.locator("pre")).not.toContainText("After the diagram");

  await assertAuthoredLayout();
  await browserArtifacts.captureScreenshot("authored-message-layout", appPage);
  await appPage.reload();
  await expect(markdown.getByRole("heading", { name: "After the diagram" })).toBeVisible();
  await expect(markdown.getByText("The paragraph is outside the code block.")).toBeVisible();
  await expect(markdown.getByRole("img", { name: "Mermaid diagram" })).toBeVisible();
  await markdown.getByRole("button", { name: "View source", exact: true }).click();
  await expect
    .poll(() => markdown.locator('[data-slot="code-block-line"]').allTextContents())
    .toEqual(expect.arrayContaining(["flowchart LR", "A --> B"]));
  await expect(markdown.locator("pre")).not.toContainText("After the diagram");
  await assertAuthoredLayout();
});
