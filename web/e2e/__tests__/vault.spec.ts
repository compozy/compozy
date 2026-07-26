import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import { sessionLifecycleSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);

const sensitivePattern =
  /agh_claim_[a-z0-9._-]+|["']claim_token["']\s*:\s*["']?[a-z0-9._-]{8,}|(?:authorization\s*:\s*bearer|bearer)\s+["']?[a-z0-9._-]{8,}|(?:api[_-]?key|bearer[_-]?token|mcp[_-]?auth|oauth[_-]?(?:access(?:[_-]?token)?|client(?:[_-]?secret)?|refresh(?:[_-]?token)?|secret|token)|pkce[_-]?(?:challenge|secret|verifier)|provider[_-]?credential|browser-settings-secret)\s*[:=]\s*["']?[a-z0-9._:-]{8,}/i;

test("operator can inspect and delete a session-scoped vault secret from the vault route", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const ref = "vault:sessions/browser_e2e_vault/api_key";

  await runtime.requestJSON<{ secret: { ref: string } }>("/api/vault/secrets", {
    method: "PUT",
    body: JSON.stringify({
      ref,
      kind: "api_key",
      secret_value: "browser-e2e-vault-token",
    }),
  });

  try {
    await ensureGlobalWorkspace(runtime);
    await appPage.goto(runtime.url("/vault"), { waitUntil: "domcontentloaded" });
    await useGlobalWorkspaceIfPrompted(sessionLifecycleSelectors(appPage));

    await expect(appPage.getByTestId("vault-shell")).toBeVisible({ timeout: 20_000 });
    await expect(appPage.getByTestId("vault-page-list")).toBeVisible();
    await expect(appPage.getByTestId(`vault-secrets-delete-${ref}`)).toBeVisible();

    await appPage.getByTestId(`vault-secrets-delete-${ref}`).click();
    await expect(appPage.getByTestId("settings-vault-delete")).toBeVisible();
    await expect(appPage.getByTestId("settings-vault-delete-description")).toContainText(ref);
    await confirmVaultSecretDelete(appPage, ref);

    await expect(appPage.getByTestId("vault-page-action-result")).toContainText(
      "Deleted vault secret"
    );
    await expect(appPage.getByTestId(`vault-secrets-delete-${ref}`)).not.toBeVisible();

    const payload = await runtime.requestJSON<{ secrets: Array<{ ref: string }> }>(
      "/api/vault/secrets?namespace=sessions"
    );
    expect(payload.secrets.some(secret => secret.ref === ref)).toBe(false);
    await browserArtifacts.captureScreenshot("tc-func-013-vault-list-delete", appPage);
  } finally {
    await deleteVaultSecretIfPresent(
      runtime.url(`/api/vault/secrets?ref=${encodeURIComponent(ref)}`)
    );
  }
});

