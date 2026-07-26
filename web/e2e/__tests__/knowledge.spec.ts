import { execFile } from "node:child_process";
import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import { sessionWindow } from "../fixtures/os-navigation";
import {
  SESSION_CREATE_FIRST_MESSAGE,
  knowledgeOperatorSelectors,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
} from "../fixtures/selectors";
import type { BrowserRuntime } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const memoryRecallFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "memory_recall_fixture.json"
);
const memoryRecallAgentName = "memory-recall-agent";
const sensitivePattern =
  /agh_claim_[a-z0-9._-]+|["']claim_token["']\s*:\s*["']?[a-z0-9._-]{8,}|(?:authorization\s*:\s*bearer|bearer)\s+["']?[a-z0-9._-]{8,}|(?:api[_-]?key|bearer[_-]?token|mcp[_-]?auth|oauth[_-]?(?:access(?:[_-]?token)?|client(?:[_-]?secret)?|refresh(?:[_-]?token)?|secret|token)|pkce[_-]?(?:challenge|secret|verifier)|provider[_-]?credential|telegram-bot-token)\s*[:=]\s*["']?[a-z0-9._:-]{8,}|\b\d{6,}:[a-z0-9_-]{20,}/i;

interface MemoryHeader {
  filename: string;
  name: string;
  scope: "global" | "workspace" | "agent";
  type: "user" | "feedback" | "project" | "reference";
  workspace_id?: string;
}

interface MemoryEntry {
  memory: {
    content: string;
    summary: MemoryHeader;
  };
}

interface MemoriesResponse {
  memories: MemoryHeader[];
}

interface MemorySearchResponse {
  results: Array<{
    memory: MemoryHeader;
    score: number;
    snippet?: string;
  }>;
}

interface MemoryMutationResponse {
  applied?: boolean;
  decision: {
    id: string;
    op: string;
    scope: "global" | "workspace" | "agent";
    target_filename?: string;
    frontmatter: {
      filename: string;
      name: string;
      type: MemoryHeader["type"];
    };
  };
  reverted?: boolean;
}

interface SessionEnvelope {
  session: {
    acp_session_id?: string;
    id: string;
    workspace_id: string;
  };
}

interface DiagnosticsRecord {
  lifecycle_event?: string;
  prompt_index: number;
  prompt: string;
  session_id?: string;
}

interface TranscriptMessage {
  parts?: TranscriptMessagePart[];
  role?: string;
}

interface TranscriptMessagePart {
  text?: string;
  type?: string;
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: memoryRecallAgentName,
          fixtureAgent: memoryRecallAgentName,
          fixturePath: memoryRecallFixture,
        },
      ],
    },
  },
});

