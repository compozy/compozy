import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import {
  chmod,
  copyFile,
  cp,
  mkdir,
  mkdtemp,
  readdir,
  readFile,
  rm,
  stat,
  symlink,
  writeFile,
} from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type {
  AutomationJob,
  AutomationRun,
  AutomationTrigger,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
} from "@/systems/automation";
import type {
  BridgeDetailResponse,
  BridgeHealth,
  BridgeProvider,
  BridgeRoute,
  BridgeSummary,
} from "@/systems/bridges";
import type {
  SettingsHookRequest,
  SettingsMCPServerRequest,
  SettingsMCPServerTarget,
  SettingsMutationResult,
  SettingsProviderRequest,
  SettingsSandboxEntry,
  SettingsSandboxRequest,
  SettingsSkillsSection,
} from "@/systems/settings";
import type {
  TaskDashboardView,
  TaskInboxView,
  TaskRecord,
  TaskRun,
  TaskRunDetailView,
} from "@/systems/tasks";

const execFileAsync = promisify(execFile);
const MOCK_AGENT_PROVIDER_NAME = "acpmock";

export interface WorkspacePayload {
  id: string;
  root_dir: string;
  name: string;
}

interface SeededSessionPayload {
  id: string;
  agent_name: string;
  workspace_id: string;
  state: string;
  name?: string | null;
  failure?: {
    kind: string;
    summary?: string | null;
  } | null;
}

interface BrowserMockAgentSeed {
  fixturePath: string;
  fixtureAgent?: string;
  agentName?: string;
  category_path?: string[];
}

interface BrowserWorkspaceSeed {
  rootDir: string;
}

interface BrowserSessionSeed {
  agentName: string;
  workspaceRootDir?: string;
}

export interface BrowserSkillSeed {
  body: string;
  description: string;
  marketplace?: {
    hashOverride?: string;
    installedAt?: string;
    registry?: string;
    slug: string;
    version?: string;
  };
  metadata?: Record<string, unknown>;
  name: string;
  resources?: Record<string, string>;
  version?: string;
}

export interface BrowserSkillMarketplaceListingSeed {
  author?: string;
  description?: string;
  downloads?: number;
  license?: string;
  name: string;
  readme?: string;
  slug: string;
  source?: string;
  tags?: string[];
  version?: string;
}

export interface BrowserSkillMarketplaceSeed {
  listings: BrowserSkillMarketplaceListingSeed[];
}

interface BrowserMarketplaceEntrySeed {
  description: string;
  entry_id: string;
  name: string;
  published_at?: string;
  updated_at?: string;
  version?: string;
}

export interface BrowserMarketplaceMCPEntrySeed extends BrowserMarketplaceEntrySeed {
  args?: string[];
  command?: string;
  default_scope?: "global" | "workspace";
  env?: Array<{
    default?: string;
    name: string;
    prompt?: string;
    required: boolean;
    secret: boolean;
  }>;
  oauth?: {
    authorization_url?: string;
    client_id: string;
    issuer_url?: string;
    scopes?: string[];
    token_url?: string;
  };
  transport: "stdio" | "http" | "sse";
  url?: string;
}

export interface BrowserMarketplaceExtensionEntrySeed extends BrowserMarketplaceEntrySeed {
  artifact_url: string;
  author?: string;
  digest_sha256: string;
  install_slug: string;
  repository?: string;
  tier: "official" | "community" | "unverified";
}

export interface BrowserMarketplaceSkillEntrySeed extends BrowserMarketplaceEntrySeed {
  author?: string;
  display_name?: string;
  install_slug: string;
  tags?: string[];
}

export interface BrowserMarketplaceCatalogSeed {
  extensions?: BrowserMarketplaceExtensionEntrySeed[];
  generatedAt?: string;
  mcp?: BrowserMarketplaceMCPEntrySeed[];
  skills?: BrowserMarketplaceSkillEntrySeed[];
}

export interface BrowserRuntimeSeed {
  marketplaceCatalog?: BrowserMarketplaceCatalogSeed;
  skillMarketplace?: BrowserSkillMarketplaceSeed;
  skills?: BrowserSkillSeed[];
  mockAgents?: BrowserMockAgentSeed[];
  workspace?: BrowserWorkspaceSeed;
  session?: BrowserSessionSeed;
}

export interface BrowserAutomationOperatorFlowSeed {
  agentName: string;
  timeoutMs?: number;
  workspaceId?: string;
}

export interface BridgeAdapterMarkerPaths {
  crashOnce: string;
  delivery: string;
  handshake: string;
  ingest: string;
  ownership: string;
  shutdown: string;
  starts: string;
  state: string;
  updates: string;
}

interface PreparedBrowserBridgeExtension {
  checksum: string;
  extensionDir: string;
  markers: BridgeAdapterMarkerPaths;
}

export interface BrowserBridgeOperatorFlowSeed {
  displayName?: string;
  prepareExtension?: () => Promise<{
    checksum: string;
    extensionDir: string;
    markers: BridgeAdapterMarkerPaths;
  }>;
  timeoutMs?: number;
}

export interface BrowserBridgeOperatorFlowResult {
  bridge: BridgeSummary;
  extension: {
    checksum: string;
    dir: string;
    markers: BridgeAdapterMarkerPaths;
    name: string;
    platform: string;
  };
  health: BridgeHealth;
  provider: BridgeProvider;
}

export interface BrowserSettingsProviderSeed {
  name: string;
  settings: SettingsProviderRequest["settings"];
}

export interface BrowserSettingsMCPServerSeed {
  name: string;
  scope?: "global" | "workspace";
  server: SettingsMCPServerRequest["server"];
  target?: SettingsMCPServerTarget;
  workspaceId?: string;
  workspaceRootDir?: string;
}

export interface BrowserSettingsHookSeed {
  name: string;
  declaration: SettingsHookRequest["declaration"];
}

export interface BrowserSettingsFixturesSeed {
  disabledSkills?: string[];
  hooks?: BrowserSettingsHookSeed[];
  installBridgeExtension?: boolean;
  mcpServers?: BrowserSettingsMCPServerSeed[];
  providers?: BrowserSettingsProviderSeed[];
  timeoutMs?: number;
}

export interface BrowserSettingsFixturesResult {
  createdHookNames: string[];
  createdMCPServers: Array<{
    name: string;
    scope: "global" | "workspace";
    target: SettingsMCPServerTarget;
    workspaceId?: string;
  }>;
  createdProviderNames: string[];
  extension?: {
    checksum: string;
    dir: string;
    markers: BridgeAdapterMarkerPaths;
    name: string;
    platform: string;
  };
  initialDisabledSkills?: string[];
  workspace?: WorkspacePayload;
}

export interface BrowserSandboxProfileSeed {
  name: string;
  profile: SettingsSandboxRequest["profile"];
}

export interface BrowserSandboxProfilesResult {
  sandboxes: SettingsSandboxEntry[];
}

type BrowserBridgeOperatorSeedRuntime = Pick<
  BrowserRuntimeSeedClient,
  "requestJSON" | "requestOperatorJSON"
> &
  Partial<Pick<BrowserRuntimeSeedClient, "resolveWorkspace">> & {
    paths?: {
      homeDir: string;
    };
    seeded?: BrowserRuntimeSeedResult;
  };

type BrowserAutomationOperatorSeedRuntime = Pick<BrowserRuntimeSeedClient, "requestJSON"> &
  Partial<Pick<BrowserRuntimeSeedClient, "resolveWorkspace">> & {
    paths?: {
      homeDir: string;
    };
    seeded?: BrowserRuntimeSeedResult;
  };

type BrowserSettingsSeedRuntime = BrowserBridgeOperatorSeedRuntime;

export interface BrowserBridgeIngressSeed {
  assistantText?: string;
  messageId?: number;
  text?: string;
  timeoutMs?: number;
  updateId?: number;
}

export interface BrowserBridgeIngressResult {
  routes: BridgeRoute[];
  sessionId: string;
  transcript: string;
}

export interface BrowserAutomationOperatorFlowResult {
  job: AutomationJob;
  trigger: AutomationTrigger;
  baselineRun: AutomationRun;
}

export interface BrowserTasksOperatorFlowSeed {
  sessionAgentName: string;
  timeoutMs?: number;
  workspaceRootDir?: string;
}

export interface BrowserTasksOperatorFlowResult {
  approvalInbox: TaskInboxView;
  approvalTask: TaskRecord;
  dashboard: TaskDashboardView;
  referenceTask: TaskRecord;
  runningRun: TaskRun;
  runningRunDetail: TaskRunDetailView;
  runningTask: TaskRecord;
  session: SeededSessionPayload;
}

export interface BrowserNetworkOperatorFlowSeed {
  channel: string;
  initiatorAgentName: string;
  responderAgentName: string;
  timeoutMs?: number;
  workspaceId: string;
}

interface BrowserNetworkOperatorFlowParticipant extends SeededSessionPayload {
  peerId: string;
}

export interface BrowserNetworkOperatorFlowResult {
  channel: string;
  directId: string;
  initiator: BrowserNetworkOperatorFlowParticipant;
  messageIds: typeof browserNetworkOperatorFlowScenario.messageIds;
  responder: BrowserNetworkOperatorFlowParticipant;
  threadId: string;
  traceId: string;
  workId: string;
  workspaceId: string;
}

export interface BrowserRuntimeSeedResult {
  workspace?: WorkspacePayload;
  session?: SeededSessionPayload;
}

