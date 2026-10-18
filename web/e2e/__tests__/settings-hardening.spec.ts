import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import { appWindow, windowFrame } from "../fixtures/os-navigation";
import { sessionLifecycleSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace, completeOnboardingIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);

const sensitivePattern =
  /compozy_claim_[a-z0-9._-]+|["']claim_token["']\s*:\s*["']?[a-z0-9._-]{8,}|(?:authorization\s*:\s*bearer|bearer)\s+["']?[a-z0-9._-]{8,}|(?:api[_-]?key|bearer[_-]?token|mcp[_-]?auth|oauth[_-]?(?:access(?:[_-]?token)?|client(?:[_-]?secret)?|refresh(?:[_-]?token)?|secret|token)|pkce[_-]?(?:challenge|secret|verifier)|provider[_-]?credential|telegram-bot-token|browser-settings-secret)\s*[:=]\s*["']?[a-z0-9._:-]{8,}|\b\d{6,}:[a-z0-9_-]{20,}/i;

test.use({
  runtimeOptions: {
    env: {
      ...process.env,
      COMPOZY_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
    },
  },
});

test("operator applies Automation, Observability, and Providers settings with config parity", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  assertLaunchRuntime(runtime, "settings parity");

  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(sessionLifecycleSelectors(appPage));

  const automationBefore = await runtime.requestJSON<{
    config: { max_concurrent_jobs: number };
  }>("/api/settings/automation");
  const nextMaxConcurrent = automationBefore.config.max_concurrent_jobs + 1;
  await appPage.goto(runtime.url("/settings/automation"), { waitUntil: "domcontentloaded" });
  await appPage
    .getByTestId("settings-page-automation-advanced")
    .getByTestId("settings-advanced-toggle")
    .click();
  await expect(appPage.getByTestId("settings-page-automation-max-concurrent-input")).toBeVisible();
  await appPage
    .getByTestId("settings-page-automation-max-concurrent-input")
    .fill(String(nextMaxConcurrent));
  await expect(appPage.getByTestId("settings-page-automation-save")).toBeEnabled();
  await appPage.getByTestId("settings-page-automation-save").click();
  await expect(appPage.getByTestId("settings-page-automation-save-message")).toContainText(
    /restart required/i
  );

  const observabilityBefore = await runtime.requestJSON<{
    config: { retention_days: number };
  }>("/api/settings/observability");
  const nextRetentionDays = observabilityBefore.config.retention_days + 1;
  await appPage.goto(runtime.url("/settings/observability"), { waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("settings-page-observability-retention-days")).toBeVisible();
  await appPage.getByTestId("settings-page-observability-retention-days").fill("-1");
  await expect(appPage.getByTestId("settings-page-observability-save-message")).toContainText(
    "Resolve validation errors"
  );
  await expect(appPage.getByTestId("settings-page-observability-save")).toBeDisabled();
  await appPage
    .getByTestId("settings-page-observability-retention-days")
    .fill(String(nextRetentionDays));
  await expect(appPage.getByTestId("settings-page-observability-save")).toBeEnabled();
  await appPage.getByTestId("settings-page-observability-save").click();
  await expect(appPage.getByTestId("settings-page-observability-save-message")).toContainText(
    /restart required/i
  );

  await appPage.goto(runtime.url("/settings/providers"), { waitUntil: "domcontentloaded" });
  const codexCard = appPage.getByTestId("settings-page-providers-card-codex");
  await expect(codexCard).toBeVisible();
  await codexCard.click();
  const providerDetail = appPage.getByTestId("provider-detail-dialog");
  await expect(providerDetail).toBeVisible();
  await expect(providerDetail.getByRole("heading", { name: "codex" })).toBeVisible();
  // The model catalog sits in the dialog's closed "Technical details" fold.
  await providerDetail
    .getByTestId("provider-detail-technical")
    .getByTestId("settings-advanced-toggle")
    .click();
  await expect(providerDetail.locator('[data-section="catalog"]')).toBeVisible();

  const parity = {
    http: {
      automation: await runtime.requestJSON<unknown>("/api/settings/automation"),
      observability: await runtime.requestJSON<unknown>("/api/settings/observability"),
      provider_catalog: await runtime.requestJSON<unknown>(
        "/api/model-catalog/providers/codex/models/status"
      ),
    },
    uds: {
      automation: await requestOperatorJSON<unknown>(runtime, "/api/settings/automation"),
      observability: await requestOperatorJSON<unknown>(runtime, "/api/settings/observability"),
      provider_catalog: await requestOperatorJSON<unknown>(
        runtime,
        "/api/model-catalog/providers/codex/models/status"
      ),
    },
    cli: {
      automation_max_concurrent_jobs: await runCLIJSON(runtime.paths, [
        "config",
        "get",
        "automation.max_concurrent_jobs",
        "-o",
        "json",
      ]),
      observability_retention_days: await runCLIJSON(runtime.paths, [
        "config",
        "get",
        "observability.retention_days",
        "-o",
        "json",
      ]),
      provider_catalog: await runCLIJSON(runtime.paths, [
        "provider",
        "models",
        "status",
        "codex",
        "-o",
        "json",
      ]),
    },
    config_file_excerpt: await readFile(runtime.paths.configFile, "utf8"),
  };

  expect(JSON.stringify(parity.http.automation)).toContain(
    `"max_concurrent_jobs":${nextMaxConcurrent}`
  );
  expect(JSON.stringify(parity.http.observability)).toContain(
    `"retention_days":${nextRetentionDays}`
  );
  expect(JSON.stringify(parity.cli.automation_max_concurrent_jobs)).toContain(
    `"value":${nextMaxConcurrent}`
  );
  expect(JSON.stringify(parity.cli.observability_retention_days)).toContain(
    `"value":${nextRetentionDays}`
  );
  expect(JSON.stringify(parity)).not.toMatch(sensitivePattern);

  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.captureScreenshot("settings-operational-sections-parity", appPage);
  await captureSettingsViewportMatrix(
    appPage,
    browserArtifacts,
    runtime,
    "/settings/observability"
  );
  await browserArtifacts.persist(appPage);
  await assertNoSettingsSensitiveLeak(appPage, runtime, []);
});

function assertLaunchRuntime(
  runtime: BrowserRuntime,
  context: string
): asserts runtime is BrowserRuntime & { paths: RuntimePaths } {
  if (!runtime.paths) {
    throw new Error(`${context} checks require launch-mode runtime paths.`);
  }
}

async function requestOperatorJSON<T>(
  runtime: BrowserRuntime & { paths: RuntimePaths },
  pathname: string,
  init?: RequestInit
): Promise<T> {
  if (!runtime.requestOperatorJSON) {
    throw new Error(`operator request ${pathname} requires UDS support`);
  }
  return await runtime.requestOperatorJSON<T>(pathname, init);
}

async function runCLIJSON(paths: RuntimePaths, args: string[]): Promise<unknown> {
  const { stdout } = await execFileAsync(paths.cliShim, args, { env: cliEnv(paths) });
  return JSON.parse(extractJSON(stdout)) as unknown;
}

function extractJSON(stdout: string): string {
  const trimmed = stdout.trim();
  const objectIndex = trimmed.indexOf("{");
  const arrayIndex = trimmed.indexOf("[");
  const start = [objectIndex, arrayIndex].filter(index => index >= 0).sort((a, b) => a - b)[0];
  if (start === undefined) {
    throw new Error(`CLI output did not contain JSON: ${stdout}`);
  }

  const stack = [trimmed[start] === "{" ? "}" : "]"];
  let inString = false;
  let escaping = false;
  for (let index = start + 1; index < trimmed.length; index += 1) {
    const char = trimmed[index];
    if (inString) {
      if (escaping) {
        escaping = false;
        continue;
      }
      if (char === "\\") {
        escaping = true;
        continue;
      }
      if (char === '"') {
        inString = false;
      }
      continue;
    }
    if (char === '"') {
      inString = true;
      continue;
    }
    if (char === "{") {
      stack.push("}");
      continue;
    }
    if (char === "[") {
      stack.push("]");
      continue;
    }
    if (char === "}" || char === "]") {
      const expected = stack.pop();
      if (expected !== char) {
        throw new Error(`CLI output did not contain balanced JSON: ${stdout}`);
      }
      if (stack.length === 0) {
        return trimmed.slice(start, index + 1);
      }
    }
  }

  throw new Error(`CLI output did not contain balanced JSON: ${stdout}`);
}

function cliEnv(paths: RuntimePaths): NodeJS.ProcessEnv {
  return {
    ...process.env,
    COMPOZY_HOME: paths.homeDir,
    HOME: paths.operatorHomeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}

async function captureSettingsViewportMatrix(
  appPage: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  runtime: BrowserRuntime,
  pathname: string
): Promise<void> {
  for (const width of [375, 768, 1280]) {
    await appPage.setViewportSize({ width, height: 820 });
    await appPage.goto(runtime.url(pathname), { waitUntil: "domcontentloaded" });
    const settingsWin = appWindow(appPage, "settings");
    await expect(settingsWin.getByTestId("settings-shell")).toBeVisible();
    await expect(settingsWin.getByTestId("settings-section-nav")).toBeVisible();
    const outerBody = windowFrame(settingsWin).locator('[data-slot="os-window-body"]');
    const outerOverflow = await outerBody.evaluate(
      element => element.scrollHeight - element.clientHeight
    );
    expect(outerOverflow).toBeLessThanOrEqual(2);
    if (width === 1280) {
      const slug = pathname.split("/").at(-1);
      if (!slug) throw new Error(`settings pathname has no section slug: ${pathname}`);
      const sectionBody = settingsWin.getByTestId(`settings-page-${slug}-body`);
      await expect(sectionBody).toBeVisible();
      const maxScrollTop = await sectionBody.evaluate(element => {
        const maximum = element.scrollHeight - element.clientHeight;
        element.scrollTop = maximum;
        return maximum;
      });
      expect(maxScrollTop).toBeGreaterThan(0);
      await expect
        .poll(async () => await sectionBody.evaluate(element => element.scrollTop))
        .toBe(maxScrollTop);
    }
    await browserArtifacts.captureScreenshot(`settings-viewport-${width}`, appPage);
  }
}

async function assertNoSettingsSensitiveLeak(
  appPage: Page,
  runtime: BrowserRuntime,
  explicitSecrets: string[]
): Promise<void> {
  await expect(appPage.locator("body")).not.toContainText(sensitivePattern);
  for (const secret of explicitSecrets) {
    await expect(appPage.locator("body")).not.toContainText(secret);
  }

  const payloads = [
    await readFileIfExists(runtime.artifactCollector.artifactPath("browser_console")),
    await readFileIfExists(runtime.artifactCollector.artifactPath("browser_network")),
    await readFileIfExists(runtime.artifactCollector.artifactPath("browser_route_state")),
    await readFileIfExists(runtime.artifactCollector.artifactPath("browser_api_snapshots")),
  ];
  for (const payload of payloads) {
    expect(payload).not.toMatch(sensitivePattern);
    for (const secret of explicitSecrets) {
      expect(payload).not.toContain(secret);
    }
  }
  if (runtime.paths?.daemonLog) {
    const daemonLog = await readFileIfExists(runtime.paths.daemonLog);
    expect(daemonLog).not.toMatch(sensitivePattern);
    for (const secret of explicitSecrets) {
      expect(daemonLog).not.toContain(secret);
    }
  }
}

async function readFileIfExists(filePath: string): Promise<string> {
  try {
    return await readFile(filePath, "utf8");
  } catch (error) {
    const nodeError = error as NodeJS.ErrnoException;
    if (nodeError.code === "ENOENT") {
      return "";
    }
    throw error;
  }
}
