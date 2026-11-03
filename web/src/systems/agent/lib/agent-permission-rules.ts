import type { AgentPayload } from "../types";

/** Operator-authored exceptions (`permission_rules`). `any` is the stored empty kind. */
export type PermissionRuleDecision = "allow" | "deny";

export interface PermissionRuleDraft {
  /** Local row identity; never sent. */
  id: string;
  /** An ACP tool kind, or `any` for a rule that names only a path. */
  kind: string;
  path: string;
  decision: PermissionRuleDecision;
}

export const PERMISSION_RULE_LIMIT = 64;

export const PERMISSION_RULE_KINDS: readonly { value: string; label: string }[] = [
  { value: "edit", label: "Edit files" },
  { value: "delete", label: "Delete files" },
  { value: "read", label: "Read files" },
  { value: "execute", label: "Run commands" },
  { value: "fetch", label: "Use the web" },
  { value: "other", label: "Anything else" },
  { value: "any", label: "Any action" },
];

export function permissionRuleKindLabel(kind: string): string {
  return PERMISSION_RULE_KINDS.find(option => option.value === kind)?.label ?? kind;
}

type AgentRulePayload = NonNullable<AgentPayload["permission_rules"]>[number];

export function permissionRuleDraftsFromAgent(
  rules: AgentPayload["permission_rules"]
): PermissionRuleDraft[] {
  return (rules ?? []).map((rule, index) => ({
    id: `rule-${index}`,
    kind: rule.kind?.trim() || "any",
    path: rule.path ?? "",
    decision: rule.decision === "deny" ? "deny" : "allow",
  }));
}

export function permissionRulesPayload(rules: readonly PermissionRuleDraft[]): AgentRulePayload[] {
  return rules.map(rule => {
    const path = rule.path.trim();
    return {
      decision: rule.decision,
      ...(rule.kind !== "any" ? { kind: rule.kind } : {}),
      ...(path ? { path } : {}),
    };
  });
}

export function permissionRulesEqual(
  left: readonly PermissionRuleDraft[],
  right: readonly PermissionRuleDraft[]
): boolean {
  if (left.length !== right.length) return false;
  return left.every((rule, index) => {
    const other = right[index];
    return (
      other !== undefined &&
      rule.kind === other.kind &&
      rule.path.trim() === other.path.trim() &&
      rule.decision === other.decision
    );
  });
}

/** Whether a path stays inside the session root, mirroring the daemon's check. */
function pathInsideProject(raw: string): boolean {
  const path = raw.trim();
  if (!path) return true;
  if (path.startsWith("/") || path.includes("\\")) return false;
  return !path.split("/").includes("..");
}

/** The first rule that can't be saved, with the reason to show beside it. */
export function invalidPermissionRule(
  rules: readonly PermissionRuleDraft[]
): { id: string; message: string } | null {
  for (const rule of rules) {
    if (!pathInsideProject(rule.path)) {
      return { id: rule.id, message: "Use a path inside the project, such as docs/** or src/app." };
    }
    if (rule.kind === "any" && !rule.path.trim()) {
      return { id: rule.id, message: "Pick an action or a folder for this exception." };
    }
  }
  return null;
}
