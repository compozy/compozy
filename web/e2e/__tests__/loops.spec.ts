import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type {
  LoopDefinition,
  LoopRunDetail,
  LoopRunGenerationOutput,
  RunLoopResult,
} from "@/systems/loops";
import { expect, test } from "../fixtures/test";
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

const loopLifecycleAgent = "loop-lifecycle-agent";
const loopLifecycleName = "node-lifecycle-e2e";
const loopAuthoringName = "editor-authoring-e2e";
const loopRuntimeAgent = "loop-runtime-agent";
const loopRuntimeName = "runtime-provenance-e2e";
const loopFeedbackWorker = "loop-feedback-worker";
const loopFeedbackJudge = "loop-feedback-exhaust-judge";
const loopFeedbackName = "feedback-best-on-exhaustion-web";
const loopWatchEventsName = "watch-events-read-model-web";
const loopWatchCursorSeedName = "watch-events-cursor-seed-web";

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
    no_progress: { window: 2, hash_fields: ["nodes.finish.output.done"] },
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
    no_progress: { window: 2, hash_fields: ["nodes.task_activity.output.events"] },
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
    no_progress: { window: 2, hash_fields: ["delivery_artifact"] },
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
    no_progress: { window: 5, hash_fields: ["delivery_artifact"] },
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
            rubric: "Score the completed candidate deterministically.",
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
    no_progress: { window: 5, hash_fields: ["delivery_artifact"] },
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
    no_progress: { window: 5, hash_fields: ["delivery_artifact"] },
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

const runtimeSourceLabels: Record<string, string> = {
  run: "per-run rule",
  frontmatter: "task frontmatter",
  config: "config rule",
  node: "node runtime",
  default: "runtime default",
  criterion: "criterion runtime",
  agent: "agent definition",
};

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

  // Retrying lane: the attempt and its due time come from the payload.
  const retryChip = appPage.getByTestId("loop-run-now-chip-execute");
  await expect(retryChip).toBeVisible();
  await expect(retryChip).toContainText("attempt");
  await expect(appPage.getByTestId("loop-run-now-node-execute")).toContainText("retrying");

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

  await expect(appPage.getByTestId("loop-run-now-node-execute")).toContainText("paused");
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
  await appPage.locator("summary", { hasText: "Reliability" }).click();

  // Declare the retry budget and a single absorption mode — route XOR allow_fail.
  await appPage.getByTestId("loop-field-max_attempts").fill("3");
  await appPage.getByTestId("loop-field-backoff_base").fill("20s");
  await appPage.getByTestId("loop-field-on_error_allow_fail").click();

  // One observational effect on the retry trigger.
  await appPage.locator("summary", { hasText: "Reactions" }).click();
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
  await expect(appPage.getByTestId("loop-run-now-node-execute")).toContainText("retrying");
  await expect(appPage.getByTestId("loop-run-now-chip-execute")).toContainText("attempt");
  await browserArtifacts.captureScreenshot("loop-editor-authored-run-retrying", appPage);
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
      source: { provider: "default", model: "run", reasoning: "run" },
    },
    {
      provider: "acpmock",
      model: "docs-model",
      reasoning: "low",
      source: { provider: "default", model: "frontmatter", reasoning: "default" },
    },
  ]);

  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(started.run.id)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
  await appPage.getByTestId("loop-run-open-inspect").click();

  const inspect = appPage.getByTestId("loop-run-inspect-sheet");
  await expect(inspect).toBeVisible();
  const runtimeRows = inspect.getByTestId("loop-run-resolved-runtime");
  await expect(runtimeRows).toHaveCount(outputs.length);

  for (const { runtime: applied } of outputs) {
    if (!applied.model) throw new Error("API runtime output omitted model");
    const row = runtimeRows.filter({ hasText: applied.model });
    await expect(row).toHaveCount(1);
    for (const field of ["provider", "model", "reasoning"] as const) {
      const value = applied[field];
      const source = applied.source[field];
      if (!value || !source) {
        throw new Error(`API runtime output omitted ${field} or its provenance`);
      }
      await expect(row).toContainText(value);
      await expect(row).toContainText(runtimeSourceLabels[source] ?? source);
    }
  }

  await expect(inspect.getByTestId("loop-run-resolved-runtimes").getByRole("button")).toHaveCount(
    0
  );
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

  const bestGeneration = appPage.locator("#loop-generation-1");
  await expect(bestGeneration).toContainText("score 0.70");
  await expect(bestGeneration.getByText("Best", { exact: true })).toBeVisible();

  const restoredGeneration = appPage.locator("#loop-generation-3");
  await expect(restoredGeneration.getByText("Restored from gen 1", { exact: true })).toBeVisible();
  await expect(restoredGeneration).toContainText("score 0.50");

  const bestLink = appPage.getByRole("link", { name: "Best result · Gen 1 · 0.70" });
  await expect(bestLink).toHaveAttribute("href", "#loop-generation-1");
  await browserArtifacts.captureScreenshot("loop-run-best-on-exhaustion", appPage);
});
