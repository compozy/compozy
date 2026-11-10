/** The SDK reference as Markdown and search text, for `llms` output and the docs search index. */
import type { AdvancedIndex } from "fumadocs-core/search/server";

import {
  bodyOf,
  countOf,
  methodName,
  optionsOf,
  returnsOf,
  SDK_NO_GO_EQUIVALENT,
  signatureOf,
  isSdkLang,
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

/** JSX attributes as the guides write them: `name="…"`, `type={'…'}`, or a bare flag. */
const ATTRS = String.raw`(?:\s+\w+(?:=(?:"[^"]*"|\{'[^']*'\}|\{"[^"]*"\}))?)*`;
const ATTR = /(\w+)(?:=(?:"([^"]*)"|\{'([^']*)'\}|\{"([^"]*)"\}))?/g;

function attrsOf(raw: string): Map<string, string> {
  const attrs = new Map<string, string>();
  for (const [, name = "", ...values] of raw.matchAll(ATTR)) {
    attrs.set(name, values.find(value => value !== undefined) ?? "");
  }
  return attrs;
}

const FIELD = new RegExp(
  String.raw`^[ \t]*<SdkField(${ATTRS})\s*(?:\/>|>([\s\S]*?)<\/SdkField>)`,
  "gm"
);
const FIELDS_OPEN = new RegExp(String.raw`^[ \t]*<SdkFields(${ATTRS})\s*>[ \t]*\n`, "gm");
const LANG_OPEN = new RegExp(String.raw`^[ \t]*<(?:SdkLang|SdkSymbol)(${ATTRS})\s*(\/?)>`, "gm");
const CLOSE = /\n*^[ \t]*<\/(?:SdkLang|SdkSymbol|SdkFields)>[ \t]*$/gm;

/** `<SdkField>`: one list item, as the resource pages list options. */
function fieldItem(tag: string, raw: string, children: string | undefined): string {
  const attrs = attrsOf(raw);
  const name = attrs.get("name");
  const type = attrs.get("type");
  if (name === undefined || type === undefined) return tag;
  const required = attrs.has("required") ? ", required" : "";
  const text = children?.replace(/\s+/g, " ").trim();
  return `- \`${name}\` (\`${type}\`${required})${text ? `: ${text}` : ""}`;
}

/** `<SdkLang>` and `<SdkSymbol>`: the content they show for one client, under its name. */
function langLabel(tag: string, raw: string, selfClosing: string): string {
  const attrs = attrsOf(raw);
  const lang = attrs.get("lang");
  if (!isSdkLang(lang)) return tag;
  if (!selfClosing) return `**${LANG_LABEL[lang]}**`;
  return attrs.has("none") ? `**${LANG_LABEL[lang]}**\n\n${SDK_NO_GO_EQUIVALENT}` : tag;
}

/**
 * Unwraps the client guides' per-language content: each `<SdkLang>` and `<SdkSymbol>` block under
 * a "TypeScript" or "Go" label, each `<SdkFields>` as a titled list of its `<SdkField>` rows. The
 * page shows one language at a time; the Markdown keeps both.
 */
export function expandSdkLanguages(source: string): string {
  return source
    .replace(FIELD, fieldItem)
    .replace(FIELDS_OPEN, (tag, raw: string) => {
      const title = attrsOf(raw).get("title");
      return title ? `${title}:\n\n` : tag;
    })
    .replace(LANG_OPEN, langLabel)
    .replace(CLOSE, "");
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
