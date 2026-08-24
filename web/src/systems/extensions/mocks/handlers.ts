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
import type { ExtensionEntry, ExtensionKitItem, ExtensionLogsSnapshot } from "../types";

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

export function resetExtensionMockState(): void {
  extensionsState = cloneExtensions();
  inventoryState = cloneInventory();
  inventoryDiagnosticsState = cloneInventoryDiagnostics();
  devExtensionsState = cloneDevExtensions();
  extensionLogsState = cloneExtensionLogs();
  profileEnablementState = {};
}

function extensionByName(name: string) {
  return extensionsState.find(extension => extension.name === name);
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
    const digest = name === "dep-kit-ops" ? "sha256:6f1c0a94d3b27e58" : undefined;
    return HttpResponse.json({
      declared_profiles: [{ create: false, credentials: [], name: "default" }],
      name,
      ...(digest ? { network_requirement_digest: digest } : {}),
      placements: [],
    });
  }),
  compozyApiMock.post("/api/extensions", async ({ request }) => {
    const body = (await request.json()) as {
      allow_unverified?: boolean;
      confirm_network_digest?: string;
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
    const digest = name === "dep-kit-ops" ? "sha256:6f1c0a94d3b27e58" : undefined;
    if (digest && body.confirm_network_digest !== digest) {
      return HttpResponse.json(
        {
          code: "extension_network_confirmation_required",
          current_digest: digest,
          error: `${name} declares Live network participation that has not been confirmed`,
        },
        { status: 409 }
      );
    }
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
  /** Mirrors the candidate gate used by update: the current digest stands in for the next release. */
  compozyApiMock.put("/api/extensions/{name}", async ({ params, request, response }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    const body = (await request.json().catch(() => ({}))) as { confirm_network_digest?: string };
    const digest = extension.network_requirement_digest;
    if (
      extension.network_confirmation_required &&
      digest &&
      body.confirm_network_digest !== digest
    ) {
      return response(409).json({
        code: "extension_network_confirmation_required",
        current_digest: digest,
        error: `${name} update changes Live network participation that has not been confirmed`,
      });
    }
    const updated = { ...extension, network_confirmation_required: false };
    extensionsState = extensionsState.map(item => (item.name === name ? updated : item));
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
    const workspace = new URL(request.url).searchParams.get("workspace")?.trim() ?? "";
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
          status: "removed",
        },
      });
    }
    if (!extensionByName(name)) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    extensionsState = extensionsState.filter(extension => extension.name !== name);
    return HttpResponse.json({
      extension: { name, path: `/var/lib/compozy/extensions/${name}`, status: "removed" },
    });
  }),
];
