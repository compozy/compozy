import { HttpResponse, type HttpHandler } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";

import {
  bundleActivationFixtures,
  extensionFixtures,
  extensionProvenanceFixtures,
} from "./fixtures";
import type { BundleActivation, ExtensionEntry } from "../types";

function cloneExtensions(): ExtensionEntry[] {
  return structuredClone(extensionFixtures);
}

function cloneBundleActivations(): BundleActivation[] {
  return structuredClone(bundleActivationFixtures);
}

let extensionsState = cloneExtensions();
let bundleActivationsState = cloneBundleActivations();

export function resetExtensionMockState(): void {
  extensionsState = cloneExtensions();
  bundleActivationsState = cloneBundleActivations();
}

function extensionByName(name: string) {
  return extensionsState.find(extension => extension.name === name);
}

function activationById(id: string) {
  return bundleActivationsState.find(activation => activation.id === id);
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/extensions", () => HttpResponse.json({ extensions: extensionsState })),
  aghApiMock.get("/api/extensions/{name}/provenance", ({ params }) => {
    const name = String(params.name);
    const provenance = extensionProvenanceFixtures[name];
    return provenance
      ? HttpResponse.json({ provenance })
      : HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
  }),
  aghApiMock.post("/api/extensions/{name}/enable", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    const enabled = { ...extension, enabled: true };
    extensionsState = extensionsState.map(item => (item.name === name ? enabled : item));
    return HttpResponse.json({ extension: enabled });
  }),
  aghApiMock.post("/api/extensions/{name}/disable", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    if (!extension) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    const disabled = { ...extension, enabled: false };
    extensionsState = extensionsState.map(item => (item.name === name ? disabled : item));
    return HttpResponse.json({ extension: disabled });
  }),
  aghApiMock.put("/api/extensions/{name}", ({ params }) => {
    const name = String(params.name);
    const extension = extensionByName(name);
    return extension
      ? HttpResponse.json({
          update: {
            current_version: extension.version,
            latest_version: extension.version,
            name,
            path: `/var/lib/agh/extensions/${name}`,
            registry: "agh",
            slug: extension.provenance?.slug ?? name,
            status: "current",
          },
        })
      : HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
  }),
  aghApiMock.delete("/api/extensions/{name}", ({ params }) => {
    const name = String(params.name);
    if (!extensionByName(name)) {
      return HttpResponse.json({ error: `Extension not found: ${name}` }, { status: 404 });
    }
    extensionsState = extensionsState.filter(extension => extension.name !== name);
    return HttpResponse.json({
      extension: { name, path: `/var/lib/agh/extensions/${name}`, status: "removed" },
    });
  }),
  aghApiMock.get("/api/bundles/activations", () =>
    HttpResponse.json({ activations: bundleActivationsState })
  ),
  aghApiMock.get("/api/bundles/activations/{id}", ({ params }) => {
    const id = String(params.id);
    const activation = activationById(id);
    return activation
      ? HttpResponse.json({ activation })
      : HttpResponse.json({ error: `Bundle activation not found: ${id}` }, { status: 404 });
  }),
  aghApiMock.patch("/api/bundles/activations/{id}", async ({ params, request }) => {
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
  aghApiMock.delete("/api/bundles/activations/{id}", ({ params }) => {
    const id = String(params.id);
    if (!activationById(id)) {
      return HttpResponse.json({ error: `Bundle activation not found: ${id}` }, { status: 404 });
    }
    bundleActivationsState = bundleActivationsState.filter(activation => activation.id !== id);
    return new HttpResponse(null, { status: 204 });
  }),
];
