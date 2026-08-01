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
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

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

const loopRuntimeAgent = "loop-runtime-agent";
const loopRuntimeName = "runtime-provenance-e2e";
const loopFeedbackWorker = "loop-feedback-worker";
const loopFeedbackJudge = "loop-feedback-exhaust-judge";
const loopFeedbackName = "feedback-best-on-exhaustion-web";

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

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
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

test("Compozy migration E2E-004: loop run renders API runtime provenance without controls", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop runtime browser test requires launch-mode runtime paths");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await useGlobalWorkspaceIfPrompted(appPage);

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
  await expect
    .poll(async () => (await runtime.requestJSON<LoopRunDetail>(runPath)).run.status, {
      timeout: 30_000,
    })
    .toMatch(/^(done|no-op|blocked|failed|exhausted|stalled)$/);

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

test("Compozy migration E2E-006: exhausted run renders score, best, restore, and best link", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("Loop feedback browser test requires launch-mode runtime paths");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await useGlobalWorkspaceIfPrompted(appPage);
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
