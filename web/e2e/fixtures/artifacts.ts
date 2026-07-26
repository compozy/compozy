import { copyFile, mkdir, mkdtemp, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

const MANIFEST_VERSION = 1;
const TEMP_DIR_PREFIX = "agh-playwright-artifacts-";
const VITE_DEV_MARKERS = ["/@vite/client", "/src/main.tsx", "vite.svg"];

const browserArtifactSpecs = {
  browser_trace: { relativePath: "browser_trace.zip", isDir: false },
  browser_screenshots: { relativePath: "browser_screenshots", isDir: true },
  browser_console: { relativePath: "browser_console.json", isDir: false },
  browser_network: { relativePath: "browser_network.json", isDir: false },
  browser_api_snapshots: { relativePath: "browser_api_snapshots.json", isDir: false },
  browser_transport_snapshots: { relativePath: "browser_transport_snapshots.json", isDir: false },
  browser_route_state: { relativePath: "browser_route_state.json", isDir: false },
} as const;

export type BrowserArtifactKind = keyof typeof browserArtifactSpecs;

export interface ArtifactEntry {
  kind: BrowserArtifactKind;
  path: string;
  media_type?: string;
}

export interface ArtifactManifest {
  version: number;
  artifacts: ArtifactEntry[];
}

export interface BrowserConsoleEntry {
  type: string;
  text: string;
  location?: {
    url?: string;
    line_number?: number;
    column_number?: number;
  };
}

export interface BrowserNetworkEntry {
  event: "response" | "requestfailed";
  url: string;
  method: string;
  resource_type: string;
  status?: number;
  ok?: boolean;
  failure?: string;
}

export interface BrowserArtifactBundle {
  tracePath: string;
  screenshotPaths: string[];
  consoleEntries: BrowserConsoleEntry[];
  networkEntries: BrowserNetworkEntry[];
  routeState?: BrowserRouteState;
}

export interface BrowserRouteState {
  url: string;
  pathname: string;
  title: string;
  automation_active_tab?: "jobs" | "triggers";
  automation_delete_visible?: boolean;
  automation_detail_overflow_visible?: boolean;
  automation_enabled_toggle_visible?: boolean;
  automation_editor_kind?: "job" | "trigger";
  automation_editor_open?: boolean;
  automation_item_count?: number;
  automation_run_count?: number;
  automation_run_history_visible?: boolean;
  automation_scheduler_visible?: boolean;
  automation_scope_filter?: "all" | "global" | "workspace";
  automation_selected_item?: string;
  automation_session_link_count?: number;
  automation_trigger_visible?: boolean;
  automation_view_visible?: boolean;
  bridge_create_dialog_open?: boolean;
  bridge_detail_visible?: boolean;
  bridge_edit_dialog_open?: boolean;
  bridge_item_count?: number;
  bridge_route_count?: number;
  bridge_scope_filter?: "all" | "global" | "workspace";
  bridge_secret_binding_count?: number;
  bridge_selected_item?: string;
  bridge_test_delivery_open?: boolean;
  bridge_test_delivery_result_visible?: boolean;
  bridge_view_visible?: boolean;
  chat_view_visible: boolean;
  composer_clear_button_enabled?: boolean;
  composer_clear_button_visible?: boolean;
  delete_button_visible?: boolean;
  session_topbar_overflow_visible?: boolean;
  home_activity_count?: number;
  home_agent_count?: number;
  home_completed_today_value?: string;
  home_connection_status?: string;
  home_metric_count?: number;
  home_needs_you_value?: string;
  home_run_count?: number;
  home_scope_text?: string;
  home_usage_value?: string;
  home_view_visible?: boolean;
  home_working_now_value?: string;
  knowledge_create_dialog_open?: boolean;
  knowledge_decisions_count?: number;
  knowledge_delete_dialog_open?: boolean;
  knowledge_detail_visible?: boolean;
  knowledge_edit_dialog_open?: boolean;
  knowledge_item_count?: number;
  knowledge_revert_button_count?: number;
  knowledge_scope?: "global" | "workspace" | "agent";
  knowledge_search_active?: boolean;
  knowledge_selected_item?: string;
  knowledge_view_visible?: boolean;
  skills_content_visible?: boolean;
  skills_detail_visible?: boolean;
  skills_enabled_state?: "enabled" | "disabled";
  skills_item_count?: number;
  skills_search_active?: boolean;
  skills_selected_item?: string;
  skills_view_visible?: boolean;
  sandbox_action_result_visible?: boolean;
  sandbox_delete_dialog_open?: boolean;
  sandbox_editor_open?: boolean;
  sandbox_empty_visible?: boolean;
  sandbox_profile_count?: number;
  sandbox_profile_names?: string[];
  sandbox_restart_notice_visible?: boolean;
  sandbox_total_text?: string;
  sandbox_view_visible?: boolean;
  sandbox_workspace_references_text?: string;
  settings_action_result_visible?: boolean;
  settings_active_section?: string;
  settings_mcp_server_count?: number;
  settings_provider_card_count?: number;
  settings_restart_notice_visible?: boolean;
  settings_save_bar_visible?: boolean;
  settings_section_count?: number;
  settings_view_visible?: boolean;
  vault_delete_dialog_open?: boolean;
  vault_editor_open?: boolean;
  vault_secret_count?: number;
  session_name?: string;
  tasks_active_mode?: "dashboard" | "inbox" | "kanban" | "list";
  tasks_children_count?: number;
  tasks_dependencies_count?: number;
  tasks_detail_cancel_visible?: boolean;
  tasks_detail_delete_dialog_open?: boolean;
  tasks_detail_visible?: boolean;
  tasks_inbox_count?: number;
  tasks_review_count?: number;
  tasks_run_cancel_visible?: boolean;
  tasks_run_detail_visible?: boolean;
  tasks_selected_run?: string;
  tasks_selected_task?: string;
  tasks_task_count?: number;
  tasks_view_visible?: boolean;
  tool_card_count?: number;
  permission_prompt_visible: boolean;
  processing_indicator_visible: boolean;
  stop_button_visible: boolean;
  resume_button_visible: boolean;
  message_count: number;
  network_view_visible: boolean;
  network_active_tab?: "threads" | "directs" | "activity";
  network_channel_count: number;
  network_create_dialog_open?: boolean;
  network_disabled_visible?: boolean;
  network_thread_count: number;
  network_direct_count: number;
  network_activity_count?: number;
  network_message_count: number;
  network_no_channels_visible?: boolean;
  network_work_count?: number;
  network_selected_channel?: string;
  network_selected_thread?: string;
  network_selected_direct?: string;
}

export function isLikelyViteDevHTML(html: string): boolean {
  return VITE_DEV_MARKERS.some(marker => html.includes(marker));
}

export function resolveBrowserArtifactPath(rootDir: string, relativePath: string): string {
  const targetPath = path.resolve(rootDir, relativePath);
  const relativeTarget = path.relative(rootDir, targetPath);
  if (relativeTarget.startsWith("..") || path.isAbsolute(relativeTarget)) {
    throw new Error(`artifact path ${relativePath} escapes root ${rootDir}`);
  }
  return targetPath;
}

export async function mirrorBrowserScreenshotForQA(
  sourcePath: string,
  qaOutputRootDir: string,
  screenshotName?: string
): Promise<string> {
  const resolvedOutputRoot = path.resolve(qaOutputRootDir);
  const screenshotsDir = path.join(resolvedOutputRoot, "qa", "screenshots");
  const targetName = sanitizeArtifactName(
    screenshotName ?? path.basename(sourcePath, path.extname(sourcePath))
  );
  const targetPath = path.join(screenshotsDir, `${targetName}.png`);

  await mkdir(screenshotsDir, { recursive: true });
  await copyFile(sourcePath, targetPath);

  return targetPath;
}

export class ArtifactCollector {
  readonly rootDir: string;
  readonly manifestPath: string;

  private readonly entries = new Map<BrowserArtifactKind, ArtifactEntry>();

  private constructor(rootDir: string) {
    this.rootDir = rootDir;
    this.manifestPath = path.join(rootDir, "manifest.json");
  }

  static async create(rootDir?: string): Promise<ArtifactCollector> {
    const resolvedRoot =
      rootDir === undefined || rootDir.trim() === ""
        ? await mkdtemp(path.join(os.tmpdir(), TEMP_DIR_PREFIX))
        : rootDir;
    await mkdir(resolvedRoot, { recursive: true });
    return new ArtifactCollector(resolvedRoot);
  }

  artifactPath(kind: BrowserArtifactKind): string {
    const spec = browserArtifactSpecs[kind];
    return resolveBrowserArtifactPath(this.rootDir, spec.relativePath);
  }

  manifest(): ArtifactManifest {
    return {
      version: MANIFEST_VERSION,
      artifacts: [...this.entries.values()].sort((left, right) =>
        left.path.localeCompare(right.path)
      ),
    };
  }

  async writeManifest(): Promise<ArtifactManifest> {
    const manifest = this.manifest();
    await writeFile(this.manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
    return manifest;
  }

  async captureJSON(kind: BrowserArtifactKind, value: unknown): Promise<void> {
    const payload = `${JSON.stringify(value, null, 2)}\n`;
    await this.captureBytes(kind, Buffer.from(payload, "utf8"), "application/json");
  }

  async captureText(kind: BrowserArtifactKind, value: string): Promise<void> {
    await this.captureBytes(kind, Buffer.from(value, "utf8"), "text/plain");
  }

  async captureFile(
    kind: BrowserArtifactKind,
    sourcePath: string,
    mediaType: string
  ): Promise<void> {
    if (browserArtifactSpecs[kind].isDir) {
      throw new Error(`artifact ${kind} requires multiple files`);
    }

    const targetPath = this.artifactPath(kind);
    await mkdir(path.dirname(targetPath), { recursive: true });
    await copyFile(sourcePath, targetPath);
    this.entries.set(kind, {
      kind,
      path: browserArtifactSpecs[kind].relativePath,
      media_type: mediaType.trim(),
    });
  }

  async captureFiles(
    kind: BrowserArtifactKind,
    sourcePaths: string[],
    mediaType: string
  ): Promise<void> {
    if (!browserArtifactSpecs[kind].isDir) {
      throw new Error(`artifact ${kind} does not accept multiple files`);
    }

    const targetDir = this.artifactPath(kind);
    await mkdir(targetDir, { recursive: true });
    for (const sourcePath of sourcePaths) {
      await copyFile(sourcePath, path.join(targetDir, path.basename(sourcePath)));
    }

    this.entries.set(kind, {
      kind,
      path: browserArtifactSpecs[kind].relativePath,
      media_type: mediaType.trim(),
    });
  }

  private async captureBytes(
    kind: BrowserArtifactKind,
    payload: Uint8Array,
    mediaType: string
  ): Promise<void> {
    if (browserArtifactSpecs[kind].isDir) {
      throw new Error(`artifact ${kind} requires file collection`);
    }

    const targetPath = this.artifactPath(kind);
    await mkdir(path.dirname(targetPath), { recursive: true });
    await writeFile(targetPath, payload);
    this.entries.set(kind, {
      kind,
      path: browserArtifactSpecs[kind].relativePath,
      media_type: mediaType.trim(),
    });
  }
}

export async function persistBrowserArtifacts(
  collector: ArtifactCollector,
  bundle: BrowserArtifactBundle
): Promise<ArtifactManifest> {
  await collector.captureFile("browser_trace", bundle.tracePath, "application/zip");
  if (bundle.screenshotPaths.length > 0) {
    await collector.captureFiles("browser_screenshots", bundle.screenshotPaths, "image/png");
  }
  await collector.captureJSON("browser_console", bundle.consoleEntries);
  await collector.captureJSON("browser_network", bundle.networkEntries);
  if (bundle.routeState) {
    await collector.captureJSON("browser_route_state", bundle.routeState);
  }
  return collector.writeManifest();
}

function sanitizeArtifactName(value: string): string {
  const trimmed = value.trim().toLowerCase();
  if (trimmed === "") {
    return "artifact";
  }

  return trimmed.replace(/[^a-z0-9._-]+/g, "-");
}
