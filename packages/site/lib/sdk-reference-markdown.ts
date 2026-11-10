/** The SDK reference as Markdown and search text, for `llms` output and the docs search index. */
import type { AdvancedIndex } from "fumadocs-core/search/server";

import {
  bodyOf,
  countOf,
  methodName,
  optionsOf,
  returnsOf,
  signatureOf,
  type SdkLang,
  type SdkMethod,
  type SdkOption,
  type SdkResource,
} from "@/lib/sdk-reference-model";
import { sdkReference, sdkResourceById, sdkResourceForUrl } from "@/lib/sdk-reference";

const LANG_LABEL: Record<SdkLang, string> = { ts: "TypeScript", go: "Go" };
const FENCE: Record<SdkLang, string> = { ts: "ts", go: "go" };

function callLine(resource: SdkResource, method: SdkMethod, lang: SdkLang): string {
  const signature = signatureOf(resource, method, lang);
  if (signature === null) return `- ${LANG_LABEL[lang]}: no client method (WebSocket route).`;
  const returns = returnsOf(method, lang);
  const body = bodyOf(method, lang);
  return [
    `- ${LANG_LABEL[lang]}: \`${signature}\``,
    returns ? ` → \`${returns}\`` : "",
    body ? `; request body \`${body}\`` : "",
  ].join("");
}

function optionLine({ field, name }: SdkOption): string {
  const required = field.required ? ", required" : "";
  const description = field.description ? `: ${field.description}` : "";
  return `- \`${name}\` (${field.in}, \`${field.type}\`${required})${description}`;
}

function methodMarkdown(resource: SdkResource, method: SdkMethod): string {
  const lines = [
    `## ${methodName(method, "ts")}`,
    "",
    method.summary,
    "",
    `- Route: \`${method.method} ${method.path}\` (${method.kind})`,
    `- App tokens: ${method.app ? "allowed" : "denied"}`,
    callLine(resource, method, "ts"),
    callLine(resource, method, "go"),
  ];
  if (method.kind === "websocket") {
    const fields = optionsOf(method, "ts");
    if (fields.length > 0) lines.push("", "Route parameters:", "", ...fields.map(optionLine));
  } else {
    for (const lang of ["ts", "go"] as const) {
      const fields = optionsOf(method, lang);
      if (fields.length > 0) {
        lines.push("", `${LANG_LABEL[lang]} options:`, "", ...fields.map(optionLine));
      }
    }
  }
  for (const lang of ["ts", "go"] as const) {
    const sample = method.samples[lang];
    if (sample) lines.push("", `\`\`\`${FENCE[lang]}`, sample, "```");
  }
  return lines.join("\n");
}

export function sdkResourceMarkdown(resource: SdkResource): string {
  return resource.ops.map(method => methodMarkdown(resource, method)).join("\n\n");
}

const RESOURCE_TAG = /<SdkResource\s+id="([^"]+)"\s*\/>/g;

/** Replaces each `<SdkResource id="…" />` of a generated stub with the methods it renders. */
export function expandSdkResources(source: string): string {
  if (!RESOURCE_TAG.test(source)) return source;
  RESOURCE_TAG.lastIndex = 0;
  // The stub's "generated" comment is for editors, not readers.
  const body = source.replace(/\{\/\*[\s\S]*?\*\/\}\n*/g, "");
  return body.replace(RESOURCE_TAG, (tag, id: string) => {
    const resource = sdkResourceById(id);
    return resource ? sdkResourceMarkdown(resource) : tag;
  });
}

/** A hand-written SDK page the overview links to. */
export interface SdkGuideLink {
  url: string;
  title: string;
  description: string;
}

/** The language switch is page chrome: it says nothing in Markdown. */
const LANG_BAR_TAG = /^<SdkLangBar\s*\/>\n+/gm;
const GUIDE_LINKS_TAG = /<SdkGuideLinks\s*\/>/g;
const RESOURCE_GRID_TAG = /<SdkResourceGrid\s*\/>/g;

/**
 * Replaces the SDK overview's `<SdkGuideLinks />` and `<SdkResourceGrid />` with the lists they
 * render: the client guides (read only when the tag is there), and every resource with its method
 * count.
 */
export function expandSdkOverview(source: string, guides: () => readonly SdkGuideLink[]): string {
  return source
    .replace(LANG_BAR_TAG, "")
    .replace(GUIDE_LINKS_TAG, () =>
      guides()
        .map(guide => `- [${guide.title}](${guide.url}): ${guide.description}`)
        .join("\n")
    )
    .replace(RESOURCE_GRID_TAG, () =>
      sdkReference()
        .resources.map(
          resource =>
            `- [${resource.title}](/docs/sdk/${resource.slug}): ${countOf(resource.ops.length, "method")}, \`compozy.${resource.id}\` in TypeScript`
        )
        .join("\n")
    );
}

/** One search entry per method, so a method name or summary finds its resource page. */
export function sdkSearchData(
  url: string,
  structuredData: AdvancedIndex["structuredData"]
): AdvancedIndex["structuredData"] {
  const resource = sdkResourceForUrl(url);
  if (!resource) return structuredData;
  return {
    headings: [
      ...structuredData.headings,
      ...resource.ops.map(method => ({ id: method.id, content: method.id })),
    ],
    contents: [
      ...structuredData.contents,
      ...resource.ops.map(method => ({
        heading: method.id,
        content: [method.summary, method.go?.name, `${method.method} ${method.path}`]
          .filter(Boolean)
          .join(" · "),
      })),
    ],
  };
}
