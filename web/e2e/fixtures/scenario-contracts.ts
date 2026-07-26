import { execFile } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import type { BrowserArtifactKind } from "./artifacts";
import type { BrowserArtifactSession } from "./browser-artifact-session";
import type { BrowserRuntime, RuntimePaths } from "./runtime";

const execFileAsync = promisify(execFile);

export const auditIDs = [
  "A1",
  "A2",
  "A3",
  "A4",
  "A5",
  "A6",
  "A7",
  "A8",
  "A9",
  "A10",
  "A11",
  "A12",
  "A13",
  "A14",
  "A15",
] as const;

export const executionAuditIDs = [
  "C1",
  "C2",
  "C3",
  "C4",
  "C5",
  "C6",
  "C7",
  "C8",
  "C9",
  "C10",
  "C11",
  "C12",
  "C13",
  "C14",
  "C15",
  "C16",
  "C17",
  "C18",
] as const;

export type AuditID = (typeof auditIDs)[number];
export type ExecutionAuditID = (typeof executionAuditIDs)[number];
export type EvidenceLevel = "L1" | "L2" | "L3" | "L4" | "L5";
export type ProviderBoundary =
  | "mock"
  | "bounded_fake"
  | "native_cli"
  | "bound_secret"
  | "live_provider"
  | "blocked";
export type ScenarioLane =
  | "make test-e2e-runtime"
  | "make test-e2e-web"
  | "make test-e2e"
  | "make test-e2e-nightly";
export type ScenarioPriority = "P0" | "P1" | "P2" | "P3";
export type ScenarioSurface =
  | "agent-runtime"
  | "artifacts"
  | "cli"
  | "config"
  | "http"
  | "persistence"
  | "runtime-harness"
  | "uds"
  | "web";

export interface ScenarioContract {
  id: string;
  title: string;
  module: string;
  priority: ScenarioPriority;
  evidenceLevel: EvidenceLevel;
  surfaces: ScenarioSurface[];
  auditIDs: AuditID[];
  executionAuditIDs: ExecutionAuditID[];
  providerBoundary: ProviderBoundary;
  artifacts: BrowserArtifactKind[];
  lanes: ScenarioLane[];
  specPath: string;
  grep?: string;
  nightly?: boolean;
  blockedReason?: string;
}

export interface ModuleCoverageRequirement {
  module: string;
  requiredArtifacts: BrowserArtifactKind[];
  requiredExecutionAuditIDs: ExecutionAuditID[];
  requiredLanes: ScenarioLane[];
}

export interface ModuleCoverageRow {
  module: string;
  scenarioIDs: string[];
  priorities: ScenarioPriority[];
  providerBoundaries: ProviderBoundary[];
  surfaces: ScenarioSurface[];
  auditIDs: AuditID[];
  executionAuditIDs: ExecutionAuditID[];
  artifacts: BrowserArtifactKind[];
  lanes: ScenarioLane[];
  specs: string[];
}

export interface ViewportEvidence {
  name: "mobile" | "tablet" | "desktop";
  width: number;
  height: number;
  screenshotName: string;
  screenshotPath: string | null;
}

export interface TransportSnapshot<THTTP, TUDS, TCLI> {
  scenarioID: string;
  http: THTTP;
  uds: TUDS;
  cli: TCLI;
}

export const standardViewportMatrix = [
  { name: "mobile", width: 375, height: 812 },
  { name: "tablet", width: 768, height: 1024 },
  { name: "desktop", width: 1280, height: 900 },
] as const;

