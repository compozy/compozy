import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  GENERAL,
  ORCHESTRATOR,
  RELEASE,
  TRIAGE,
} from "../stays-on-agents-shared/fixtures";
import type { PermissionRule } from "./levels";
import { PermissionsEditor } from "./permissions-editor";
import { SettingsPermissions } from "./settings-view";

export const meta = definePrototype({
  title: "Stays-on permissions",
  spec: "stays-on-hardening",
  surface: "S5 S6 S14 S15",
  route: "/agents/triage/permissions",
  states: [
    { id: "default", label: "S5 · Nothing set" },
    { id: "ask-changes", label: "S5 · Ask before changes" },
    { id: "reviewer", label: "S5 · Reviewer, Claude" },
    { id: "reviewer-none", label: "S5 · Provider has no reviewer" },
    { id: "reviewer-degraded", label: "S5 · Reviewer fell back" },
    { id: "never", label: "S5 · Never ask" },
    { id: "never-codex", label: "S5 · Never ask, Codex" },
    { id: "wide-reach", label: "S6 · Any session it can reach" },
    { id: "exceptions", label: "S14 · Exceptions" },
    { id: "exception-invalid", label: "S14 · Path outside the project" },
    { id: "unsaved", label: "Unsaved changes" },
    { id: "on-demand", label: "On-demand agent" },
    { id: "extension", label: "From an extension" },
    { id: "settings", label: "S15 · Settings, five levels" },
  ],
});

const RULES: readonly PermissionRule[] = [
  { id: "docs", kind: "edit", path: "docs/**", decision: "allow" },
  { id: "infra", kind: "edit", path: "infra/**", decision: "deny" },
  { id: "migrations", kind: "delete", path: "db/migrations/**", decision: "deny" },
];

export default function StaysOnPermissionsDraft({ state }: PrototypeProps) {
  switch (state) {
    case "settings":
      return <SettingsPermissions initial="approve-all" />;
    case "ask-changes":
      return <PermissionsEditor agent={TRIAGE} key={state} level="approve-reads" />;
    case "reviewer":
      return <PermissionsEditor agent={TRIAGE} key={state} level="auto" />;
    case "reviewer-none":
      return (
        <PermissionsEditor
          agent={{ ...TRIAGE, provider: "cursor" }}
          key={state}
          level="auto"
          noReviewer
        />
      );
    case "reviewer-degraded":
      return <PermissionsEditor agent={TRIAGE} degraded key={state} level="auto" />;
    case "never":
      return <PermissionsEditor agent={TRIAGE} key={state} level="approve-all" />;
    case "never-codex":
      return <PermissionsEditor agent={RELEASE} key={state} level="approve-all" />;
    case "wide-reach":
      return <PermissionsEditor agent={ORCHESTRATOR} key={state} reach="workspace" />;
    case "exceptions":
      return <PermissionsEditor agent={TRIAGE} key={state} rules={RULES} />;
    case "exception-invalid":
      return (
        <PermissionsEditor
          agent={TRIAGE}
          invalidRuleId="outside"
          key={state}
          rules={[
            ...RULES,
            { id: "outside", kind: "edit", path: "../secrets", decision: "allow" },
          ]}
          unsaved
        />
      );
    case "unsaved":
      return <PermissionsEditor agent={TRIAGE} key={state} level="approve-edits" unsaved />;
    case "on-demand":
      return <PermissionsEditor agent={GENERAL} followsSettings key={state} />;
    case "extension":
      return <PermissionsEditor agent={ORCHESTRATOR} key={state} readOnly />;
    default:
      return <PermissionsEditor agent={TRIAGE} key={state} unset />;
  }
}
