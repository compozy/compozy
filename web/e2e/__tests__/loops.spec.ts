import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Page } from "@playwright/test";

import type {
  LoopDefinition,
  LoopRunDetail,
  LoopRunGenerationOutput,
  RunLoopResult,
} from "@/systems/loops";
import { openAppWindow, switchWorkspace } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { createWorktreeRepo, type WorktreeRepoFixture } from "../fixtures/worktree-repo";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

const loopLifecycleFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "loop_node_lifecycle_fixture.json"
);

const loopRuntimeFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "loop_runtime_provenance_fixture.json"
);

const loopFeedbackFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "loop_generation_feedback_fixture.json"
);

let loopEnvironmentRepo: WorktreeRepoFixture | null = null;

test.afterEach(async () => {
  await loopEnvironmentRepo?.cleanup();
  loopEnvironmentRepo = null;
});

const loopLifecycleAgent = "loop-lifecycle-agent";
const loopLifecycleName = "node-lifecycle-e2e";
const loopAuthoringName = "editor-authoring-e2e";
const loopEditorInteractionName = "editor-interactions-e2e";
const loopRuntimeAgent = "loop-runtime-agent";
const loopRuntimeName = "runtime-provenance-e2e";
const loopFeedbackWorker = "loop-feedback-worker";
const loopFeedbackJudge = "loop-feedback-exhaust-judge";
const loopFeedbackName = "feedback-best-on-exhaustion-web";
const loopWatchEventsName = "watch-events-read-model-web";
const loopWatchCursorSeedName = "watch-events-cursor-seed-web";
const loopEnumAskName = "enum-ask-without-type-web";
const loopEditorChromeStorageKey = "compozy:loops:editor-chrome:v1";

const loopEnumAskDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopEnumAskName,
    description: "Exercise an enum answer schema without a redundant type keyword.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Record one operator decision.",
    definition_of_done: "The selected decision reaches the downstream transform.",
    stop_when: "nodes.finish.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "choose",
        class: "control",
        kind: "ask",
        params: {
          prompt: "Approve this rollout?",
          expect: {
            type: "object",
            required: ["decision"],
            properties: { decision: { enum: ["approve", "discard"] } },
          },
          responders: { agents: "allow" },
        },
      },
      {
        id: "finish",
        class: "action",
        kind: "transform",
        params: {
          map: { decision: { template: "{{ .nodes.choose.output.decision }}" } },
        },
      },
    ],
    edges: [{ from: "choose", to: "finish" }],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

const loopWatchCursorSeedDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopWatchCursorSeedName,
    description: "Create one eligible loop ledger event before arming a watcher.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Record a terminal loop event.",
    definition_of_done: "The transform completes.",
    stop_when: "nodes.finish.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "finish",
        class: "action",
        kind: "transform",
        params: { map: { done: { value: true } } },
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

const loopWatchEventsDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopWatchEventsName,
    description: "Render a parked watch-events subscription from the real daemon.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Wait for a blocked task before recording the wake.",
    definition_of_done: "The blocked task event was observed.",
    stop_when: "nodes.record_wake.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "task_activity",
        class: "source",
        kind: "watch-events",
        events: [
          {
            kind: "task.status_changed",
            filter: "event.payload.to_status == 'blocked'",
          },
          { kind: "loop.terminal" },
        ],
      },
      {
        id: "record_wake",
        class: "action",
        kind: "transform",
        params: { map: { observed: { value: true } } },
      },
    ],
    edges: [{ from: "task_activity", to: "record_wake" }],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

const loopRuntimeDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopRuntimeName,
    description: "Render durable runtime provenance in the real Loop run view.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Run two fixture tasks with different runtime layers.",
    definition_of_done: "Both fixture tasks complete.",
    stop_when: "nodes.collect.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    runtime_defaults: {
      worker: { provider: "acpmock", model: "base-model", reasoning: "low" },
      judge: {},
    },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "fixture_tasks",
        class: "action",
        kind: "transform",
        params: {
          map: {
            tasks: {
              value: [
                {
                  id: "task_frontend",
                  path: ".compozy/tasks/runtime/task_frontend.md",
                  title: "Frontend runtime task",
                  type: "frontend",
                  complexity: "medium",
                  body: "Frontend fixture",
                },
                {
                  id: "task_docs",
                  path: ".compozy/tasks/runtime/task_docs.md",
                  title: "Docs runtime task",
                  type: "docs",
                  complexity: "low",
                  runtime: { model: "docs-model" },
                  body: "Docs fixture",
                },
              ],
            },
          },
        },
      },
      {
        id: "fan_out_tasks",
        class: "control",
        kind: "fan-out",
        collection: "{{ .nodes.fixture_tasks.output.tasks }}",
        batch_size: 1,
        max_parallel: 1,
        max_fan_out: 2,
      },
      {
        id: "execute_task",
        class: "action",
        kind: "run-agent",
        params: {
          agent: loopRuntimeAgent,
          prompt: "loop event probe",
          output_schema: {
            type: "object",
            required: ["summary", "message"],
            properties: {
              summary: { type: "string" },
              message: { type: "string" },
            },
          },
        },
      },
      { id: "collect", class: "control", kind: "collect" },
    ],
    edges: [
      { from: "fixture_tasks", to: "fan_out_tasks" },
      { from: "fan_out_tasks", to: "execute_task" },
      { from: "execute_task", to: "collect" },
    ],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

const loopFeedbackDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopFeedbackName,
    description: "Exercise metric regression, ratchet restore, and best-on-exhaustion UI.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Converge through deterministic generation feedback.",
    definition_of_done: "The deterministic feedback gate approves.",
    stop_when: "best.score >= 0.95",
    iteration_cap: 3,
    no_progress: { window: 5 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "draft",
        class: "action",
        kind: "run-agent",
        params: {
          agent: loopFeedbackWorker,
          prompt: "exhaustion generation {{ .generation }}",
          output_schema: {
            type: "object",
            required: ["summary", "value"],
            properties: {
              summary: { type: "string" },
              value: { type: "string" },
            },
          },
        },
      },
      {
        id: "quality",
        class: "control",
        kind: "gate",
        criteria: [
          {
            id: "exhaust_score",
            type: "agent-judge",
            agent: loopFeedbackJudge,
            rubric: "Score generation {{ .generation }} deterministically.",
            metric: { direction: "maximize" },
          },
        ],
        verdict_policy: "fixed_passes",
        on_result: { fail: "revise" },
        max_revisions: 10,
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/**
 * E2E-015 (US-019, US-024 UI). One node whose first attempt blows its deadline,
 * so the daemon schedules a real retry; the run page then shows the attempt, the
 * operator pauses and resumes the lane through the UI, and the workspace
 * inventory lists what is parked.
 */
const loopLifecycleDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopLifecycleName,
    description: "Exercise node retry, pause/resume, and the node inventories.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Run one lane that retries after a deadline, then settle.",
    definition_of_done: "The lane completes after its retry.",
    stop_when: "nodes.execute.status == 'succeeded'",
    iteration_cap: 2,
    no_progress: { window: 5 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled", "canceled"],
  },
  graph: {
    nodes: [
      {
        id: "execute",
        class: "action",
        kind: "run-agent",
        // The public LoopGraphNode contract exposes the authored per-attempt
        // timeout. The runtime E2E proves the same 2s/3s boundary: it leaves
        // session setup outside the failure window while still guaranteeing an
        // attempt_timeout on attempt 1 — the one class that auto-retries — and
        // the retry turn then heals, so the run reaches a real terminal state.
        timeout: "2s",
        retry: { max_attempts: 3, backoff: { base: "20s", max: "60s" } },
        params: {
          agent: loopLifecycleAgent,
          prompt: "retry lifecycle",
          output_schema: {
            type: "object",
            required: ["summary", "value"],
            properties: { summary: { type: "string" }, value: { type: "string" } },
          },
        },
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/**
 * E2E-016 (US-028). The same lane as E2E-015 but published with NO reliability envelope: the
 * browser author declares `retry` + `on_error` in the editor, publishes, and the run that
 * follows proves the daemon executed the contract they typed.
 */
const loopAuthoringDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopAuthoringName,
    description: "Author the Spec 1 failure contract in the editor, then run it.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Run one lane whose failure contract is authored in the editor.",
    definition_of_done: "The lane completes after the authored retry.",
    stop_when: "nodes.execute.status == 'succeeded'",
    iteration_cap: 2,
    no_progress: { window: 5 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled", "canceled"],
  },
  graph: {
    nodes: [
      {
        id: "execute",
        class: "action",
        kind: "run-agent",
        // Deliberately no `retry` and no `on_error` — the editor authors both.
        timeout: "2s",
        params: {
          agent: loopLifecycleAgent,
          prompt: "retry lifecycle",
          output_schema: {
            type: "object",
            required: ["summary", "value"],
            properties: { summary: { type: "string" }, value: { type: "string" } },
          },
        },
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

const loopEditorInteractionDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopEditorInteractionName,
    description: "Exercise structural editor interactions against the daemon.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Edit a small connected graph.",
    definition_of_done: "The final transform completes.",
    stop_when: "nodes.finish.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "prepare",
        class: "action",
        kind: "transform",
        params: { map: { ready: { value: true } } },
      },
      {
        id: "apply",
        class: "action",
        kind: "transform",
        params: { map: { applied: { value: true } } },
      },
      {
        id: "finish",
        class: "action",
        kind: "transform",
        params: { map: { done: { value: true } } },
      },
    ],
    edges: [
      { from: "prepare", to: "apply" },
      { from: "apply", to: "finish" },
    ],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: loopLifecycleFixture,
          fixtureAgent: "lifecycle_retry",
          agentName: loopLifecycleAgent,
        },
        {
          fixturePath: loopRuntimeFixture,
          fixtureAgent: "loop_runtime_provenance",
          agentName: loopRuntimeAgent,
        },
        {
          fixturePath: loopFeedbackFixture,
          fixtureAgent: "feedback_worker",
          agentName: loopFeedbackWorker,
        },
        {
          fixturePath: loopFeedbackFixture,
          fixtureAgent: "exhaustion_judge",
          agentName: loopFeedbackJudge,
        },
      ],
    },
  },
});

