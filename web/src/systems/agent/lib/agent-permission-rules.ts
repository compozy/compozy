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

/** POSIX `path.Clean` for a relative path, as the daemon applies it. */
function cleanRelativePath(path: string): string {
  const parts: string[] = [];
  for (const segment of path.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === ".." && parts.length > 0 && parts[parts.length - 1] !== "..") parts.pop();
    else parts.push(segment);
  }
  return parts.join("/") || ".";
}

/** Whether a path stays inside the session root, with the daemon's exact rule. */
function pathInsideProject(raw: string): boolean {
  const path = raw.trim();
  if (!path) return true;
  if (path.startsWith("/") || path.includes("\\")) return false;
  const clean = cleanRelativePath(path);
  return clean !== ".." && !clean.startsWith("../");
}

/** Whether `[` classes and `{` alternations close, which the daemon's glob parser requires. */
function globWellFormed(path: string): boolean {
  let inClass = false;
  let braces = 0;
  for (const char of path) {
    if (inClass) {
      if (char === "]") inClass = false;
      continue;
    }
    if (char === "[") inClass = true;
    else if (char === "{") braces += 1;
    else if (char === "}") {
      if (braces === 0) return false;
      braces -= 1;
    }
  }
  return !inClass && braces === 0;
}

const PATH_OUTSIDE_MESSAGE = "Use a path inside the project, such as docs/** or src/app.";
const GLOB_MESSAGE = "Close every [ and { in this pattern, such as docs/[ab]* or src/{app,lib}.";

/** A daemon refusal of `permission_rules[i]`, placed on the row it names. */
export function permissionRuleErrorFromDaemon(
  message: string | null | undefined,
  rules: readonly PermissionRuleDraft[]
): { id: string; message: string } | null {
  const match = message?.match(/permission_rules\[(\d+)\](?:\.(\w+))?/u);
  const rule = match ? rules[Number(match[1])] : undefined;
  if (!match || !rule) return null;
  return {
    id: rule.id,
    message:
      match[2] !== "path"
        ? "CompozyOS can't save this exception. Check its action and folder."
        : /relative/u.test(message ?? "")
          ? PATH_OUTSIDE_MESSAGE
          : GLOB_MESSAGE,
  };
}

/** The first rule that can't be saved, with the reason to show beside it. */
export function invalidPermissionRule(
  rules: readonly PermissionRuleDraft[]
): { id: string; message: string } | null {
  for (const rule of rules) {
    if (!pathInsideProject(rule.path)) return { id: rule.id, message: PATH_OUTSIDE_MESSAGE };
    if (!globWellFormed(rule.path)) return { id: rule.id, message: GLOB_MESSAGE };
    if (rule.kind === "any" && !rule.path.trim()) {
      return { id: rule.id, message: "Pick an action or a folder for this exception." };
    }
  }
  return null;
}
