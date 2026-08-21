import { execFile } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import type { Locator, Page } from "@playwright/test";

import type { LoopDefinition, LoopRunDetail, RunLoopResult } from "@/systems/loops";
import type { BrowserRuntime } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

/**
 * The redesigned loop run page — one canonical spec (`_tests.md` Suite Placement).
 *
 * These are public cross-boundary journeys, not re-runs of view-model
 * invariants: the Vitest suites beside `web/src/systems/loops/lib` own how a
 * chip or a step count is derived, and re-asserting that here would freeze the
 * same fact in two layers. What only a browser can prove is what a person
 * actually sees with everything collapsed, and whether the two registers agree
 * once real daemon state moves underneath them.
 *
 * Every run below is seeded through the public API against a real daemon, with
 * deterministic acpmock agents, so an assertion that passes here is a statement
 * about the product rather than about a fixture. Nothing is conditional: a seed
 * that fails to produce the state a case is about fails the case rather than
 * skipping the assertions that would have caught it.
 */

const execFileAsync = promisify(execFile);

const acpmockFixture = (name: string) =>
  path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
    "..",
    "..",
    "internal",
    "testutil",
    "acpmock",
    "testdata",
    name
  );

const lifecycleFixture = acpmockFixture("loop_node_lifecycle_fixture.json");

const RETRY_AGENT = "loop-run-retry-agent";
const QUARANTINE_AGENT = "loop-run-quarantine-agent";

const NEEDS_YOU_LOOP = "run-page-needs-you-e2e";
const FANOUT_LOOP = "run-page-fanout-e2e";
const QUARANTINE_LOOP = "run-page-quarantine-e2e";
const DONE_LOOP = "run-page-done-e2e";
const RETRY_LOOP = "run-page-retry-e2e";
const GRAPH_STATES_LOOP = "run-page-graph-states-e2e";
const RACE_LOOP = "run-page-race-e2e";
const DEEP_CHAIN_LOOP = "run-page-deep-chain-e2e";

// The acpmock agents these journeys drive. `lifecycle_retry` blows a 2s deadline
// on its first attempt and heals on the retry; `lifecycle_quarantine` fails
// deterministically until a requeue lands it on generation 3.
test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath: lifecycleFixture, fixtureAgent: "lifecycle_retry", agentName: RETRY_AGENT },
        {
          fixturePath: lifecycleFixture,
          fixtureAgent: "lifecycle_quarantine",
          agentName: QUARANTINE_AGENT,
        },
      ],
    },
  },
});

const TERMINAL_STATES = ["done", "failed", "blocked", "exhausted", "stalled"];

/** The one token in a published `respond` unblocker the daemon leaves to the answerer. */
const PAYLOAD_PLACEHOLDER = "<json>";

function contract(goal: string, stopWhen: string, iterationCap = 1, noProgress = 2) {
  return {
    goal,
    definition_of_done: goal,
    stop_when: stopWhen,
    iteration_cap: iterationCap,
    no_progress: { window: noProgress },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" as const },
    verification: [],
    terminal_states: [...TERMINAL_STATES, "canceled"],
  };
}

const AGENT_OUTPUT_SCHEMA = {
  type: "object",
  required: ["summary"],
  properties: { summary: { type: "string" }, error: { type: "string" } },
};

/**
 * Two blockers at once, on purpose: a human gate and an open question with no
 * edge between them, so the page has to order them and count them rather than
 * showing whichever happens to be first.
 */
const needsYouDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: NEEDS_YOU_LOOP,
    description: "Parks on an approval and a question at the same time.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract("Collect one approval and one answer.", "nodes.choose.status == 'succeeded'"),
  graph: {
    nodes: [
      {
        id: "approval",
        class: "control",
        kind: "gate",
        verdict_policy: "revise_until_clean",
        criteria: [{ id: "operator", type: "human" }],
      },
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
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/**
 * A wide fan-out over run inputs. Workers are transforms, so the width is real
 * and the run still settles quickly — and one run produces enough durable events
 * that the story genuinely has to page.
 */
const fanOutDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: FANOUT_LOOP,
    description: "Spreads one review across every named area and joins the results.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract("Review every area and join the results.", "nodes.join.status == 'succeeded'"),
  graph: {
    nodes: [
      {
        id: "revisores",
        class: "control",
        kind: "fan-out",
        collection: "{{ .inputs.areas }}",
        bind_as: "area",
        index_as: "areaIndex",
        max_parallel: 8,
        max_fan_out: 512,
      },
      {
        id: "revisar",
        class: "action",
        kind: "transform",
        params: { map: { area: { template: "{{ .area }}" } } },
      },
      { id: "join", class: "control", kind: "collect" },
    ],
    edges: [
      { from: "revisores", to: "revisar" },
      { from: "revisar", to: "join" },
    ],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/**
 * A fan-out the loop's own strategy closes.
 *
 * `race` settles the scope on the first branch to succeed and the runtime
 * cancels the rest. Nobody pressed anything, which is exactly the distinction
 * US-012.EC-2 is about: the survivors are cancelled by the strategy, not by an
 * operator, and the page has to say so without inventing a person.
 */
const raceFanOutDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: RACE_LOOP,
    description: "Races several areas and cancels the losers.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract("Take the first area that finishes.", "nodes.join.status == 'succeeded'"),
  graph: {
    nodes: [
      {
        // One at a time, on purpose: the first item is then provably the winner,
        // so which siblings the strategy cancels is a fact rather than a race
        // between workers. Transforms keep it agent-free and instant.
        id: "revisores",
        class: "control",
        kind: "fan-out",
        collection: "{{ .inputs.areas }}",
        bind_as: "area",
        max_parallel: 1,
        max_fan_out: 16,
        strategy: { kind: "race" },
      },
      {
        id: "revisar",
        class: "action",
        kind: "transform",
        params: { map: { area: { template: "{{ .area }}" } } },
      },
      { id: "join", class: "control", kind: "collect" },
    ],
    edges: [
      { from: "revisores", to: "revisar" },
      { from: "revisar", to: "join" },
    ],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/**
 * One node that fails deterministically until an operator requeues it — the
 * parked row, its attempts, and the verb that unparks it.
 */
const quarantineDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: QUARANTINE_LOOP,
    description: "Parks one repeatedly failing step until an operator requeues it.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract(
    "Requeue the parked step until it succeeds.",
    "nodes.primary.status == 'succeeded'",
    4,
    3
  ),
  graph: {
    nodes: [
      {
        id: "primary",
        class: "action",
        kind: "run-agent",
        retry: { max_attempts: 0 },
        result_contract: { failure_field: "error", message_field: "error" },
        params: {
          agent: QUARANTINE_AGENT,
          prompt: "quarantine probe generation {{ .generation }}",
          output_schema: AGENT_OUTPUT_SCHEMA,
        },
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/**
 * Depth, not width.
 *
 * Fourteen steps wired head to tail, so `topoOrder` lays them out as fourteen
 * columns and the last one sits genuinely far from where the lane opens. A wide
 * fan-out proves nothing about this: it stays one entity with a rollup, however
 * many workers it spread.
 */
const DEEP_CHAIN_LENGTH = 14;

const deepChainDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: DEEP_CHAIN_LOOP,
    description: "A long chain of steps, one after another.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  // Underscores, not hyphens: `stop_when` is an expression, and `nodes.step-14`
  // would read as subtraction rather than as a node id.
  contract: contract(
    "Walk the whole chain.",
    `nodes.step_${DEEP_CHAIN_LENGTH}.status == 'succeeded'`
  ),
  graph: {
    nodes: Array.from({ length: DEEP_CHAIN_LENGTH }, (_, index) => ({
      id: `step_${index + 1}`,
      class: "action",
      kind: "transform",
      params: { map: { depth: { value: String(index + 1) } } },
    })),
    edges: Array.from({ length: DEEP_CHAIN_LENGTH - 1 }, (_, index) => ({
      from: `step_${index + 1}`,
      to: `step_${index + 2}`,
    })),
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/** The plain terminal read: one step, one output, nothing to explain away. */
const doneDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: DONE_LOOP,
    description: "Produces one output and finishes.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract("Publish one note.", "nodes.publish.status == 'succeeded'"),
  graph: {
    nodes: [
      {
        id: "publish",
        class: "action",
        kind: "transform",
        params: { map: { status: { value: "published" } } },
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

/** A step whose first attempt blows its deadline and whose retry heals it. */
const retryDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: RETRY_LOOP,
    description: "One step that times out once and succeeds on its retry.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract("Finish the step, retrying once.", "nodes.execute.status == 'succeeded'"),
  graph: {
    nodes: [
      {
        id: "execute",
        class: "action",
        kind: "run-agent",
        timeout: "2s",
        retry: { max_attempts: 3, backoff: { base: "10ms", max: "10ms" } },
        params: {
          agent: RETRY_AGENT,
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
 * One run holding every graph state at once: a taken route, a route the run
 * declined, and a step that fails outright. `risk: low` elects `quick`, so
 * `thorough` carries durable route evidence against it rather than never having
 * been reached.
 */
const graphStatesDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: GRAPH_STATES_LOOP,
    description: "Takes one route, declines another, and fails a third step.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: contract("Route, then finish.", "nodes.quick.status == 'succeeded'"),
  graph: {
    nodes: [
      {
        id: "router",
        class: "control",
        kind: "route",
        routes: [
          { when: "inputs.risk == 'low'", to: "quick" },
          { when: "inputs.risk == 'high'", to: "thorough" },
        ],
        default: "thorough",
        on_eval_error: "fail",
      },
      {
        id: "quick",
        class: "action",
        kind: "transform",
        params: { map: { path: { value: "quick" } } },
      },
      {
        id: "thorough",
        class: "action",
        kind: "transform",
        params: { map: { path: { value: "thorough" } } },
      },
      {
        id: "flaky",
        class: "action",
        kind: "run-agent",
        retry: { max_attempts: 0 },
        result_contract: { failure_field: "error", message_field: "error" },
        params: {
          agent: QUARANTINE_AGENT,
          prompt: "quarantine probe generation {{ .generation }}",
          output_schema: AGENT_OUTPUT_SCHEMA,
        },
      },
    ],
    edges: [
      { from: "router", to: "quick" },
      { from: "router", to: "thorough" },
      { from: "quick", to: "flaky" },
    ],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

interface SeededRun {
  runId: string;
  runPath: string;
  workspacePath: string;
}

interface SeedOptions {
  inputs?: Record<string, unknown>;
  humanGate?: boolean;
}

function requirePaths(runtime: BrowserRuntime) {
  if (!runtime.paths) throw new Error("The run page spec requires launch-mode runtime paths");
  return runtime.paths;
}

async function seedRun(
  appPage: Page,
  runtime: BrowserRuntime,
  definition: LoopDefinition,
  name: string,
  options: SeedOptions = {}
): Promise<SeededRun> {
  const paths = requirePaths(runtime);
  const workspace = await runtime.resolveWorkspace(paths.homeDir);
  await completeOnboardingIfPrompted(appPage);
  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition }),
  });
  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(name)}/run`,
    {
      method: "POST",
      body: JSON.stringify({
        ...(options.inputs ? { inputs: options.inputs } : {}),
        ...(options.humanGate ? { config_overrides: { human_gate_enabled: true } } : {}),
      }),
    }
  );
  if (!started.run) throw new Error(`Seeding ${name} produced no run`);
  const runId = started.run.id;
  return {
    runId,
    runPath: `${workspacePath}/loop-runs/${encodeURIComponent(runId)}`,
    workspacePath,
  };
}

/** Waits on daemon truth, never on the page, so a UI bug cannot mask a seed. */
async function waitForRun(
  runtime: BrowserRuntime,
  runPath: string,
  predicate: (detail: LoopRunDetail) => boolean,
  timeout = 90_000
): Promise<void> {
  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
        return predicate(detail) ? "ready" : `waiting: ${detail.run.status}`;
      },
      { timeout }
    )
    .toBe("ready");
}

function isTerminal(detail: LoopRunDetail): boolean {
  return [...TERMINAL_STATES, "canceled"].includes(detail.run.status);
}

interface RosterRead {
  nodes: {
    node_id: string;
    item_index: number;
    state: string;
    usage?: { tokens: number } | null;
    cancellation?: { disposition: string; actor_kind?: string; actor_ref?: string } | null;
  }[];
}

async function readRoster(runtime: BrowserRuntime, runPath: string): Promise<RosterRead> {
  return runtime.requestJSON<RosterRead>(`${runPath}/nodes?state=all&limit=500`);
}

/** Waits for one authored step to reach exactly the state the case is about. */
async function waitForRosterState(
  runtime: BrowserRuntime,
  runPath: string,
  nodeId: string,
  state: string,
  timeout = 120_000
): Promise<void> {
  await expect
    .poll(
      async () => {
        const roster = await readRoster(runtime, runPath);
        return roster.nodes.find(node => node.node_id === nodeId)?.state ?? "absent";
      },
      { timeout }
    )
    .toBe(state);
}

interface TimelinePage {
  entries: { seq: number; first_seq?: number }[];
  head_seq: number;
  next_cursor?: string;
}

async function readTimelineHead(runtime: BrowserRuntime, runPath: string): Promise<number> {
  const page = await runtime.requestJSON<TimelinePage>(`${runPath}/timeline?view=notable&limit=1`);
  return page.head_seq;
}

/** Every notable sequence the daemon holds, newest first — the expected story. */
async function readAllTimelineSequences(
  runtime: BrowserRuntime,
  runPath: string
): Promise<number[]> {
  const sequences: number[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < 200; page += 1) {
    const query = cursor === undefined ? "" : `&cursor=${encodeURIComponent(cursor)}`;
    const body = await runtime.requestJSON<TimelinePage>(
      `${runPath}/timeline?view=notable&limit=200${query}`
    );
    sequences.push(...body.entries.map(entry => entry.seq));
    if (!body.next_cursor) return sequences;
    cursor = body.next_cursor;
  }
  throw new Error("The timeline did not terminate within 200 pages");
}

function cliEnv(paths: { cliShim: string; homeDir: string }): NodeJS.ProcessEnv {
  return {
    ...process.env,
    COMPOZY_E2E_CLI_BIN: paths.cliShim,
    COMPOZY_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}

/**
 * Splits a published command back into argv.
 *
 * The daemon builds every `unblocker` with `shellquote.Join`, so a token that
 * contains shell metacharacters — `<json>` above all — comes back quoted.
 * Splitting on whitespace would hand the CLI a literal `'<json>'` and the test
 * would "pass" a command no operator could run. This is the counterpart of that
 * join: single quotes, double quotes and backslash escapes, nothing else.
 */
function parseShellWords(command: string): string[] {
  const words: string[] = [];
  let current = "";
  let started = false;
  let quote: "'" | '"' | null = null;
  for (let index = 0; index < command.length; index += 1) {
    const char = command[index];
    if (quote === "'") {
      if (char === "'") quote = null;
      else current += char;
      continue;
    }
    if (quote === '"') {
      if (char === '"') quote = null;
      else if (char === "\\" && index + 1 < command.length) current += command[(index += 1)];
      else current += char;
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      started = true;
      continue;
    }
    if (char === "\\" && index + 1 < command.length) {
      current += command[(index += 1)];
      started = true;
      continue;
    }
    if (/\s/u.test(char)) {
      if (started) words.push(current);
      current = "";
      started = false;
      continue;
    }
    current += char;
    started = true;
  }
  if (quote !== null) throw new Error(`Unterminated quote in command: ${command}`);
  if (started) words.push(current);
  return words;
}

function unblockerArgv(command: string): string[] {
  const argv = parseShellWords(command);
  if (argv[0] !== "compozy") throw new Error(`Unexpected unblocker command: ${command}`);
  return argv.slice(1);
}

async function runServedUnblocker(runtime: BrowserRuntime, command: string): Promise<void> {
  const paths = requirePaths(runtime);
  await execFileAsync(paths.cliShim, unblockerArgv(command), {
    env: cliEnv(paths),
    timeout: 60_000,
  });
}

/**
 * Runs a published unblocker after filling in the one value the daemon cannot
 * know: the answer itself. Only the explicit `<json>` placeholder is replaced,
 * as a single argv element, so nothing is re-quoted and no other token can be
 * rewritten by accident.
 */
async function runServedUnblockerWithPayload(
  runtime: BrowserRuntime,
  command: string,
  payload: unknown
): Promise<void> {
  const paths = requirePaths(runtime);
  const argv = unblockerArgv(command);
  const placeholders = argv.filter(word => word === PAYLOAD_PLACEHOLDER).length;
  if (placeholders !== 1) {
    throw new Error(`Expected exactly one ${PAYLOAD_PLACEHOLDER} in: ${command}`);
  }
  const filled = argv.map(word => (word === PAYLOAD_PLACEHOLDER ? JSON.stringify(payload) : word));
  await execFileAsync(paths.cliShim, filled, { env: cliEnv(paths), timeout: 60_000 });
}

interface BriefingBlocker {
  kind: string;
  gate_id?: string;
  node_id?: string;
  unblocker: string;
}

async function readBriefingBlockers(
  runtime: BrowserRuntime,
  runPath: string
): Promise<BriefingBlocker[]> {
  const briefing = await runtime.requestJSON<{ blockers: BriefingBlocker[] }>(
    `${runPath}/briefing`
  );
  return briefing.blockers;
}

async function openRun(appPage: Page, runtime: BrowserRuntime, runId: string): Promise<void> {
  await appPage.goto(runtime.url(`/loop-runs/${encodeURIComponent(runId)}`), {
    waitUntil: "domcontentloaded",
  });
  await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
}

async function openInspect(appPage: Page, lane: "graph" | "nodes" | "generations"): Promise<void> {
  await appPage.getByTestId("loop-run-open-inspect").click();
  await expect(appPage.getByTestId("loop-run-inspect-panel")).toBeVisible();
  await appPage.getByTestId(`loop-lane-${lane}`).click();
  await expect(appPage.getByTestId(`loop-run-inspect-lane-${lane}`)).toBeVisible();
}

async function seqAttributes(beats: Locator): Promise<number[]> {
  return beats.evaluateAll(nodes => nodes.map(node => Number(node.getAttribute("data-seq"))));
}

/**
 * One roster row, by the step it is about.
 *
 * The row's own test id carries round, step and item, because the same step id
 * exists once per round and once per fan-out worker. The table opens on the
 * round the run is on, so naming the step is unambiguous without hard-coding a
 * round number the run decides.
 */
function rosterRow(appPage: Page, nodeId: string, itemIndex?: number): Locator {
  // A fan-out worker shares its step id with every sibling, so anything that
  // opens "the first row called revisar" opens whichever one is listed first —
  // not the one the case is about.
  const item = itemIndex === undefined ? "" : `[data-item-index="${itemIndex}"]`;
  return appPage
    .getByTestId("loop-node-roster")
    .locator(`[data-testid^="loop-roster-row-"][data-node-id="${nodeId}"]${item}`);
}

test.describe("Loop run page — two registers", () => {
  test("E2E-012: the default read answers everything without opening a disclosure", async ({
    appPage,
    runtime,
  }) => {
    const { runId, runPath } = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    await waitForRun(runtime, runPath, detail => (detail.requests ?? []).length > 0);
    await openRun(appPage, runtime, runId);

    // Nothing is expanded: this is the page as it arrives.
    await expect(appPage.getByTestId("loop-run-briefing-headline")).toBeVisible();
    await expect(appPage.getByTestId("loop-run-progress-label")).toBeVisible();
    await expect(appPage.getByTestId("loop-run-progress-meta")).toContainText(/step \d+ of \d+/i);
    await expect(appPage.getByTestId("loop-run-story")).toBeVisible();
    await expect(appPage.getByTestId("loop-run-usage-tokens")).toBeVisible();
    await expect(appPage.getByTestId("loop-run-usage-cost")).toBeVisible();

    // The operator register is present but closed — depth costs exactly one click.
    await expect(appPage.getByTestId("loop-run-inspect")).toBeVisible();
    await expect(appPage.getByTestId("loop-run-inspect-panel")).toBeHidden();

    // The id assertion, scoped to the main column. The About rail's labelled
    // `Run` row is the one place an id belongs in the default read, so grepping
    // the whole page would fail on the very element the design calls for.
    const mainColumn = appPage.locator("main");
    await expect(mainColumn).not.toContainText("looprun-");
    await expect(mainColumn).not.toContainText("loop.");
    await expect(appPage.getByTestId("loop-run-detail-rail")).toContainText(runId);
  });

  test("E2E-013: two things need you, ordered and counted", async ({ appPage, runtime }) => {
    const { runId, runPath } = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    // Both blockers open together — the whole point of the case.
    await waitForRun(
      runtime,
      runPath,
      detail => detail.run.status === "needs-approval" && (detail.requests ?? []).length > 0
    );
    const blockers = await readBriefingBlockers(runtime, runPath);
    expect(blockers.length).toBe(2);
    // The daemon orders approval before request; the page renders that order.
    expect(blockers.map(blocker => blocker.kind)).toEqual(["approval", "request"]);

    await openRun(appPage, runtime, runId);
    const needsYou = appPage.getByTestId("loop-run-needs-you");
    await expect(needsYou).toBeVisible();
    await expect(needsYou).toContainText("2");
    await expect(appPage.getByTestId("loop-request-card")).toHaveCount(1);

    // The briefing points at the decision; it never carries one. One primary per
    // decision, in one viewport.
    await expect(appPage.getByTestId("loop-run-briefing-action")).toContainText(
      "Review the request"
    );
    await expect(
      appPage.getByTestId("loop-run-briefing").getByTestId("loop-request-decision-approve")
    ).toHaveCount(0);
  });

  test("E2E-013: resolving one at the command line clears it live and leaves the other", async ({
    appPage,
    runtime,
  }) => {
    const { runId, runPath } = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    await waitForRun(
      runtime,
      runPath,
      detail => detail.run.status === "needs-approval" && (detail.requests ?? []).length > 0
    );
    await openRun(appPage, runtime, runId);
    await expect(appPage.getByTestId("loop-request-card")).toHaveCount(1);

    // Answered outside the browser entirely, through the command the daemon
    // itself published for this blocker. Everything but the answer comes from
    // the served string — workspace, run, generation, node, item and decision —
    // so a drift in any of them fails here rather than in someone's terminal.
    const blockers = await readBriefingBlockers(runtime, runPath);
    const request = blockers.find(blocker => blocker.kind === "request");
    if (!request) throw new Error("The briefing published no request blocker");
    expect(request.unblocker).toContain(runId);
    await runServedUnblockerWithPayload(runtime, request.unblocker, { decision: "approve" });

    // The page notices on its own: no reload, no click.
    await waitForRun(runtime, runPath, answered =>
      (answered.requests ?? []).every(entry => Boolean(entry.answered_at))
    );
    await expect(appPage.getByTestId("loop-request-card")).toHaveCount(0, { timeout: 60_000 });

    // Answering one blocker never answers the other: the gate is still holding
    // the run, and the page still says so.
    const remaining = await readBriefingBlockers(runtime, runPath);
    expect(remaining.map(blocker => blocker.kind)).toEqual(["approval"]);
    await expect(appPage.getByTestId("loop-run-needs-you")).toBeVisible();
    await expect(appPage.getByTestId("loop-run-briefing")).toHaveAttribute(
      "data-tone",
      "needs_you"
    );
  });

  test("E2E-013: the served approval unblocker is a command that actually runs", async ({
    appPage,
    runtime,
  }) => {
    const { runId, runPath } = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    await waitForRun(
      runtime,
      runPath,
      detail => detail.run.status === "needs-approval" && (detail.requests ?? []).length > 0
    );
    await openRun(appPage, runtime, runId);

    // Executed verbatim, with no substitution: an approval unblocker is complete
    // as published, and publishing a command nobody can run is worse than
    // publishing none.
    const blockers = await readBriefingBlockers(runtime, runPath);
    const approval = blockers.find(blocker => blocker.kind === "approval");
    if (!approval) throw new Error("The briefing published no approval blocker");
    expect(approval.unblocker).toContain(runId);
    await runServedUnblocker(runtime, approval.unblocker);

    // The approval leaves the briefing — but the open question does not, so the
    // run still needs a person and the strip still says so. Expecting the
    // needs-you tone to clear here would be expecting the page to lie about the
    // request that is still sitting there.
    await expect
      .poll(
        async () => (await readBriefingBlockers(runtime, runPath)).map(blocker => blocker.kind),
        { timeout: 60_000 }
      )
      .toEqual(["request"]);
    await expect(appPage.getByTestId("loop-request-card")).toHaveCount(1);
    await expect(appPage.getByTestId("loop-run-briefing")).toHaveAttribute(
      "data-tone",
      "needs_you"
    );
  });

  test("E2E-014: terminal runs lead with their outcome, uncollapsed", async ({
    appPage,
    runtime,
  }) => {
    const done = await seedRun(appPage, runtime, doneDefinition, DONE_LOOP);
    await waitForRun(runtime, done.runPath, detail => detail.run.status === "done");
    await openRun(appPage, runtime, done.runId);

    await expect(appPage.getByTestId("loop-run-briefing-outcome")).toContainText("Done");
    // Visible with every disclosure shut: a signal you have to expand to find is
    // a signal you will miss.
    await expect(appPage.getByTestId("loop-run-inspect-panel")).toBeHidden();
    // A run that produced nothing says so rather than showing an empty shelf.
    await expect(
      appPage.getByTestId("loop-run-artifacts").or(appPage.getByTestId("loop-run-artifacts-none"))
    ).toBeVisible();
  });

  test("E2E-014: a failed run leads with its failure, uncollapsed", async ({
    appPage,
    runtime,
  }) => {
    const { runId, runPath } = await seedRun(
      appPage,
      runtime,
      quarantineDefinition,
      QUARANTINE_LOOP
    );
    // A real failure, not a label: the step exhausts and the run settles on one
    // of its failing terminal states.
    await waitForRun(runtime, runPath, detail =>
      ["failed", "exhausted", "stalled", "blocked"].includes(detail.run.status)
    );
    await openRun(appPage, runtime, runId);

    const briefing = appPage.getByTestId("loop-run-briefing");
    await expect(briefing).toHaveAttribute("data-tone", "failed");
    await expect(appPage.getByTestId("loop-run-briefing-headline")).not.toBeEmpty();
    // The plain cause is readable with every disclosure still shut.
    await expect(appPage.getByTestId("loop-run-inspect-panel")).toBeHidden();
    await expect(briefing).toContainText(/fail|exhaust|stall|block/i);
  });

  test("E2E-014: a canceled run names who stopped it and when, calmly", async ({
    appPage,
    runtime,
  }) => {
    const { runId, runPath } = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    await waitForRun(runtime, runPath, detail => (detail.requests ?? []).length > 0);
    await runtime.requestJSON(`${runPath}/cancel`, { method: "POST", body: JSON.stringify({}) });
    await waitForRun(runtime, runPath, detail => detail.run.status === "canceled");

    // The daemon recorded both halves of the answer, so the page has both to show.
    const briefingRead = await runtime.requestJSON<{
      outcome?: { actor_kind?: string; actor_ref?: string; at: string } | null;
    }>(`${runPath}/briefing`);
    expect(briefingRead.outcome?.at).toBeTruthy();
    expect(briefingRead.outcome?.actor_kind || briefingRead.outcome?.actor_ref).toBeTruthy();

    await openRun(appPage, runtime, runId);
    const briefing = appPage.getByTestId("loop-run-briefing");
    await expect(appPage.getByTestId("loop-run-briefing-outcome")).toContainText("Canceled");
    // Who, and when — a cancellation without a time is a fact nobody can place.
    await expect(briefing).toContainText("by");
    await expect(appPage.getByTestId("loop-run-briefing-outcome-at")).toBeVisible();
    // Cancellation is calm — the actor travels in words, never in an alarm colour.
    await expect(briefing).not.toHaveAttribute("data-tone", "failed");
  });

  test("E2E-015: a long run pages its whole story back with no gap and no repeat", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    // Wide enough that the run genuinely outgrows one timeline page set.
    const areas = Array.from({ length: 200 }, (_, index) => `area-${index + 1}`);
    const { runId, runPath } = await seedRun(appPage, runtime, fanOutDefinition, FANOUT_LOOP, {
      inputs: { areas },
    });
    await waitForRun(runtime, runPath, isTerminal, 180_000);

    // The premise, asserted rather than assumed: an under-producing seed fails
    // here instead of quietly passing a test about paging that never paged.
    const headSeq = await readTimelineHead(runtime, runPath);
    expect(headSeq).toBeGreaterThan(500);

    await openRun(appPage, runtime, runId);
    const beats = appPage.getByTestId(/^loop-run-beat-/);
    await expect(beats.first()).toBeVisible();

    // Reload: history that lived in a client frame buffer would be gone here. It
    // comes from the durable timeline, so it is not.
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
    await expect(beats.first()).toBeVisible();

    // Page back to the run's start. Backward paging only ever adds.
    const loadOlder = appPage.getByTestId("loop-run-story-load-older");
    for (let page = 0; page < 60; page += 1) {
      if ((await loadOlder.count()) === 0) break;
      const before = await beats.count();
      await loadOlder.click();
      await expect.poll(async () => beats.count()).toBeGreaterThan(before);
    }
    await expect(loadOlder).toHaveCount(0);

    const rendered = await seqAttributes(beats);
    // No repeat: every sequence appears exactly once.
    expect(new Set(rendered).size).toBe(rendered.length);
    // Strictly descending: the fence held across every page and the live seam.
    for (let index = 1; index < rendered.length; index += 1) {
      expect(rendered[index]).toBeLessThan(rendered[index - 1]);
    }
    // No missing tail: the page holds exactly what the daemon holds, down to the
    // run's very first notable sequence.
    const expected = await readAllTimelineSequences(runtime, runPath);
    expect(rendered).toEqual(expected);
    expect(rendered.at(-1)).toBe(Math.min(...expected));
  });

  test("E2E-016: the operator register draws a live run, then the same run terminal", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    const { runId, runPath } = await seedRun(
      appPage,
      runtime,
      graphStatesDefinition,
      GRAPH_STATES_LOOP,
      { inputs: { risk: "low" } }
    );
    // Every state this case is about is really in the run before it is asserted:
    // the taken arm succeeded, the declined arm carries route evidence, and the
    // third step failed outright.
    await waitForRun(runtime, runPath, isTerminal, 120_000);
    const roster = await runtime.requestJSON<{ nodes: { node_id: string; state: string }[] }>(
      `${runPath}/nodes?state=all&limit=500`
    );
    const stateOf = (nodeId: string) =>
      roster.nodes.find(node => node.node_id === nodeId)?.state ?? "absent";
    expect(stateOf("quick")).toBe("succeeded");
    expect(stateOf("thorough")).toBe("not_taken");
    expect(stateOf("flaky")).toBe("failed");

    await openRun(appPage, runtime, runId);
    await openInspect(appPage, "graph");
    const dag = appPage.getByTestId("loop-run-dag");
    await expect(dag).toBeVisible();

    // Three dispositions, three chips, each carrying its own literal word.
    await expect(appPage.getByTestId("loop-dag-node-quick")).toHaveAttribute(
      "data-state",
      "succeeded"
    );
    await expect(appPage.getByTestId("loop-dag-node-flaky")).toHaveAttribute(
      "data-state",
      "failed"
    );
    // Not-taken is durable route evidence, not a failure: neutral, and it says so.
    const declined = appPage.getByTestId("loop-dag-node-thorough");
    await expect(declined).toHaveAttribute("data-state", "not_taken");
    await expect(declined.getByTestId("loop-state-chip-not_taken")).toContainText("not taken");
    await expect(declined.getByTestId("loop-state-chip-failed")).toHaveCount(0);

    // A terminal graph reads its final state — no pulse left running on a
    // finished run, which was the "is it stalled?" misreading in the first place.
    await expect(appPage.getByTestId("loop-dag-edge-pulse")).toHaveCount(0);

    // Links stay valid after the run ends; that is when people come looking.
    await appPage.getByTestId("loop-dag-node-quick").click();
    const panel = appPage.getByTestId("loop-node-panel");
    await expect(panel).toBeVisible();
    await expect(panel.getByTestId("loop-node-panel-link-record")).toHaveAttribute(
      "href",
      /\/tasks\//
    );
    // A node the run never took offers no links at all rather than a dead one.
    await appPage.getByTestId("loop-dag-node-thorough").click();
    await expect(panel.getByTestId("loop-node-panel-never")).toBeVisible();
    await expect(panel.getByTestId("loop-node-panel-link-record")).toHaveCount(0);
  });

  test("E2E-016: a wide fan-out stays one rollup, and a live run pulses into what is running", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    const areas = Array.from({ length: 120 }, (_, index) => `area-${index + 1}`);
    const { runId, runPath } = await seedRun(appPage, runtime, fanOutDefinition, FANOUT_LOOP, {
      inputs: { areas },
    });
    await waitForRun(runtime, runPath, detail => (detail.generations ?? []).length > 0);
    await openRun(appPage, runtime, runId);
    await openInspect(appPage, "graph");

    // Asserted while the daemon still calls the run live, so a pulse that only
    // ever appears on a finished run cannot satisfy this.
    const live = await runtime.requestJSON<LoopRunDetail>(runPath);
    expect(isTerminal(live)).toBe(false);
    await expect(appPage.getByTestId("loop-run-inspect-live")).toBeVisible();
    await expect(appPage.getByTestId("loop-dag-edge-pulse").first()).toBeVisible();

    // 120 workers stay one entity carrying a rollup; they are not 120 nodes.
    const fanOut = appPage.getByTestId("loop-dag-fanout-revisores");
    await expect(fanOut).toBeVisible();
    await expect(fanOut).toContainText("120");
    await expect(appPage.getByTestId("loop-dag-node-revisar")).toHaveCount(0);

    // Width is locatable: the lane scrolls, and scrolling moves it. Depth is a
    // different property and has its own case below.
    const dag = appPage.getByTestId("loop-run-dag");
    const before = await dag.evaluate(node => node.scrollLeft);
    await dag.evaluate(node => node.scrollBy({ left: node.clientWidth }));
    await expect.poll(async () => dag.evaluate(node => node.scrollLeft)).toBeGreaterThan(before);

    // Terminal faithfulness: the same graph, re-read after the run settles.
    await waitForRun(runtime, runPath, isTerminal, 180_000);
    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("loop-run-detail-content")).toBeVisible();
    await openInspect(appPage, "graph");
    await expect(appPage.getByTestId("loop-dag-edge-pulse")).toHaveCount(0);
    await expect(appPage.getByTestId("loop-dag-node-join")).toHaveAttribute(
      "data-state",
      "succeeded"
    );
  });

  test("E2E-016: a step deep down the graph can still be found", async ({ appPage, runtime }) => {
    test.slow();
    const { runId, runPath } = await seedRun(
      appPage,
      runtime,
      deepChainDefinition,
      DEEP_CHAIN_LOOP
    );
    await waitForRun(runtime, runPath, isTerminal, 120_000);
    await openRun(appPage, runtime, runId);
    await openInspect(appPage, "graph");

    const dag = appPage.getByTestId("loop-run-dag");
    const first = appPage.getByTestId("loop-dag-node-step_1");
    const last = appPage.getByTestId(`loop-dag-node-step_${DEEP_CHAIN_LENGTH}`);
    // Every step of the chain is drawn: depth is columns, not a rollup.
    await expect(first).toBeVisible();
    await expect(appPage.getByTestId(/^loop-dag-node-step_/)).toHaveCount(DEEP_CHAIN_LENGTH);

    // The tail really is out of reach on arrival — otherwise "locatable after
    // navigation" would be satisfied by a graph that never needed navigating.
    const laneBox = await dag.boundingBox();
    const tailBox = await last.boundingBox();
    if (!laneBox || !tailBox) throw new Error("The graph lane rendered without a box");
    expect(tailBox.x).toBeGreaterThan(laneBox.x + laneBox.width);

    await last.scrollIntoViewIfNeeded();

    // Found: on screen, inside the lane, and reading its own final state.
    await expect(last).toBeInViewport();
    const movedBox = await last.boundingBox();
    if (!movedBox) throw new Error("The tail step vanished after navigation");
    expect(movedBox.x).toBeLessThan(laneBox.x + laneBox.width);
    await expect(last).toHaveAttribute("data-state", "succeeded");
  });

  test("E2E-017: the roster lists healthy and parked steps, and a requeue lands live", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    const { runId, runPath } = await seedRun(
      appPage,
      runtime,
      quarantineDefinition,
      QUARANTINE_LOOP
    );
    // The step really parks. Accepting "failed or quarantined" would let the
    // case pass on a step that merely failed and was still on its way to being
    // parked — a different state, and not the one this is about.
    await waitForRosterState(runtime, runPath, "primary", "quarantined");
    await openRun(appPage, runtime, runId);
    await openInspect(appPage, "nodes");

    await expect(appPage.getByTestId("loop-node-roster")).toBeVisible();
    const parked = rosterRow(appPage, "primary");
    await expect(parked).toHaveAttribute("data-state", "quarantined");
    // Attempts are metadata on the row, never sibling steps.
    await expect(parked).toContainText(/\d+ of \d+/);

    // A verb offered from the register is one the daemon accepts, and its effect
    // reaches the page without a reload.
    await parked.click();
    const panel = appPage.getByTestId("loop-node-panel");
    await expect(panel).toBeVisible();
    // Every recorded attempt is available to the reader, with its own content —
    // the attempt number and what became of it, not just a count on the row.
    const attempts = panel.getByTestId("loop-node-panel-attempts");
    await expect(attempts).toBeVisible();
    await expect(attempts).toContainText(/Attempt \d+/);
    await expect(attempts.getByTestId(/^loop-state-chip-/).first()).toBeVisible();

    const requeue = panel.getByTestId(/^loop-node-primary-requeue-/);
    await expect(requeue).toBeVisible();
    const generationsBefore = (await runtime.requestJSON<LoopRunDetail>(runPath)).generations ?? [];
    await requeue.click();
    await expect
      .poll(
        async () => {
          const detail = await runtime.requestJSON<LoopRunDetail>(runPath);
          return (detail.generations ?? []).length;
        },
        { timeout: 90_000 }
      )
      .toBeGreaterThan(generationsBefore.length);
    // The register reflects the new round without a reload.
    await expect(rosterRow(appPage, "primary")).toBeVisible();
  });

  test("E2E-017: a healthy row carries its usage, and the round carries its own", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    const { runId, runPath } = await seedRun(appPage, runtime, retryDefinition, RETRY_LOOP);
    await waitForRun(runtime, runPath, detail => detail.run.status === "done");

    // The step really reported tokens, so the row has a number to price.
    const roster = await runtime.requestJSON<{
      nodes: { node_id: string; usage?: { tokens: number } | null }[];
    }>(`${runPath}/nodes?state=all&limit=500`);
    const tokens = roster.nodes.find(node => node.node_id === "execute")?.usage?.tokens ?? 0;
    expect(tokens).toBeGreaterThan(0);

    await openRun(appPage, runtime, runId);
    await openInspect(appPage, "nodes");
    // The whole run, not its exceptions: a succeeded step has a row here, with
    // its usage priced as an estimate rather than stated as a fact — in the row,
    // not only in the header that labels it.
    const healthy = rosterRow(appPage, "execute");
    await expect(healthy).toHaveAttribute("data-state", "succeeded");
    await expect(healthy).toContainText(/\d+ of \d+/);
    await expect(healthy).toContainText(/~\$\d+\.\d{2}/);
    await expect(appPage.getByRole("columnheader", { name: /est\. cost/i })).toBeVisible();

    await appPage.getByTestId("loop-lane-generations").click();
    const history = appPage.getByTestId("loop-generation-history");
    await expect(history).toBeVisible();
    const round = appPage.getByTestId("loop-generation-1");
    await expect(round).toBeVisible();
    // The round states an outcome in words and prices its own usage.
    await expect(round).toContainText(/[a-z]/i);
    await expect(appPage.getByTestId("loop-generation-usage-1")).toContainText(
      /~\$\d+\.\d{2} est\./
    );
    // Raw verdict enums never reach the page.
    await expect(history).not.toContainText("invalid_output");
    await expect(history).not.toContainText("awaiting_approval");
    await expect(history).not.toContainText("_");
  });

  test("E2E-017: the loop's own strategy and an operator read as different cancellations", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    // US-012.EC-2, both halves, from two real runs. A `race` fan-out settles on
    // the first branch to succeed and the runtime cancels the rest — nobody
    // pressed anything, and the panel must not imply somebody did.
    const race = await seedRun(appPage, runtime, raceFanOutDefinition, RACE_LOOP, {
      inputs: { areas: ["fast", "slow-one", "slow-two"] },
    });
    await waitForRun(runtime, race.runPath, isTerminal, 120_000);
    const raceRoster = await readRoster(runtime, race.runPath);
    const strategyCanceled = raceRoster.nodes.find(
      node => node.cancellation?.disposition === "strategy"
    );
    if (!strategyCanceled) {
      throw new Error(
        `The race strategy cancelled no sibling: ${JSON.stringify(raceRoster.nodes)}`
      );
    }
    // The runtime is not an actor: a strategy cancellation has nobody to name.
    expect(strategyCanceled.cancellation?.actor_kind ?? "").toBe("");

    await openRun(appPage, runtime, race.runId);
    await openInspect(appPage, "nodes");
    await rosterRow(appPage, strategyCanceled.node_id, strategyCanceled.item_index).click();
    const strategyPanel = appPage
      .getByTestId("loop-node-panel")
      .getByTestId("loop-node-panel-cancellation");
    await expect(strategyPanel).toContainText("Canceled by the loop's strategy");
    // No operator is invented for a decision no operator made.
    await expect(strategyPanel).not.toContainText("by an operator");
    await expect(strategyPanel).not.toContainText("canceled_by_strategy");

    // The other half: one node cancelled deliberately, through the public verb.
    // An open ask is durably waiting until somebody answers it, so the mutation
    // cannot lose a race against a set of instant transform workers finishing.
    const operator = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    await waitForRosterState(runtime, operator.runPath, "choose", "waiting");
    await runtime.requestJSON(
      `${operator.workspacePath}/loop-runs/${encodeURIComponent(
        operator.runId
      )}/nodes/${encodeURIComponent("choose")}/cancel`,
      {
        method: "POST",
        body: JSON.stringify({ reason: "Stopped by hand for the E2E." }),
      }
    );
    // Poll the provenance, not only the state: the case is about who canceled
    // the step, and a generic canceled row cannot satisfy it.
    await expect
      .poll(
        async () => {
          const roster = await readRoster(runtime, operator.runPath);
          return roster.nodes.some(node => node.cancellation?.disposition === "operator");
        },
        { timeout: 120_000 }
      )
      .toBe(true);
    const operatorRoster = await readRoster(runtime, operator.runPath);
    const operatorCanceled = operatorRoster.nodes.find(
      node => node.cancellation?.disposition === "operator"
    );
    if (!operatorCanceled) {
      throw new Error("The node cancel recorded no operator disposition");
    }

    await openRun(appPage, runtime, operator.runId);
    await openInspect(appPage, "nodes");
    await rosterRow(appPage, operatorCanceled.node_id, operatorCanceled.item_index).click();
    const operatorPanel = appPage
      .getByTestId("loop-node-panel")
      .getByTestId("loop-node-panel-cancellation");
    await expect(operatorPanel).toContainText("Canceled by an operator");
    // Who did it travels with it — that is the whole difference from the above.
    await expect(operatorPanel).toContainText("by");
    await expect(operatorPanel).toContainText("Stopped by hand for the E2E.");
    // Cancellation is calm on both paths: cause in words, never an alarm colour.
    await expect(operatorPanel.getByTestId("loop-state-chip-failed")).toHaveCount(0);
  });

  test("E2E-018: the runs roster puts what needs you first and reads outcomes plainly", async ({
    appPage,
    runtime,
  }) => {
    test.slow();
    const needsYou = await seedRun(appPage, runtime, needsYouDefinition, NEEDS_YOU_LOOP, {
      humanGate: true,
    });
    const done = await seedRun(appPage, runtime, doneDefinition, DONE_LOOP);
    // Three different terminal endings, so "plain outcome" is tested as a
    // vocabulary rather than as one lucky word.
    const failed = await seedRun(appPage, runtime, quarantineDefinition, QUARANTINE_LOOP);
    const canceled = await seedRun(appPage, runtime, retryDefinition, RETRY_LOOP);
    await waitForRun(runtime, done.runPath, detail => detail.run.status === "done");
    await runtime.requestJSON(`${canceled.runPath}/cancel`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    await waitForRun(runtime, canceled.runPath, detail => detail.run.status === "canceled");
    await waitForRun(runtime, failed.runPath, isTerminal, 120_000);
    await waitForRun(runtime, needsYou.runPath, detail => (detail.requests ?? []).length > 0);

    await appPage.goto(runtime.url("/loop-runs"), { waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("loop-runs-view")).toBeVisible();

    // The needs-you group leads the page, and it is the daemon's ordering that
    // put it there — the page never re-sorts a loaded slice.
    const groups = appPage.getByTestId(/^loop-runs-group-/);
    await expect(groups.first()).toHaveAttribute("data-group", "needs-you");

    const needsYouRow = appPage
      .getByTestId("loop-runs-group-needs-you")
      .getByTestId("loop-run-row")
      .first();
    await expect(needsYouRow).toHaveAttribute("data-needs-you", "true");
    await expect(needsYouRow.getByTestId("loop-run-status")).toContainText("Needs you");
    // Plain words lead; the id is present but demoted.
    await expect(needsYouRow.getByTestId("loop-run-name")).toContainText(NEEDS_YOU_LOOP);

    // Every terminal run reads as a plain outcome, never as a status enum.
    const outcomeOf = (loopName: string) =>
      appPage
        .getByTestId("loop-run-row")
        .filter({ hasText: loopName })
        .first()
        .getByTestId("loop-run-status");
    await expect(outcomeOf(DONE_LOOP)).toContainText(/done/i);
    await expect(outcomeOf(RETRY_LOOP)).toContainText(/canceled/i);
    await expect(outcomeOf(QUARANTINE_LOOP)).toContainText(/fail|exhaust|stall|block/i);
    for (const loopName of [DONE_LOOP, RETRY_LOOP, QUARANTINE_LOOP]) {
      await expect(outcomeOf(loopName)).not.toContainText("_");
    }
  });

  test("E2E-018: a workspace with no runs explains how to start one", async ({
    appPage,
    runtime,
  }) => {
    const paths = requirePaths(runtime);
    await runtime.resolveWorkspace(paths.homeDir);
    await completeOnboardingIfPrompted(appPage);

    await appPage.goto(runtime.url("/loop-runs"), { waitUntil: "domcontentloaded" });
    // A fresh workspace gets a sentence, not a blank table.
    await expect(appPage.getByTestId("loop-runs-empty")).toContainText("No runs yet");
    await expect(appPage.getByTestId("loop-runs-empty")).toContainText("catalog");
  });

  test("E2E-019: reduced motion unmounts the pulse and every chip says its state", async ({
    appPage,
    runtime,
  }) => {
    // Emulated on the page rather than declared as a suite option: the shared
    // `appPage` fixture owns the context, so the preference has to be set on the
    // page the harness handed us.
    await appPage.emulateMedia({ reducedMotion: "reduce" });
    const { runId, runPath } = await seedRun(appPage, runtime, retryDefinition, RETRY_LOOP);
    await waitForRun(runtime, runPath, detail => (detail.generations ?? []).length > 0);
    await openRun(appPage, runtime, runId);
    await openInspect(appPage, "graph");
    await expect(appPage.getByTestId("loop-run-dag")).toBeVisible();

    // Removed from the render, not paused mid-frame: a frozen dot on an edge
    // reads as a stalled run, which is the exact misreading to avoid.
    await expect(appPage.getByTestId("loop-dag-edge-pulse")).toHaveCount(0);

    // Colour is never the sole carrier. Every state chip says its state in words
    // AND carries its glyph, or a screenshot and a colour-blind reader both come
    // away with nothing.
    const chips = appPage.getByTestId(/^loop-state-chip-/);
    const count = await chips.count();
    expect(count).toBeGreaterThan(0);
    for (let index = 0; index < count; index += 1) {
      const chip = chips.nth(index);
      await expect(chip).toHaveAccessibleName(/\S/);
      await expect(chip.locator("svg")).toHaveCount(1);
    }
  });
});
