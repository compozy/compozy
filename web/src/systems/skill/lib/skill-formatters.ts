import type { PillTone } from "@agh/ui";

import type { SkillPayload } from "../types";

const SOURCE_ORDER: Record<string, number> = {
  bundled: 0,
  workspace: 1,
  marketplace: 2,
  user: 3,
  additional: 4,
};

const SOURCE_TONE: Record<string, PillTone> = {
  bundled: "success",
  workspace: "info",
  marketplace: "accent",
  user: "warning",
  additional: "neutral",
};

export const MARKETPLACE_CATEGORIES = [
  "all",
  "testing",
  "database",
  "deploy",
  "ai",
  "devops",
  "security",
] as const;

export type MarketplaceCategory = (typeof MARKETPLACE_CATEGORIES)[number];

export function compareSkillSource(left: string, right: string): number {
  return (SOURCE_ORDER[left] ?? 99) - (SOURCE_ORDER[right] ?? 99);
}

export function skillSourceTone(source: string): PillTone {
  return SOURCE_TONE[source] ?? "neutral";
}

export function skillSourceLabel(source: string): string {
  return source;
}

export function skillStatusTone(enabled: boolean): "success" | "neutral" {
  return enabled ? "success" : "neutral";
}

export function deriveSkillAuthor(skill: SkillPayload): string | undefined {
  const provenanceSlug = skill.provenance?.slug;
  if (provenanceSlug && provenanceSlug !== "workspace") return provenanceSlug;
  const metaAuthor = skill.metadata?.author;
  if (typeof metaAuthor === "string" && metaAuthor.trim() !== "") return metaAuthor;
  return provenanceSlug;
}

export function deriveSkillTags(skill: SkillPayload): string[] {
  const raw = skill.metadata?.tags;
  return Array.isArray(raw) ? raw.filter((tag): tag is string => typeof tag === "string") : [];
}

export function deriveSkillCapabilities(skill: SkillPayload): string[] {
  const raw = skill.metadata?.capabilities;
  return Array.isArray(raw) ? raw.filter((cap): cap is string => typeof cap === "string") : [];
}

export interface SkillRecentCall {
  label: string;
  status: "success" | "error" | "pending";
  timestamp?: string;
}

export function deriveSkillRecentCalls(skill: SkillPayload): SkillRecentCall[] {
  const raw = skill.metadata?.recent_calls;
  if (!Array.isArray(raw)) return [];
  const result: SkillRecentCall[] = [];
  for (const entry of raw) {
    if (!entry || typeof entry !== "object") continue;
    const record = entry as Record<string, unknown>;
    const label = typeof record.label === "string" ? record.label : undefined;
    if (!label) continue;
    const rawStatus = typeof record.status === "string" ? record.status : "success";
    const status: SkillRecentCall["status"] =
      rawStatus === "error" || rawStatus === "pending" ? rawStatus : "success";
    const timestamp = typeof record.timestamp === "string" ? record.timestamp : undefined;
    result.push({ label, status, timestamp });
  }
  return result;
}

export function matchesMarketplaceCategory(
  skill: SkillPayload,
  category: MarketplaceCategory
): boolean {
  if (category === "all") return true;
  const tags = deriveSkillTags(skill);
  const needle = category;
  return tags.some(tag => tag.toLowerCase() === needle);
}

export function filterSkillsByQuery(skills: SkillPayload[], query: string): SkillPayload[] {
  const normalized = query.trim().toLowerCase();
  if (normalized === "") return skills;
  return skills.filter(skill => {
    const inName = skill.name.toLowerCase().includes(normalized);
    const inDescription = (skill.description ?? "").toLowerCase().includes(normalized);
    const inTags = deriveSkillTags(skill).some(tag => tag.toLowerCase().includes(normalized));
    return inName || inDescription || inTags;
  });
}