export interface BrowserRuntimeSeedClient {
  requestJSON<T>(pathname: string, init?: RequestInit): Promise<T>;
  requestOperatorJSON?<T>(pathname: string, init?: RequestInit): Promise<T>;
  resolveWorkspace(rootDir: string): Promise<WorkspacePayload>;
}

interface BrowserRuntimeSeedPaths {
  homeDir: string;
  repoRoot: string;
}

interface MockFixtureAgent {
  name: string;
  provider: string;
  model?: string;
  permissions?: string;
  prompt?: string;
  category_path?: string[];
}

interface MockFixture {
  agents?: MockFixtureAgent[];
}

interface NetworkChannelSeedPayload {
  channel: string;
  sessions?: SeededSessionPayload[];
}

interface NetworkPeerSeedPayload {
  channel: string;
  peer_id: string;
  session_id?: string;
}

interface NetworkMessageSeedPayload {
  direct_id?: string;
  message_id: string;
  surface?: string;
  thread_id?: string;
}

interface NetworkStatusSeedPayload {
  network?: {
    kind_metrics?: Array<{
      kind?: string;
      sent?: number;
      delivered?: number;
    }>;
  };
}

type BrowserTasksOperatorSeedRuntime = Pick<BrowserRuntimeSeedClient, "requestJSON"> &
  Partial<Pick<BrowserRuntimeSeedClient, "resolveWorkspace">> & {
    paths?: {
      homeDir: string;
    };
    seeded?: BrowserRuntimeSeedResult;
  };

const NETWORK_OPERATOR_FLOW_TIMEOUT_MS = 15_000;
const AUTOMATION_OPERATOR_FLOW_TIMEOUT_MS = 15_000;
const TASKS_OPERATOR_FLOW_TIMEOUT_MS = 15_000;
const BRIDGE_OPERATOR_FLOW_TIMEOUT_MS = 45_000;
const SETTINGS_OPERATOR_FLOW_TIMEOUT_MS = 15_000;
const BROWSER_SEED_POLL_MS = 150;
const BROWSER_SEED_SESSION_ACTIVE_TIMEOUT_MS = 30_000;
const BRIDGE_EXTENSION_NAME = "telegram-reference";
const BRIDGE_PLATFORM = "telegram";

let acpMockDriverBinaryPromise: Promise<string> | undefined;

export const browserNetworkOperatorFlowScenario = {
  messageIds: {
    say: "browser_msg_say_01",
    direct: "browser_msg_direct_01",
    trace: "browser_msg_trace_01",
    summary: "browser_msg_summary_01",
  },
  texts: {
    say: "Who can take the failing migration tests in internal/store/sessiondb?",
    direct: "I can take the failing migration tests and send back a patch summary.",
    trace: "Patch prepared and local tests now pass.",
    summary: "Summary: migration test patch prepared and local verification is passing.",
  },
  threadId: "thread_browser_patch_42",
  traceId: "browser_trace_ops_patch_42",
  workId: "browser_work_patch_42",
} as const;

export const browserAutomationOperatorFlowScenario = {
  job: {
    initialName: "deploy-review",
    editedName: "deploy-review-updated",
    prompt: "Review payload deploy for main",
    scheduleExpr: "0 9 * * *",
    updatedScheduleExpr: "15 10 * * *",
  },
  trigger: {
    endpointSlug: "browser-deploy-review",
    event: "webhook",
    name: "deploy-review-webhook",
    prompt: `Review payload {{ index .Data "payload" }} for {{ index .Data "branch" }}`,
    webhookID: "wbh_browser_deploy_review",
    webhookSecret: "shared-secret",
  },
  transcript: {
    assistant: "Automation review completed for deploy on main.",
  },
} as const;

export const browserBridgeOperatorFlowScenario = {
  bridge: {
    initialName: "Telegram Browser Bridge",
    initialProviderConfig: {
      mode: "bot",
      webhook_url: "https://example.test/browser-bridge",
    },
    editedName: "Telegram Bridge Ops",
    editedProviderConfig: {
      mode: "bot",
      webhook_url: "https://example.test/browser-bridge-updated",
    },
  },
  ingress: {
    assistant: "Bridge summary: initial route handled.",
    messageId: 321,
    text: "Need a runtime bridge summary",
    updateId: 94001,
  },
  secretBinding: {
    name: "bot_token",
    value: "telegram-bot-token",
  },
  testDelivery: {
    message: "Deliver a short operator ping.",
    mode: "direct-send",
    peerId: "telegram-peer-321",
    threadId: "654",
  },
} as const;

export const browserSettingsOperatorFlowScenario = {
  general: {
    primarySessionTimeoutSeconds: 75,
    fallbackSessionTimeoutSeconds: 90,
  },
  hooks: {
    hookName: "browser-turn-end",
  },
  mcpServers: {
    global: {
      command: "npx",
      name: "browser-global-mcp",
      target: "sidecar",
    },
    workspace: {
      command: "uvx",
      name: "browser-workspace-mcp",
      target: "config",
    },
  },
  providers: {
    overlayCommand: "codex-browser",
    overlayModel: "gpt-5.4",
  },
  skills: {
    disabledSkill: "browser-disabled-skill",
    policyRegistry: "browser-marketplace-registry",
  },
} as const;

export const browserTasksOperatorFlowScenario = {
  approvalTask: {
    identifier: "TASK-BROWSER-APPROVAL",
    title: "Approve browser regression rollout",
  },
  referenceTask: {
    identifier: "TASK-BROWSER-QUEUE",
    title: "Prepare operator queue summary",
  },
  runningTask: {
    identifier: "TASK-BROWSER-RUN",
    title: "Capture tasks operator evidence",
  },
  runningRun: {
    claimIdempotencyKey: "tasks-browser-claim-01",
    enqueueIdempotencyKey: "tasks-browser-enqueue-01",
  },
} as const;
export async function seedBrowserRuntimeHome(
  paths: BrowserRuntimeSeedPaths,
  seed: BrowserRuntimeSeed | undefined
): Promise<void> {
  const skills = seed?.skills ?? [];
  if (skills.length > 0) {
    await seedBrowserSkills(paths.homeDir, skills);
  }

  const mockAgents = seed?.mockAgents ?? [];
  if (mockAgents.length === 0) {
    return;
  }

  const driverPath = await ensureACPmockDriverBinary(paths.repoRoot);
  await installACPmockDriverShim(paths.homeDir, driverPath);
  const agentsDir = path.join(paths.homeDir, "agents");
  const diagnosticsDir = path.join(paths.homeDir, "logs", "acpmock");

  await mkdir(agentsDir, { recursive: true });
  await mkdir(diagnosticsDir, { recursive: true });

  for (const spec of mockAgents) {
    const registration = await loadMockAgentRegistration(driverPath, diagnosticsDir, spec);
    const agentDir = path.join(agentsDir, registration.agentName);
    const agentDefPath = path.join(agentDir, "AGENT.md");
    await mkdir(agentDir, { recursive: true });
    await writeFile(
      agentDefPath,
      renderMockAgentDef(registration.agentName, registration.agent, registration.command),
      "utf8"
    );
  }
}

async function seedBrowserSkills(homeDir: string, skills: BrowserSkillSeed[]): Promise<void> {
  const skillsDir = path.join(homeDir, "skills");
  await mkdir(skillsDir, { recursive: true });

  for (const skill of skills) {
    const skillName = normalizeSeedSkillName(skill.name);
    const skillDir = path.join(skillsDir, skillName);
    await mkdir(skillDir, { recursive: true });

    const skillFile = path.join(skillDir, "SKILL.md");
    await writeFile(skillFile, renderSeedSkillDocument(skill), { encoding: "utf8", mode: 0o600 });
    await chmod(skillFile, 0o600);

    for (const [relativePath, content] of Object.entries(skill.resources ?? {})) {
      const target = resolveSeedSkillPath(skillDir, relativePath);
      await mkdir(path.dirname(target), { recursive: true });
      await writeFile(target, content, { encoding: "utf8", mode: 0o600 });
      await chmod(target, 0o600);
    }

    if (skill.marketplace) {
      const hash = await computeSeedSkillDirectoryHash(skillDir);
      const sidecar = {
        hash: skill.marketplace.hashOverride ?? hash,
        registry: skill.marketplace.registry ?? "clawhub",
        slug: skill.marketplace.slug,
        version: skill.marketplace.version ?? skill.version ?? "",
        installed_at: skill.marketplace.installedAt ?? "2026-05-09T00:00:00Z",
      };
      await writeFile(
        path.join(skillDir, ".agh-meta.json"),
        `${JSON.stringify(sidecar, null, 2)}\n`,
        {
          encoding: "utf8",
          mode: 0o600,
        }
      );
      await chmod(path.join(skillDir, ".agh-meta.json"), 0o600);
    }
  }
}

function normalizeSeedSkillName(name: string): string {
  const trimmed = name.trim();
  if (!/^[A-Za-z0-9._-]+$/.test(trimmed)) {
    throw new Error(`browser skill seed name ${JSON.stringify(name)} is invalid`);
  }
  return trimmed;
}

function resolveSeedSkillPath(skillDir: string, relativePath: string): string {
  const target = path.resolve(skillDir, relativePath);
  const relative = path.relative(skillDir, target);
  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new Error(`browser skill seed resource ${relativePath} escapes ${skillDir}`);
  }
  return target;
}

