// Controlled real-daemon/browser benchmark. The ACP fixture removes provider/network
// variation; these measurements are not production INP or live-model latency.
import { copyFile, mkdir, writeFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import path from "node:path";
import { pathToFileURL } from "node:url";

const [checkoutInput, binaryInput, outputInput, countInput = "32"] = process.argv.slice(2);
if (!checkoutInput || !binaryInput || !outputInput) {
  throw new Error("Pass checkout, its daemon binary, output directory, and optional active count");
}
const checkout = path.resolve(checkoutInput);
const checkoutRequire = createRequire(path.join(checkout, "web/package.json"));
const { chromium, expect } = (await import(
  pathToFileURL(checkoutRequire.resolve("@playwright/test")).href
)) as typeof import("@playwright/test");
const output = path.resolve(outputInput);
const activeCount = Number(countInput);
if (!Number.isInteger(activeCount) || activeCount < 16 || activeCount > 64) {
  throw new Error("Active session count must be between 16 and 64");
}
await mkdir(output, { recursive: true });
const fixtures = path.join(checkout, "web/e2e/fixtures");
const { createBrowserRuntime, waitForSeedSessionActive } = (await import(
  pathToFileURL(path.join(fixtures, "runtime.ts")).href
)) as typeof import("../fixtures/runtime");
const { seedSessionsBoard } = (await import(
  pathToFileURL(path.join(fixtures, "sessions-board.ts")).href
)) as typeof import("../fixtures/sessions-board");
const { completeOnboardingIfPrompted } = (await import(
  pathToFileURL(path.join(fixtures, "workspace.ts")).href
)) as typeof import("../fixtures/workspace");
const agentName = "performance-session-agent";
const streamCount = 8;
const fixturePath = path.join(output, "stream-fixture.json");
await writeFile(
  fixturePath,
  JSON.stringify({
    version: 2,
    agents: [
      {
        name: agentName,
        provider: "acpmock",
        permissions: "approve-reads",
        prompt: "Emit the controlled benchmark transcript.",
        turns: [
          {
            name: "steady-stream",
            match: { turn_source: "user", user_text: "benchmark stream" },
            steps: Array.from({ length: 600 }, (_, index) => [
              { kind: "assistant", chunks: [`Update ${index}: concurrent session output. `] },
              { kind: "driver_control", driver_control: { action: "delay", delay_ms: 100 } },
            ]).flat(),
          },
        ],
      },
    ],
  })
);
console.log("[browser benchmark] starting isolated runtime");
const runtime = await createBrowserRuntime({
  artifactRootDir: path.join(output, "runtime"),
  env: {
    ...process.env,
    COMPOZY_E2E_BASE_URL: "",
    COMPOZY_TEST_DAEMON_BIN: path.resolve(binaryInput),
  },
  modelsDevEnabled: false,
  newWindowPolicy: "floating",
  seed: {
    marketplaceCatalog: { extensions: [] },
    mockAgents: [
      {
        agentName,
        fixtureAgent: agentName,
        fixturePath,
      },
    ],
  },
});
let browser: Awaited<ReturnType<typeof chromium.launch>> | undefined;
const failures: unknown[] = [];
try {
  const workspace = runtime.seeded.workspace;
  if (!workspace) throw new Error("The isolated runtime must expose its seeded workspace");
  await seedSessionsBoard(runtime, {
    workspaceId: workspace.id,
    agentName,
    done: 512,
    archived: 128,
    prefix: "perf-retained",
  });
  console.log("[browser benchmark] seeded 640 retained sessions; starting active sessions");
  const sessions: { id: string; name: string }[] = [];
  for (let index = 0; index < activeCount; index++) {
    const name = `Performance ${String(index).padStart(2, "0")}`;
    const { session } = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ agent_name: agentName, name, workspace: workspace.id }),
    });
    await waitForSeedSessionActive(runtime, session.id);
    sessions.push({ id: session.id, name });
  }
  const errors: string[] = [];
  const streams: Promise<void>[] = [];
  for (const session of sessions.slice(-streamCount)) {
    const response = await fetch(
      runtime.url(`/api/workspaces/${workspace.id}/sessions/${session.id}/prompt`),
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          message: "benchmark stream",
          message_id: randomUUID(),
          idempotency_key: randomUUID(),
        }),
      }
    );
    if (!response.ok)
      throw new Error(`Prompt rejected: ${response.status} ${await response.text()}`);
    streams.push(
      response.text().then(
        body => {
          if (!body.includes("Update 599:"))
            errors.push(`Stream ${session.id} did not finish its controlled output`);
        },
        error => {
          errors.push(String(error));
        }
      )
    );
  }
  console.log(
    `[browser benchmark] ${activeCount} active sessions and ${streamCount} streams ready`
  );
  browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1500, height: 1050 } });
  const page = await context.newPage();
  const requests = new Map<string, number>();
  page.on("pageerror", error => errors.push(error.message));
  page.on("request", request => {
    const url = new URL(request.url());
    if (url.pathname.startsWith("/api/"))
      requests.set(url.pathname, (requests.get(url.pathname) ?? 0) + 1);
  });
  await page.addInitScript(() => {
    const durations: number[] = [];
    Reflect.set(window, "__performanceLongTasks", durations);
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) durations.push(entry.duration);
    }).observe({ type: "longtask", buffered: true });
  });
  const initial = sessions[0]!;
  await page.goto(runtime.url(`/agents/${agentName}/sessions/${initial.id}`), {
    waitUntil: "domcontentloaded",
  });
  await completeOnboardingIfPrompted(page);
  await expect(page.getByTestId("chat-view")).toBeVisible({ timeout: 45_000 });
  const cdp = await context.newCDPSession(page);
  const snapshots: unknown[] = [];
  async function snapshot(checkpoint: string) {
    await cdp.send("HeapProfiler.collectGarbage");
    const [heap, dom] = await Promise.all([
      cdp.send("Runtime.getHeapUsage"),
      cdp.send("Memory.getDOMCounters"),
    ]);
    snapshots.push({ checkpoint, heap, dom });
  }
  await snapshot("initial");
  const selected = sessions.slice(-16);
  const selections: { id: string; elapsedMs: number }[] = [];
  for (let pass = 0; pass < 4; pass++) {
    for (const session of selected) {
      const row = page.getByTestId(`session-sidebar-session-${session.id}`);
      await expect(row).toBeVisible();
      const start = performance.now();
      await row.click();
      await expect(row).toHaveAttribute("aria-current", "true");
      await expect(page.getByTestId("chat-view")).toBeVisible();
      await page.evaluate(
        () =>
          new Promise<void>(resolve =>
            requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
          )
      );
      selections.push({ id: session.id, elapsedMs: performance.now() - start });
    }
    await snapshot(`selection-${selections.length}`);
    console.log(`[browser benchmark] ${selections.length} selections complete`);
  }
  await Promise.all(streams);
  await page.waitForTimeout(2500);
  await snapshot("settled");
  await page.screenshot({ path: path.join(output, "sessions.png") });
  const durations = selections
    .slice(16)
    .map(value => value.elapsedMs)
    .sort((a, b) => a - b);
  const result = {
    checkout,
    binary: path.resolve(binaryInput),
    runtime: `Bun ${process.versions.bun}`,
    chromium: browser.version(),
    activeCount,
    streamCount,
    chunksPerStream: 600,
    retained: 640,
    selections,
    warmMedianMs:
      (durations[Math.floor((durations.length - 1) / 2)]! +
        durations[Math.floor(durations.length / 2)]!) /
      2,
    warmP95Ms: durations[Math.ceil(durations.length * 0.95) - 1],
    snapshots,
    errors,
    requests: Object.fromEntries(requests),
    longTasks: await page.evaluate(() => Reflect.get(window, "__performanceLongTasks") as number[]),
  };
  await writeFile(path.join(output, "measurements.json"), JSON.stringify(result, null, 2) + "\n");
  console.log(
    JSON.stringify({
      activeCount,
      warmMedianMs: result.warmMedianMs,
      warmP95Ms: result.warmP95Ms,
      errors,
      snapshots,
    })
  );
  if (errors.length > 0) throw new Error("Browser errors occurred; inspect the recorded result");
} catch (error) {
  const page = browser?.contexts()[0]?.pages()[0];
  await page?.screenshot({ path: path.join(output, "failure.png") }).catch(() => undefined);
  if (runtime.paths) {
    await copyFile(runtime.paths.daemonLog, path.join(output, "daemon-failure.log")).catch(
      () => undefined
    );
  }
  await writeFile(
    path.join(output, "failure.json"),
    JSON.stringify({ error: String(error) }) + "\n"
  );
  failures.push(error);
} finally {
  const cleanup = await Promise.allSettled([browser?.close(), runtime.dispose()]);
  await writeFile(
    path.join(output, "teardown.json"),
    JSON.stringify({
      browser: cleanup[0].status,
      runtime: cleanup[1].status,
    }) + "\n"
  );
  failures.push(
    ...cleanup.flatMap(result => (result.status === "rejected" ? [result.reason] : []))
  );
}
if (failures.length) throw new AggregateError(failures, "Benchmark or cleanup failed");
