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
  // Invariant: authored soft breaks, lists, links, and inline code keep their visual structure through
  // send/reload, typed the way an operator types them (#872, #967).
  // Owner: session transcript browser rendering; canonical suite: this file.
  const url = "https://example.com/docs/transcript";
  const pastedList =
    "- first item\n- second item\n  - nested item\n\nRun `printf 'a  b'` then `printf 'a b'`.";
  // The fixture answers only this exact text, so the reply proves the wire kept every break.
  await ui.composerTextarea.click();
  await ui.composerTextarea.pressSequentially("show the diagram");
  await ui.composerTextarea.press("Shift+Enter");
  // Typed one character at a time, the URL first matches as a short prefix.
  await ui.composerTextarea.pressSequentially(`on a separate line, see ${url}`);
  await expect(
    window.getByTestId("composer-input").locator('span[style*="--compozy-composer-link"]')
  ).toHaveText([url]);
  await ui.composerTextarea.press("Shift+Enter");
  await ui.composerTextarea.press("Shift+Enter");
  await ui.composerTextarea.evaluate((element, text) => {
    const clipboardData = new DataTransfer();
    clipboardData.setData("text/plain", text);
    element.dispatchEvent(
      new ClipboardEvent("paste", { clipboardData, bubbles: true, cancelable: true })
    );
  }, pastedList);
  await expect(ui.composerTextarea).toContainText("nested item");
  await ui.composerTextarea.press("Enter");

  const bubble = window.getByTestId("user-message-bubble");
  const assertAuthoredLayout = async () => {
    await expect(bubble.getByRole("listitem")).toHaveText([
      "first item",
      /^second item\s+nested item\s*$/,
      "nested item",
    ]);
    await expect(bubble.getByRole("link", { name: url })).toHaveAttribute("target", "_blank");
    const inlineCode = bubble.locator("p code");
    await expect(inlineCode).toHaveText(["printf 'a  b'", "printf 'a b'"]);
    await expect
      .poll(() =>
        inlineCode.evaluateAll(elements => {
          const [doubleSpace, singleSpace] = elements;
          if (!doubleSpace || !singleSpace) return 0;
          return (
            doubleSpace.getBoundingClientRect().width - singleSpace.getBoundingClientRect().width
          );
        })
      )
      .toBeGreaterThan(1);
    // Markdown's whitespace between blocks never renders as blank lines inside the bubble.
    await expect
      .poll(() => bubble.evaluate(element => (element as HTMLElement).innerText))
      .not.toMatch(/\n{3,}/);
    await expect
      .poll(() =>
        bubble
          .locator("p")
          .first()
          .evaluate(element => {
            const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
            const lines: Text[] = [];
            for (let node = walker.nextNode(); node; node = walker.nextNode()) {
              lines.push(node as Text);
            }
            const first = lines.find(node => node.data.includes("show the diagram"));
            const next = lines.find(node => node.data.includes("on a separate line"));
            if (!first || !next) return false;
            const top = (node: Text) => {
              const range = document.createRange();
              range.setStart(node, 0);
              range.setEnd(node, 1);
              return range.getBoundingClientRect().top;
            };
            return top(next) > top(first);
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