function renderSeedSkillDocument(skill: BrowserSkillSeed): string {
  const lines = [
    "---",
    `name: ${JSON.stringify(normalizeSeedSkillName(skill.name))}`,
    `description: ${JSON.stringify(skill.description)}`,
  ];
  if (skill.version?.trim()) {
    lines.push(`version: ${JSON.stringify(skill.version.trim())}`);
  }
  if (skill.metadata && Object.keys(skill.metadata).length > 0) {
    lines.push("metadata:");
    lines.push(...renderSeedYAMLMap(skill.metadata, 2));
  }
  lines.push("---", "", skill.body.trim(), "");
  return lines.join("\n");
}

function renderSeedYAMLMap(value: Record<string, unknown>, indent: number): string[] {
  return Object.entries(value).flatMap(([key, entry]) => renderSeedYAMLEntry(key, entry, indent));
}

function renderSeedYAMLEntry(key: string, value: unknown, indent: number): string[] {
  const prefix = " ".repeat(indent);
  if (Array.isArray(value)) {
    return [
      `${prefix}${key}:`,
      ...value.map(item => `${" ".repeat(indent + 2)}- ${renderSeedYAMLScalar(item)}`),
    ];
  }
  if (value && typeof value === "object") {
    return [`${prefix}${key}:`, ...renderSeedYAMLMap(value as Record<string, unknown>, indent + 2)];
  }
  return [`${prefix}${key}: ${renderSeedYAMLScalar(value)}`];
}

function renderSeedYAMLScalar(value: unknown): string {
  if (typeof value === "string") {
    return JSON.stringify(value);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (value === null || value === undefined) {
    return "null";
  }
  return JSON.stringify(value);
}

async function computeSeedSkillDirectoryHash(skillDir: string): Promise<string> {
  const entries = await collectSeedSkillFiles(skillDir, skillDir);
  const hash = createHash("sha256");

  for (const relativePath of entries.sort()) {
    const absolutePath = path.join(skillDir, relativePath);
    const info = await stat(absolutePath);
    if (!info.isFile()) {
      throw new Error(`browser skill seed hash only supports regular files: ${absolutePath}`);
    }

    hash.update(
      `file:${relativePath.split(path.sep).join("/")}\nmode:${formatSeedFileMode(info.mode)}\n`
    );
    hash.update(await readFile(absolutePath));
    hash.update(Buffer.from([0]));
  }

  return hash.digest("hex");
}

async function collectSeedSkillFiles(rootDir: string, currentDir: string): Promise<string[]> {
  const entries = await readdir(currentDir, { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    if (entry.name === ".agh-meta.json") {
      continue;
    }
    const absolutePath = path.join(currentDir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectSeedSkillFiles(rootDir, absolutePath)));
      continue;
    }
    if (!entry.isFile()) {
      throw new Error(`browser skill seed hash only supports files: ${absolutePath}`);
    }
    files.push(path.relative(rootDir, absolutePath));
  }
  return files;
}

function formatSeedFileMode(mode: number): string {
  return `0${(mode & 0o777).toString(8).padStart(3, "0")}`;
}

export async function applyBrowserRuntimeSeed(
  runtime: BrowserRuntimeSeedClient,
  seed: BrowserRuntimeSeed | undefined
): Promise<BrowserRuntimeSeedResult> {
  if (seed === undefined) {
    return {};
  }

  let workspace = await resolveSeedWorkspace(runtime, seed);
  if (seed.session === undefined) {
    return { workspace };
  }

  const sessionWorkspaceRoot = seed.session.workspaceRootDir ?? seed.workspace?.rootDir;
  if (sessionWorkspaceRoot === undefined || sessionWorkspaceRoot.trim() === "") {
    throw new Error(
      "session runtime seed requires a workspace root via seed.workspace or session.workspaceRootDir"
    );
  }

  if (!workspace || workspace.root_dir !== sessionWorkspaceRoot) {
    workspace = await runtime.resolveWorkspace(sessionWorkspaceRoot);
  }

  const payload = await runtime.requestJSON<{ session: SeededSessionPayload }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({
      agent_name: seed.session.agentName,
      workspace: workspace.id,
    }),
  });

  // Gate before handing the session to specs: none of them may hit an
  // agent-authenticated endpoint while the session is still `starting`.
  const session = await waitForSeedSessionActive(runtime, payload.session.id);

  return {
    workspace,
    session,
  };
}

export async function seedBrowserNetworkOperatorFlow(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  seed: BrowserNetworkOperatorFlowSeed
): Promise<BrowserNetworkOperatorFlowResult> {
  const channel = seed.channel.trim();
  const initiatorAgentName = seed.initiatorAgentName.trim();
  const responderAgentName = seed.responderAgentName.trim();
  const workspaceId = requireSeedWorkspaceID(seed.workspaceId, "network operator flow seed");

  if (channel === "") {
    throw new Error("network operator flow seed requires a non-empty channel");
  }
  if (initiatorAgentName === "" || responderAgentName === "") {
    throw new Error("network operator flow seed requires both initiator and responder agents");
  }

  const timeoutMs = seed.timeoutMs ?? NETWORK_OPERATOR_FLOW_TIMEOUT_MS;

  const channelState = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ channel: NetworkChannelSeedPayload }>(
        workspaceNetworkPath(workspaceId, `/channels/${encodeURIComponent(channel)}`)
      );
      const sessions = payload.channel.sessions ?? [];
      const initiatorSession = sessions.find(session => session.agent_name === initiatorAgentName);
      const responderSession = sessions.find(session => session.agent_name === responderAgentName);

      if (!initiatorSession || !responderSession) {
        return null;
      }

      return {
        initiatorSession,
        responderSession,
      };
    },
    `network channel ${channel} to include both operator-flow agents`,
    timeoutMs
  );

  const peerState = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ peers: NetworkPeerSeedPayload[] }>(
        workspaceNetworkPath(workspaceId, `/peers?channel=${encodeURIComponent(channel)}`)
      );
      const initiatorPeer = payload.peers.find(
        peer => peer.session_id === channelState.initiatorSession.id
      );
      const responderPeer = payload.peers.find(
        peer => peer.session_id === channelState.responderSession.id
      );

      if (!initiatorPeer || !responderPeer) {
        return null;
      }

      return {
        initiatorPeer,
        responderPeer,
      };
    },
    `network peers for ${channel}`,
    timeoutMs
  );

  const directRoom = await runtime.requestJSON<{ direct: { direct_id: string } }>(
    workspaceNetworkPath(workspaceId, `/channels/${encodeURIComponent(channel)}/directs/resolve`),
    {
      method: "POST",
      body: JSON.stringify({
        session_id: channelState.responderSession.id,
        peer_id: peerState.initiatorPeer.peer_id,
      }),
    }
  );
  const directId = directRoom.direct.direct_id.trim();
  if (directId === "") {
    throw new Error("network operator flow seed direct resolve returned an empty direct_id");
  }

  await sendNetworkSeedMessage(runtime, workspaceId, {
    session_id: channelState.initiatorSession.id,
    channel,
    kind: "say",
    surface: "thread",
    thread_id: browserNetworkOperatorFlowScenario.threadId,
    id: browserNetworkOperatorFlowScenario.messageIds.say,
    trace_id: browserNetworkOperatorFlowScenario.traceId,
    body: {
      text: browserNetworkOperatorFlowScenario.texts.say,
      intent: "request-help",
      artifacts: [],
    },
  });

  await sendNetworkSeedMessage(runtime, workspaceId, {
    session_id: channelState.responderSession.id,
    channel,
    kind: "say",
    surface: "direct",
    direct_id: directId,
    to: peerState.initiatorPeer.peer_id,
    work_id: browserNetworkOperatorFlowScenario.workId,
    reply_to: browserNetworkOperatorFlowScenario.messageIds.say,
    trace_id: browserNetworkOperatorFlowScenario.traceId,
    causation_id: browserNetworkOperatorFlowScenario.messageIds.say,
    id: browserNetworkOperatorFlowScenario.messageIds.direct,
    body: {
      text: browserNetworkOperatorFlowScenario.texts.direct,
      intent: "handoff",
      artifacts: [],
    },
  });

  await sendNetworkSeedMessage(runtime, workspaceId, {
    session_id: channelState.responderSession.id,
    channel,
    kind: "trace",
    surface: "direct",
    direct_id: directId,
    to: peerState.initiatorPeer.peer_id,
    work_id: browserNetworkOperatorFlowScenario.workId,
    reply_to: browserNetworkOperatorFlowScenario.messageIds.direct,
    trace_id: browserNetworkOperatorFlowScenario.traceId,
    causation_id: browserNetworkOperatorFlowScenario.messageIds.direct,
    id: browserNetworkOperatorFlowScenario.messageIds.trace,
    body: {
      state: "completed",
      message: browserNetworkOperatorFlowScenario.texts.trace,
      result: {
        summary: "Fixed migration assertion mismatch in sessiondb tests.",
      },
      artifact_refs: [],
    },
  });

  await sendNetworkSeedMessage(runtime, workspaceId, {
    session_id: channelState.responderSession.id,
    channel,
    kind: "say",
    surface: "thread",
    thread_id: browserNetworkOperatorFlowScenario.threadId,
    reply_to: browserNetworkOperatorFlowScenario.messageIds.trace,
    trace_id: browserNetworkOperatorFlowScenario.traceId,
    causation_id: browserNetworkOperatorFlowScenario.messageIds.trace,
    id: browserNetworkOperatorFlowScenario.messageIds.summary,
    body: {
      text: browserNetworkOperatorFlowScenario.texts.summary,
      intent: "summarize-back",
      artifacts: [],
    },
  });

  await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ messages: NetworkMessageSeedPayload[] }>(
        workspaceNetworkPath(
          workspaceId,
          `/channels/${encodeURIComponent(channel)}/threads/${encodeURIComponent(
            browserNetworkOperatorFlowScenario.threadId
          )}/messages`
        )
      );
      const messageIds = new Set(payload.messages.map(message => message.message_id));

      return messageIds.has(browserNetworkOperatorFlowScenario.messageIds.say) &&
        messageIds.has(browserNetworkOperatorFlowScenario.messageIds.summary)
        ? payload.messages
        : null;
    },
    `network thread ${browserNetworkOperatorFlowScenario.threadId} for ${channel}`,
    timeoutMs
  );

  await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ messages: NetworkMessageSeedPayload[] }>(
        workspaceNetworkPath(
          workspaceId,
          `/channels/${encodeURIComponent(channel)}/directs/${encodeURIComponent(directId)}/messages`
        )
      );
      const messageIds = new Set(payload.messages.map(message => message.message_id));

      return messageIds.has(browserNetworkOperatorFlowScenario.messageIds.direct) &&
        messageIds.has(browserNetworkOperatorFlowScenario.messageIds.trace)
        ? payload.messages
        : null;
    },
    `network direct ${directId} for ${channel}`,
    timeoutMs
  );

  await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<NetworkStatusSeedPayload>("/api/network/status");
      const kindMetrics = new Map(
        (payload.network?.kind_metrics ?? []).map(metric => [metric.kind ?? "", metric])
      );
      const say = kindMetrics.get("say");
      const trace = kindMetrics.get("trace");

      if ((say?.sent ?? 0) < 3 || (say?.delivered ?? 0) < 2) {
        return null;
      }
      if ((trace?.sent ?? 0) < 1 || (trace?.delivered ?? 0) < 1) {
        return null;
      }

      return payload.network?.kind_metrics ?? [];
    },
    `network operator metrics for ${channel}`,
    timeoutMs
  );

  return {
    channel,
    directId,
    initiator: {
      ...channelState.initiatorSession,
      peerId: peerState.initiatorPeer.peer_id,
    },
    responder: {
      ...channelState.responderSession,
      peerId: peerState.responderPeer.peer_id,
    },
    messageIds: browserNetworkOperatorFlowScenario.messageIds,
    threadId: browserNetworkOperatorFlowScenario.threadId,
    traceId: browserNetworkOperatorFlowScenario.traceId,
    workId: browserNetworkOperatorFlowScenario.workId,
    workspaceId,
  };
}

