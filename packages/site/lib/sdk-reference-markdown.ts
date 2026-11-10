/** The SDK reference as Markdown and search text, for `llms` output and the docs search index. */
import type { AdvancedIndex } from "fumadocs-core/search/server";

import {
  bodyOf,
  methodName,
  returnsOf,
  signatureOf,
  type SdkLang,
  type SdkMethod,
  type SdkResource,
} from "@/lib/sdk-reference-model";
import { sdkResourceById, sdkResourceForUrl } from "@/lib/sdk-reference";

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
  if (method.params.length > 0) {
    lines.push("", "Options:", "");
    for (const field of method.params) {
      const required = field.required ? ", required" : "";
      const description = field.description ? `: ${field.description}` : "";
      lines.push(`- \`${field.name}\` (${field.in}, \`${field.type}\`${required})${description}`);
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
