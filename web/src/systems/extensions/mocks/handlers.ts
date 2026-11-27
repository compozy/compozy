import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  DEV_EXTENSION_WORKSPACE_ID,
  devExtensionFixture,
  extensionFixtures,
  extensionInventoryDiagnosticsFixtures,
  extensionInventoryFixtures,
  extensionLogFixtures,
  extensionProvenanceFixtures,
} from "./fixtures";
import type { ExtensionInventoryDiagnostic } from "../lib/extension-skipped-diagnostics";
import type {
  ExtensionEntry,
  ExtensionKitItem,
  ExtensionLogsSnapshot,
  ExtensionRemovedBuiltin,
} from "../types";

function cloneExtensions(): ExtensionEntry[] {
  return structuredClone(extensionFixtures);
}

function cloneInventory(): Record<string, ExtensionKitItem[]> {
  return structuredClone(extensionInventoryFixtures);
}

function cloneInventoryDiagnostics(): Record<string, ExtensionInventoryDiagnostic[]> {
  return structuredClone(extensionInventoryDiagnosticsFixtures);
}

function cloneDevExtensions(): Record<string, ExtensionEntry[]> {
  return { [DEV_EXTENSION_WORKSPACE_ID]: [structuredClone(devExtensionFixture)] };
}

function cloneExtensionLogs() {
  return structuredClone(extensionLogFixtures);
}

function emptyExtensionLogs(name: string, workspace: string): ExtensionLogsSnapshot {
  return {
    logs: [],
    stream_epoch: `mock-${workspace || "global"}-${name}`,
  };
}

let extensionsState = cloneExtensions();
let inventoryState = cloneInventory();
let inventoryDiagnosticsState = cloneInventoryDiagnostics();
let devExtensionsState = cloneDevExtensions();
let extensionLogsState = cloneExtensionLogs();
let profileEnablementState: Record<string, Record<string, boolean>> = {};
/** Tombstones from permanent removal, in the order the daemon lists them. */
let removedBuiltinsState: ExtensionRemovedBuiltin[] = [];

export function resetExtensionMockState(): void {
  extensionsState = cloneExtensions();
  inventoryState = cloneInventory();
  inventoryDiagnosticsState = cloneInventoryDiagnostics();
  devExtensionsState = cloneDevExtensions();
  extensionLogsState = cloneExtensionLogs();
  profileEnablementState = {};
  removedBuiltinsState = [];
}

function extensionByName(name: string) {
  return extensionsState.find(extension => extension.name === name);
}

/**
 * Mirrors the extensions_profile_enablement_delete trigger: deleting an extension row deletes its enablement
 * exceptions in every profile. The daemon skips that cleanup for a dev-linked name; the mock models no dev links.
 */
function dropProfileEnablement(name: string): void {
  const next: Record<string, Record<string, boolean>> = {};
  for (const [profile, byName] of Object.entries(profileEnablementState)) {
    next[profile] = Object.fromEntries(Object.entries(byName).filter(([item]) => item !== name));
  }
  profileEnablementState = next;
}

/** Test seam: installs a bundled built-in row, the only source permanent removal accepts. */
export function seedBundledExtensionMockState(entry: ExtensionEntry): void {
  extensionsState = [...extensionsState.filter(item => item.name !== entry.name), entry];
}

