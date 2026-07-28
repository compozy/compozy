import { expect, type Locator, type Page } from "@playwright/test";

import type { BrowserRuntime } from "./runtime";

export interface WorkspaceShellSelectors {
  osDesktop: Locator;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

interface WorkspaceShell {
  osDesktop: Locator;
  firstRunOnboarding: Locator;
  page: Page;
  workspaceOnboarding: Locator;
  workspaceUseGlobal: Locator;
}

type WorkspaceShellInput = Page | WorkspaceShellSelectors;

export async function ensureGlobalWorkspace(runtime: BrowserRuntime): Promise<void> {
  if (runtime.seeded.workspace || !runtime.paths?.homeDir) {
    return;
  }
  await runtime.resolveWorkspace(runtime.paths.homeDir);
}

export async function useGlobalWorkspaceIfPrompted(input: WorkspaceShellInput): Promise<void> {
  const ui = resolveWorkspaceShell(input);

  await Promise.race([
    ui.firstRunOnboarding.waitFor({ state: "visible", timeout: 20_000 }).catch(() => null),
    ui.workspaceOnboarding.waitFor({ state: "visible", timeout: 20_000 }).catch(() => null),
    ui.osDesktop.waitFor({ state: "visible", timeout: 20_000 }).catch(() => null),
  ]);

  if (await ui.firstRunOnboarding.isVisible().catch(() => false)) {
    await completeFirstRunOnboarding(ui.page);
  }

  if (await ui.workspaceOnboarding.isVisible().catch(() => false)) {
    await ui.workspaceUseGlobal.click();
    await expect(ui.workspaceOnboarding).toBeHidden();
  }

  await expect(ui.osDesktop).toBeVisible({ timeout: 20_000 });
}

function resolveWorkspaceShell(input: WorkspaceShellInput): WorkspaceShell {
  if (isPage(input)) {
    return {
      osDesktop: input.getByTestId("os-desktop"),
      firstRunOnboarding: input.getByTestId("onboarding-setup-panel"),
      page: input,
      workspaceOnboarding: input.getByTestId("workspace-onboarding"),
      workspaceUseGlobal: input.getByTestId("workspace-use-global"),
    };
  }

  return {
    osDesktop: input.osDesktop,
    firstRunOnboarding: input.osDesktop.page().getByTestId("onboarding-setup-panel"),
    page: input.osDesktop.page(),
    workspaceOnboarding: input.workspaceOnboarding,
    workspaceUseGlobal: input.workspaceUseGlobal,
  };
}

function isPage(input: WorkspaceShellInput): input is Page {
  return "goto" in input && "getByTestId" in input;
}

async function completeFirstRunOnboarding(page: Page): Promise<void> {
  await page.evaluate(async () => {
    async function requestJSON<T>(pathname: string, init?: RequestInit): Promise<T> {
      const headers = new Headers(init?.headers);
      if (init?.body !== undefined) {
        headers.set("content-type", "application/json");
      }
      const response = await fetch(pathname, {
        ...init,
        headers,
      });
      if (!response.ok) {
        const body = await response.text();
        throw new Error(`${pathname} failed with ${response.status}: ${body}`);
      }
      return (await response.json()) as T;
    }

    async function configureDefaultProvider(): Promise<void> {
      const providers = await requestJSON<{
        providers?: Array<{
          default?: boolean;
          name?: string;
          settings?: Record<string, unknown>;
        }>;
      }>("/api/settings/providers");
      const candidates = providers.providers ?? [];
      const provider =
        candidates.find(candidate => candidate.name === "acpmock") ??
        candidates.find(candidate => candidate.default === true) ??
        candidates.find(candidate => candidate.name === "codex");
      if (!provider?.name || !provider.settings) {
        return;
      }

      await requestJSON(`/api/settings/providers/${encodeURIComponent(provider.name)}`, {
        body: JSON.stringify({ settings: provider.settings }),
        method: "PUT",
      });
      const general = await requestJSON<{ config?: { defaults?: Record<string, unknown> } }>(
        "/api/settings/general"
      );
      if (!general.config) {
        throw new Error("First-run E2E bootstrap could not load general settings.");
      }
      await requestJSON("/api/settings/general", {
        body: JSON.stringify({
          config: {
            ...general.config,
            defaults: {
              ...general.config.defaults,
              provider: provider.name,
            },
          },
        }),
        method: "PATCH",
      });
    }

    const browse = await requestJSON<{ home?: string; path?: string }>(
      "/api/fs/browse?dirs_only=true"
    );
    const workspaces = await requestJSON<{ workspaces?: unknown[] }>("/api/workspaces");
    if ((workspaces.workspaces?.length ?? 0) === 0) {
      const workspacePath = browse.home?.trim() || browse.path?.trim();
      if (!workspacePath) {
        throw new Error("First-run E2E bootstrap could not resolve a workspace path.");
      }

      await requestJSON("/api/workspaces/resolve", {
        body: JSON.stringify({ path: workspacePath }),
        method: "POST",
      });
    }

    await configureDefaultProvider();
    await requestJSON("/api/onboarding/complete", { method: "POST" });
  });
  await page.reload({ waitUntil: "domcontentloaded" });
}
