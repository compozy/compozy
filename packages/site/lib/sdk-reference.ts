/**
 * The SDK reference data, written by `scripts/generate-sdk-reference.ts` (`bun run generate:sdk`).
 * Server-side: pages, `llms` output and the search index read it; client components receive
 * one resource at a time.
 */
import data from "@/lib/sdk-reference-data.json";
import { countOf, type SdkReference, type SdkResource } from "@/lib/sdk-reference-model";

export * from "@/lib/sdk-reference-model";

const REFERENCE = data as unknown as SdkReference;
const BY_SLUG = new Map(REFERENCE.resources.map(resource => [resource.slug, resource]));
const BY_ID = new Map(REFERENCE.resources.map(resource => [resource.id, resource]));

export function sdkReference(): SdkReference {
  return REFERENCE;
}

export function sdkResourceById(id: string): SdkResource | undefined {
  return BY_ID.get(id);
}

/** The resource a docs URL (`/docs/sdk/sessions`) shows, if any. */
export function sdkResourceForUrl(url: string): SdkResource | undefined {
  const match = /^\/docs\/sdk\/([^/]+)\/?$/.exec(url);
  return match ? BY_SLUG.get(match[1] ?? "") : undefined;
}

export function isSdkUrl(url: string): boolean {
  return /^\/docs\/sdk(\/|$)/.test(url);
}

/** The masthead line of a resource page and of the overview. */
export function sdkFacts(url: string): string | undefined {
  const resource = sdkResourceForUrl(url);
  if (resource) {
    const allowed = resource.ops.filter(method => method.app).length;
    return `${countOf(resource.ops.length, "method")} · ${allowed} available to app tokens`;
  }
  if (/^\/docs\/sdk\/?$/.test(url)) {
    const { stats } = REFERENCE;
    return `${countOf(stats.operations, "operation")} · ${countOf(stats.resources, "resource")} · ${stats.app} available to app tokens`;
  }
  return undefined;
}
