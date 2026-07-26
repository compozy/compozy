// @vitest-environment node

import { describe, expect, it } from "vitest";

import {
  closeSkillMarketplaceServer,
  resolveBrowserRuntimeEnv,
  startSkillMarketplaceServer,
} from "../runtime";
import {
  closeMarketplaceCatalogServer,
  startMarketplaceCatalogServer,
} from "../marketplace-server";
import { stopBrowserDaemonProcess, stopRegisteredDaemon } from "../runtime-process";
import {
  assertDaemonServedHTML,
  buildLaunchRuntimeEnv,
  buildResolveWorkspaceRequest,
  normalizeBaseURL,
  requiresHTTPAPIReadinessProbe,
  renderRuntimeConfig,
  resolveRuntimeMode,
  runtimeURL,
} from "../runtime-helpers";

describe("runtime helpers", () => {
  it("preserves lane control variables without leaking unrelated ambient variables", () => {
    expect(
      resolveBrowserRuntimeEnv(
        {
          AGH_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
          PATH: "/runtime/bin",
        },
        {
          AGH_TEST_ACPMOCK_DRIVER_BIN: "/lane/acpmock-driver",
          AGH_TEST_DAEMON_BIN: "/lane/agh",
          AGH_WEB_DIST_DIR: "/lane/web-dist",
          OPERATOR_SECRET: "must-not-propagate",
        }
      )
    ).toEqual({
      AGH_TEST_ACPMOCK_DRIVER_BIN: "/lane/acpmock-driver",
      AGH_TEST_DAEMON_BIN: "/lane/agh",
      AGH_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
      AGH_WEB_DIST_DIR: "/lane/web-dist",
      PATH: "/runtime/bin",
    });
  });

  it("stops the registered restart replacement before the originally spawned daemon", async () => {
    const calls: string[] = [];
    const child = {} as Parameters<typeof stopBrowserDaemonProcess>[0];

    await stopBrowserDaemonProcess(
      child,
      {
        cliShim: "/tmp/agh-home/bin/agh",
        homeDir: "/tmp/agh-home",
        repoRoot: "/tmp/repo",
      },
      {
        executeFile: async (file, args, options) => {
          calls.push("registered");
          expect(file).toBe("/tmp/agh-home/bin/agh");
          expect(args).toEqual(["daemon", "stop", "-o", "json"]);
          expect(options).toMatchObject({
            cwd: "/tmp/repo",
            env: { AGH_HOME: "/tmp/agh-home", HOME: "/tmp/agh-home" },
          });
          return { stderr: "", stdout: "{}" };
        },
        stopSpawned: async receivedChild => {
          expect(receivedChild).toBe(child);
          calls.push("spawned");
        },
      }
    );

    expect(calls).toEqual(["registered", "spawned"]);
  });

  it("treats an already stopped registered daemon as idempotent cleanup", async () => {
    await expect(
      stopRegisteredDaemon(
        {
          cliShim: "/tmp/agh-home/bin/agh",
          homeDir: "/tmp/agh-home",
          repoRoot: "/tmp/repo",
        },
        async () => {
          throw new Error("cli: daemon is not running");
        }
      )
    ).resolves.toBeUndefined();
  });

  it("defaults to launch mode when no attach URL is configured", () => {
    expect(resolveRuntimeMode({})).toEqual({ kind: "launch" });
  });

  it("normalizes attach mode base URLs", () => {
    expect(resolveRuntimeMode({ AGH_E2E_BASE_URL: "http://127.0.0.1:4213/" })).toEqual({
      kind: "attach",
      baseURL: "http://127.0.0.1:4213",
    });
  });

  it("rejects attach URLs that target a non-root path", () => {
    expect(() => normalizeBaseURL("http://127.0.0.1:4213/ui")).toThrow(
      /AGH_E2E_BASE_URL must point at the daemon root/
    );
  });

  it("renders the seeded daemon config with socket and HTTP bindings", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        port: 4321,
        socketPath: "/tmp/agh.sock",
      })
    ).toBe(
      [
        "[daemon]",
        'socket = "/tmp/agh.sock"',
        "",
        "[http]",
        'host = "127.0.0.1"',
        "port = 4321",
        "",
      ].join("\n")
    );
  });

  it("renders network enablement when requested by the browser runtime", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        networkEnabled: true,
        port: 4321,
        socketPath: "/tmp/agh.sock",
      })
    ).toContain("[network]\nenabled = true\n");
  });

  it("renders models.dev disablement when a browser scenario requires offline catalog refresh", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        modelsDevEnabled: false,
        port: 4321,
        socketPath: "/tmp/agh.sock",
      })
    ).toContain("[model_catalog.sources.models_dev]\nenabled = false\n");
  });

  it("renders network disablement when requested by the browser runtime", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        networkEnabled: false,
        port: 4321,
        socketPath: "/tmp/agh.sock",
      })
    ).toContain("[network]\nenabled = false\n");
  });

  it("Should render unverified extension policy only when explicitly requested", () => {
    expect(
      renderRuntimeConfig({
        extensionsAllowUnverified: true,
        host: "127.0.0.1",
        port: 4321,
        socketPath: "/tmp/agh.sock",
      })
    ).toContain("[extensions.marketplace]\nallow_unverified = true\n");
  });

  it("renders a seeded skill marketplace base URL for launch-mode browser E2E", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        port: 4321,
        skillsMarketplaceBaseURL: "http://127.0.0.1:9876",
        socketPath: "/tmp/agh.sock",
      })
    ).toContain(
      [
        "[skills.marketplace]",
        'registry = "clawhub"',
        'base_url = "http://127.0.0.1:9876"',
        "",
      ].join("\n")
    );
  });

  it("renders a seeded curated marketplace base URL for launch-mode browser E2E", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        marketplaceCatalogBaseURL: "http://127.0.0.1:8765",
        port: 4321,
        socketPath: "/tmp/agh.sock",
      })
    ).toContain(
      [
        "[marketplace.catalog]",
        'base_url = "http://127.0.0.1:8765"',
        'ttl = "1h"',
        'timeout = "5s"',
        "",
      ].join("\n")
    );
  });

  it("serves strict per-kind curated marketplace documents", async () => {
    const marketplace = await startMarketplaceCatalogServer({
      generatedAt: "2026-07-14T00:00:00Z",
      mcp: [
        {
          command: "npx",
          description: "Read repository metadata",
          entry_id: "browser-github-mcp",
          name: "Browser GitHub MCP",
          transport: "stdio",
        },
      ],
      skills: [
        {
          description: "Review browser changes",
          entry_id: "browser-review-skill",
          install_slug: "@agh/browser-review-skill",
          name: "Browser review skill",
        },
      ],
    });
    if (marketplace === undefined) {
      throw new Error("expected seeded curated marketplace server");
    }
    try {
      const mcpResponse = await fetch(`${marketplace.baseURL}/mcp.json`);
      expect(mcpResponse.status).toBe(200);
      await expect(mcpResponse.json()).resolves.toEqual({
        manifest_version: 1,
        generated_at: "2026-07-14T00:00:00Z",
        entries: [
          {
            command: "npx",
            description: "Read repository metadata",
            entry_id: "browser-github-mcp",
            name: "Browser GitHub MCP",
            transport: "stdio",
          },
        ],
      });
      const extensionResponse = await fetch(`${marketplace.baseURL}/extensions.json`);
      await expect(extensionResponse.json()).resolves.toMatchObject({ entries: [] });
      expect((await fetch(`${marketplace.baseURL}/unknown.json`)).status).toBe(404);
    } finally {
      await closeMarketplaceCatalogServer(marketplace.server);
    }
  });

  it("serves seeded skill marketplace listings through the ClawHub search contract", async () => {
    const marketplace = await startSkillMarketplaceServer({
      listings: [
        {
          author: "agh",
          description: "Marketplace metadata visible through the daemon catalog.",
          downloads: 12,
          name: "browser-marketplace-skill",
          slug: "@agh/browser-marketplace-skill",
          version: "2.0.0",
        },
      ],
    });
    if (marketplace === undefined) {
      throw new Error("expected seeded marketplace test server");
    }
    try {
      const oldPathResponse = await fetch(`${marketplace.baseURL}/api/v1/skills?q=browser`);
      expect(oldPathResponse.status).toBe(404);
      await expect(oldPathResponse.json()).resolves.toEqual({ error: "not_found" });

      const missingTypeResponse = await fetch(
        `${marketplace.baseURL}/api/v1/search?q=browser-marketplace`
      );
      expect(missingTypeResponse.status).toBe(400);
      await expect(missingTypeResponse.json()).resolves.toEqual({ error: "skill_type_required" });

      const response = await fetch(
        `${marketplace.baseURL}/api/v1/search?q=browser-marketplace&type=skill&limit=1`
      );
      expect(response.status).toBe(200);
      await expect(response.json()).resolves.toEqual({
        results: [
          {
            author: "agh",
            description: "Marketplace metadata visible through the daemon catalog.",
            downloads: 12,
            name: "browser-marketplace-skill",
            slug: "@agh/browser-marketplace-skill",
            source: "clawhub",
            type: "skill",
            version: "2.0.0",
          },
        ],
      });

      const detailResponse = await fetch(
        `${marketplace.baseURL}/api/v1/skills/${encodeURIComponent("@agh/browser-marketplace-skill")}`
      );
      expect(detailResponse.status).toBe(200);
      await expect(detailResponse.json()).resolves.toMatchObject({
        author: "agh",
        name: "browser-marketplace-skill",
        slug: "@agh/browser-marketplace-skill",
        version: "2.0.0",
      });
      const downloadResponse = await fetch(
        `${marketplace.baseURL}/api/v1/skills/${encodeURIComponent("@agh/browser-marketplace-skill")}/download`
      );
      expect(downloadResponse.status).toBe(404);
    } finally {
      await closeSkillMarketplaceServer(marketplace.server);
    }
  });

  it("renders the auth-free acpmock provider when browser E2E seeds mock agents", () => {
    const config = renderRuntimeConfig({
      host: "127.0.0.1",
      includeMockAgentProvider: true,
      port: 4321,
      socketPath: "/tmp/agh.sock",
    });

    expect(config).toContain(
      [
        "[providers.acpmock]",
        'command = "acpmock-driver"',
        'display_name = "ACP Mock"',
        'harness = "acp"',
        'auth_mode = "none"',
        'none_security = "local_transport"',
        "",
      ].join("\n")
    );
    expect(config).toContain(
      [
        "[providers.acpmock.models.reasoning]",
        'apply = "acp_option"',
        "[[providers.acpmock.models.curated]]",
        'id = "qa-browser-model-alt"',
        'display_name = "QA Browser Model Alt"',
        "supports_tools = true",
        "supports_reasoning = true",
        'reasoning_efforts = ["low", "medium", "high"]',
        'default_reasoning_effort = "medium"',
        "",
      ].join("\n")
    );
    expect(config).not.toContain("[providers.acpmock.reasoning]");
  });

  it("requires API readiness probes only for loopback HTTP bindings", () => {
    expect(requiresHTTPAPIReadinessProbe("")).toBe(true);
    expect(requiresHTTPAPIReadinessProbe("localhost")).toBe(true);
    expect(requiresHTTPAPIReadinessProbe("127.0.0.1")).toBe(true);
    expect(requiresHTTPAPIReadinessProbe("[::1]")).toBe(true);
    expect(requiresHTTPAPIReadinessProbe("0.0.0.0")).toBe(false);
    expect(requiresHTTPAPIReadinessProbe("192.168.1.10")).toBe(false);
  });

  it("joins runtime URLs against the daemon origin", () => {
    expect(runtimeURL("http://127.0.0.1:4317", "/api/status")).toBe(
      "http://127.0.0.1:4317/api/status"
    );
    expect(runtimeURL("http://127.0.0.1:4317/", "api/workspaces")).toBe(
      "http://127.0.0.1:4317/api/workspaces"
    );
  });

  it("encodes workspace resolution requests with the public path contract", () => {
    expect(buildResolveWorkspaceRequest("/tmp/agh-home")).toEqual({
      path: "/tmp/agh-home",
    });
  });

  it("rejects vite dev HTML when enforcing daemon-served assets", () => {
    expect(() =>
      assertDaemonServedHTML(
        '<!doctype html><html><head><script type="module" src="/@vite/client"></script></head></html>',
        "http://127.0.0.1:3000"
      )
    ).toThrow(/daemon-served embedded assets/);
  });

  it("accepts built embedded HTML without vite dev markers", () => {
    expect(() =>
      assertDaemonServedHTML(
        '<!doctype html><html><head><script type="module" src="/assets/index-abc123.js"></script></head></html>',
        "http://127.0.0.1:4213"
      )
    ).not.toThrow();
  });

  it("serves the freshly built checkout bundle in launch mode", () => {
    expect(
      buildLaunchRuntimeEnv("/work/agh", {
        AGH_WEB_DIST_DIR: "/tmp/stale-dist",
        PATH: "/usr/bin",
      })
    ).toMatchObject({
      AGH_WEB_DIST_DIR: "/work/agh/web/dist",
      PATH: "/usr/bin",
    });
  });
});