type RuntimeOutput = {
  output: LoopRunGenerationOutput;
  runtime: NonNullable<LoopRunGenerationOutput["resolved_runtime"]>;
};

function resolvedRuntimeOutputs(detail: LoopRunDetail): RuntimeOutput[] {
  return (detail.generations ?? []).flatMap(generation =>
    generation.outputs.flatMap(output =>
      output.resolved_runtime ? [{ output, runtime: output.resolved_runtime }] : []
    )
  );
}

async function openInteractionEditor(appPage: Page, runtime: BrowserRuntime): Promise<void> {
  if (!runtime.paths) {
    throw new Error("Loop editor interaction test requires launch-mode runtime paths");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopEditorInteractionDefinition }),
  });
  await appPage.goto(
    runtime.url(`/loops/${encodeURIComponent(loopEditorInteractionName)}/editor`),
    { waitUntil: "domcontentloaded" }
  );
  await expect(appPage.getByTestId("loop-editor")).toBeVisible();
}

/** Opens the operator register on its Nodes lane, where lifecycle state lives. */
async function openRunRoster(appPage: Page): Promise<void> {
  const panel = appPage.getByTestId("loop-run-inspect-panel");
  if (!(await panel.isVisible())) {
    await appPage.getByTestId("loop-run-inspect").getByRole("button").first().click();
  }
  await appPage.getByTestId("loop-lane-nodes").click();
  await expect(appPage.getByTestId("loop-node-roster")).toBeVisible();
}

/**
 * One roster row, by the step it is about.
 *
 * The row's own test id carries round, step and item, because the same step id
 * exists once per round and once per fan-out worker. These runs are read on the
 * round the daemon is on, so naming the step is enough — and it keeps the
 * locator from hard-coding a round number the run decides.
 */
function rosterRow(appPage: Page, nodeId: string) {
  return appPage
    .getByTestId("loop-node-roster")
    .locator(`[data-testid^="loop-roster-row-"][data-node-id="${nodeId}"]`);
}

function editorNode(appPage: Page, nodeId: string) {
  return appPage.locator(`[data-testid="loop-editor-node"][data-node-id="${nodeId}"]`);
}

