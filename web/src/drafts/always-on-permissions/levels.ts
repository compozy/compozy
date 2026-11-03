/** One plain vocabulary for the five levels, shared by the agent editor and Settings. */
import { Eye, Lock, PencilLine, ShieldCheck, Zap } from "@compozy/ui/icons";
import type { ComponentType } from "react";

export type PermissionLevel =
  | "deny-all"
  | "approve-reads"
  | "approve-edits"
  | "auto"
  | "approve-all";

export const LEVELS: readonly {
  value: PermissionLevel;
  label: string;
  description: string;
  icon: ComponentType<{ className?: string; size?: number }>;
}[] = [
  {
    value: "deny-all",
    label: "Ask before every action",
    description: "Every action waits for your approval.",
    icon: Lock,
  },
  {
    value: "approve-reads",
    label: "Ask before changes",
    description: "Reading is automatic. Changes and commands still ask.",
    icon: Eye,
  },
  {
    value: "approve-edits",
    label: "Accept edits",
    description: "Edits files and organizes its own work. Riskier actions still ask.",
    icon: PencilLine,
  },
  {
    value: "auto",
    label: "Auto-review",
    description: "The provider's AI reviewer approves routine work and stops risky actions.",
    icon: ShieldCheck,
  },
  {
    value: "approve-all",
    label: "Full access",
    description: "Acts without asking. For agents you trust.",
    icon: Zap,
  },
];

export type RuleKind = "any" | "read" | "edit" | "delete" | "execute" | "fetch" | "other";

export const RULE_KINDS: readonly { value: RuleKind; label: string }[] = [
  { value: "edit", label: "Edit files" },
  { value: "delete", label: "Delete files" },
  { value: "read", label: "Read files" },
  { value: "execute", label: "Run commands" },
  { value: "fetch", label: "Use the web" },
  { value: "other", label: "Anything else" },
  { value: "any", label: "Any action" },
];

export interface PermissionRule {
  id: string;
  kind: RuleKind;
  path: string;
  decision: "allow" | "deny";
}
