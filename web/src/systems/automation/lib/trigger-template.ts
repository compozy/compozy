/**
 * Client-side approximation of the Go `text/template` rendering the runtime
 * applies to a trigger prompt (`internal/automation/dispatch.go`). Used only for
 * the live preview, so it returns token arrays the UI maps to spans — never raw
 * HTML.
 *
 * The variable roots match the real `ActivationEnvelope` fields exactly:
 * `.Kind`, `.Scope`, `.WorkspaceID`, `.Source`, and `.Data.<field>`.
 */

import type { TriggerEnvelope } from "./trigger-catalog";

export const VARIABLE_ROOTS = ["Kind", "Scope", "WorkspaceID", "Source"] as const;

const ROOT_TO_ENVELOPE_KEY: Record<string, keyof TriggerEnvelope> = {
  Kind: "kind",
  Scope: "scope",
  WorkspaceID: "workspace_id",
  Source: "source",
};

/** Builds the click-to-insert variable chips for the selected event. */
export function buildVariableChips(dataFields: string[]): string[] {
  const roots = VARIABLE_ROOTS.map(root => `{{ .${root} }}`);
  const data = dataFields.map(field => `{{ .${field.replace(/^data\./, "Data.")} }}`);
  return [...roots, ...data];
}

export type RenderToken =
  | { id: string; type: "text"; value: string }
  | { id: string; type: "var"; value: string }
  | { id: string; type: "missing"; name: string };

const TEMPLATE_PATTERN = /\{\{\s*(?:index\s+\.Data\s+"([^"]+)"|\.([\w.]+))\s*\}\}/g;

function lookup(env: TriggerEnvelope, path: string): string | undefined {
  const envelopeKey = ROOT_TO_ENVELOPE_KEY[path];
  if (envelopeKey) {
    const value = env[envelopeKey];
    return typeof value === "string" ? value : undefined;
  }
  if (path.startsWith("Data.")) {
    return env.data[path.slice(5)];
  }
  return undefined;
}

/**
 * Renders the prompt against the sample envelope. Resolved variables become
 * `var` tokens; unresolved ones become `missing` tokens flagged in the preview.
 */
export function renderTemplate(template: string, env: TriggerEnvelope): RenderToken[] {
  const tokens: RenderToken[] = [];
  let lastIndex = 0;
  TEMPLATE_PATTERN.lastIndex = 0;

  let match: RegExpExecArray | null = TEMPLATE_PATTERN.exec(template);
  while (match !== null) {
    if (match.index > lastIndex) {
      tokens.push({
        id: `text:${lastIndex}`,
        type: "text",
        value: template.slice(lastIndex, match.index),
      });
    }
    const dataKey = match[1];
    const path = dataKey ? `Data.${dataKey}` : match[2];
    const value = lookup(env, path);
    if (value === undefined) {
      tokens.push({
        id: `missing:${match.index}`,
        type: "missing",
        name: dataKey ?? match[2] ?? "",
      });
    } else {
      tokens.push({ id: `var:${match.index}`, type: "var", value });
    }
    lastIndex = TEMPLATE_PATTERN.lastIndex;
    match = TEMPLATE_PATTERN.exec(template);
  }

  if (lastIndex < template.length) {
    tokens.push({ id: `text:${lastIndex}`, type: "text", value: template.slice(lastIndex) });
  }
  return tokens;
}

export type TemplateToken = { id: string; type: "text" | "var"; value: string };

/** Splits the raw template into text/variable tokens for the tinted source view. */
export function tokenizeTemplate(template: string): TemplateToken[] {
  const tokens: TemplateToken[] = [];
  const parts = template.split(/(\{\{[^}]*\}\})/);
  let offset = 0;
  for (const part of parts) {
    if (part === "") continue;
    const type = part.startsWith("{{") ? "var" : "text";
    tokens.push({ id: `${type}:${offset}`, type, value: part });
    offset += part.length;
  }
  return tokens;
}