export async function seedBrowserSandboxProfiles(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  profiles: BrowserSandboxProfileSeed[]
): Promise<BrowserSandboxProfilesResult> {
  const sandboxes: SettingsSandboxEntry[] = [];

  for (const seed of profiles) {
    const name = seed.name.trim();
    if (name === "") {
      throw new Error("sandbox profile seed requires a non-empty name");
    }

    const payload = await runtime.requestJSON<{ sandbox: SettingsSandboxEntry }>(
      `/api/settings/sandboxes/${encodeURIComponent(name)}`,
      {
        method: "PUT",
        body: JSON.stringify({ profile: seed.profile }),
      }
    );
    sandboxes.push(payload.sandbox);
  }

  return { sandboxes };
}

export async function seedBrowserAutomationOperatorFlow(
  runtime: BrowserAutomationOperatorSeedRuntime,
  seed: BrowserAutomationOperatorFlowSeed
): Promise<BrowserAutomationOperatorFlowResult> {
  const agentName = seed.agentName.trim();
  if (agentName === "") {
    throw new Error("automation operator flow seed requires a non-empty agent name");
  }

  const timeoutMs = seed.timeoutMs ?? AUTOMATION_OPERATOR_FLOW_TIMEOUT_MS;
  const job = await createAutomationOperatorJob(runtime, agentName);
  const trigger = await createAutomationOperatorTrigger(runtime, agentName);

  const initialRun = (
    await runtime.requestJSON<{ run: AutomationRun }>(
      `/api/automation/jobs/${encodeURIComponent(job.id)}/trigger`,
      {
        method: "POST",
      }
    )
  ).run;

  const baselineRun = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ run: AutomationRun }>(
        `/api/automation/runs/${encodeURIComponent(initialRun.id)}`
      );
      const run = payload.run;
      return run.status === "completed" && Boolean(run.session_id) ? run : null;
    },
    `completed automation run ${initialRun.id}`,
    timeoutMs
  );

  await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ runs: AutomationRun[] }>(
        `/api/automation/jobs/${encodeURIComponent(job.id)}/runs?limit=10`
      );
      return payload.runs.some(
        run => run.id === baselineRun.id && run.session_id === baselineRun.session_id
      )
        ? payload.runs
        : null;
    },
    `visible run history for automation job ${job.id}`,
    timeoutMs
  );

  await waitForSeedCondition(
    async () => {
      const workspaceId = await resolveBrowserAutomationWorkspaceID(
        runtime,
        seed,
        timeoutMs,
        (baselineRun as { workspace_id?: string }).workspace_id
      );
      const payload = await runtime.requestJSON<{ entries: Array<{ message: unknown }> }>(
        workspaceSessionPath(workspaceId, baselineRun.session_id ?? "", "/transcript")
      );
      const transcript = JSON.stringify(payload.entries.map(entry => entry.message));

      return transcript.includes(browserAutomationOperatorFlowScenario.job.prompt) &&
        transcript.includes(browserAutomationOperatorFlowScenario.transcript.assistant)
        ? transcript
        : null;
    },
    `automation transcript for session ${baselineRun.session_id}`,
    timeoutMs
  );

  return {
    job,
    trigger,
    baselineRun,
  };
}

export async function seedBrowserTasksOperatorFlow(
  runtime: BrowserTasksOperatorSeedRuntime,
  seed: BrowserTasksOperatorFlowSeed
): Promise<BrowserTasksOperatorFlowResult> {
  const sessionAgentName = seed.sessionAgentName.trim();
  if (sessionAgentName === "") {
    throw new Error("tasks operator flow seed requires a non-empty session agent name");
  }

  const timeoutMs = seed.timeoutMs ?? TASKS_OPERATOR_FLOW_TIMEOUT_MS;
  const sessionWorkspace = await resolveBrowserTasksWorkspace(runtime, seed, timeoutMs);
  const createdSession = (
    await runtime.requestJSON<{ session: SeededSessionPayload }>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({
        agent_name: sessionAgentName,
        workspace: sessionWorkspace.id,
      }),
    })
  ).session;

  // The claim-next call below authenticates as this session; it 401s with
  // `identity_stale` until background-role activation commits. Gate on active.
  const activeSession = await waitForSeedSessionActive(runtime, createdSession.id, timeoutMs);

  const referenceTask = await createBrowserTask(runtime, {
    description: "Seeded ready task for list and dashboard coverage.",
    identifier: browserTasksOperatorFlowScenario.referenceTask.identifier,
    owner: {
      kind: "human",
      ref: "qa-operator",
    },
    priority: "medium",
    scope: "global",
    title: browserTasksOperatorFlowScenario.referenceTask.title,
  });

  const approvalTask = await createBrowserTask(runtime, {
    approval_policy: "manual",
    description: "Approval-gated task for inbox regression coverage.",
    identifier: browserTasksOperatorFlowScenario.approvalTask.identifier,
    owner: {
      kind: "human",
      ref: "release-manager",
    },
    priority: "high",
    scope: "global",
    title: browserTasksOperatorFlowScenario.approvalTask.title,
  });

  const runningTask = await createBrowserTask(runtime, {
    description: "Running task for dashboard, detail, and run-detail regression coverage.",
    identifier: browserTasksOperatorFlowScenario.runningTask.identifier,
    owner: {
      kind: "automation",
      ref: "browser-task-runner",
    },
    priority: "urgent",
    scope: "global",
    title: browserTasksOperatorFlowScenario.runningTask.title,
  });

  const runningRun = (
    await runtime.requestJSON<{ run: TaskRun }>(
      `/api/tasks/${encodeURIComponent(runningTask.id)}/runs`,
      {
        method: "POST",
        body: JSON.stringify({
          idempotency_key: browserTasksOperatorFlowScenario.runningRun.enqueueIdempotencyKey,
        }),
      }
    )
  ).run;

  // Operator HTTP claim was removed (Task #03); seed claims via the agent claim-next path.
  // Claim-next binds the caller session and leaves the run claimed — enough for dashboard
  // active_runs. Operator /start would try to spawn a second task session and 500.
  await runtime.requestJSON<{ claim: { run: TaskRun } }>("/api/agent/tasks/claim-next", {
    method: "POST",
    headers: {
      "X-AGH-Session-ID": activeSession.id,
      "X-AGH-Agent": sessionAgentName,
    },
    body: JSON.stringify({
      run_id: runningRun.id,
      idempotency_key: browserTasksOperatorFlowScenario.runningRun.claimIdempotencyKey,
    }),
  });

  const runningRunDetail = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ run: TaskRunDetailView }>(
        `/api/task-runs/${encodeURIComponent(runningRun.id)}`
      );
      const detail = payload.run;
      return detail.session?.session_id === activeSession.id &&
        ["claimed", "queued", "running", "starting"].includes(detail.run.status)
        ? detail
        : null;
    },
    `task run detail ${runningRun.id} to expose linked session`,
    timeoutMs
  );

  const dashboard = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ dashboard: TaskDashboardView }>(
        "/api/observe/tasks/dashboard"
      );
      const candidate = payload.dashboard;
      return candidate.totals.tasks_total >= 3 &&
        candidate.active_runs.items?.some(item => item.run_id === runningRun.id)
        ? candidate
        : null;
    },
    "task dashboard seeded operator state",
    timeoutMs
  );

  const approvalInbox = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ inbox: TaskInboxView }>(
        "/api/observe/tasks/inbox?lane=approvals&limit=10"
      );
      const candidate = payload.inbox;
      return candidate.groups?.some(group =>
        (group.items ?? []).some(item => item.task.id === approvalTask.id)
      )
        ? candidate
        : null;
    },
    "task approvals inbox seeded operator state",
    timeoutMs
  );

  return {
    approvalInbox,
    approvalTask,
    dashboard,
    referenceTask,
    runningRun,
    runningRunDetail,
    runningTask,
    session: activeSession,
  };
}

