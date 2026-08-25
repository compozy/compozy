import { expect, type Locator, type Page } from "@playwright/test";

import { switchWorkspace } from "./os-navigation";
import type { BrowserRuntime } from "./runtime";

export interface WorkspaceShellSelectors {
  osDesktop: Locator;
}

interface WorkspaceShell {
  osDesktop: Locator;
  firstRunOnboarding: Locator;
  page: Page;
}

type WorkspaceShellInput = Page | WorkspaceShellSelectors;

export async function ensureProjectWorkspace(page: Page, runtime: BrowserRuntime): Promise<void> {
  const workspace =
    runtime.seeded.workspace ??
    (runtime.paths?.workspaceDir
      ? await runtime.resolveWorkspace(runtime.paths.workspaceDir)
      : undefined);
  if (!workspace) return;

  await completeOnboardingIfPrompted(page);
  await switchWorkspace(page, workspace.id, workspace.name);
}

export async function completeOnboardingIfPrompted(input: WorkspaceShellInput): Promise<void> {
  const ui = resolveWorkspaceShell(input);

  await Promise.race([
    ui.firstRunOnboarding.waitFor({ state: "visible", timeout: 20_000 }).catch(() => null),
    ui.osDesktop.waitFor({ state: "visible", timeout: 20_000 }).catch(() => null),
  ]);

  if (await ui.firstRunOnboarding.isVisible().catch(() => false)) {
    await completeFirstRunOnboarding(ui.page);
  }

  await expect(ui.osDesktop).toBeVisible({ timeout: 20_000 });
}

function resolveWorkspaceShell(input: WorkspaceShellInput): WorkspaceShell {
  if (isPage(input)) {
    return {
      osDesktop: input.getByTestId("os-desktop"),
      firstRunOnboarding: input.getByTestId("onboarding-setup-panel"),
      page: input,
    };
  }

  return {
    osDesktop: input.osDesktop,
    firstRunOnboarding: input.osDesktop.page().getByTestId("onboarding-setup-panel"),
    page: input.osDesktop.page(),
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
      const persona = await requestJSON<{
        config?: { agent?: string; provider?: string; sandbox?: string };
      }>("/api/settings/persona?scope=user");
      if (!persona.config) {
        throw new Error("First-run E2E bootstrap could not load default profile settings.");
      }
      await requestJSON("/api/settings/persona?scope=user", {
        body: JSON.stringify({
          config: {
            ...persona.config,
            provider: provider.name,
          },
        }),
        method: "PATCH",
      });
    }

    await configureDefaultProvider();
    await requestJSON("/api/onboarding/complete", { method: "POST" });
  });
  await page.reload({ waitUntil: "domcontentloaded" });
}
