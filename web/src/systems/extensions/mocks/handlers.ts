import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  bundleActivationFixtures,
  DEV_EXTENSION_WORKSPACE_ID,
  devExtensionFixture,
  extensionFixtures,
  extensionLogFixtures,
  extensionProvenanceFixtures,
} from "./fixtures";
import type { BundleActivation, ExtensionEntry } from "../types";

function cloneExtensions(): ExtensionEntry[] {
  return structuredClone(extensionFixtures);
}

function cloneBundleActivations(): BundleActivation[] {
  return structuredClone(bundleActivationFixtures);
}

function cloneDevExtensions(): Record<string, ExtensionEntry[]> {
  return { [DEV_EXTENSION_WORKSPACE_ID]: [structuredClone(devExtensionFixture)] };
}

function cloneExtensionLogs() {
  return structuredClone(extensionLogFixtures);
}

let extensionsState = cloneExtensions();
let bundleActivationsState = cloneBundleActivations();
let devExtensionsState = cloneDevExtensions();
let extensionLogsState = cloneExtensionLogs();

export function resetExtensionMockState(): void {
  extensionsState = cloneExtensions();
  bundleActivationsState = cloneBundleActivations();
  devExtensionsState = cloneDevExtensions();
  extensionLogsState = cloneExtensionLogs();
}

function extensionByName(name: string) {
  return extensionsState.find(extension => extension.name === name);
}

function activationById(id: string) {
  return bundleActivationsState.find(activation => activation.id === id);
}

/** Mirrors the daemon: `?workspace=` resolves that workspace's instances, absent the global rows. */
function extensionsForWorkspace(workspace: string): ExtensionEntry[] {
  if (workspace === "") return extensionsState;
  const overlays = devExtensionsState[workspace] ?? [];
  return [...overlays, ...extensionsState];
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/extensions", ({ request }) => {
    const workspace = new URL(request.url).searchParams.get("workspace")?.trim() ?? "";
    return HttpResponse.json({ extensions: extensionsForWorkspace(workspace) });
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
    const installed: ExtensionEntry = {
      ...structuredClone(extensionFixtures[0]!),
      digest_matched: source === "github",
      name: ref.split("/").pop() ?? ref,
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
    const workspace = search.get("workspace")?.trim() ?? "";
    const logs = (extensionLogsState[workspace]?.[name] ?? []).filter(
      entry => entry.sequence > after
    );
    return HttpResponse.json({ logs });
  }),
  compozyApiMock.get("/api/extensions/{name}/provenance", ({ params }) => {
    const name = String(params.name);
    const provenance = extensionProvenanceFixtures[name];
    return provenance
      ? HttpResponse.json({ provenance })
      : HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
  }),
  compozyApiMock.post("/api/extensions/{name}/enable", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    const enabled = { ...extension, enabled: true };
    extensionsState = extensionsState.map(item => (item.name === name ? enabled : item));
    return HttpResponse.json({ automation_started: [], extension: enabled });
  }),
  compozyApiMock.post("/api/extensions/{name}/disable", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    const disabled = { ...extension, enabled: false };
    extensionsState = extensionsState.map(item => (item.name === name ? disabled : item));
    return HttpResponse.json({ extension: disabled });
  }),
  compozyApiMock.put("/api/extensions/{name}", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    return extension
      ? HttpResponse.json({
          update: {
            current_version: extension.version,
            latest_version: extension.version,
            name,
            path: `/var/lib/compozy/extensions/${name}`,
            registry: "compozy",
            slug: extension.provenance?.slug ?? name,
            status: "current",
          },
        })
      : HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
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
        [workspace]: { ...extensionLogsState[workspace], [name]: [] },
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
  compozyApiMock.get("/api/bundles/activations", () =>
    HttpResponse.json({ activations: bundleActivationsState })
  ),
  compozyApiMock.get("/api/bundles/activations/{id}", ({ params }) => {
    const id = String(params.id);
    const activation = activationById(id);
    return activation
      ? HttpResponse.json({ activation })
      : HttpResponse.json({ error: `Bundle activation not found: ${id}` }, { status: 404 });
  }),
  compozyApiMock.patch("/api/bundles/activations/{id}", async ({ params, request }) => {
    const id = String(params.id);
    const activation = activationById(id);
    if (!activation) {
      return HttpResponse.json({ error: `Bundle activation not found: ${id}` }, { status: 404 });
    }
    const body = (await request.json()) as {
      confirm_network_requirement?: boolean;
      expected_version?: number;
    };
    if (body.expected_version !== activation.version) {
      return HttpResponse.json({ error: "Bundle activation version conflict" }, { status: 409 });
    }
    const updated = {
      ...activation,
      network_requirement_confirmed_at: body.confirm_network_requirement
        ? "2026-07-14T18:00:00Z"
        : activation.network_requirement_confirmed_at,
      network_requirement_confirmed_by: body.confirm_network_requirement
        ? "operator"
        : activation.network_requirement_confirmed_by,
      spec_drift: false,
      updated_at: "2026-07-14T18:00:00Z",
      version: activation.version + 1,
    };
    bundleActivationsState = bundleActivationsState.map(item => (item.id === id ? updated : item));
    return HttpResponse.json({ activation: updated });
  }),
  compozyApiMock.delete("/api/bundles/activations/{id}", ({ params }) => {
    const id = String(params.id);
    if (!activationById(id)) {
      return HttpResponse.json({ error: `Bundle activation not found: ${id}` }, { status: 404 });
    }
    bundleActivationsState = bundleActivationsState.filter(activation => activation.id !== id);
    return new HttpResponse(null, { status: 204 });
  }),
];