export async function seedBrowserBridgeOperatorFlow(
  runtime: BrowserBridgeOperatorSeedRuntime,
  seed: BrowserBridgeOperatorFlowSeed = {}
): Promise<BrowserBridgeOperatorFlowResult> {
  const timeoutMs = seed.timeoutMs ?? BRIDGE_OPERATOR_FLOW_TIMEOUT_MS;
  const displayName =
    seed.displayName?.trim() || browserBridgeOperatorFlowScenario.bridge.initialName;
  const installedExtension = await installBrowserBridgeExtension(runtime, {
    prepareExtension: seed.prepareExtension,
    timeoutMs,
  });

  const provider = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ providers: BridgeProvider[] }>(
        "/api/bridges/providers"
      );
      return (
        payload.providers.find(
          candidate =>
            candidate.extension_name === BRIDGE_EXTENSION_NAME &&
            candidate.platform === BRIDGE_PLATFORM
        ) ?? null
      );
    },
    `${BRIDGE_EXTENSION_NAME} bridge provider`,
    timeoutMs
  );

  const workspace = await resolveBrowserBridgeWorkspace(runtime, timeoutMs);

  const createResponse = await runtime.requestJSON<BridgeDetailResponse>("/api/bridges", {
    method: "POST",
    body: JSON.stringify({
      display_name: displayName,
      enabled: false,
      extension_name: BRIDGE_EXTENSION_NAME,
      platform: BRIDGE_PLATFORM,
      provider_config: browserBridgeOperatorFlowScenario.bridge.initialProviderConfig,
      routing_policy: {
        include_group: false,
        include_peer: true,
        include_thread: true,
      },
      scope: "workspace",
      workspace_id: workspace.id,
    }),
  });

  await runtime.requestJSON<{ binding: { binding_name: string; secret_ref: string } }>(
    `/api/bridges/${encodeURIComponent(createResponse.bridge.id)}/secret-bindings/${encodeURIComponent(
      browserBridgeOperatorFlowScenario.secretBinding.name
    )}`,
    {
      method: "PUT",
      body: JSON.stringify({
        kind: browserBridgeOperatorFlowScenario.secretBinding.name,
        secret_ref: `vault:bridges/${createResponse.bridge.id}/${browserBridgeOperatorFlowScenario.secretBinding.name}`,
        secret_value: browserBridgeOperatorFlowScenario.secretBinding.value,
      }),
    }
  );

  const bridgeDetail = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<BridgeDetailResponse>(
        `/api/bridges/${encodeURIComponent(createResponse.bridge.id)}`
      );
      return payload.health?.status === "disabled" ? payload : null;
    },
    `bridge detail for ${createResponse.bridge.id}`,
    timeoutMs
  );

  return {
    bridge: bridgeDetail.bridge,
    extension: {
      ...installedExtension,
    },
    health: bridgeDetail.health,
    provider,
  };
}

export async function seedBrowserSettingsFixtures(
  runtime: BrowserSettingsSeedRuntime,
  seed: BrowserSettingsFixturesSeed = {}
): Promise<BrowserSettingsFixturesResult> {
  const timeoutMs = seed.timeoutMs ?? SETTINGS_OPERATOR_FLOW_TIMEOUT_MS;
  const result: BrowserSettingsFixturesResult = {
    createdHookNames: [],
    createdMCPServers: [],
    createdProviderNames: [],
  };

  if (seed.disabledSkills !== undefined) {
    const payload = await runtime.requestJSON<SettingsSkillsSection>("/api/settings/skills");
    const currentDisabled = normalizeStringList(payload.config.disabled_skills ?? []);
    const desiredDisabled = normalizeStringList(seed.disabledSkills);
    result.initialDisabledSkills = currentDisabled;

    if (!sameStringList(currentDisabled, desiredDisabled)) {
      await runtime.requestJSON<SettingsMutationResult>("/api/settings/skills", {
        method: "PATCH",
        body: JSON.stringify({
          config: {
            ...payload.config,
            disabled_skills: desiredDisabled,
          },
        }),
      });

      await waitForSeedCondition(
        async () => {
          const current = await runtime.requestJSON<SettingsSkillsSection>("/api/settings/skills");
          return sameStringList(
            normalizeStringList(current.config.disabled_skills ?? []),
            desiredDisabled
          )
            ? current.config
            : null;
        },
        "settings disabled skills seed",
        timeoutMs
      );
    }
  }

  for (const provider of seed.providers ?? []) {
    const name = provider.name.trim();
    if (name === "") {
      throw new Error("settings provider seed requires a non-empty name");
    }

    await runtime.requestJSON<SettingsMutationResult>(
      `/api/settings/providers/${encodeURIComponent(name)}`,
      {
        method: "PUT",
        body: JSON.stringify({
          settings: provider.settings,
        }),
      }
    );

    result.createdProviderNames.push(name);
    await waitForSeedCondition(
      async () => {
        const payload = await runtime.requestJSON<{
          providers: Array<{ name: string }>;
        }>("/api/settings/providers");
        return payload.providers.some(entry => entry.name === name) ? payload.providers : null;
      },
      `settings provider ${name}`,
      timeoutMs
    );
  }

  let resolvedWorkspace = runtime.seeded?.workspace;

  for (const serverSeed of seed.mcpServers ?? []) {
    const name = serverSeed.name.trim();
    if (name === "") {
      throw new Error("settings MCP server seed requires a non-empty name");
    }

    const scope = serverSeed.scope ?? "global";
    const target = serverSeed.target ?? "auto";
    let workspaceId = serverSeed.workspaceId?.trim();

    if (scope === "workspace") {
      if (!workspaceId) {
        const rootDir =
          serverSeed.workspaceRootDir?.trim() || resolvedWorkspace?.root_dir?.trim() || "";
        if (rootDir === "") {
          throw new Error(
            `workspace MCP server seed for ${name} requires workspaceId, workspaceRootDir, or a seeded workspace`
          );
        }
        if (!runtime.resolveWorkspace) {
          throw new Error(
            `workspace MCP server seed for ${name} requires resolveWorkspace support on the runtime`
          );
        }
        resolvedWorkspace = await runtime.resolveWorkspace(rootDir);
        workspaceId = resolvedWorkspace.id;
      }
      if (!resolvedWorkspace || resolvedWorkspace.id !== workspaceId) {
        resolvedWorkspace = {
          id: workspaceId,
          root_dir: serverSeed.workspaceRootDir?.trim() || resolvedWorkspace?.root_dir || "",
          name: resolvedWorkspace?.name || workspaceId,
        };
      }
      result.workspace = resolvedWorkspace;
    }

    await runtime.requestJSON<SettingsMutationResult>(
      buildSettingsMCPServerPath(name, scope, workspaceId, target),
      {
        method: "PUT",
        body: JSON.stringify({
          server: {
            ...serverSeed.server,
            name,
          },
        }),
      }
    );

    result.createdMCPServers.push({
      name,
      scope,
      target,
      workspaceId,
    });

    await waitForSeedCondition(
      async () => {
        const payload = await runtime.requestJSON<{
          mcp_servers: Array<{ name: string; workspace_id?: string }>;
        }>(buildSettingsMCPServersListPath(scope, workspaceId));
        return payload.mcp_servers.some(
          entry =>
            entry.name === name &&
            (scope === "global" || (entry.workspace_id ?? "") === workspaceId)
        )
          ? payload.mcp_servers
          : null;
      },
      `settings MCP server ${name}`,
      timeoutMs
    );
  }

  for (const hook of seed.hooks ?? []) {
    const name = hook.name.trim();
    if (name === "") {
      throw new Error("settings hook seed requires a non-empty name");
    }

    await runtime.requestJSON<SettingsMutationResult>(
      `/api/settings/hooks/${encodeURIComponent(name)}`,
      {
        method: "PUT",
        body: JSON.stringify({
          declaration: {
            ...hook.declaration,
            name,
          },
        }),
      }
    );

    result.createdHookNames.push(name);
    await waitForSeedCondition(
      async () => {
        const payload = await runtime.requestJSON<{
          hooks: Array<{ name: string }>;
        }>("/api/settings/hooks");
        return payload.hooks.some(entry => entry.name === name) ? payload.hooks : null;
      },
      `settings hook ${name}`,
      timeoutMs
    );
  }

  if (seed.installBridgeExtension) {
    result.extension = await installBrowserBridgeExtension(runtime, { timeoutMs });
  }

  return result;
}