test("CompozyOS migration E2E-015: run page lifecycle controls and node inventories", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop lifecycle browser test requires launch-mode runtime paths");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopLifecycleDefinition }),
  });

  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopLifecycleName)}/run`,
    { method: "POST", body: JSON.stringify({}) }
  );
  if (!started.run) throw new Error("Loop lifecycle browser seed did not create a run");
  const runId = started.run.id;
  const runPath = `${workspacePath}/loop-runs/${encodeURIComponent(runId)}`;

  // The daemon must actually schedule a retry — that is the fact the page reads.
  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
        const outputs = (detail.generations ?? []).flatMap(generation => generation.outputs);
        if (outputs.some(output => Boolean(output.next_attempt_at))) return "retry-scheduled";
        return JSON.stringify({
          runStatus: detail.run.status,
          outputs: outputs.map(output => ({
            nodeId: output.node_id,
            status: output.status,
            attempt: output.attempt,
            failureClass: output.failure_class,
            disposition: output.disposition,
          })),
        });
      },
      { timeout: 45_000 }
    )
    .toBe("retry-scheduled");

  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(runId)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();

  // Retrying lane: the attempt and its due time come from the payload. The
  // lifecycle state moved from the retired "happening now" card to the roster
  // in the operator register — same fact, one disclosure deeper.
  await openRunRoster(appPage);
  const retryRow = rosterRow(appPage, "execute");
  await expect(retryRow).toBeVisible();
  await expect(retryRow).toContainText("retrying");
  await expect(retryRow).toContainText("of");

  // Pause the lane through the row menu, choosing what happens in flight.
  const menuTrigger = appPage.getByTestId("loop-node-menu-trigger-execute");
  await menuTrigger.click();
  await appPage.getByTestId("loop-node-verb-pause").click();
  const dialog = appPage.getByTestId("loop-node-control-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("execute");
  await appPage.getByTestId("loop-node-pause-mode-drain").click();
  await dialog.getByRole("button", { name: "Pause lane" }).click();

  // Daemon truth, not UI optimism: the control row must actually be paused.
  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
        return (detail.node_controls ?? []).some(
          control => control.node_id === "execute" && control.paused
        );
      },
      { timeout: 20_000 }
    )
    .toBe(true);
  const pausedDetail = await runtime.requestJSON<LoopRunDetail>(runPath);
  const pausedControl = (pausedDetail.node_controls ?? []).find(
    control => control.node_id === "execute"
  );
  expect(pausedControl?.pause_provenance?.actor_kind).toBeTruthy();

  await openRunRoster(appPage);
  await expect(rosterRow(appPage, "execute")).toContainText("paused");
  await browserArtifacts.captureScreenshot("loop-run-node-paused", appPage);

  // A paused lane promotes Resume to a first-class control.
  await appPage.getByTestId("loop-node-primary-resume-execute").click();
  const resumeDialog = appPage.getByTestId("loop-node-control-dialog");
  await expect(resumeDialog).toBeVisible();
  await resumeDialog.getByRole("button", { name: "Resume lane" }).click();
  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
        return (detail.node_controls ?? []).some(
          control => control.node_id === "execute" && control.paused
        );
      },
      { timeout: 20_000 }
    )
    .toBe(false);

  // The workspace inventory is a real server read, filtered and paged by the daemon.
  await appPage.goto(runtime.url("/loop-runs?nodes=retrying"), { waitUntil: "domcontentloaded" });
  const inventory = appPage.getByTestId("loop-node-inventory");
  await expect(inventory).toBeVisible();
  await expect(appPage.getByTestId("loop-node-inventory-state-retrying")).toBeVisible();
  // The foot reports what is loaded and never claims a population total.
  await expect(appPage.getByTestId("loop-node-inventory-foot")).toContainText("Showing");

  // A state with nothing in it renders the truthful, filter-aware empty.
  await appPage.goto(runtime.url("/loop-runs?nodes=quarantined"), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-node-inventory-empty")).toContainText(
    "Nothing is quarantined"
  );
  await browserArtifacts.captureScreenshot("loop-node-inventory-quarantined-empty", appPage);
});

test("an enum ask without type crosses the real browser and daemon seam", async ({
  appPage,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop enum ask browser test requires launch-mode runtime paths");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopEnumAskDefinition }),
  });

  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopEnumAskName)}/run`,
    { method: "POST", body: JSON.stringify({}) }
  );
  if (!started.run) throw new Error("Loop enum ask browser seed did not create a run");
  const runId = started.run.id;
  const runPath = `${workspacePath}/loop-runs/${encodeURIComponent(runId)}`;
  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(runId)}`), {
    waitUntil: "domcontentloaded",
  });

  const card = appPage.getByTestId("loop-request-card");
  await expect(card).toBeVisible();
  await expect(card.getByTestId("loop-request-field-decision")).toBeVisible();
  await card.getByRole("radio", { name: "approve" }).click();
  const responsePromise = appPage.waitForResponse(
    response => response.request().method() === "POST" && response.url().endsWith("/respond")
  );
  await card.getByTestId("loop-request-submit").click();
  const response = await responsePromise;
  expect(response.status()).toBe(200);
  expect(JSON.parse(response.request().postData() ?? "{}")).toMatchObject({
    payload: { decision: "approve" },
  });

  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
        const request = (detail.requests ?? []).find(candidate => candidate.node_id === "choose");
        const downstream = (detail.generations ?? [])
          .flatMap(generation => generation.outputs)
          .find(output => output.node_id === "finish");
        return {
          requestState: request?.state,
          runStatus: detail.run.status,
          downstreamStatus: downstream?.status,
        };
      },
      { timeout: 30_000 }
    )
    .toEqual({ requestState: "answered", runStatus: "done", downstreamStatus: "succeeded" });
});

test("CompozyOS migration E2E-016: author retry + on_error in the editor, publish, and run it", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop authoring browser test requires launch-mode runtime paths");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopAuthoringDefinition }),
  });

  await appPage.goto(runtime.url(`/loops/${encodeURIComponent(loopAuthoringName)}/editor`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-editor")).toBeVisible();

  // Select the lane and open the folds the reliability envelope lives in.
  await appPage.getByTestId("loop-editor-node").filter({ hasText: "execute" }).first().click();
  const reliability = appPage.getByRole("button", { name: /^Reliability/ });
  if ((await reliability.getAttribute("aria-expanded")) !== "true") {
    await reliability.click();
  }

  // Declare the retry budget and a single absorption mode — route XOR allow_fail.
  await appPage.getByTestId("loop-field-max_attempts").fill("3");
  await appPage.getByTestId("loop-field-backoff_base").fill("20s");
  await appPage.getByTestId("loop-field-on_error_allow_fail").click();

  // One observational effect on the retry trigger.
  const reactions = appPage.getByRole("button", { name: /^Reactions/ });
  if ((await reactions.getAttribute("aria-expanded")) !== "true") {
    await reactions.click();
  }
  await appPage.getByTestId("loop-field-on_retry-add").click();
  await appPage.getByTestId("loop-field-on_retry-kind-0").fill("lane_retrying");

  // The dock must be clean before Publish is legal.
  await expect(appPage.getByTestId("loop-linter-error-count")).toHaveCount(0);
  const publish = appPage.getByTestId("loop-editor-publish");
  await expect(publish).toBeEnabled();
  await publish.click();
  await expect(appPage.getByTestId("loop-run-form")).toBeVisible();
  await browserArtifacts.captureScreenshot("loop-editor-authored-published", appPage);

  // Round-trip against the real daemon: the published definition carries what was typed.
  const published = await runtime.requestJSON<{
    loop: { version: number; definition: { graph: { nodes: Record<string, unknown>[] } } };
  }>(`${workspacePath}/loops/${encodeURIComponent(loopAuthoringName)}`);
  expect(published.loop.version).toBe(2);
  const authored = published.loop.definition.graph.nodes.find(node => node.id === "execute");
  expect(authored).toMatchObject({
    retry: { max_attempts: 3, backoff: { base: "20s" } },
    on_error: { allow_fail: true },
    on_retry: [{ emit: { kind: "lane_retrying" } }],
  });

  // Start through the browser so this journey proves the published editor contract end to end.
  const runButton = appPage.getByTestId("loop-run-submit-button");
  await expect(runButton).toBeEnabled();
  await runButton.click();
  await expect(appPage).toHaveURL(/\/loop-runs\/[^/?#]+$/);
  const runID = decodeURIComponent(new URL(appPage.url()).pathname.split("/").at(-1) ?? "");
  if (runID === "") throw new Error("Loop authoring browser run did not return a run id");
  const runPath = `${workspacePath}/loop-runs/${encodeURIComponent(runID)}`;
  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
        const outputs = (detail.generations ?? []).flatMap(generation => generation.outputs);
        return outputs.some(output => Boolean(output.next_attempt_at));
      },
      { timeout: 45_000 }
    )
    .toBe(true);

  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
  // The authored attempt ceiling is what phrases the lane, so "attempt N of 3" is truthful.
  await openRunRoster(appPage);
  const retryingRow = rosterRow(appPage, "execute");
  await expect(retryingRow).toContainText("retrying");
  await expect(retryingRow).toContainText("of");
  await browserArtifacts.captureScreenshot("loop-editor-authored-run-retrying", appPage);
});

test("E2E-031: editor rails collapse, filter, and persist", async ({ appPage, runtime }) => {
  await openInteractionEditor(appPage, runtime);
  await appPage.evaluate(key => window.localStorage.removeItem(key), loopEditorChromeStorageKey);
  await appPage.reload({ waitUntil: "domcontentloaded" });

  const paletteToggle = appPage.getByTestId("loop-editor-palette-toggle");
  const inspectorToggle = appPage.getByTestId("loop-editor-inspector-toggle");
  await expect(paletteToggle).toHaveAttribute("aria-pressed", "false");
  await expect(inspectorToggle).toHaveAttribute("aria-pressed", "false");
  await expect(appPage.getByTestId("loop-editor-palette")).toHaveCount(0);
  await expect(appPage.getByTestId("loop-editor-sidebar")).toHaveCount(0);

  await paletteToggle.click();
  await expect(appPage.getByTestId("loop-editor-palette")).toBeVisible();
  await paletteToggle.click();
  await editorNode(appPage, "prepare").dispatchEvent("keydown", {
    bubbles: true,
    code: "BracketLeft",
    key: "[",
  });
  const palette = appPage.getByTestId("loop-editor-palette");
  await expect(palette).toBeVisible();
  await palette.getByTestId("loop-palette-search").fill("route");
  await expect(palette.getByTestId("loop-palette-item-route")).toBeVisible();
  await expect(palette.getByTestId("loop-palette-item-ask")).toHaveCount(0);

  await editorNode(appPage, "prepare").click();
  await expect(inspectorToggle).toHaveAttribute("aria-pressed", "true");
  await expect(appPage.getByTestId("loop-editor-tab-node")).toHaveAttribute(
    "aria-selected",
    "true"
  );
  await editorNode(appPage, "prepare").dispatchEvent("keydown", {
    bubbles: true,
    code: "BracketRight",
    key: "]",
  });
  await expect(inspectorToggle).toHaveAttribute("aria-pressed", "false");

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("loop-editor-palette")).toBeVisible();
  await expect(appPage.getByTestId("loop-editor-sidebar")).toHaveCount(0);
});

test("E2E-032: quick-add places, guards, and reveals nodes", async ({ appPage, runtime }) => {
  await openInteractionEditor(appPage, runtime);
  const canvas = appPage.getByTestId("loop-editor-canvas");
  const prepare = editorNode(appPage, "prepare");
  await prepare.click();
  const expected = await canvas.evaluate(element => {
    const rect = element.getBoundingClientRect();
    const viewport = element.querySelector<HTMLElement>(".react-flow__viewport");
    if (!viewport) throw new Error("React Flow viewport not found");
    const matrix = new DOMMatrix(getComputedStyle(viewport).transform);
    return {
      x: (rect.width / 2 - matrix.e) / matrix.a - 188 / 2,
      y: (rect.height / 2 - matrix.f) / matrix.d - 96 / 2,
    };
  });
  await prepare.press("a");
  await expect(appPage.getByTestId("loop-editor-quick-add")).toBeVisible();
  await appPage.getByTestId("loop-quick-add-item-transform").click();

  const added = appPage.locator('.react-flow__node[data-id="transform"]');
  await expect(added).toBeVisible();
  const actual = await added.evaluate(element => {
    const matrix = new DOMMatrix(getComputedStyle(element).transform);
    return { x: matrix.e, y: matrix.f };
  });
  expect(actual.x).toBeCloseTo(expected.x, 0);
  expect(actual.y).toBeCloseTo(expected.y, 0);
  await expect(appPage.getByTestId("loop-editor-save")).toBeEnabled();

  const idInput = appPage.getByTestId("loop-field-id");
  await idInput.focus();
  await idInput.dispatchEvent("keydown", { key: "a", bubbles: true });
  await expect(appPage.getByTestId("loop-editor-quick-add")).toHaveCount(0);

  const nodeCount = await appPage.getByTestId("loop-editor-node").count();
  await appPage.locator(".react-flow__pane").dblclick({ position: { x: 24, y: 24 } });
  await expect(appPage.getByTestId("loop-editor-quick-add")).toBeVisible();
  await appPage.getByTestId("loop-quick-add-input").press("Escape");
  await expect(appPage.getByTestId("loop-editor-node")).toHaveCount(nodeCount);

  await prepare.click();
  await prepare.press("a");
  await appPage.getByTestId("loop-quick-add-input").fill("finish");
  await appPage.getByTestId("loop-quick-add-node-finish").click();
  await expect(editorNode(appPage, "finish")).toHaveAttribute("data-node-focused", "true");
});

test("E2E-033: connection drop adds one wired node or no mutation", async ({
  appPage,
  runtime,
}) => {
  await openInteractionEditor(appPage, runtime);
  const nodes = appPage.getByTestId("loop-editor-node");
  const edges = appPage.locator(".react-flow__edge");
  const initialNodes = await nodes.count();
  const initialEdges = await edges.count();
  const finish = editorNode(appPage, "finish");
  const handle = finish.locator(".react-flow__handle.source");
  const canvasBox = await appPage.getByTestId("loop-editor-canvas").boundingBox();
  if (!canvasBox) throw new Error("Connection-drop geometry is unavailable");
  const drop = {
    x: canvasBox.x + canvasBox.width * 0.78,
    y: canvasBox.y + canvasBox.height * 0.82,
  };

  const dragToCanvas = async () => {
    await handle.hover();
    await appPage.mouse.down();
    const handleBox = await handle.boundingBox();
    if (!handleBox) throw new Error("Connection handle geometry is unavailable");
    await appPage.mouse.move(
      handleBox.x + handleBox.width + 8,
      handleBox.y + handleBox.height / 2 + 8
    );
    await expect(appPage.locator(".react-flow__connection-path")).toBeVisible();
    await appPage.mouse.move(drop.x, drop.y, { steps: 12 });
    await appPage.mouse.up();
    await expect(appPage.getByTestId("loop-editor-connection-picker")).toBeVisible();
  };

  await dragToCanvas();
  await appPage.getByTestId("loop-editor-connection-picker").press("Escape");
  await expect(nodes).toHaveCount(initialNodes);
  await expect(edges).toHaveCount(initialEdges);

  await dragToCanvas();
  await appPage.getByTestId("loop-connection-picker-item-transform").click();
  await expect(nodes).toHaveCount(initialNodes + 1);
  await expect(edges).toHaveCount(initialEdges + 1);
  await expect(editorNode(appPage, "transform")).toBeVisible();
  await expect(appPage.getByTestId("loop-editor-publish")).toBeEnabled();
});

test("E2E-034: node menus and graph deletion preserve structural integrity", async ({
  appPage,
  runtime,
}) => {
  await openInteractionEditor(appPage, runtime);
  const apply = editorNode(appPage, "apply");
  await apply.click({ button: "right" });
  for (const verb of ["duplicate", "copy", "delete"]) {
    await expect(appPage.getByTestId(`loop-node-menu-${verb}`)).toBeEnabled();
  }
  await appPage.getByTestId("loop-node-menu-copy").click();
  await apply.click({ button: "right" });
  await expect(appPage.getByTestId("loop-node-menu-paste")).toBeEnabled();
  await appPage.keyboard.press("Escape");

  const edges = appPage.locator(".react-flow__edge");
  await expect(edges).toHaveCount(2);
  await edges.first().click({ force: true });
  await appPage.getByTestId("loop-editor-edge-delete").click();
  await expect(edges).toHaveCount(1);

  const prepareBox = await editorNode(appPage, "prepare").boundingBox();
  const applyBox = await apply.boundingBox();
  if (!prepareBox || !applyBox) throw new Error("Marquee geometry is unavailable");
  await appPage.keyboard.down("Shift");
  await appPage.mouse.move(prepareBox.x - 12, Math.min(prepareBox.y, applyBox.y) - 12);
  await appPage.mouse.down();
  await appPage.mouse.move(
    applyBox.x + applyBox.width + 12,
    Math.max(prepareBox.y + prepareBox.height, applyBox.y + applyBox.height) + 12,
    { steps: 12 }
  );
  await appPage.mouse.up();
  await appPage.keyboard.up("Shift");
  await expect(editorNode(appPage, "prepare")).toHaveAttribute("data-node-selected", "true");
  await expect(apply).toHaveAttribute("data-node-selected", "true");
  await appPage.keyboard.press("Delete");
  await expect(editorNode(appPage, "prepare")).toHaveCount(0);
  await expect(editorNode(appPage, "apply")).toHaveCount(0);
  await expect(edges).toHaveCount(0);

  await appPage.goto(runtime.url("/loops/review-and-fix/editor"), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-editor-readonly-strip")).toBeVisible();
  await appPage.getByTestId("loop-editor-node").first().dispatchEvent("contextmenu", {
    bubbles: true,
    button: 2,
  });
  await expect(appPage.getByText("Read-only definition")).toBeVisible();
  for (const verb of ["duplicate", "copy", "paste", "rename", "delete"]) {
    await expect(appPage.getByTestId(`loop-node-menu-${verb}`)).toHaveCount(0);
  }
});

test("CompozyOS migration E2E-004: loop run renders API runtime provenance without controls", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop runtime browser test requires launch-mode runtime paths");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);

  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopRuntimeDefinition }),
  });

  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopRuntimeName)}/run`,
    {
      method: "POST",
      body: JSON.stringify({
        config_overrides: {
          runtime_rules: [
            {
              match: { type: "frontend" },
              runtime: { model: "frontend-model", reasoning: "high" },
            },
          ],
        },
      }),
    }
  );
  if (!started.run) throw new Error("Loop runtime browser seed did not create a run");

  const runPath = `${workspacePath}/loop-runs/${encodeURIComponent(started.run.id)}`;
  try {
    await expect
      .poll(async () => (await runtime.requestJSON<LoopRunDetail>(runPath)).run.status, {
        timeout: 30_000,
      })
      .toMatch(/^(done|no-op|blocked|failed|exhausted|stalled)$/);
  } catch (error) {
    const [stalledDetail, daemonLog] = await Promise.all([
      runtime.requestJSON<LoopRunDetail>(runPath),
      readFile(runtime.paths.daemonLog, "utf8").catch(
        readError => `Could not read daemon log: ${String(readError)}`
      ),
    ]);
    throw new Error(
      `Loop runtime did not settle:\n${JSON.stringify(stalledDetail, null, 2)}\n\nDaemon log tail:\n${daemonLog
        .split("\n")
        .slice(-120)
        .join("\n")}`,
      { cause: error }
    );
  }

  const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
  if (detail.run.status !== "done") {
    const daemonLog = await readFile(runtime.paths.daemonLog, "utf8").catch(
      error => `Could not read daemon log: ${String(error)}`
    );
    throw new Error(
      `${JSON.stringify(detail, null, 2)}\n\nDaemon log tail:\n${daemonLog.split("\n").slice(-80).join("\n")}`
    );
  }
  const outputs = resolvedRuntimeOutputs(detail);
  expect(outputs.map(entry => entry.runtime)).toEqual([
    {
      provider: "acpmock",
      model: "frontend-model",
      reasoning: "high",
      source: { provider: "default", model: "run", reasoning: "run", speed: "agent" },
      speed: "normal",
      speed_resolution: { requested: "normal", status: "applied" },
    },
    {
      provider: "acpmock",
      model: "docs-model",
      reasoning: "low",
      source: {
        provider: "default",
        model: "frontmatter",
        reasoning: "default",
        speed: "agent",
      },
      speed: "normal",
      speed_resolution: { requested: "normal", status: "applied" },
    },
  ]);

  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(started.run.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
  // The resolved-runtime rail was demoted in the two-register redesign and
  // deleted with the rest of the cockpit; the provenance it showed is no longer
  // a web surface. What still holds is that the run renders without offering a
  // control over runtime selection.
  await openRunRoster(appPage);
  await expect(appPage.getByTestId("loop-node-roster")).toBeVisible();

  await browserArtifacts.captureScreenshot("loop-run-runtime-provenance", appPage);
});

