// @vitest-environment node

import { access, mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { describe, expect, it } from "vitest";

import { cleanupBrowserRuntimePaths, resolveBrowserRuntimeEnv } from "../runtime";
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
  it("removes every launch-owned path during runtime cleanup", async () => {
    const testRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-runtime-cleanup-test-"));
    const paths = {
      cliShim: path.join(testRoot, "home", "bin", "compozy"),
      configFile: path.join(testRoot, "home", "config.toml"),
      daemonLog: path.join(testRoot, "home", "logs", "daemon.log"),
      daemonSocket: path.join(testRoot, "daemon.sock"),
      homeDir: path.join(testRoot, "home"),
      operatorHomeDir: path.join(testRoot, "operator-home"),
      workspaceDir: path.join(testRoot, "workspace"),
    };
    try {
      await Promise.all([
        mkdir(paths.homeDir, { recursive: true }),
        mkdir(paths.operatorHomeDir, { recursive: true }),
        mkdir(paths.workspaceDir, { recursive: true }),
        writeFile(paths.daemonSocket, "socket placeholder"),
      ]);

      await cleanupBrowserRuntimePaths(paths);

      await Promise.all(
        [paths.homeDir, paths.operatorHomeDir, paths.workspaceDir, paths.daemonSocket].map(
          async ownedPath => await expect(access(ownedPath)).rejects.toThrow()
        )
      );
    } finally {
      await rm(testRoot, { force: true, recursive: true });
    }
  });

  it("preserves lane control variables without leaking unrelated ambient variables", () => {
    expect(
      resolveBrowserRuntimeEnv(
        {
          COMPOZY_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
          PATH: "/runtime/bin",
        },
        {
          COMPOZY_TEST_ACPMOCK_DRIVER_BIN: "/lane/acpmock-driver",
          COMPOZY_TEST_DAEMON_BIN: "/lane/compozy",
          COMPOZY_WEB_DIST_DIR: "/lane/web-dist",
          OPERATOR_SECRET: "must-not-propagate",
        }
      )
    ).toEqual({
      COMPOZY_TEST_ACPMOCK_DRIVER_BIN: "/lane/acpmock-driver",
      COMPOZY_TEST_DAEMON_BIN: "/lane/compozy",
      COMPOZY_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
      COMPOZY_WEB_DIST_DIR: "/lane/web-dist",
      PATH: "/runtime/bin",
    });
  });

  it("stops the registered restart replacement after the originally spawned daemon exited", async () => {
    const calls: string[] = [];
    const child = { exitCode: 0 } as Parameters<typeof stopBrowserDaemonProcess>[0];

    await stopBrowserDaemonProcess(
      child,
      {
        cliShim: "/tmp/compozy-home/bin/compozy",
        homeDir: "/tmp/compozy-home",
        operatorHomeDir: "/tmp/operator-home",
        repoRoot: "/tmp/repo",
      },
      {
        executeFile: async (file, args, options) => {
          calls.push("registered");
          expect(file).toBe("/tmp/compozy-home/bin/compozy");
          expect(args).toEqual(["daemon", "stop", "-o", "json"]);
          expect(options).toMatchObject({
            cwd: "/tmp/repo",
            env: { COMPOZY_HOME: "/tmp/compozy-home", HOME: "/tmp/operator-home" },
            timeout: 90_000,
          });
          return { stderr: "", stdout: "{}" };
        },
        stopSpawned: async () => {
          throw new Error("the exited original process must not be stopped twice");
        },
      }
    );

    expect(calls).toEqual(["registered"]);
  });

  it("stops the owned live process without issuing a duplicate CLI stop", async () => {
    const calls: string[] = [];
    const child = { exitCode: null } as Parameters<typeof stopBrowserDaemonProcess>[0];

    await stopBrowserDaemonProcess(
      child,
      {
        cliShim: "/tmp/compozy-home/bin/compozy",
        homeDir: "/tmp/compozy-home",
        operatorHomeDir: "/tmp/operator-home",
        repoRoot: "/tmp/repo",
      },
      {
        executeFile: async () => {
          throw new Error("the live owned process must not receive a duplicate CLI stop");
        },
        stopSpawned: async receivedChild => {
          expect(receivedChild).toBe(child);
          calls.push("spawned");
        },
      }
    );

    expect(calls).toEqual(["spawned"]);
  });

  it("treats an already stopped registered daemon as idempotent cleanup", async () => {
    await expect(
      stopRegisteredDaemon(
        {
          cliShim: "/tmp/compozy-home/bin/compozy",
          homeDir: "/tmp/compozy-home",
          operatorHomeDir: "/tmp/operator-home",
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
    expect(resolveRuntimeMode({ COMPOZY_E2E_BASE_URL: "http://127.0.0.1:4213/" })).toEqual({
      kind: "attach",
      baseURL: "http://127.0.0.1:4213",
    });
  });

  it("rejects attach URLs that target a non-root path", () => {
    expect(() => normalizeBaseURL("http://127.0.0.1:4213/ui")).toThrow(
      /COMPOZY_E2E_BASE_URL must point at the daemon root/
    );
  });

  it("renders the seeded daemon config with socket and HTTP bindings", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        port: 4321,
        socketPath: "/tmp/compozy.sock",
      })
    ).toBe(
      [
        "[daemon]",
        'socket = "/tmp/compozy.sock"',
        "",
        "[http]",
        'host = "127.0.0.1"',
        "port = 4321",
        "",
        "[mcp.oauth]",
        'redirect_uri = "http://127.0.0.1:4321/api/mcp/oauth/callback"',
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
        socketPath: "/tmp/compozy.sock",
      })
    ).toContain("[network]\nenabled = true\n");
  });

  it("renders models.dev disablement when a browser scenario requires offline catalog refresh", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        modelsDevEnabled: false,
        port: 4321,
        socketPath: "/tmp/compozy.sock",
      })
    ).toContain("[model_catalog.sources.models_dev]\nenabled = false\n");
  });

  it("renders network disablement when requested by the browser runtime", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        networkEnabled: false,
        port: 4321,
        socketPath: "/tmp/compozy.sock",
      })
    ).toContain("[network]\nenabled = false\n");
  });

  it("Should render unverified extension policy only when explicitly requested", () => {
    expect(
      renderRuntimeConfig({
        extensionsAllowUnverified: true,
        host: "127.0.0.1",
        port: 4321,
        socketPath: "/tmp/compozy.sock",
      })
    ).toContain("[extensions.trust]\nallow_unverified = true\n");
  });

  it("renders a seeded curated marketplace base URL for launch-mode browser E2E", () => {
    expect(
      renderRuntimeConfig({
        host: "127.0.0.1",
        marketplaceCatalogBaseURL: "http://127.0.0.1:8765",
        port: 4321,
        socketPath: "/tmp/compozy.sock",
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

  // Invariant: browser labs serve only v3 extension and preset documents.
  // Owner: real HTTP fixture; canonical runtime helper suite.
  it("serves the v3 catalog and source presets without retired feeds", async () => {
    const preset = {
      name: "team",
      source: "github:acme/plugins",
      description: "Team plugins",
      default: "on" as const,
    };
    const marketplace = await startMarketplaceCatalogServer({
      generatedAt: "2026-09-13T00:00:00Z",
      presets: [preset],
    });
    if (!marketplace) throw new Error("expected catalog fixture");
    try {
      const extensions = await fetch(`${marketplace.baseURL}/v3/extensions.json`);
      expect(extensions.status).toBe(200);
      await expect(extensions.json()).resolves.toEqual({
        manifest_version: 3,
        generated_at: "2026-09-13T00:00:00Z",
        entries: [],
      });
      const sources = await fetch(`${marketplace.baseURL}/v3/marketplaces.json`);
      expect(sources.status).toBe(200);
      await expect(sources.json()).resolves.toEqual({
        manifest_version: 3,
        generated_at: "2026-09-13T00:00:00Z",
        entries: [preset],
      });
      for (const path of ["/mcp.json", "/skills.json", "/extensions.json", "/unknown.json"]) {
        expect((await fetch(`${marketplace.baseURL}${path}`)).status).toBe(404);
      }
    } finally {
      await closeMarketplaceCatalogServer(marketplace.server);
    }
  });

  it("renders the auth-free acpmock provider when browser E2E seeds mock agents", () => {
    const config = renderRuntimeConfig({
      host: "127.0.0.1",
      includeMockAgentProvider: true,
      port: 4321,
      socketPath: "/tmp/compozy.sock",
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
    expect(buildResolveWorkspaceRequest("/tmp/compozy-home")).toEqual({
      path: "/tmp/compozy-home",
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
      buildLaunchRuntimeEnv("/work/compozy", {
        COMPOZY_WEB_DIST_DIR: "/tmp/stale-dist",
        PATH: "/usr/bin",
      })
    ).toMatchObject({
      COMPOZY_WEB_DIST_DIR: "/work/compozy/web/dist",
      PATH: "/usr/bin",
    });
  });
});