export async function cleanupBrowserSettingsFixtures(
  runtime: BrowserSettingsSeedRuntime,
  seeded: BrowserSettingsFixturesResult
): Promise<void> {
  if (seeded.initialDisabledSkills !== undefined) {
    const payload = await runtime.requestJSON<SettingsSkillsSection>("/api/settings/skills");
    const desiredDisabled = normalizeStringList(seeded.initialDisabledSkills);
    if (
      !sameStringList(normalizeStringList(payload.config.disabled_skills ?? []), desiredDisabled)
    ) {
      await runtime.requestJSON<SettingsMutationResult>("/api/settings/skills", {
        method: "PATCH",
        body: JSON.stringify({
          config: {
            ...payload.config,
            disabled_skills: desiredDisabled,
          },
        }),
      });
    }
  }

  for (const hookName of [...seeded.createdHookNames].reverse()) {
    await ignoreNotFound(
      runtime.requestJSON<SettingsMutationResult>(
        `/api/settings/hooks/${encodeURIComponent(hookName)}`,
        {
          method: "DELETE",
        }
      )
    );
  }

  for (const server of [...seeded.createdMCPServers].reverse()) {
    await ignoreNotFound(
      runtime.requestJSON<SettingsMutationResult>(
        buildSettingsMCPServerPath(server.name, server.scope, server.workspaceId, server.target),
        {
          method: "DELETE",
        }
      )
    );
  }

  for (const providerName of [...seeded.createdProviderNames].reverse()) {
    await ignoreNotFound(
      runtime.requestJSON<SettingsMutationResult>(
        `/api/settings/providers/${encodeURIComponent(providerName)}`,
        {
          method: "DELETE",
        }
      )
    );
  }

  const restartsDir = runtime.paths?.homeDir
    ? path.join(runtime.paths.homeDir, "restarts")
    : undefined;
  if (restartsDir) {
    await rm(restartsDir, { recursive: true, force: true });
  }
}

async function installBrowserBridgeExtension(
  runtime: BrowserSettingsSeedRuntime,
  options: {
    prepareExtension?: () => Promise<{
      checksum: string;
      extensionDir: string;
      markers: BridgeAdapterMarkerPaths;
    }>;
    timeoutMs?: number;
  } = {}
): Promise<NonNullable<BrowserSettingsFixturesResult["extension"]>> {
  const requestOperatorJSON = async <T>(pathname: string, init?: RequestInit): Promise<T> => {
    if (runtime.requestOperatorJSON) {
      return await runtime.requestOperatorJSON<T>(pathname, init);
    }
    return await runtime.requestJSON<T>(pathname, init);
  };
  const prepareExtension = options.prepareExtension ?? prepareBrowserBridgeExtension;
  const timeoutMs = options.timeoutMs ?? BRIDGE_OPERATOR_FLOW_TIMEOUT_MS;
  const prepared = await prepareExtension();

  await requestOperatorJSON<{ extension: { name: string } }>("/api/extensions", {
    method: "POST",
    body: JSON.stringify({
      allow_unverified: true,
      checksum: prepared.checksum,
      path: prepared.extensionDir,
    }),
  });

  await waitForSeedCondition(
    async () => {
      const payload = await requestOperatorJSON<{
        extension: { enabled: boolean; name: string };
      }>(`/api/extensions/${encodeURIComponent(BRIDGE_EXTENSION_NAME)}`);
      return payload.extension.name === BRIDGE_EXTENSION_NAME && payload.extension.enabled
        ? payload.extension
        : null;
    },
    `${BRIDGE_EXTENSION_NAME} extension install`,
    timeoutMs
  );

  return {
    checksum: prepared.checksum,
    dir: prepared.extensionDir,
    markers: prepared.markers,
    name: BRIDGE_EXTENSION_NAME,
    platform: BRIDGE_PLATFORM,
  };
}

function buildSettingsMCPServersListPath(
  scope: "global" | "workspace",
  workspaceId?: string
): string {
  const params = new URLSearchParams({ scope });
  if (scope === "workspace" && workspaceId) {
    params.set("workspace_id", workspaceId);
  }
  return `/api/settings/mcp-servers?${params.toString()}`;
}

function buildSettingsMCPServerPath(
  name: string,
  scope: "global" | "workspace",
  workspaceId: string | undefined,
  target: SettingsMCPServerTarget
): string {
  const params = new URLSearchParams({
    scope,
    target,
  });
  if (scope === "workspace" && workspaceId) {
    params.set("workspace_id", workspaceId);
  }
  return `/api/settings/mcp-servers/${encodeURIComponent(name)}?${params.toString()}`;
}

function normalizeStringList(values: string[]): string[] {
  return [...new Set(values.map(value => value.trim()).filter(value => value !== ""))].sort();
}

function sameStringList(left: string[], right: string[]): boolean {
  if (left.length !== right.length) {
    return false;
  }
  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) {
      return false;
    }
  }
  return true;
}

async function ignoreNotFound<T>(operation: Promise<T>): Promise<T | undefined> {
  try {
    return await operation;
  } catch (error) {
    if (error instanceof Error && error.message.includes(" 404:")) {
      return undefined;
    }
    throw error;
  }
}

async function resolveBrowserBridgeWorkspace(
  runtime: BrowserBridgeOperatorSeedRuntime,
  timeoutMs: number
): Promise<WorkspacePayload> {
  const seededWorkspace = runtime.seeded?.workspace;
  if (seededWorkspace) {
    return seededWorkspace;
  }

  const resolveWorkspace = runtime.resolveWorkspace?.bind(runtime);
  const homeDir = runtime.paths?.homeDir;
  if (resolveWorkspace && homeDir) {
    return await waitForSeedCondition(
      async () => await resolveWorkspace(homeDir),
      "browser bridge workspace",
      timeoutMs
    );
  }

  return await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>(
        "/api/workspaces"
      );
      return payload.workspaces[0] ?? null;
    },
    "browser bridge workspace",
    timeoutMs
  );
}

async function resolveBrowserTasksWorkspace(
  runtime: BrowserTasksOperatorSeedRuntime,
  seed: BrowserTasksOperatorFlowSeed,
  timeoutMs: number
): Promise<WorkspacePayload> {
  const workspaceRootDir = seed.workspaceRootDir?.trim();
  if (workspaceRootDir) {
    if (!runtime.resolveWorkspace) {
      throw new Error(
        "tasks operator flow seed requires resolveWorkspace when workspaceRootDir is provided"
      );
    }
    return await runtime.resolveWorkspace(workspaceRootDir);
  }

  const seededWorkspace = runtime.seeded?.workspace;
  if (seededWorkspace) {
    return seededWorkspace;
  }

  const resolveWorkspace = runtime.resolveWorkspace?.bind(runtime);
  const homeDir = runtime.paths?.homeDir;
  if (resolveWorkspace && homeDir) {
    return await waitForSeedCondition(
      async () => await resolveWorkspace(homeDir),
      "browser tasks workspace",
      timeoutMs
    );
  }

  return await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>(
        "/api/workspaces"
      );
      return payload.workspaces[0] ?? null;
    },
    "browser tasks workspace",
    timeoutMs
  );
}

async function resolveBrowserAutomationWorkspaceID(
  runtime: BrowserAutomationOperatorSeedRuntime,
  seed: BrowserAutomationOperatorFlowSeed,
  timeoutMs: number,
  runWorkspaceId?: string
): Promise<string> {
  const explicitWorkspaceID = runWorkspaceId?.trim() || seed.workspaceId?.trim();
  if (explicitWorkspaceID) {
    return explicitWorkspaceID;
  }

  const seededWorkspaceID = runtime.seeded?.workspace?.id?.trim();
  if (seededWorkspaceID) {
    return seededWorkspaceID;
  }

  const resolveWorkspace = runtime.resolveWorkspace?.bind(runtime);
  const homeDir = runtime.paths?.homeDir;
  if (resolveWorkspace && homeDir) {
    const workspace = await waitForSeedCondition(
      async () => await resolveWorkspace(homeDir),
      "browser automation workspace",
      timeoutMs
    );
    return workspace.id;
  }

  const workspace = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ workspaces: WorkspacePayload[] }>(
        "/api/workspaces"
      );
      return payload.workspaces[0] ?? null;
    },
    "browser automation workspace",
    timeoutMs
  );
  return workspace.id;
}

async function createBrowserTask(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  body: Record<string, unknown>
): Promise<TaskRecord> {
  return (
    await runtime.requestJSON<{ task: TaskRecord }>("/api/tasks", {
      method: "POST",
      body: JSON.stringify(body),
    })
  ).task;
}