test("operator stores and deletes a vault secret without plaintext readback", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  assertLaunchRuntime(runtime, "vault lifecycle");

  const secretRef = `vault:providers/browser-settings-secret-${Date.now()}`;
  const secretValue = "browser-settings-secret-value-11";

  await ensureGlobalWorkspace(runtime);
  await appPage.goto(runtime.url("/vault"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(sessionLifecycleSelectors(appPage));
  await expect(appPage.getByTestId("vault-shell")).toBeVisible({ timeout: 20_000 });
  await expect(appPage.getByTestId("vault-page-create")).toBeVisible();

  await appPage.getByTestId("vault-page-create").click();
  await expect(appPage.getByTestId("settings-vault-editor")).toBeVisible();
  await appPage.getByTestId("settings-vault-editor-ref-input").fill("providers/bad-ref");
  await appPage.getByTestId("settings-vault-editor-kind-input").fill("api_key");
  await appPage.getByTestId("settings-vault-editor-secret-value-input").fill(secretValue);
  await expect(appPage.getByTestId("settings-vault-editor-error")).toContainText(
    "Vault refs must start with vault:."
  );
  await expect(appPage.getByTestId("settings-vault-editor-save")).toBeDisabled();

  await appPage.getByTestId("settings-vault-editor-ref-input").fill(secretRef);
  await expect(appPage.getByTestId("settings-vault-editor-save")).toBeEnabled();
  await appPage.getByTestId("settings-vault-editor-save").click();

  await expect(appPage.getByTestId("settings-vault-editor")).toBeHidden();
  await expect(appPage.getByTestId("vault-page-action-result")).toContainText(secretRef);
  await expect(appPage.locator("body")).not.toContainText(secretValue);

  await selectVaultNamespace(appPage, "providers");
  await appPage.getByTestId("vault-page-prefix").fill(secretRef);
  await expect(appPage.getByTestId("vault-secrets-row")).toHaveCount(1);
  await expect(appPage.getByTestId("vault-secrets-row")).toContainText(secretRef);
  await expect(appPage.getByTestId("vault-secrets-row")).toContainText("api_key");
  await expect(appPage.getByTestId("vault-secrets-row")).not.toContainText(secretValue);

  const httpMetadata = await runtime.requestJSON<unknown>(
    `/api/vault/secrets/metadata?ref=${encodeURIComponent(secretRef)}`
  );
  const udsMetadata = await requestOperatorJSON<unknown>(
    runtime,
    `/api/vault/secrets/metadata?ref=${encodeURIComponent(secretRef)}`
  );
  const cliMetadata = await runCLIJSON(runtime.paths, ["vault", "get", secretRef, "-o", "json"]);
  const cliList = await runCLIJSON(runtime.paths, [
    "vault",
    "list",
    "--namespace",
    "providers",
    "--prefix",
    secretRef,
    "-o",
    "json",
  ]);

  const snapshot = {
    http_metadata: httpMetadata,
    uds_metadata: udsMetadata,
    cli_metadata: cliMetadata,
    cli_list: cliList,
    ui_row_count: await appPage.getByTestId("vault-secrets-row").count(),
  };
  expect(JSON.stringify(snapshot)).not.toContain(secretValue);
  expect(JSON.stringify(snapshot)).not.toMatch(sensitivePattern);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", snapshot);
  await browserArtifacts.captureScreenshot("vault-lifecycle-desktop", appPage);
  await captureVaultViewportMatrix(appPage, browserArtifacts, runtime);

  await appPage.getByTestId(`vault-secrets-delete-${secretRef}`).click();
  await expect(appPage.getByTestId("settings-vault-delete")).toBeVisible();
  await expect(appPage.getByTestId("settings-vault-delete-description")).toContainText(secretRef);
  await confirmVaultSecretDelete(appPage, secretRef);

  await expect(appPage.getByTestId("vault-page-action-result")).toContainText("Deleted");
  await expect(appPage.getByTestId("vault-secrets-row")).toHaveCount(0);

  const deletedResponse = await appPage.request.get(
    runtime.url(`/api/vault/secrets/metadata?ref=${encodeURIComponent(secretRef)}`)
  );
  expect(deletedResponse.status()).toBe(404);

  await browserArtifacts.persist(appPage);
  await assertNoVaultSensitiveLeak(appPage, runtime, [secretValue]);
});

async function deleteVaultSecretIfPresent(url: string) {
  const response = await fetch(url, { method: "DELETE" });
  if (response.ok || response.status === 404) return;
  const body = await response.text();
  throw new Error(`cleanup delete vault secret failed with ${response.status}: ${body.trim()}`);
}

async function confirmVaultSecretDelete(page: Page, ref: string): Promise<void> {
  const typingInput = page.getByTestId("settings-vault-delete-confirm-typing");
  if (await typingInput.isVisible().catch(() => false)) {
    await typingInput.fill(ref);
  }
  await page.getByTestId("settings-vault-delete-confirm").click();
}

async function selectVaultNamespace(page: Page, namespace: string): Promise<void> {
  await page.getByTestId("vault-list-filters-add").click();
  const namespaceField = page.getByRole("option", { name: "Namespace", exact: true });
  await namespaceField.hover();
  const namespaceOption = page.getByRole("option", { name: namespace, exact: true });
  await expect(namespaceOption).toBeVisible();
  await namespaceOption.click();
}

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
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}

async function captureVaultViewportMatrix(
  appPage: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  runtime: BrowserRuntime
): Promise<void> {
  const sessionUI = sessionLifecycleSelectors(appPage);
  for (const width of [375, 768, 1280]) {
    await appPage.setViewportSize({ width, height: 820 });
    await appPage.goto(runtime.url("/vault"), { waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("vault-shell")).toBeVisible();
    await expect(sessionUI.osDesktop).toBeVisible();
    await browserArtifacts.captureScreenshot(`vault-viewport-${width}`, appPage);
  }
}

async function assertNoVaultSensitiveLeak(
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
