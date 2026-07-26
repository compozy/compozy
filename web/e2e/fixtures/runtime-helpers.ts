import path from "node:path";
import process from "node:process";

import { isLikelyViteDevHTML } from "./artifacts";

interface RuntimeModeAttach {
  kind: "attach";
  baseURL: string;
}

interface RuntimeModeLaunch {
  kind: "launch";
}

export type RuntimeMode = RuntimeModeAttach | RuntimeModeLaunch;

export interface RuntimeConfigInput {
  extensionsAllowUnverified?: boolean;
  host: string;
  includeMockAgentProvider?: boolean;
  modelsDevEnabled?: boolean;
  marketplaceCatalogBaseURL?: string;
  networkEnabled?: boolean;
  port: number;
  skillsMarketplaceBaseURL?: string;
  socketPath: string;
  toolsExternalDefault?: "disabled" | "ask" | "enabled";
}

export function resolveRuntimeMode(env: NodeJS.ProcessEnv = process.env): RuntimeMode {
  const rawBaseURL = env.AGH_E2E_BASE_URL?.trim();
  if (rawBaseURL === undefined || rawBaseURL === "") {
    return { kind: "launch" };
  }

  return {
    kind: "attach",
    baseURL: normalizeBaseURL(rawBaseURL),
  };
}

export function normalizeBaseURL(rawValue: string): string {
  const baseURL = new URL(rawValue);
  baseURL.hash = "";
  baseURL.search = "";

  if (baseURL.pathname !== "/" && baseURL.pathname !== "") {
    throw new Error(
      `AGH_E2E_BASE_URL must point at the daemon root, received path ${baseURL.pathname}`
    );
  }

  return baseURL.toString().replace(/\/$/, "");
}

export function renderRuntimeConfig(input: RuntimeConfigInput): string {
  return [
    "[daemon]",
    `socket = ${tomlString(input.socketPath)}`,
    "",
    "[http]",
    `host = ${tomlString(input.host)}`,
    `port = ${input.port}`,
    "",
    ...(input.modelsDevEnabled === undefined
      ? []
      : [
          "[model_catalog.sources.models_dev]",
          `enabled = ${input.modelsDevEnabled ? "true" : "false"}`,
          "",
        ]),
    ...(input.networkEnabled === undefined
      ? []
      : ["[network]", `enabled = ${input.networkEnabled ? "true" : "false"}`, ""]),
    ...(input.extensionsAllowUnverified === undefined
      ? []
      : [
          "[extensions.marketplace]",
          `allow_unverified = ${input.extensionsAllowUnverified ? "true" : "false"}`,
          "",
        ]),
    ...(input.skillsMarketplaceBaseURL === undefined
      ? []
      : [
          "[skills.marketplace]",
          'registry = "clawhub"',
          `base_url = ${tomlString(input.skillsMarketplaceBaseURL)}`,
          "",
        ]),
    ...(input.marketplaceCatalogBaseURL === undefined
      ? []
      : [
          "[marketplace.catalog]",
          `base_url = ${tomlString(input.marketplaceCatalogBaseURL)}`,
          'ttl = "1h"',
          'timeout = "5s"',
          "",
        ]),
    ...(input.toolsExternalDefault === undefined
      ? []
      : ["[tools.policy]", `external_default = ${tomlString(input.toolsExternalDefault)}`, ""]),
    ...(input.includeMockAgentProvider === true
      ? [
          "[providers.acpmock]",
          'command = "acpmock-driver"',
          'display_name = "ACP Mock"',
          'harness = "acp"',
          'auth_mode = "none"',
          'none_security = "local_transport"',
          // Apply reasoning via the ACP config option the mock advertises, and expose
          // ONE curated catalog model that carries selectable efforts (matching the
          // fixture's `reasoning_effort` values). `qa-browser-model` stays uncurated so
          // the custom-id flows keep testing an unknown model. The reasoning table is
          // `[providers.<id>.models.reasoning]` (schema ProviderConfig.Models.Reasoning) —
          // NOT `[providers.<id>.reasoning]`, which is an unknown table that fails boot.
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
        ]
      : []),
  ].join("\n");
}

export function requiresHTTPAPIReadinessProbe(host: string | undefined): boolean {
  const normalized = host?.trim().replace(/^\[/, "").replace(/\]$/, "").toLowerCase() ?? "";
  if (normalized === "" || normalized === "localhost" || normalized === "::1") {
    return true;
  }
  return /^127(?:\.|$)/.test(normalized);
}

export function runtimeURL(baseURL: string, pathname = "/"): string {
  const url = new URL(ensureLeadingSlash(pathname), `${baseURL.replace(/\/$/, "")}/`);
  return url.toString();
}

export function buildResolveWorkspaceRequest(path: string): { path: string } {
  return { path };
}

export function assertDaemonServedHTML(html: string, baseURL: string): void {
  if (isLikelyViteDevHTML(html)) {
    throw new Error(`expected daemon-served embedded assets at ${baseURL}, received Vite dev HTML`);
  }
}

function ensureLeadingSlash(value: string): string {
  return value.startsWith("/") ? value : `/${value}`;
}

export function prependPath(prefix: string, currentPath: string | undefined): string {
  if (currentPath === undefined || currentPath.trim() === "") {
    return prefix;
  }
  return `${prefix}${process.platform === "win32" ? ";" : ":"}${currentPath}`;
}

export function buildLaunchRuntimeEnv(repoRoot: string, env: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return {
    ...env,
    AGH_WEB_DIST_DIR: path.join(repoRoot, "web", "dist"),
  };
}

function tomlString(value: string): string {
  return JSON.stringify(value);
}