export async function triggerBrowserBridgeIngress(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  seeded: Pick<BrowserBridgeOperatorFlowResult, "bridge" | "extension">,
  seed: BrowserBridgeIngressSeed = {}
): Promise<BrowserBridgeIngressResult> {
  const timeoutMs = seed.timeoutMs ?? BRIDGE_OPERATOR_FLOW_TIMEOUT_MS;
  const update = createBrowserBridgeIngressUpdate(seeded.bridge.id, {
    messageId: seed.messageId,
    text: seed.text,
    updateId: seed.updateId,
  });
  await appendJSONLine(seeded.extension.markers.updates, update);

  await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<BridgeDetailResponse>(
        `/api/bridges/${encodeURIComponent(seeded.bridge.id)}`
      );
      return (payload.health?.route_count ?? 0) >= 1 && Boolean(payload.health?.last_success_at)
        ? payload
        : null;
    },
    `bridge ingress health for ${seeded.bridge.id}`,
    timeoutMs
  );

  const routes = await waitForSeedCondition(
    async () => {
      const payload = await runtime.requestJSON<{ routes: BridgeRoute[] }>(
        `/api/bridges/${encodeURIComponent(seeded.bridge.id)}/routes`
      );
      return payload.routes.length > 0 ? payload.routes : null;
    },
    `bridge routes for ${seeded.bridge.id}`,
    timeoutMs
  );

  const sessionId = routes[0]?.session_id?.trim();
  if (!sessionId) {
    throw new Error(`bridge ingress for ${seeded.bridge.id} did not produce a session route`);
  }

  const transcriptPayload = await waitForSeedCondition(
    async () => {
      const workspaceId = requireSeedWorkspaceID(
        (routes[0] as { workspace_id?: string } | undefined)?.workspace_id,
        `bridge route for session ${sessionId}`
      );
      const payload = await runtime.requestJSON<{ entries: Array<{ message: unknown }> }>(
        workspaceSessionPath(workspaceId, sessionId, "/transcript")
      );
      const transcript = JSON.stringify(payload.entries.map(entry => entry.message));
      const assistantText =
        seed.assistantText?.trim() || browserBridgeOperatorFlowScenario.ingress.assistant;

      return transcript.includes(assistantText) ? payload : null;
    },
    `bridge transcript for session ${sessionId}`,
    timeoutMs
  );

  return {
    routes,
    sessionId,
    transcript: JSON.stringify(transcriptPayload.entries.map(entry => entry.message)),
  };
}

async function resolveSeedWorkspace(
  runtime: BrowserRuntimeSeedClient,
  seed: BrowserRuntimeSeed
): Promise<WorkspacePayload | undefined> {
  const rootDir = seed.workspace?.rootDir?.trim();
  if (!rootDir) {
    return undefined;
  }

  return runtime.resolveWorkspace(rootDir);
}

async function loadMockAgentRegistration(
  driverPath: string,
  diagnosticsDir: string,
  spec: BrowserMockAgentSeed
): Promise<{
  agentName: string;
  command: string;
  agent: MockFixtureAgent;
}> {
  const fixturePath = path.resolve(spec.fixturePath);
  const rawFixture = await readFile(fixturePath, "utf8");
  const fixture = JSON.parse(rawFixture) as MockFixture;
  const fixtureAgentName = spec.fixtureAgent?.trim() || spec.agentName?.trim() || "";
  if (fixtureAgentName === "") {
    throw new Error(`mock agent seed for ${fixturePath} requires fixtureAgent or agentName`);
  }

  const agent = fixture.agents?.find(candidate => candidate.name === fixtureAgentName);
  if (!agent) {
    throw new Error(`mock agent fixture ${fixturePath} does not contain agent ${fixtureAgentName}`);
  }

  const agentName = spec.agentName?.trim() || fixtureAgentName;
  const diagnosticsPath = path.join(diagnosticsDir, `${agentName}.jsonl`);
  const command = shellQuote([
    driverPath,
    "--fixture",
    fixturePath,
    "--agent",
    fixtureAgentName,
    "--diagnostics",
    diagnosticsPath,
  ]);

  const resolvedAgent: MockFixtureAgent = spec.category_path
    ? { ...agent, category_path: spec.category_path }
    : agent;

  return { agentName, command, agent: resolvedAgent };
}

async function ensureACPmockDriverBinary(repoRoot: string): Promise<string> {
  const override = process.env.AGH_TEST_ACPMOCK_DRIVER_BIN?.trim();
  if (override) {
    return path.isAbsolute(override) ? override : path.resolve(repoRoot, override);
  }

  if (acpMockDriverBinaryPromise === undefined) {
    acpMockDriverBinaryPromise = buildACPmockDriverBinary(repoRoot).catch(error => {
      acpMockDriverBinaryPromise = undefined;
      throw error;
    });
  }
  return await acpMockDriverBinaryPromise;
}

async function buildACPmockDriverBinary(repoRoot: string): Promise<string> {
  const buildDir = await mkdtemp(path.join(os.tmpdir(), "agh-acpmock-driver-"));
  const outputPath = path.join(
    buildDir,
    process.platform === "win32" ? "acpmock-driver.exe" : "acpmock-driver"
  );

  await execFileAsync(
    "go",
    ["build", "-o", outputPath, "./internal/testutil/acpmock/cmd/acpmock-driver"],
    {
      cwd: repoRoot,
      env: process.env,
      maxBuffer: 20 * 1024 * 1024,
    }
  );

  return outputPath;
}

async function installACPmockDriverShim(homeDir: string, driverPath: string): Promise<void> {
  const binDir = path.join(homeDir, "bin");
  const targetPath = path.join(binDir, acpMockDriverBinaryName());
  await mkdir(binDir, { recursive: true });
  await rm(targetPath, { force: true });

  if (process.platform !== "win32") {
    try {
      await symlink(driverPath, targetPath);
      return;
    } catch {
      // Fall back to copying below when symlinks are unavailable.
    }
  }

  await copyFile(driverPath, targetPath);
  await chmod(targetPath, 0o755);
}

function acpMockDriverBinaryName(): string {
  return process.platform === "win32" ? "acpmock-driver.exe" : "acpmock-driver";
}

function renderMockAgentDef(name: string, agent: MockFixtureAgent, command: string): string {
  const prompt = agent.prompt?.trim() || `You are ${name}.`;
  const lines = [
    "---",
    `name: ${name}`,
    `provider: ${MOCK_AGENT_PROVIDER_NAME}`,
    `command: ${command}`,
  ];

  if (agent.model?.trim()) {
    lines.push(`model: ${agent.model.trim()}`);
  }
  if (agent.permissions?.trim()) {
    lines.push(`permissions: ${agent.permissions.trim()}`);
  }
  const segments = agent.category_path?.map(seg => seg.trim()).filter(seg => seg.length > 0) ?? [];
  if (segments.length > 0) {
    lines.push(`category_path: [${segments.map(seg => JSON.stringify(seg)).join(", ")}]`);
  }

  lines.push("---", "", prompt, "");
  return lines.join("\n");
}

async function sendNetworkSeedMessage(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  workspaceId: string,
  body: Record<string, unknown>
): Promise<void> {
  await runtime.requestJSON(workspaceNetworkPath(workspaceId, "/send"), {
    method: "POST",
    body: JSON.stringify(body),
  });
}

function requireSeedWorkspaceID(workspaceId: string | undefined, context: string): string {
  const normalized = workspaceId?.trim() ?? "";
  if (normalized === "") {
    throw new Error(`${context} requires a workspace_id`);
  }
  return normalized;
}

function workspaceNetworkPath(workspaceId: string, suffix: string): string {
  return workspacePath(workspaceId, `/network${suffix.startsWith("/") ? suffix : `/${suffix}`}`);
}

function workspaceSessionPath(workspaceId: string, sessionId: string, suffix: string): string {
  const normalizedSessionID = sessionId.trim();
  if (normalizedSessionID === "") {
    throw new Error("workspace session path requires a session_id");
  }
  return workspacePath(
    workspaceId,
    `/sessions/${encodeURIComponent(normalizedSessionID)}${suffix.startsWith("/") ? suffix : `/${suffix}`}`
  );
}

function workspacePath(workspaceId: string, suffix: string): string {
  const normalizedWorkspaceID = requireSeedWorkspaceID(workspaceId, "workspace-scoped path");
  return `/api/workspaces/${encodeURIComponent(normalizedWorkspaceID)}${suffix}`;
}

async function createAutomationOperatorJob(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  agentName: string
): Promise<AutomationJob> {
  const request: CreateAutomationJobRequest = {
    agent_name: agentName,
    enabled: true,
    fire_limit: { max: 12, window: "1h" },
    name: browserAutomationOperatorFlowScenario.job.initialName,
    prompt: browserAutomationOperatorFlowScenario.job.prompt,
    retry: { strategy: "none", max_retries: 0, base_delay: "" },
    schedule: {
      mode: "cron",
      expr: browserAutomationOperatorFlowScenario.job.scheduleExpr,
    },
    scope: "global",
  };

  return (
    await runtime.requestJSON<{ job: AutomationJob }>("/api/automation/jobs", {
      method: "POST",
      body: JSON.stringify(request),
    })
  ).job;
}

async function createAutomationOperatorTrigger(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  agentName: string
): Promise<AutomationTrigger> {
  const request: CreateAutomationTriggerRequest = {
    agent_name: agentName,
    enabled: true,
    endpoint_slug: browserAutomationOperatorFlowScenario.trigger.endpointSlug,
    event: browserAutomationOperatorFlowScenario.trigger.event,
    filter: {
      "data.branch": "main",
    },
    fire_limit: { max: 12, window: "1h" },
    name: browserAutomationOperatorFlowScenario.trigger.name,
    prompt: browserAutomationOperatorFlowScenario.trigger.prompt,
    retry: { strategy: "none", max_retries: 0, base_delay: "" },
    scope: "global",
    webhook_id: browserAutomationOperatorFlowScenario.trigger.webhookID,
    webhook_secret_value: browserAutomationOperatorFlowScenario.trigger.webhookSecret,
  };

  return (
    await runtime.requestJSON<{ trigger: AutomationTrigger }>("/api/automation/triggers", {
      method: "POST",
      body: JSON.stringify(request),
    })
  ).trigger;
}

