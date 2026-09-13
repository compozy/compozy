import { MarketplaceSourceApiError } from "../adapters/marketplace-sources-api";
import type { MarketplaceSourcePreview } from "../types";

/** Where the daemon looks when no `checked[]` came back with the refusal. */
const DEFAULT_CHECKED_PATHS = ["marketplace.json", ".claude-plugin/marketplace.json"] as const;

export interface AddMarketplaceDraft {
  ref: string;
  name: string;
}

export interface AddMarketplaceFailure {
  tone: "danger" | "warning";
  code: string | null;
  title: string;
  description?: string;
  /** Which field carries `aria-invalid`. */
  field: "ref" | "name" | null;
  /** A name the daemon proposed instead (`marketplace_source_exists`). */
  suggestedName?: string;
  /** Extensions still carrying the retained name. */
  retainedBy: string[];
  /** Document paths the daemon checked before refusing the reference. */
  checked: string[];
}

export function normalizeMarketplaceDraft(draft: AddMarketplaceDraft): AddMarketplaceDraft {
  return { ref: draft.ref.trim(), name: draft.name.trim() };
}

export function sameMarketplaceDraft(left: AddMarketplaceDraft, right: AddMarketplaceDraft) {
  const a = normalizeMarketplaceDraft(left);
  const b = normalizeMarketplaceDraft(right);
  return a.ref === b.ref && a.name === b.name;
}

export function pluralPlugins(count: number): string {
  return count === 1 ? "1 plugin" : `${count} plugins`;
}

/** "claude-plugins-official · owner Anthropic · 6 plugins listed, 6 can be installed." */
export function previewSummary(preview: MarketplaceSourcePreview): string {
  const parts = [preview.name];
  if (preview.owner?.trim()) parts.push(`owner ${preview.owner.trim()}`);
  parts.push(`${pluralPlugins(preview.plugins)} listed, ${preview.installable} can be installed`);
  return `${parts.join(" · ")}.`;
}

function joinPaths(paths: readonly string[]): string {
  if (paths.length === 0) return "";
  if (paths.length === 1) return paths[0]!;
  return `${paths.slice(0, -1).join(", ")} and ${paths.at(-1)}`;
}

function joinNames(names: readonly string[]): string {
  return joinPaths(names);
}

/**
 * Maps a dry-run or registration refusal onto one actionable notice. Every sentence restates a
 * field the daemon sent; nothing is inferred from the reference the person typed.
 */
export function describeMarketplaceFailure(
  error: unknown,
  draft: AddMarketplaceDraft,
  fallback: string
): AddMarketplaceFailure {
  const base: AddMarketplaceFailure = {
    tone: "danger",
    code: null,
    title: fallback,
    field: null,
    retainedBy: [],
    checked: [],
  };
  if (!(error instanceof MarketplaceSourceApiError)) {
    return error instanceof Error && error.message.trim() !== ""
      ? { ...base, title: error.message }
      : base;
  }
  const code = error.diagnosticCode ?? null;
  const name = normalizeMarketplaceDraft(draft).name;
  switch (code) {
    case "marketplace_not_a_marketplace": {
      const checked = error.checked.length > 0 ? error.checked : [...DEFAULT_CHECKED_PATHS];
      return {
        ...base,
        code,
        field: "ref",
        checked,
        title: "This repository has no plugin list.",
        description: `Compozy looked for ${joinPaths(checked)}. If it is a single plugin, install it from Add ▾ › Install extension from GitHub.`,
      };
    }
    case "marketplace_document_too_large":
      return {
        ...base,
        code,
        field: "ref",
        title: "This plugin list is too large to read.",
        description: error.message,
      };
    case "marketplace_source_invalid_ref":
      return {
        ...base,
        code,
        field: "ref",
        title: "Compozy cannot read this reference.",
        description: error.message,
      };
    case "marketplace_source_exists":
      return {
        ...base,
        tone: "warning",
        code,
        field: "name",
        suggestedName: error.suggestedName,
        title: name
          ? `A marketplace named ${name} is already registered.`
          : "A marketplace with this name is already registered.",
        description: error.suggestedName
          ? `Register it as ${error.suggestedName} instead, or pick another name.`
          : "Pick another name to register it again.",
      };
    case "marketplace_source_name_retained":
      return {
        ...base,
        code,
        field: "name",
        retainedBy: error.retainedBy,
        title: `The name ${name || "you chose"} is still used by installed extensions.`,
        description:
          error.retainedBy.length > 0
            ? `${joinNames(error.retainedBy)} came from a different marketplace under this name. Remove them or pick another name.`
            : "Remove those extensions or pick another name.",
      };
    case "marketplace_source_name_reserved":
      return {
        ...base,
        code,
        field: "name",
        title: `${name || "This name"} is reserved for the Compozy catalog.`,
        description: "Pick another name.",
      };
    case "marketplace_source_preset_readonly":
      return { ...base, code, field: null, title: error.message };
    default:
      return { ...base, code, title: error.message || fallback };
  }
}