export const sensitiveArtifactPatterns = [
  {
    name: "raw claim token",
    pattern: /agh_claim_[a-z0-9._-]+/i,
  },
  {
    name: "claim_token field",
    pattern: /["']claim_token["']\s*:\s*["']?[a-z0-9._-]{8,}/i,
  },
  {
    name: "bearer header or value",
    pattern: /(?:authorization\s*:\s*bearer|bearer)\s+["']?[a-z0-9._-]{8,}/i,
  },
  {
    name: "secret-like assignment",
    pattern:
      /(?:api[_-]?key|bearer[_-]?token|mcp[_-]?auth|oauth[_-]?(?:access(?:[_-]?token)?|client(?:[_-]?secret)?|refresh(?:[_-]?token)?|secret|token)|pkce[_-]?(?:challenge|secret|verifier)|provider[_-]?credential|telegram-bot-token)\s*[:=]\s*["']?[a-z0-9._:-]{8,}/i,
  },
  {
    name: "telegram bot token value",
    pattern: /\b\d{6,}:[a-z0-9_-]{20,}/i,
  },
] as const;

export const sensitiveArtifactPattern = new RegExp(
  sensitiveArtifactPatterns.map(({ pattern }) => pattern.source).join("|"),
  "i"
);

const standardBrowserArtifacts: BrowserArtifactKind[] = [
  "browser_trace",
  "browser_screenshots",
  "browser_console",
  "browser_network",
  "browser_route_state",
  "browser_api_snapshots",
  "browser_transport_snapshots",
];

const standardWebExecutionAuditIDs: ExecutionAuditID[] = [
  "C1",
  "C2",
  "C3",
  "C4",
  "C5",
  "C6",
  "C9",
  "C12",
  "C13",
  "C16",
  "C17",
  "C18",
];

export const e2eScenarioContracts: ScenarioContract[] = [
  webScenario(
    "TC-DASHBOARD-001",
    "dashboard",
    "operator sees truthful Dashboard health, metrics, navigation, artifacts, and parity evidence",
    "web/e2e/__tests__/dashboard.spec.ts"
  ),
  webScenario(
    "TC-SESSIONS-001",
    "sessions",
    "operator rejects a permission request and records tool-event evidence",
    "web/e2e/__tests__/session-hardening.spec.ts"
  ),
  webScenario(
    "TC-NETWORK-001",
    "network",
    "operator verifies thread and direct network surfaces with final conversation artifacts",
    "web/e2e/__tests__/network.spec.ts"
  ),
  webScenario(
    "TC-TASKS-001",
    "tasks",
    "operator cancels a running task run and sees transport parity",
    "web/e2e/__tests__/tasks-hardening.spec.ts"
  ),
  webScenario(
    "TC-JOBS-001",
    "jobs",
    "operator creates edits runs disables re-enables and deletes a workspace job with parity evidence",
    "web/e2e/__tests__/jobs-hardening.spec.ts"
  ),
  webScenario(
    "TC-TRIGGERS-001",
    "triggers",
    "operator creates edits fires and deletes a webhook trigger with parity evidence",
    "web/e2e/__tests__/triggers-hardening.spec.ts"
  ),
  webScenario(
    "TC-KNOWLEDGE-001",
    "knowledge",
    "operator creates edits reverts searches recalls and deletes workspace knowledge with parity evidence",
    "web/e2e/__tests__/knowledge.spec.ts"
  ),
  webScenario(
    "TC-SKILLS-001",
    "skills",
    "operator manages Skills against a real daemon and proves next-session prompt impact",
    "web/e2e/__tests__/skills.spec.ts"
  ),
  webScenario(
    "TC-BRIDGES-001",
    "bridges",
    "operator manages a Telegram bridge lifecycle with route and secret parity evidence",
    "web/e2e/__tests__/bridges.spec.ts",
    "bound_secret"
  ),
  webScenario(
    "TC-SANDBOX-001",
    "sandbox",
    "operator manages sandbox profiles and proves a local sandbox session boundary",
    "web/e2e/__tests__/sandbox.spec.ts"
  ),
  webScenario(
    "TC-SETTINGS-001",
    "settings",
    "operator applies Memory, Network, Automation, and Observability settings with config parity",
    "web/e2e/__tests__/settings-hardening.spec.ts"
  ),
  {
    ...webScenario(
      "TC-EXT-001",
      "extensibility-tools-resources",
      "operator installs a local extension tool provider, invokes it over transports, activates bundle resources, and verifies fail-closed manifest security",
      "web/e2e/__tests__/marketplace.spec.ts"
    ),
    priority: "P0",
    auditIDs: ["A1", "A3", "A6", "A8", "A9", "A10", "A12", "A13", "A15"],
    executionAuditIDs: [
      "C1",
      "C2",
      "C3",
      "C4",
      "C5",
      "C6",
      "C9",
      "C12",
      "C13",
      "C14",
      "C16",
      "C17",
      "C18",
    ],
  },
  {
    ...webScenario(
      "TC-EXT-004",
      "extensibility-tools-resources",
      "operator refreshes provider model catalog and carries the selected model into session creation",
      "web/e2e/__tests__/session-provider-override.spec.ts"
    ),
    priority: "P0",
    grep: "operator can create a provider/model override session",
  },
  {
    id: "TC-EXT-005",
    title:
      "SDK-generated TypeScript and Go extensions build install launch call a tool update disable and remove",
    module: "extensibility-tools-resources",
    priority: "P1",
    evidenceLevel: "L5",
    surfaces: ["web", "http", "uds", "cli", "agent-runtime", "persistence", "artifacts"],
    auditIDs: ["A1", "A3", "A6", "A8", "A9", "A10", "A12", "A13", "A15"],
    executionAuditIDs: ["C1", "C2", "C3", "C5", "C6", "C12", "C13", "C16", "C17", "C18"],
    providerBoundary: "blocked",
    artifacts: standardBrowserArtifacts,
    lanes: ["make test-e2e-nightly"],
    specPath: "web/e2e/__tests__/marketplace.spec.ts",
    nightly: true,
    blockedReason:
      "The local release gate covers SDK contracts with sdk/typescript, sdk/go, create-extension unit/integration tests and installs a hand-authored local extension fixture through the daemon. The full scaffold-build-install-launch-update-disable-remove chain is intentionally mapped to nightly because it spans generated TS and Go extension workspaces plus daemon lifecycle mutation and would exceed the daemon-served browser gate budget.",
  },
  {
    id: "TC-EXT-002",
    title: "hosted MCP OAuth login reaches a session-bound tool and recovers after expired auth",
    module: "extensibility-tools-resources",
    priority: "P0",
    evidenceLevel: "L5",
    surfaces: ["web", "http", "uds", "cli", "agent-runtime", "persistence", "artifacts"],
    auditIDs: ["A1", "A3", "A6", "A8", "A9", "A10", "A12", "A13", "A15"],
    executionAuditIDs: ["C1", "C2", "C3", "C5", "C6", "C12", "C13", "C16", "C17", "C18"],
    providerBoundary: "blocked",
    artifacts: standardBrowserArtifacts,
    lanes: ["make test-e2e-nightly"],
    specPath: "web/e2e/__tests__/marketplace.spec.ts",
    nightly: true,
    blockedReason:
      "Requires hosted MCP OAuth credentials and an external server; local daemon-served coverage proves MCP config and tool registry parity, while the credentialed OAuth path remains mapped to nightly.",
  },
  {
    ...webScenario(
      "TC-COMBINED-001",
      "combined-flows",
      "operator can follow a bridge-created route into the shipped session view",
      "web/e2e/__tests__/combined-flows.spec.ts",
      "bound_secret"
    ),
    grep: "operator can follow a bridge-created route into the shipped session view",
    lanes: ["make test-e2e-nightly"],
    nightly: true,
  },
  {
    id: "TC-HARNESS-001",
    title: "runtime harness exposes comparable scenario contracts and transport evidence",
    module: "runtime-harness-transport",
    priority: "P0",
    evidenceLevel: "L4",
    surfaces: ["runtime-harness", "web", "http", "uds", "cli", "artifacts"],
    auditIDs: ["A1", "A3", "A6", "A9", "A10", "A15"],
    executionAuditIDs: ["C1", "C2", "C3", "C5", "C6", "C12", "C13", "C16", "C17", "C18"],
    providerBoundary: "bounded_fake",
    artifacts: standardBrowserArtifacts,
    lanes: ["make test-e2e-runtime", "make test-e2e-web", "make test-e2e"],
    specPath: "web/e2e/__tests__/harness-smoke.spec.ts",
  },
];

export const defaultModuleCoverageRequirements: ModuleCoverageRequirement[] = [
  "dashboard",
  "sessions",
  "network",
  "tasks",
  "jobs",
  "triggers",
  "knowledge",
  "skills",
  "bridges",
  "sandbox",
  "settings",
  "extensibility-tools-resources",
  "runtime-harness-transport",
].map(module => ({
  module,
  requiredArtifacts: ["browser_screenshots", "browser_route_state", "browser_api_snapshots"],
  requiredExecutionAuditIDs: ["C1", "C6", "C12", "C13", "C16", "C17", "C18"],
  requiredLanes: ["make test-e2e-web"],
}));

function webScenario(
  id: string,
  module: string,
  title: string,
  specPath: string,
  providerBoundary: ProviderBoundary = "bounded_fake"
): ScenarioContract {
  return {
    id,
    title,
    module,
    priority: "P1",
    evidenceLevel: "L4",
    surfaces: ["web", "http", "uds", "cli", "artifacts"],
    auditIDs: ["A1", "A3", "A6", "A8", "A9", "A10", "A12", "A15"],
    executionAuditIDs: standardWebExecutionAuditIDs,
    providerBoundary,
    artifacts: standardBrowserArtifacts,
    lanes: ["make test-e2e-web", "make test-e2e"],
    specPath,
  };
}

export function validateScenarioContracts(contracts: ScenarioContract[]): string[] {
  const errors: string[] = [];
  const seenIDs = new Set<string>();

  for (const contract of contracts) {
    const label = contract.id || `${contract.module}:${contract.title}`;
    if (contract.id.trim() === "") {
      errors.push("scenario id is required");
    }
    if (seenIDs.has(contract.id)) {
      errors.push(`scenario ${contract.id} is duplicated`);
    }
    seenIDs.add(contract.id);
    if (contract.module.trim() === "") {
      errors.push(`scenario ${label} module is required`);
    }
    if (contract.surfaces.length === 0) {
      errors.push(`scenario ${label} must declare surfaces`);
    }
    if (contract.auditIDs.length === 0) {
      errors.push(`scenario ${label} must declare behavioral audit ids`);
    }
    if (contract.executionAuditIDs.length === 0) {
      errors.push(`scenario ${label} must declare execution audit ids`);
    }
    if (contract.artifacts.length === 0) {
      errors.push(`scenario ${label} must declare artifact evidence`);
    }
    if (contract.lanes.length === 0) {
      errors.push(`scenario ${label} must declare release gate lanes`);
    }
    if (contract.specPath.trim() === "") {
      errors.push(`scenario ${label} must declare a spec path`);
    }
    if (contract.providerBoundary === "blocked" && !contract.blockedReason?.trim()) {
      errors.push(`scenario ${label} has blocked provider boundary without a reason`);
    }
    if (contract.nightly && !contract.lanes.includes("make test-e2e-nightly")) {
      errors.push(`scenario ${label} is nightly but is not mapped to make test-e2e-nightly`);
    }
    if (!contract.executionAuditIDs.includes("C1")) {
      errors.push(`scenario ${label} is missing C1 scenario contract evidence`);
    }
    if (!contract.executionAuditIDs.includes("C16")) {
      errors.push(`scenario ${label} is missing C16 provider boundary evidence`);
    }
    if (!contract.executionAuditIDs.includes("C17")) {
      errors.push(`scenario ${label} is missing C17 release gate mapping`);
    }
    if (["P0", "P1"].includes(contract.priority) && !contract.executionAuditIDs.includes("C12")) {
      errors.push(`scenario ${label} is P0/P1 but lacks C12 viewport evidence`);
    }
  }

  return errors;
}

export function buildCoverageMatrix(contracts: ScenarioContract[]): ModuleCoverageRow[] {
  const rows = new Map<string, ModuleCoverageRow>();

  for (const contract of contracts) {
    const row = rows.get(contract.module) ?? {
      module: contract.module,
      scenarioIDs: [],
      priorities: [],
      providerBoundaries: [],
      surfaces: [],
      auditIDs: [],
      executionAuditIDs: [],
      artifacts: [],
      lanes: [],
      specs: [],
    };
    row.scenarioIDs.push(contract.id);
    appendUnique(row.priorities, contract.priority);
    appendUnique(row.providerBoundaries, contract.providerBoundary);
    appendUnique(row.surfaces, ...contract.surfaces);
    appendUnique(row.auditIDs, ...contract.auditIDs);
    appendUnique(row.executionAuditIDs, ...contract.executionAuditIDs);
    appendUnique(row.artifacts, ...contract.artifacts);
    appendUnique(row.lanes, ...contract.lanes);
    appendUnique(row.specs, contract.specPath);
    rows.set(contract.module, row);
  }

  return [...rows.values()].sort((left, right) => left.module.localeCompare(right.module));
}

export function validateCoverageMatrix(
  rows: ModuleCoverageRow[],
  requirements: ModuleCoverageRequirement[] = defaultModuleCoverageRequirements
): string[] {
  const errors: string[] = [];
  const byModule = new Map(rows.map(row => [row.module, row]));

  for (const requirement of requirements) {
    const row = byModule.get(requirement.module);
    if (!row) {
      errors.push(`module ${requirement.module} has no scenario contract row`);
      continue;
    }
    for (const artifact of requirement.requiredArtifacts) {
      if (!row.artifacts.includes(artifact)) {
        errors.push(`module ${requirement.module} is missing artifact ${artifact}`);
      }
    }
    for (const executionID of requirement.requiredExecutionAuditIDs) {
      if (!row.executionAuditIDs.includes(executionID)) {
        errors.push(`module ${requirement.module} is missing execution audit ${executionID}`);
      }
    }
    for (const lane of requirement.requiredLanes) {
      if (!row.lanes.includes(lane)) {
        errors.push(`module ${requirement.module} is missing lane ${lane}`);
      }
    }
  }

  return errors;
}

export function nightlyScenarioContracts(
  contracts: ScenarioContract[] = e2eScenarioContracts
): ScenarioContract[] {
  return contracts.filter(contract => contract.nightly && contract.providerBoundary !== "blocked");
}

export function validateNightlySpecCoverage(
  contracts: ScenarioContract[],
  specTexts: Record<string, string>
): string[] {
  const errors: string[] = [];
  const nightlyContracts = nightlyScenarioContracts(contracts);
  if (nightlyContracts.length === 0) {
    errors.push("nightly browser contract has no expected scenarios");
    return errors;
  }

  for (const contract of nightlyContracts) {
    const expectedText = contract.grep ?? contract.title;
    const specText = specTexts[contract.specPath] ?? "";
    if (!specText.includes("@nightly")) {
      errors.push(`${contract.id} spec ${contract.specPath} does not contain @nightly`);
    }
    if (!specText.includes(expectedText)) {
      errors.push(`${contract.id} expected nightly test text is absent: ${expectedText}`);
    }
  }

  return errors;
}

export async function captureViewportEvidence(input: {
  page: Page;
  browserArtifacts: BrowserArtifactSession;
  moduleName: string;
  assertVisible: () => Promise<void>;
}): Promise<ViewportEvidence[]> {
  const evidence: ViewportEvidence[] = [];
  for (const viewport of standardViewportMatrix) {
    await input.page.setViewportSize({ width: viewport.width, height: viewport.height });
    await input.assertVisible();
    const screenshotName = `${input.moduleName}-viewport-${viewport.name}`;
    const screenshotPath = await input.browserArtifacts.captureScreenshot(
      screenshotName,
      input.page
    );
    evidence.push({ ...viewport, screenshotName, screenshotPath });
  }
  return evidence;
}

export async function requestBrowserRuntimeOperatorJSON<T>(
  runtime: BrowserRuntime,
  pathname: string,
  init?: RequestInit
): Promise<T> {
  if (!runtime.requestOperatorJSON) {
    throw new Error(`operator UDS request ${pathname} requires launch-mode runtime access`);
  }
  return await runtime.requestOperatorJSON<T>(pathname, init);
}

export async function runBrowserRuntimeCLIJSON<T>(
  runtime: BrowserRuntime,
  args: string[],
  options: { timeoutMs?: number } = {}
): Promise<T> {
  if (!runtime.paths) {
    throw new Error(`CLI request ${args.join(" ")} requires launch-mode runtime paths`);
  }

  const finalArgs =
    args.includes("-o") || args.includes("--output") ? args : [...args, "-o", "json"];
  const { stdout } = await execFileAsync(runtime.paths.cliShim, finalArgs, {
    env: browserRuntimeCLIEnv(runtime.paths),
    maxBuffer: 10 * 1024 * 1024,
    timeout: options.timeoutMs ?? 30_000,
  });

  try {
    return JSON.parse(stdout) as T;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`CLI request ${finalArgs.join(" ")} returned invalid JSON: ${message}`);
  }
}

export async function captureBrowserTransportSnapshot<THTTP, TUDS, TCLI>(
  runtime: BrowserRuntime,
  scenarioID: string,
  snapshot: { http: THTTP; uds: TUDS; cli: TCLI }
): Promise<TransportSnapshot<THTTP, TUDS, TCLI>> {
  const payload = { scenarioID, ...snapshot };
  await runtime.artifactCollector.captureJSON("browser_transport_snapshots", payload);
  return payload;
}

export function assertSameRuntimeFields(
  label: string,
  expected: Record<string, unknown>,
  actual: Record<string, unknown>,
  fields: string[]
): void {
  for (const field of fields) {
    if (actual[field] !== expected[field]) {
      throw new Error(
        `${label} field ${field} mismatch: got ${String(actual[field])}, want ${String(expected[field])}`
      );
    }
  }
}

export function assertNoSensitiveArtifactPayload(
  payloads: unknown[],
  pattern: RegExp = sensitiveArtifactPattern
): void {
  for (const payload of payloads) {
    const serialized = typeof payload === "string" ? payload : JSON.stringify(payload);
    if (serialized && pattern.test(serialized)) {
      throw new Error("sensitive token-like value leaked into browser E2E artifact payload");
    }
  }
}

function browserRuntimeCLIEnv(paths: RuntimePaths): NodeJS.ProcessEnv {
  return {
    ...process.env,
    AGH_E2E_CLI_BIN: paths.cliShim,
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: `${path.dirname(paths.cliShim)}:${process.env.PATH ?? ""}`,
  };
}

function appendUnique<T>(target: T[], ...values: T[]): void {
  for (const value of values) {
    if (!target.includes(value)) {
      target.push(value);
    }
  }
}