async function prepareBrowserBridgeExtension(): Promise<PreparedBrowserBridgeExtension> {
  const repoRoot = resolveBrowserRepoRoot();
  const sourceDir = path.join(repoRoot, "sdk", "examples", BRIDGE_EXTENSION_NAME);
  const tempRoot = await mkdtemp(path.join(os.tmpdir(), "agh-browser-bridge-extension-"));
  const extensionDir = path.join(tempRoot, BRIDGE_EXTENSION_NAME);
  const markers = createBridgeAdapterMarkerPaths(path.join(extensionDir, "markers"));

  await cp(sourceDir, extensionDir, { recursive: true });
  await mkdir(path.join(extensionDir, "bin"), { recursive: true });
  await mkdir(path.dirname(markers.handshake), { recursive: true });

  const manifestPath = path.join(extensionDir, "extension.toml");
  const rawManifest = await readFile(manifestPath, "utf8");
  await writeFile(manifestPath, patchBridgeExtensionManifest(rawManifest, markers), "utf8");

  await execFileAsync(
    "go",
    [
      "build",
      "-o",
      path.join(extensionDir, "bin", BRIDGE_EXTENSION_NAME),
      "./sdk/examples/telegram-reference",
    ],
    {
      cwd: repoRoot,
      env: process.env,
    }
  );

  return {
    checksum: await computeDirectoryChecksum(extensionDir),
    extensionDir,
    markers,
  };
}

function resolveBrowserRepoRoot(): string {
  return path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
}

function createBridgeAdapterMarkerPaths(rootDir: string): BridgeAdapterMarkerPaths {
  return {
    crashOnce: path.join(rootDir, "adapter-crash-once.json"),
    delivery: path.join(rootDir, "adapter-deliveries.jsonl"),
    handshake: path.join(rootDir, "adapter-handshake.json"),
    ingest: path.join(rootDir, "adapter-ingest.jsonl"),
    ownership: path.join(rootDir, "adapter-ownership.json"),
    shutdown: path.join(rootDir, "adapter-shutdown.log"),
    starts: path.join(rootDir, "adapter-starts.log"),
    state: path.join(rootDir, "adapter-states.jsonl"),
    updates: path.join(rootDir, "adapter-updates.jsonl"),
  };
}

function patchBridgeExtensionManifest(manifest: string, markers: BridgeAdapterMarkerPaths): string {
  let next = manifest.replace('min_agh_version = "0.5.0"', 'min_agh_version = "0.0.0"');
  const values = {
    AGH_BRIDGE_ADAPTER_CRASH_ONCE_PATH: markers.crashOnce,
    AGH_BRIDGE_ADAPTER_DELIVERY_PATH: markers.delivery,
    AGH_BRIDGE_ADAPTER_HANDSHAKE_PATH: markers.handshake,
    AGH_BRIDGE_ADAPTER_INGEST_PATH: markers.ingest,
    AGH_BRIDGE_ADAPTER_OWNERSHIP_PATH: markers.ownership,
    AGH_BRIDGE_ADAPTER_SHUTDOWN_PATH: markers.shutdown,
    AGH_BRIDGE_ADAPTER_STARTS_PATH: markers.starts,
    AGH_BRIDGE_ADAPTER_STATE_PATH: markers.state,
    AGH_BRIDGE_ADAPTER_UPDATES_PATH: markers.updates,
  } as const;

  for (const [envName, value] of Object.entries(values)) {
    const placeholder = `"{{env:${envName}}}"`;
    next = next.replace(new RegExp(escapeRegExp(placeholder), "g"), JSON.stringify(value));
  }

  return next;
}

async function computeDirectoryChecksum(rootDir: string): Promise<string> {
  const repoRoot = resolveBrowserRepoRoot();
  await mkdir(path.join(repoRoot, ".tmp"), { recursive: true });
  const helperRoot = await mkdtemp(path.join(repoRoot, ".tmp", "agh-browser-checksum-"));
  const helperPath = path.join(helperRoot, "main.go");

  await writeFile(
    helperPath,
    [
      "package main",
      "",
      "import (",
      '\t"fmt"',
      '\t"os"',
      '\t"strings"',
      "",
      '\textensionpkg "github.com/compozy/agh/internal/extension"',
      ")",
      "",
      "func main() {",
      "\tif len(os.Args) != 2 {",
      '\t\tfmt.Fprintln(os.Stderr, "extension directory is required")',
      "\t\tos.Exit(1)",
      "\t}",
      "",
      "\tchecksum, err := extensionpkg.ComputeDirectoryChecksum(strings.TrimSpace(os.Args[1]))",
      "\tif err != nil {",
      "\t\tfmt.Fprintln(os.Stderr, err)",
      "\t\tos.Exit(1)",
      "\t}",
      "",
      "\tfmt.Print(checksum)",
      "}",
      "",
    ].join("\n"),
    "utf8"
  );

  try {
    const { stdout } = await execFileAsync("go", ["run", helperPath, rootDir], {
      cwd: repoRoot,
      env: process.env,
    });
    const checksum = stdout.trim();
    if (checksum === "") {
      throw new Error(`go checksum helper returned an empty checksum for ${rootDir}`);
    }
    return checksum;
  } finally {
    await rm(helperRoot, { force: true, recursive: true });
  }
}

function createBrowserBridgeIngressUpdate(
  bridgeInstanceID: string,
  input: Pick<BrowserBridgeIngressSeed, "messageId" | "text" | "updateId">
) {
  return {
    bridge_instance_id: bridgeInstanceID,
    message: {
      chat: {
        id: 777,
        title: "ops",
        type: "supergroup",
      },
      date: Math.floor(Date.now() / 1000),
      from: {
        first_name: "Alice",
        id: 888,
        last_name: "Example",
        username: "alice",
      },
      message_id: input.messageId ?? browserBridgeOperatorFlowScenario.ingress.messageId,
      message_thread_id: Number(browserBridgeOperatorFlowScenario.testDelivery.threadId),
      text: input.text?.trim() || browserBridgeOperatorFlowScenario.ingress.text,
    },
    update_id: input.updateId ?? browserBridgeOperatorFlowScenario.ingress.updateId,
  };
}

async function appendJSONLine(targetPath: string, value: unknown): Promise<void> {
  await mkdir(path.dirname(targetPath), { recursive: true });
  const line = `${JSON.stringify(value)}\n`;
  await writeFile(targetPath, line, {
    encoding: "utf8",
    flag: "a",
  });
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

async function waitForSeedCondition<T>(
  read: () => Promise<T | null>,
  label: string,
  timeoutMs: number
): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown;

  while (Date.now() < deadline) {
    try {
      const value = await read();
      if (value !== null) {
        return value;
      }
    } catch (error) {
      lastError = error;
    }

    await delay(BROWSER_SEED_POLL_MS);
  }

  const detail = lastError instanceof Error ? `; last error: ${lastError.message}` : "";
  throw new Error(`timed out waiting for ${label}${detail}`);
}

/**
 * Block until an accepted seeded session is durably `active`. Accepted sessions
 * report `starting` until activation commits, and agent-authenticated
 * endpoints reject with `identity_stale` until the session is `active`. Poll the
 * real session record — never a sleep — and fail fast (throw) if the session ends
 * up `stopped`, surfacing a broken session as a setup failure rather than a later
 * 401. Exported so specs that create their own session can gate it before calling
 * agent-authenticated endpoints.
 */
export async function waitForSeedSessionActive(
  runtime: Pick<BrowserRuntimeSeedClient, "requestJSON">,
  sessionId: string,
  timeoutMs = BROWSER_SEED_SESSION_ACTIVE_TIMEOUT_MS
): Promise<SeededSessionPayload> {
  const deadline = Date.now() + timeoutMs;
  let lastState = "unknown";
  let lastError: unknown;

  while (Date.now() < deadline) {
    let session: SeededSessionPayload | undefined;
    try {
      ({ session } = await runtime.requestJSON<{ session: SeededSessionPayload }>(
        `/api/sessions/${encodeURIComponent(sessionId)}`
      ));
      lastState = session.state;
      lastError = undefined;
    } catch (error) {
      lastError = error;
    }

    if (session?.state === "active") {
      return session;
    }
    if (session?.state === "stopped") {
      const failureKind = session.failure?.kind?.trim() || "unknown";
      const failureSummary = session.failure?.summary?.trim() || "no failure summary";
      throw new Error(
        `seed session ${sessionId} entered "stopped" before activating; ` +
          `failure ${failureKind}: ${failureSummary}`
      );
    }

    await delay(BROWSER_SEED_POLL_MS);
  }

  const detail = lastError instanceof Error ? `; last error: ${lastError.message}` : "";
  throw new Error(
    `timed out waiting for seed session ${sessionId} to become active ` +
      `(last state: ${lastState})${detail}`
  );
}

async function delay(ms: number): Promise<void> {
  await new Promise(resolve => {
    setTimeout(resolve, ms);
  });
}

function shellQuote(argv: string[]): string {
  return argv
    .map(argument => {
      if (/^[A-Za-z0-9_./:@%+=,-]+$/.test(argument)) {
        return argument;
      }
      return `'${argument.replace(/'/g, `'"'"'`)}'`;
    })
    .join(" ");
}
