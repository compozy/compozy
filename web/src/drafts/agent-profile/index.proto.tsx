import { Plus } from "lucide-react";

import { Button } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { AgentAppPage, SaveActions } from "../persistent-agents-shared/agent-page";
import { ScopePicker, type ScopePickerProps } from "../persistent-agents-shared/add-pickers";
import { GENERAL, ORCHESTRATOR, TRIAGE } from "../persistent-agents-shared/fixtures";
import { InvalidBanner, NewSessionAction, ProfileHub } from "./hub";
import { IdentityEditor } from "./identity-editor";
import { WhereItRuns, type PlaceRow } from "./where-it-runs";

export const meta = definePrototype({
  title: "Agent profile",
  spec: "always-on-agent",
  surface: "S4 S5 S8",
  route: "/agents/triage",
  states: [
    { id: "persistent", label: "S4 · Persistent" },
    { id: "ephemeral", label: "S4 · Ephemeral" },
    { id: "extension", label: "S4 · Extension-owned, read-only" },
    { id: "invalid", label: "S4 · Invalid definition" },
    { id: "deep-link", label: "S4 · Old section link opens its editor" },
    { id: "identity", label: "S5 · Identity" },
    { id: "identity-avatar", label: "S5 · Avatar popover, Generated" },
    { id: "identity-avatar-color", label: "S5 · Generated, color picked" },
    { id: "identity-symbol", label: "S5 · Avatar popover, Icon or emoji" },
    { id: "identity-invalid", label: "S5 · Invalid values" },
    { id: "identity-extension", label: "S5 · Read-only extension agent" },
    { id: "where-active", label: "S8 · Scopes" },
    { id: "where-add-scope", label: "S16 · Add picks a scope" },
    { id: "where-add-none", label: "S16 · No scope left" },
    { id: "where-add-cap", label: "S16 · Cap reached" },
    { id: "where-add-auth", label: "S16 · Provider not authenticated" },
    { id: "where-empty", label: "S8 · Empty" },
    { id: "where-cap", label: "S8 · Cap reached" },
    { id: "where-manual", label: "S8 · Paused by you" },
    { id: "where-fatigue", label: "S8 · Paused after failures" },
    { id: "where-not-persistent", label: "S8 · No longer persistent" },
    { id: "where-missing", label: "S8 · Definition gone" },
  ],
});

const COMPOZY: PlaceRow = {
  scope: "compozy",
  state: "working",
  last: "12m ago",
  next: "in 18m",
  session: "Today, 4 reports",
};
const ADVOS: PlaceRow = {
  scope: "advos",
  state: "idle",
  last: "31m ago",
  next: "in 29m",
  session: "Yesterday",
};
const GLOBAL: PlaceRow = {
  scope: "@user",
  state: "idle",
  last: "2h ago",
  next: "in 1h",
  session: "Tuesday",
};

function paused(reason: PlaceRow["pauseReason"]): PlaceRow {
  return { ...COMPOZY, state: "paused", pauseReason: reason, next: undefined, last: "2h ago" };
}

const WHERE: Record<string, { places: readonly PlaceRow[]; capped?: boolean }> = {
  "where-active": { places: [COMPOZY, ADVOS, GLOBAL] },
  "where-add-scope": { places: [COMPOZY, ADVOS] },
  "where-add-none": { places: [COMPOZY, ADVOS, GLOBAL] },
  "where-add-cap": { places: [COMPOZY, ADVOS] },
  "where-add-auth": { places: [COMPOZY, ADVOS] },
  "where-empty": { places: [] },
  "where-cap": { places: [paused("manual"), ADVOS], capped: true },
  "where-manual": { places: [paused("manual"), ADVOS] },
  "where-fatigue": { places: [paused("fatigue"), ADVOS] },
  "where-not-persistent": { places: [paused("definition_not_persistent")] },
  "where-missing": { places: [paused("definition_missing")] },
};

const ADD_MODE: Record<string, ScopePickerProps["mode"]> = {
  "where-add-scope": "scopes",
  "where-add-none": "none",
  "where-add-cap": "cap",
  "where-add-auth": "auth",
};

function AddAction({ state }: { state: string }) {
  const mode = ADD_MODE[state];
  return (
    <ScopePicker
      defaultOpen={mode !== undefined}
      mode={mode ?? "scopes"}
      provider="Claude"
      scopes={mode === "none" ? [] : ["reloaddev", "@user"]}
      trigger={
        <Button size="sm" type="button" variant="secondary">
          <Plus aria-hidden="true" />
          Add to a workspace
        </Button>
      }
    />
  );
}

export default function AgentProfileDraft({ state }: PrototypeProps) {
  const where = WHERE[state];
  if (where) {
    return (
      <AgentAppPage actions={<AddAction state={state} />} agent={TRIAGE} leaf="Where it runs">
        <WhereItRuns agent={TRIAGE} {...where} />
      </AgentAppPage>
    );
  }
  switch (state) {
    case "ephemeral":
      return (
        <AgentAppPage actions={<NewSessionAction />} agent={GENERAL}>
          <ProfileHub agent={GENERAL} />
        </AgentAppPage>
      );
    case "extension":
      return (
        <AgentAppPage actions={<NewSessionAction />} agent={ORCHESTRATOR}>
          <ProfileHub agent={ORCHESTRATOR} readOnly />
        </AgentAppPage>
      );
    case "invalid":
      return (
        <AgentAppPage actions={<NewSessionAction />} agent={TRIAGE}>
          <ProfileHub agent={TRIAGE} invalid={<InvalidBanner />} />
        </AgentAppPage>
      );
    case "deep-link":
    case "identity":
      return (
        <AgentAppPage actions={<SaveActions unsaved={0} />} agent={TRIAGE} leaf="Identity">
          <IdentityEditor agent={TRIAGE} />
        </AgentAppPage>
      );
    case "identity-avatar":
      return (
        <AgentAppPage actions={<SaveActions unsaved={1} />} agent={TRIAGE} leaf="Identity">
          <IdentityEditor agent={TRIAGE} popover="generated" />
        </AgentAppPage>
      );
    case "identity-avatar-color":
      return (
        <AgentAppPage actions={<SaveActions unsaved={1} />} agent={TRIAGE} leaf="Identity">
          <IdentityEditor agent={TRIAGE} popover="generated" popoverColor="#4cb782" />
        </AgentAppPage>
      );
    case "identity-symbol":
      return (
        <AgentAppPage actions={<SaveActions unsaved={1} />} agent={TRIAGE} leaf="Identity">
          <IdentityEditor agent={TRIAGE} popover="symbol" />
        </AgentAppPage>
      );
    case "identity-invalid":
      return (
        <AgentAppPage actions={<SaveActions unsaved={2} />} agent={TRIAGE} leaf="Identity">
          <IdentityEditor agent={TRIAGE} invalid />
        </AgentAppPage>
      );
    case "identity-extension":
      return (
        <AgentAppPage agent={ORCHESTRATOR} hubState="extension" leaf="Identity">
          <IdentityEditor agent={ORCHESTRATOR} readOnly />
        </AgentAppPage>
      );
    default:
      return (
        <AgentAppPage actions={<NewSessionAction />} agent={TRIAGE}>
          <ProfileHub agent={TRIAGE} />
        </AgentAppPage>
      );
  }
}
