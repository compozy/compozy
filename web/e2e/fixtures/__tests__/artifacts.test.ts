// @vitest-environment node

import { mkdtemp, mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { describe, expect, it } from "vitest";

import {
  ArtifactCollector,
  type BrowserRouteState,
  isLikelyViteDevHTML,
  mirrorBrowserScreenshotForQA,
  persistBrowserArtifacts,
  resolveBrowserArtifactPath,
} from "../artifacts";

describe("artifact collector", () => {
  it("persists browser artifacts in stable manifest locations", async () => {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "agh-playwright-artifact-test-"));
    const collector = await ArtifactCollector.create(rootDir);
    const sourceDir = await mkdtemp(path.join(os.tmpdir(), "agh-playwright-source-"));

    const tracePath = path.join(sourceDir, "trace.zip");
    const screenshotOne = path.join(sourceDir, "first.png");
    const screenshotTwo = path.join(sourceDir, "second.png");

    await writeFile(tracePath, "trace");
    await writeFile(screenshotOne, "first");
    await writeFile(screenshotTwo, "second");
    const routeState: BrowserRouteState = {
      url: "http://127.0.0.1/session/sess_browser_01",
      pathname: "/session/sess_browser_01",
      title: "AGH",
      chat_view_visible: true,
      message_count: 2,
      network_channel_count: 0,
      network_thread_count: 0,
      network_direct_count: 0,
      network_message_count: 0,
      network_view_visible: false,
      permission_prompt_visible: false,
      processing_indicator_visible: false,
      resume_button_visible: false,
      session_name: "browser-session",
      stop_button_visible: true,
    };

    const manifest = await persistBrowserArtifacts(collector, {
      tracePath,
      screenshotPaths: [screenshotOne, screenshotTwo],
      consoleEntries: [{ type: "error", text: "console boom" }],
      networkEntries: [
        {
          event: "response",
          url: "http://127.0.0.1/api/demo",
          method: "GET",
          resource_type: "fetch",
          status: 200,
          ok: true,
        },
      ],
      routeState,
    });

    expect(manifest).toEqual({
      version: 1,
      artifacts: [
        { kind: "browser_console", path: "browser_console.json", media_type: "application/json" },
        { kind: "browser_network", path: "browser_network.json", media_type: "application/json" },
        {
          kind: "browser_route_state",
          path: "browser_route_state.json",
          media_type: "application/json",
        },
        { kind: "browser_screenshots", path: "browser_screenshots", media_type: "image/png" },
        { kind: "browser_trace", path: "browser_trace.zip", media_type: "application/zip" },
      ],
    });

    expect(await readFile(collector.artifactPath("browser_trace"), "utf8")).toBe("trace");
    expect(JSON.parse(await readFile(collector.artifactPath("browser_console"), "utf8"))).toEqual([
      { type: "error", text: "console boom" },
    ]);
    expect(JSON.parse(await readFile(collector.artifactPath("browser_network"), "utf8"))).toEqual([
      {
        event: "response",
        method: "GET",
        ok: true,
        resource_type: "fetch",
        status: 200,
        url: "http://127.0.0.1/api/demo",
      },
    ]);
    expect(
      JSON.parse(await readFile(collector.artifactPath("browser_route_state"), "utf8"))
    ).toEqual(routeState);
    expect(await readdir(collector.artifactPath("browser_screenshots"))).toEqual([
      "first.png",
      "second.png",
    ]);
    expect(JSON.parse(await readFile(collector.manifestPath, "utf8"))).toEqual(manifest);
  });

  it("rejects artifact paths that escape the artifact root", async () => {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "agh-playwright-artifact-root-"));
    await mkdir(rootDir, { recursive: true });

    expect(() => resolveBrowserArtifactPath(rootDir, "../escape.json")).toThrow();
  });

  it("identifies vite development HTML markers", () => {
    expect(
      isLikelyViteDevHTML(
        '<!doctype html><html><head><script type="module" src="/@vite/client"></script></head></html>'
      )
    ).toBe(true);

    expect(
      isLikelyViteDevHTML(
        '<!doctype html><html><head><script type="module" src="/assets/index-abc123.js"></script></head></html>'
      )
    ).toBe(false);
  });

  it("mirrors named screenshots into the task QA artifact root", async () => {
    const qaOutputRoot = await mkdtemp(path.join(os.tmpdir(), "agh-qa-output-root-"));
    const sourceDir = await mkdtemp(path.join(os.tmpdir(), "agh-qa-source-"));
    const sourcePath = path.join(sourceDir, "dashboard-capture.png");

    await writeFile(sourcePath, "dashboard");

    const mirroredPath = await mirrorBrowserScreenshotForQA(
      sourcePath,
      qaOutputRoot,
      "tasks-dashboard"
    );

    expect(mirroredPath).toBe(path.join(qaOutputRoot, "qa", "screenshots", "tasks-dashboard.png"));
    expect(await readFile(mirroredPath, "utf8")).toBe("dashboard");
  });

  it("records browser transport snapshots as first-class artifacts", async () => {
    const rootDir = await mkdtemp(path.join(os.tmpdir(), "agh-playwright-transport-artifact-"));
    const collector = await ArtifactCollector.create(rootDir);
    const snapshot = {
      scenarioID: "TC-HARNESS-001",
      http: { status: "running" },
      uds: { status: "running" },
      cli: { status: "running" },
    };

    await collector.captureJSON("browser_transport_snapshots", snapshot);
    const manifest = await collector.writeManifest();

    expect(
      JSON.parse(await readFile(collector.artifactPath("browser_transport_snapshots"), "utf8"))
    ).toEqual(snapshot);
    expect(manifest.artifacts).toContainEqual({
      kind: "browser_transport_snapshots",
      path: "browser_transport_snapshots.json",
      media_type: "application/json",
    });
  });
});