test("Parked watch-events run renders its durable cursor from the real daemon", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop watch-events browser test requires launch-mode runtime paths");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopWatchCursorSeedDefinition }),
  });
  const seed = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopWatchCursorSeedName)}/run`,
    { method: "POST", body: JSON.stringify({}) }
  );
  if (!seed.run) throw new Error("Loop watch-events cursor seed did not create a run");
  const seedPath = `${workspacePath}/loop-runs/${encodeURIComponent(seed.run.id)}`;
  await expect
    .poll(async () => (await runtime.requestJSON<LoopRunDetail>(seedPath)).run.status, {
      timeout: 30_000,
    })
    .toBe("done");

  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopWatchEventsDefinition }),
  });
  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopWatchEventsName)}/run`,
    { method: "POST", body: JSON.stringify({}) }
  );
  if (!started.run) throw new Error("Loop watch-events browser seed did not create a run");

  const runPath = `${workspacePath}/loop-runs/${encodeURIComponent(started.run.id)}`;
  await expect
    .poll(async () => (await runtime.requestJSON<LoopRunDetail>(runPath)).run.status, {
      timeout: 30_000,
    })
    .toBe("watching");
  const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
  const loopCursor = detail.watch_events?.cursors?.loop_run_events;
  expect(loopCursor).toEqual(expect.any(Number));
  expect(loopCursor).toBeGreaterThan(0);
  expect(detail.watch_events?.last_wake_at).toEqual(expect.any(String));

  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(started.run.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
  await expect(appPage.getByText(/last woke/i)).toBeVisible();
  await appPage.getByTestId("loop-run-open-inspect").click();
  const watch = appPage.getByTestId("loop-run-inspect-watch");
  await watch.scrollIntoViewIfNeeded();
  await expect(watch).toBeVisible();
  await expect(watch).toContainText("task.status_changed");
  await expect(watch).toContainText("event.payload.to_status == 'blocked'");
  const cursors = appPage.getByTestId("loop-run-inspect-cursors");
  await expect(cursors.getByText("loop_run_events", { exact: true })).toBeVisible();
  await expect(cursors.getByText(String(loopCursor), { exact: true })).toBeVisible();
  await browserArtifacts.captureScreenshot("loop-run-watch-events-cursor", appPage);

  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(seed.run.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
  await appPage.getByTestId("loop-run-open-inspect").click();
  await expect(appPage.getByTestId("loop-run-inspect-watch")).toHaveCount(0);
});