test("operator creates edits reverts searches recalls and deletes workspace knowledge with parity evidence", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Knowledge browser E2E requires launch-mode runtime paths.");
  }

  await ensureGlobalWorkspace(runtime);
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(appPage);

  await appPage.goto(runtime.url("/knowledge"), { waitUntil: "domcontentloaded" });
  const kWin = appPage.getByTestId("os-window-app:knowledge");
  await expect(kWin).toBeVisible();
  const knowledgeUI = knowledgeOperatorSelectors(kWin);
  await expect(knowledgeUI.shell).toBeVisible();
  await expect(knowledgeUI.tabGlobal).toHaveAttribute("aria-pressed", "true");
  await knowledgeUI.tabWorkspace.click();
  await expect(knowledgeUI.tabWorkspace).toHaveAttribute("aria-pressed", "true");

  const marker = `browser-knowledge-${randomUUID().slice(0, 8)}`;
  const memoryName = `Browser Knowledge ${marker}`;
  const originalContent = `Remember me: auth migration uses sessions and workspace-scoped recall. ${marker}`;
  const editedContent = `Remember me: auth migration uses sessions after browser edit and revert. ${marker}`;

  await knowledgeUI.createButton.click();
  await expect(knowledgeUI.createDialog).toBeVisible();
  await expect(knowledgeUI.createDialog).toHaveAttribute("data-frame", "framed");
  await expect(knowledgeUI.createDialog.locator('[data-slot="dialog-header"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(knowledgeUI.createDialog.locator('[data-slot="dialog-footer"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  const projectType = appPage.getByTestId("knowledge-create-type-project");
  await projectType.click();
  await expect(projectType).toHaveAttribute("aria-checked", "true");
  await knowledgeUI.createName.fill(memoryName);
  await knowledgeUI.createDescription.fill("browser-created workspace recall contract");
  await knowledgeUI.createContent.fill(originalContent);
  const createResponsePromise = appPage.waitForResponse(
    response => response.request().method() === "POST" && response.url().endsWith("/api/memory")
  );
  await knowledgeUI.confirmCreateMemory.click();
  const createPayload = (await (await createResponsePromise).json()) as MemoryMutationResponse;
  expect(JSON.stringify(createPayload)).not.toMatch(sensitivePattern);
  const filename =
    createPayload.decision.target_filename ?? createPayload.decision.frontmatter.filename;
  expect(filename).toMatch(/\.md$/);
  await expect(knowledgeUI.createDialog).toBeHidden();
  await expect(knowledgeUI.item(`workspace:${filename}`)).toBeVisible({ timeout: 20_000 });
  await knowledgeUI.item(`workspace:${filename}`).click();
  await expect(kWin.getByTestId("knowledge-detail-title")).toContainText(memoryName);
  await expect(knowledgeUI.contentPreview).toContainText(originalContent);

  await knowledgeUI.searchInput.fill("auth migration sessions");
  await expect(knowledgeUI.searchInfo).toContainText("Recall");
  await expect(knowledgeUI.item(`workspace:${filename}`)).toBeVisible();

  const httpEntry = await readMemoryHTTP(runtime, filename, workspace.id);
  const udsEntry = await requestOperatorJSONOrThrow<MemoryEntry>(
    runtime,
    memoryReadPath(filename, workspace.id)
  );
  const cliEntry = await memoryCLI<MemoryEntry>(runtime, [
    "memory",
    "show",
    filename,
    "--scope",
    "workspace",
    "--workspace",
    workspace.id,
  ]);
  expect(httpEntry.memory.content).toBe(originalContent);
  expect(udsEntry.memory.content).toBe(originalContent);
  expect(cliEntry.memory.content).toBe(originalContent);

  const editResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "PATCH" &&
      response.url().includes(`/api/memory/${encodeURIComponent(filename)}`)
  );
  await knowledgeUI.editButton.click();
  await expect(knowledgeUI.editDialog).toBeVisible();
  await expect(knowledgeUI.editDialog).toHaveAttribute("data-frame", "unframed");
  await expect(knowledgeUI.editDialog.locator('[data-slot="dialog-header"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(knowledgeUI.editDialog.locator('[data-slot="dialog-footer"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await knowledgeUI.editDescription.fill("browser-edited workspace recall contract");
  await knowledgeUI.editContent.fill(editedContent);
  await knowledgeUI.confirmEditMemory.click();
  const editPayload = (await (await editResponsePromise).json()) as MemoryMutationResponse;
  expect(editPayload.decision.op).toBe("update");
  expect(JSON.stringify(editPayload)).not.toMatch(sensitivePattern);
  await expect(knowledgeUI.editDialog).toBeHidden();
  await expect(knowledgeUI.contentPreview).toContainText(editedContent);

  await expect(knowledgeUI.revertDecision(editPayload.decision.id)).toBeVisible({
    timeout: 20_000,
  });
  const revertResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/memory/decisions/${editPayload.decision.id}/revert`)
  );
  await knowledgeUI.revertDecision(editPayload.decision.id).click();
  const revertPayload = (await (await revertResponsePromise).json()) as MemoryMutationResponse;
  expect(revertPayload.reverted).toBe(true);
  expect(JSON.stringify(revertPayload)).not.toMatch(sensitivePattern);
  await expect(knowledgeUI.contentPreview).toContainText(originalContent);
  await expect(knowledgeUI.contentPreview).not.toContainText("after browser edit and revert");

  await assertKnowledgeViewportAndDialogMatrix(appPage, browserArtifacts, knowledgeUI);

  const parity = await captureKnowledgeParity(runtime, filename, workspace.id, marker);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.captureScreenshot("knowledge-workspace-detail", appPage);
  await browserArtifacts.persist(appPage);
  const routeState = await readRouteState(runtime);
  expect(routeState).toMatchObject({
    knowledge_detail_visible: true,
    knowledge_item_count: expect.any(Number),
    knowledge_scope: "workspace",
    knowledge_search_active: true,
    knowledge_view_visible: true,
  });

  const recalledSession = await createSessionThroughBrowser(appPage, memoryRecallAgentName);
  const recalledSessionUI = sessionWindowSelectors(
    sessionWindow(appPage, recalledSession.session.id)
  );
  await recalledSessionUI.composerTextarea.fill("remember me");
  await recalledSessionUI.composerTextarea.press("Enter");
  await expect(recalledSessionUI.chatView).toContainText("qa-memory acknowledged", {
    timeout: 30_000,
  });
  const recalledACPSessionID = await acpSessionIDForSession(
    runtime,
    recalledSession.session.workspace_id,
    recalledSession.session.id
  );
  const recalledPrompt = await promptForSession(
    runtime,
    memoryRecallAgentName,
    recalledACPSessionID
  );
  expect(recalledPrompt).toContain("Relevant durable memory for this turn:");
  expect(recalledPrompt).toContain("auth migration uses sessions");
  expect(recalledPrompt).toContain(marker);
  expect(recalledPrompt).toMatch(
    /<\/turn-recall>\r?\n\r?\n<user-message>\r?\nremember me\r?\n<\/user-message>/
  );
  expect(recalledPrompt).not.toMatch(sensitivePattern);
  await assertStoredUserMessageClean(
    runtime,
    recalledSession.session.workspace_id,
    recalledSession.session.id
  );

  await appPage.goto(runtime.url("/knowledge"), { waitUntil: "domcontentloaded" });
  await knowledgeUI.tabWorkspace.click();
  await expect(knowledgeUI.item(`workspace:${filename}`)).toBeVisible();
  await knowledgeUI.item(`workspace:${filename}`).click();
  await knowledgeUI.deleteButton.click();
  await expect(knowledgeUI.deleteDialog).toBeVisible();
  await expect(knowledgeUI.deleteDialog).toHaveAttribute("data-frame", "unframed");
  await expect(knowledgeUI.deleteDialog.locator('[data-slot="dialog-header"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(knowledgeUI.deleteDialog.locator('[data-slot="dialog-footer"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await appPage.getByTestId("knowledge-delete-confirm-typing").fill(filename);
  const deleteResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "DELETE" &&
      response.url().includes(`/api/memory/${encodeURIComponent(filename)}`)
  );
  await knowledgeUI.confirmDeleteMemory.click();
  const deletePayload = (await (await deleteResponsePromise).json()) as MemoryMutationResponse;
  expect(deletePayload.decision.op).toBe("delete");
  expect(JSON.stringify(deletePayload)).not.toMatch(sensitivePattern);
  await expect(knowledgeUI.item(`workspace:${filename}`)).toBeHidden({ timeout: 20_000 });

  const listAfterDelete = await runtime.requestJSON<MemoriesResponse>(
    `/api/memory?scope=workspace&workspace_id=${encodeURIComponent(workspace.id)}`
  );
  expect(listAfterDelete.memories.some(memory => memory.filename === filename)).toBe(false);
  const searchAfterDelete = await searchMemory(runtime, workspace.id, marker);
  expect(searchAfterDelete.results.some(result => result.memory.filename === filename)).toBe(false);

  const postDeleteSession = await createSessionThroughBrowser(appPage, memoryRecallAgentName);
  const postDeleteSessionUI = sessionWindowSelectors(
    sessionWindow(appPage, postDeleteSession.session.id)
  );
  await postDeleteSessionUI.composerTextarea.fill("remember me");
  await postDeleteSessionUI.composerTextarea.press("Enter");
  await expect(postDeleteSessionUI.chatView).toContainText("qa-memory acknowledged", {
    timeout: 30_000,
  });
  const postDeleteACPSessionID = await acpSessionIDForSession(
    runtime,
    postDeleteSession.session.workspace_id,
    postDeleteSession.session.id
  );
  const postDeletePrompt = await promptForSession(
    runtime,
    memoryRecallAgentName,
    postDeleteACPSessionID
  );
  expect(postDeletePrompt).not.toContain(marker);
  expect(postDeletePrompt).not.toContain("auth migration uses sessions");
  expect((await appPage.textContent("body")) ?? "").not.toMatch(sensitivePattern);
  await expect(
    readFileIfExists(runtime.artifactCollector.artifactPath("browser_route_state"))
  ).resolves.not.toMatch(sensitivePattern);
  await expect(
    readFileIfExists(runtime.artifactCollector.artifactPath("browser_api_snapshots"))
  ).resolves.not.toMatch(sensitivePattern);
});

async function createSessionThroughBrowser(
  page: Page,
  agentName: string
): Promise<SessionEnvelope> {
  await page.goto(new URL(`/agents/${agentName}`, page.url()).toString(), {
    waitUntil: "domcontentloaded",
  });
  const agentsWin = page.getByTestId("os-window-app:agents");
  await expect(agentsWin).toBeVisible();
  const agentsUI = sessionLifecycleSelectors(agentsWin);
  await expect(agentsUI.agentPageNewSession).toBeVisible();
  await agentsUI.agentPageNewSession.click();
  await expect(page.getByTestId("session-create-dialog")).toBeVisible();
  const createResponsePromise = page.waitForResponse(
    response => response.request().method() === "POST" && response.url().endsWith("/api/sessions")
  );
  await page.getByTestId("session-create-prompt").fill(SESSION_CREATE_FIRST_MESSAGE);
  await page.getByTestId("session-create-send").click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBe(true);
  const session = (await createResponse.json()) as SessionEnvelope;
  await expect
    .poll(() => new URL(page.url()).pathname)
    .toBe(`/agents/${agentName}/sessions/${session.session.id}`);
  const sessionWin = sessionWindow(page, session.session.id);
  await expect(sessionWin).toBeVisible();
  await expect(sessionWindowSelectors(sessionWin).composerTextarea).toBeVisible();
  return session;
}

async function assertKnowledgeViewportAndDialogMatrix(
  page: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  ui: ReturnType<typeof knowledgeOperatorSelectors>
): Promise<void> {
  const viewports = [
    { name: "desktop", width: 1280, height: 900 },
    { name: "tablet", width: 768, height: 900 },
    { name: "mobile", width: 375, height: 812 },
  ];
  for (const viewport of viewports) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await expect(ui.shell).toBeVisible();
    await expect(ui.listPanel).toBeVisible();
    await expect(ui.tabWorkspace).toHaveAttribute("aria-pressed", "true");
    await expect(ui.detailPanel).toBeVisible();
    await browserArtifacts.captureScreenshot(`knowledge-${viewport.name}-detail`, page);
  }

  await ui.editButton.click();
  await expect(ui.editDialog).toBeVisible();
  await browserArtifacts.captureScreenshot("knowledge-edit-dialog-responsive", page);
  await page.keyboard.press("Escape");
  await expect(ui.editDialog).toBeHidden();

  await ui.deleteButton.click();
  await expect(ui.deleteDialog).toBeVisible();
  await browserArtifacts.captureScreenshot("knowledge-delete-dialog-responsive", page);
  await page.keyboard.press("Escape");
  await expect(ui.deleteDialog).toBeHidden();

  await ui.createButton.click();
  await expect(ui.createDialog).toBeVisible();
  await browserArtifacts.captureScreenshot("knowledge-create-dialog-responsive", page);
  await ui.cancelCreateMemory.click();
  await expect(ui.createDialog).toBeHidden();
}

async function captureKnowledgeParity(
  runtime: BrowserRuntime,
  filename: string,
  workspaceID: string,
  marker: string
) {
  const httpEntry = await readMemoryHTTP(runtime, filename, workspaceID);
  const udsEntry = await requestOperatorJSONOrThrow<MemoryEntry>(
    runtime,
    memoryReadPath(filename, workspaceID)
  );
  const httpList = await runtime.requestJSON<MemoriesResponse>(
    `/api/memory?scope=workspace&workspace_id=${encodeURIComponent(workspaceID)}`
  );
  const udsList = await requestOperatorJSONOrThrow<MemoriesResponse>(
    runtime,
    `/api/memory?scope=workspace&workspace_id=${encodeURIComponent(workspaceID)}`
  );
  const httpSearch = await searchMemory(runtime, workspaceID, marker);
  const udsSearch = await requestOperatorJSONOrThrow<MemorySearchResponse>(
    runtime,
    "/api/memory/search",
    {
      method: "POST",
      body: JSON.stringify({
        query_text: marker,
        scope: "workspace",
        workspace_id: workspaceID,
        top_k: 5,
      }),
    }
  );
  const cliList = await memoryCLI<MemoriesResponse>(runtime, [
    "memory",
    "list",
    "--scope",
    "workspace",
    "--workspace",
    workspaceID,
  ]);
  const cliSearch = await memoryCLI<MemorySearchResponse>(runtime, [
    "memory",
    "search",
    marker,
    "--scope",
    "workspace",
    "--workspace",
    workspaceID,
  ]);

  expect(httpEntry.memory.content).toContain(marker);
  expect(udsEntry.memory.content).toContain(marker);
  expect(httpList.memories.some(memory => memory.filename === filename)).toBe(true);
  expect(udsList.memories.some(memory => memory.filename === filename)).toBe(true);
  expect(cliList.memories.some(memory => memory.filename === filename)).toBe(true);
  expect(httpSearch.results.some(result => result.memory.filename === filename)).toBe(true);
  expect(udsSearch.results.some(result => result.memory.filename === filename)).toBe(true);
  expect(cliSearch.results.some(result => result.memory.filename === filename)).toBe(true);

  return {
    cliList,
    cliSearch,
    httpEntry,
    httpList,
    httpSearch,
    udsEntry,
    udsList,
    udsSearch,
  };
}

async function readMemoryHTTP(
  runtime: BrowserRuntime,
  filename: string,
  workspaceID: string
): Promise<MemoryEntry> {
  return await runtime.requestJSON<MemoryEntry>(memoryReadPath(filename, workspaceID));
}

function memoryReadPath(filename: string, workspaceID: string): string {
  return `/api/memory/${encodeURIComponent(filename)}?scope=workspace&workspace_id=${encodeURIComponent(
    workspaceID
  )}`;
}

async function searchMemory(
  runtime: BrowserRuntime,
  workspaceID: string,
  queryText: string
): Promise<MemorySearchResponse> {
  return await runtime.requestJSON<MemorySearchResponse>("/api/memory/search", {
    method: "POST",
    body: JSON.stringify({
      query_text: queryText,
      scope: "workspace",
      workspace_id: workspaceID,
      top_k: 5,
    }),
  });
}

async function memoryCLI<T>(runtime: BrowserRuntime, args: string[]): Promise<T> {
  if (!runtime.paths) {
    throw new Error("memory CLI checks require launch-mode runtime paths.");
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, [...args, "-o", "json"], {
    env: cliEnv(runtime.paths),
  });
  expect(stdout).not.toMatch(sensitivePattern);
  return JSON.parse(stdout) as T;
}

async function requestOperatorJSONOrThrow<T>(
  runtime: BrowserRuntime,
  pathname: string,
  init?: RequestInit
): Promise<T> {
  if (!runtime.requestOperatorJSON) {
    throw new Error("operator UDS parity checks require requestOperatorJSON.");
  }
  return await runtime.requestOperatorJSON<T>(pathname, init);
}

async function promptDiagnostics(
  runtime: BrowserRuntime,
  agentName: string
): Promise<DiagnosticsRecord[]> {
  if (!runtime.paths) {
    throw new Error("prompt diagnostics require launch-mode runtime paths.");
  }
  const diagnosticsPath = path.join(runtime.paths.homeDir, "logs", "acpmock", `${agentName}.jsonl`);
  await expect.poll(() => readFileIfExists(diagnosticsPath)).not.toBe("");
  const text = await readFile(diagnosticsPath, "utf8");
  const lines = text
    .split("\n")
    .map(line => line.trim())
    .filter(Boolean);
  return lines
    .flatMap((line, index) => {
      try {
        return [JSON.parse(line) as DiagnosticsRecord];
      } catch (error) {
        if (index === lines.length - 1) {
          return [];
        }
        throw error;
      }
    })
    .filter(record => !record.lifecycle_event && record.prompt_index > 0);
}

async function promptForSession(
  runtime: BrowserRuntime,
  agentName: string,
  acpSessionID: string
): Promise<string> {
  let prompt = "";
  await expect
    .poll(async () => {
      const records = await promptDiagnostics(runtime, agentName);
      const record = [...records]
        .reverse()
        .find(candidate => candidate.session_id === acpSessionID);
      prompt = record?.prompt ?? "";
      return prompt !== "";
    })
    .toBe(true);
  return prompt;
}

async function acpSessionIDForSession(
  runtime: BrowserRuntime,
  workspaceID: string,
  sessionID: string
): Promise<string> {
  let acpSessionID = "";
  await expect
    .poll(async () => {
      const detail = await runtime.requestJSON<SessionEnvelope>(
        sessionAPIPath(workspaceID, sessionID)
      );
      acpSessionID = detail.session.acp_session_id ?? "";
      return acpSessionID !== "";
    })
    .toBe(true);
  return acpSessionID;
}

async function assertStoredUserMessageClean(
  runtime: BrowserRuntime,
  workspaceID: string,
  sessionID: string
): Promise<void> {
  let userMessage: TranscriptMessage | undefined;
  await expect
    .poll(async () => {
      const transcript = await runtime.requestJSON<{
        entries: Array<{ message: TranscriptMessage }>;
      }>(sessionAPIPath(workspaceID, sessionID, "/transcript"));
      userMessage = transcript.entries
        .map(entry => entry.message)
        .find(message => message.role === "user");
      return transcriptMessageText(userMessage);
    })
    .toBe("remember me");
  const serialized = JSON.stringify(userMessage ?? {});
  expect(serialized).not.toContain("Relevant durable memory for this turn:");
  expect(serialized).not.toMatch(sensitivePattern);
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

function transcriptMessageText(message: TranscriptMessage | undefined): string {
  if (!message?.parts) {
    return "";
  }
  return message.parts
    .filter(part => part.type === "text")
    .map(part => part.text ?? "")
    .join("");
}

async function readRouteState(runtime: BrowserRuntime): Promise<Record<string, unknown>> {
  return JSON.parse(
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8")
  ) as Record<string, unknown>;
}

async function readFileIfExists(filePath: string): Promise<string> {
  try {
    return await readFile(filePath, "utf8");
  } catch (error) {
    const maybeNodeError = error as NodeJS.ErrnoException;
    if (maybeNodeError.code === "ENOENT") {
      return "";
    }
    throw error;
  }
}

function cliEnv(paths: { cliShim: string; homeDir: string }): NodeJS.ProcessEnv {
  return {
    ...process.env,
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}
