import type { AgentSoulPayload } from "../types";
import { serializeAgentSoulSource } from "./agent-authored-file-source";
import {
  markdownBlock,
  parseFrontmatter,
  splitFrontmatter,
  type FileBlock,
  type FrontmatterValue,
} from "./agent-frontmatter";

export type SoulTone = "direct" | "friendly" | "formal" | "custom";

const PRESET_TONES: readonly string[] = ["direct", "friendly", "formal"];

/** SOUL.md as What it does shows it; `kept` is what the page doesn't edit but must not lose. */
export interface SoulForm {
  body: string;
  /** `null` when the file says nothing, so saving doesn't invent a tone. */
  tone: SoulTone | null;
  customTone: string;
  never: readonly string[];
  kept: {
    version?: string;
    role?: string;
    principles: readonly string[];
    collaboration: readonly string[];
    tags: readonly string[];
  };
}

export const BLANK_SOUL: SoulForm = {
  body: "",
  tone: "direct",
  customTone: "",
  never: [],
  kept: { version: "1", principles: [], collaboration: [], tags: [] },
};

const KNOWN_KEYS: ReadonlySet<string> = new Set([
  "version",
  "role",
  "tone",
  "principles",
  "constraints",
  "collaboration",
  "tags",
]);

function strings(value: FrontmatterValue | undefined): readonly string[] | null {
  if (value === undefined || value === null) return [];
  if (typeof value === "string") return value.trim() ? [value.trim()] : [];
  if (!Array.isArray(value)) return null;
  const items = value as readonly FrontmatterValue[];
  return items.every(item => typeof item === "string") ? (items as readonly string[]) : null;
}

function text(value: FrontmatterValue | undefined) {
  return typeof value === "string" || typeof value === "number" ? String(value) : undefined;
}

export interface ParsedSoul {
  form: SoulForm;
  block: FileBlock | null;
}

function blocked(block: FileBlock): ParsedSoul {
  return { form: BLANK_SOUL, block };
}

export function parseSoulFile(source: string): ParsedSoul {
  const file = splitFrontmatter(source);
  if (!file) return blocked("mistake");
  const data = file.metadata === null ? {} : parseFrontmatter(file.metadata);
  if (!data) return blocked("settings");
  if (Object.keys(data).some(key => !KNOWN_KEYS.has(key))) return blocked("settings");
  const tone = strings(data.tone);
  const never = strings(data.constraints);
  const principles = strings(data.principles);
  const collaboration = strings(data.collaboration);
  const tags = strings(data.tags);
  if (!tone || !never || !principles || !collaboration || !tags || tone.length > 1) {
    return blocked("settings");
  }
  const body = file.body.trimEnd();
  const bodyBlock = markdownBlock(body);
  if (bodyBlock) return blocked(bodyBlock);
  const word = tone[0];
  return {
    form: {
      body,
      tone: word === undefined ? null : PRESET_TONES.includes(word) ? (word as SoulTone) : "custom",
      customTone: word !== undefined && !PRESET_TONES.includes(word) ? word : "",
      never,
      kept: {
        version: text(data.version),
        role: text(data.role),
        principles,
        collaboration,
        tags,
      },
    },
    block: null,
  };
}

function toneWord(form: SoulForm) {
  if (form.tone === null) return null;
  return form.tone === "custom" ? form.customTone.trim() || null : form.tone;
}

export function composeSoulFile(form: SoulForm) {
  const tone = toneWord(form);
  return serializeAgentSoulSource({
    body: form.body,
    frontmatter: {
      version: form.kept.version,
      role: form.kept.role,
      tone: tone ? [tone] : [],
      principles: [...form.kept.principles],
      constraints: [...form.never],
      collaboration: [...form.kept.collaboration],
      tags: [...form.kept.tags],
    },
  });
}

/** A missing file opens the page's blank form; a broken one has only the raw view. */
export function soulFromPayload(payload: AgentSoulPayload) {
  const digest = payload.digest ?? "";
  if (!payload.present) {
    return { digest, source: "", form: BLANK_SOUL, block: null, missing: true };
  }
  const source = serializeAgentSoulSource(payload);
  if (!payload.valid) {
    return { digest, source, form: BLANK_SOUL, block: "mistake" as const, missing: false };
  }
  return { digest, source, ...parseSoulFile(source), missing: false };
}

export function countSoulChanges(draft: SoulForm, saved: SoulForm) {
  return [
    draft.body !== saved.body,
    draft.tone !== saved.tone || draft.customTone !== saved.customTone,
    draft.never.join("\n") !== saved.never.join("\n"),
  ].filter(Boolean).length;
}