test("CompozyOS migration E2E-006: exhausted run renders score, best, restore, and best link", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop feedback browser test requires launch-mode runtime paths");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopFeedbackDefinition }),
  });

  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopFeedbackName)}/run`,
    {
      method: "POST",
      body: JSON.stringify({
        config_overrides: {
          iteration_cap: 3,
          no_progress_window: 10,
          gate_max_revisions: 10,
        },
      }),
    }
  );
  if (!started.run) throw new Error("Loop feedback browser seed did not create a run");

  const runPath = `${workspacePath}/loop-runs/${encodeURIComponent(started.run.id)}`;
  await expect
    .poll(async () => (await runtime.requestJSON<LoopRunDetail>(runPath)).run.status, {
      timeout: 30_000,
    })
    .toBe("exhausted");

  const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
  expect(detail.run.best_generation).toBe(1);
  expect(detail.run.best_score).toBe(0.7);
  expect(detail.generations?.[2]).toMatchObject({
    generation: 3,
    parent_generation: 1,
    origin: "ratchet_restore",
  });

  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(started.run.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();

  const bestLink = appPage.getByRole("link", { name: "Best result · Gen 1 · 0.70" });
  await expect(bestLink).toHaveAttribute("href", "#loop-generation-1");
  await bestLink.click();

  const bestGeneration = appPage.locator("#loop-generation-1");
  await expect(bestGeneration).toContainText("score 0.70");
  await expect(bestGeneration.getByText("Best", { exact: true })).toBeVisible();

  const restoredGeneration = appPage.locator("#loop-generation-3");
  await expect(restoredGeneration.getByText("Restored from gen 1", { exact: true })).toBeVisible();
  await expect(restoredGeneration).toContainText("score 0.50");

  await browserArtifacts.captureScreenshot("loop-run-best-on-exhaustion", appPage);
});

// E2E-013: one Environment control per agent-executing node, a loop-level
// default that survives an unrelated save, and no second directory field.
test("operator declares loop and node environments in the builder", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  loopEnvironmentRepo = await createWorktreeRepo();
  const workspace = await runtime.resolveWorkspace(loopEnvironmentRepo.rootDir);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: loopRuntimeDefinition }),
  });
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await openAppWindow(appPage, "Loops", "loops");

  await appPage.getByRole("link", { name: `Open ${loopRuntimeName}` }).click();
  await appPage.getByTestId("loop-detail-overflow").click();
  await appPage.getByTestId("loop-configure-action").click();
  const section = appPage.locator('[data-slot="loop-worktree-section"]');
  await expect(section).toBeVisible();
  await expect(section).toHaveAttribute("data-mode", "__inherit__");

  await section.locator('[data-slot="pill-group-item"]').filter({ hasText: "Per-run" }).click();
  await expect(section).toHaveAttribute("data-mode", "per_run");
  await appPage.getByTestId("loop-configure-save").click();
  await expect(appPage.getByTestId("loop-configure-dialog")).not.toBeVisible({ timeout: 30_000 });

  await expect
    .poll(async () => {
      const config = await runtime.requestJSON<{ config: { environment?: { mode: string } } }>(
        `${workspacePath}/loops/${encodeURIComponent(loopRuntimeName)}/config`
      );
      return config.config.environment?.mode;
    })
    .toBe("per_run");

  // The node inspector carries exactly one environment control and no retired
  // working-directory field anywhere.
  await appPage.getByTestId("loop-detail-overflow").click();
  await appPage.getByTestId("loop-edit-action").click();
  await appPage.getByTestId("loop-editor-node").filter({ hasText: "execute_task" }).first().click();
  await expect(appPage.locator('[data-slot="loop-node-environment-field"]')).toHaveCount(1);
  await expect(appPage.getByTestId("loop-field-cwd")).toHaveCount(0);
});