/** Mirrors the daemon: `?workspace=` resolves that workspace's instances, absent the global rows. */
function extensionsForWorkspace(workspace: string): ExtensionEntry[] {
  if (workspace === "") return extensionsState;
  const overlays = devExtensionsState[workspace] ?? [];
  return [...overlays, ...extensionsState];
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/extensions", ({ request }) => {
    const search = new URL(request.url).searchParams;
    const workspace = search.get("workspace")?.trim() ?? "";
    const profile = search.get("profile")?.trim() || "default";
    return HttpResponse.json({
      extensions: extensionsForWorkspace(workspace).map(extension => ({
        ...extension,
        enabled: profileEnablementState[profile]?.[extension.name] ?? extension.enabled,
      })),
    });
  }),
  compozyApiMock.post("/api/extensions/preview-install", async ({ request }) => {
    const body = (await request.json()) as { ref?: string; source?: string };
    const name = body.ref?.trim().split("/").pop() ?? "";
    if (name === "" || body.source?.trim() === "") {
      return HttpResponse.json({ error: "extension source and ref are required" }, { status: 400 });
    }
    return HttpResponse.json({
      inputs: [],
      declared_profiles: [{ create: false, credentials: [], name: "default" }],
      name,
      placements: [],
    });
  }),
  compozyApiMock.post("/api/extensions", async ({ request }) => {
    const body = (await request.json()) as {
      allow_unverified?: boolean;
      ref?: string;
      source?: string;
    };
    const source = body.source?.trim() ?? "";
    const ref = body.ref?.trim() ?? "";
    if (source === "" || ref === "") {
      return HttpResponse.json({ error: "extension source and ref are required" }, { status: 400 });
    }
    if (!["curated", "github", "git", "local_path"].includes(source)) {
      return HttpResponse.json(
        { error: `unsupported extension source "${source}"` },
        { status: 400 }
      );
    }
    if (source !== "curated" && body.allow_unverified !== true) {
      return HttpResponse.json(
        { error: "extension trust decision required: rerun with allow_unverified" },
        { status: 422 }
      );
    }
    const name = ref.split("/").pop() ?? ref;
    const installed: ExtensionEntry = {
      ...structuredClone(extensionFixtures[0]!),
      digest_matched: source === "github",
      name,
      provenance: undefined,
      source,
      trust: undefined,
      update_available: false,
    };
    extensionsState = [...extensionsState, installed];
    return HttpResponse.json({ extension: installed }, { status: 201 });
  }),
  compozyApiMock.get("/api/extensions/{name}/logs", ({ params, request }) => {
    const name = String(params.name);
    const search = new URL(request.url).searchParams;
    const after = Number(search.get("after") ?? "0");
    const requestedEpoch = search.get("stream_epoch")?.trim() ?? "";
    const workspace = search.get("workspace")?.trim() ?? "";
    if (after > 0 && requestedEpoch === "") {
      return HttpResponse.json(
        { error: "stream_epoch is required when after is greater than zero" },
        { status: 400 }
      );
    }
    const snapshot = extensionLogsState[workspace]?.[name] ?? emptyExtensionLogs(name, workspace);
    const reset = requestedEpoch !== "" && requestedEpoch !== snapshot.stream_epoch;
    return HttpResponse.json({
      logs: reset ? snapshot.logs : snapshot.logs.filter(entry => entry.sequence > after),
      stream_epoch: snapshot.stream_epoch,
    });
  }),
  compozyApiMock.get("/api/extensions/{name}/provenance", ({ params }) => {
    const name = String(params.name);
    const provenance = extensionProvenanceFixtures[name];
    return provenance
      ? HttpResponse.json({ provenance })
      : HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
  }),
  compozyApiMock.get("/api/extensions/{name}/inventory", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json({
      diagnostics: inventoryDiagnosticsState[name] ?? [],
      enabled: extension.enabled,
      extension: name,
      format: extension.format,
      items: inventoryState[name] ?? [],
    });
  }),
  compozyApiMock.put("/api/extensions/{name}/enablement", async ({ params, request }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json()) as { enabled?: boolean; profile?: string };
    const profile = body.profile?.trim() ?? "";
    if (profile === "" || typeof body.enabled !== "boolean") {
      return HttpResponse.json({ error: "profile and enabled are required" }, { status: 400 });
    }
    profileEnablementState = {
      ...profileEnablementState,
      [profile]: { ...profileEnablementState[profile], [name]: body.enabled },
    };
    return HttpResponse.json({ enabled: body.enabled, profile });
  }),
  compozyApiMock.put("/api/extensions/{name}", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    return HttpResponse.json({
      update: {
        current_version: extension.version,
        latest_version: extension.version,
        name,
        path: `/var/lib/compozy/extensions/${name}`,
        registry: "compozy",
        slug: extension.provenance?.slug ?? name,
        status: "current",
      },
    });
  }),
  compozyApiMock.delete("/api/extensions/{name}", ({ params, request }) => {
    const name = String(params.name);
    const search = new URL(request.url).searchParams;
    const workspace = search.get("workspace")?.trim() ?? "";
    const profile = search.get("profile")?.trim() ?? "";
    const permanent = search.get("permanent") === "true";
    // Mirror the daemon: permanent removal is global, so a workspace or profile scope is refused before any branch
    // below mutates state.
    if (permanent && (workspace !== "" || profile !== "")) {
      return HttpResponse.json(
        {
          error:
            "permanent removal applies to the global built-in; drop the workspace or profile query",
        },
        { status: 400 }
      );
    }
    if (workspace !== "") {
      const overlays = devExtensionsState[workspace] ?? [];
      const extension = overlays.find(item => item.name === name);
      if (!extension) {
        return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
      }
      devExtensionsState = {
        ...devExtensionsState,
        [workspace]: overlays.filter(item => item.name !== name),
      };
      extensionLogsState = {
        ...extensionLogsState,
        [workspace]: {
          ...extensionLogsState[workspace],
          [name]: emptyExtensionLogs(name, workspace),
        },
      };
      return HttpResponse.json({
        extension: {
          name,
          path: extension.origin_path ?? `/var/lib/compozy/extensions/${name}`,
          permanent: false,
          status: "removed",
          tombstoned_at: "",
        },
      });
    }
    const existing = extensionByName(name);
    if (!existing) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    if (permanent && existing.source !== "bundled") {
      return HttpResponse.json(
        {
          code: "extension_not_builtin",
          error: `${name} has source "${existing.source}"; --permanent applies only to built-in extensions`,
        },
        { status: 422 }
      );
    }
    if (permanent) {
      removedBuiltinsState = [
        ...removedBuiltinsState,
        { name, removed_at: new Date().toISOString(), removed_by: "operator" },
      ];
    }
    extensionsState = extensionsState.filter(extension => extension.name !== name);
    dropProfileEnablement(name);
    return HttpResponse.json({
      extension: {
        name,
        path: `/var/lib/compozy/extensions/${name}`,
        permanent,
        status: "removed",
        tombstoned_at: permanent ? new Date().toISOString() : "",
      },
    });
  }),
  compozyApiMock.get("/api/extensions/removed-builtins", () =>
    HttpResponse.json({ removed: removedBuiltinsState })
  ),
  /**
   * Contract: restore always leaves the built-in enabled. The daemon enrolls a fresh row, enables it for the default
   * profile, and reloads; the removal before it deleted every enablement exception, so no profile keeps a disabled
   * state. The mock clears the name's exceptions and answers enabled, so the response and the next list agree.
   */
  compozyApiMock.post("/api/extensions/{name}/restore", ({ params }) => {
    const name = String(params.name);
    if (!removedBuiltinsState.some(item => item.name === name)) {
      return HttpResponse.json(
        { code: "extension_builtin_not_removed", error: `${name} is not permanently removed` },
        { status: 409 }
      );
    }
    const installed = extensionByName(name);
    if (installed && installed.source !== "bundled") {
      return HttpResponse.json(
        {
          code: "extension_exists",
          error: `${name} is already installed from ${installed.source}; remove it first`,
        },
        { status: 409 }
      );
    }
    removedBuiltinsState = removedBuiltinsState.filter(item => item.name !== name);
    dropProfileEnablement(name);
    const restored: ExtensionEntry = {
      ...structuredClone(extensionFixtures[0]!),
      name,
      enabled: true,
      source: "bundled",
    };
    extensionsState = [...extensionsState.filter(item => item.name !== name), restored];
    return HttpResponse.json({
      name,
      status: "restored",
      source: "bundled",
      version: restored.version,
      enabled: true,
      path: `/var/lib/compozy/extensions/${name}`,
    });
  }),
];
