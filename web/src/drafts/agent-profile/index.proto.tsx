import { definePrototype, type PrototypeProps } from "@compozy/proto";
import type { ComponentProps } from "react";

import type { ScopePickerProps } from "../persistent-agents-shared/add-pickers";
import { AgentAppPage } from "../persistent-agents-shared/agent-page";
import { TRIAGE } from "../persistent-agents-shared/fixtures";
import type { IdentityEditor } from "./identity-editor";
import { PROFILES, type ProfileId, type ProfilePlace } from "./profile-fixtures";
import { ProfileHome } from "./profile-home";
import type { ProfilePanel } from "./profile-setup";
import { AddPlace, WhereItWorks } from "./where-it-works";

export const meta = definePrototype({
  title: "Agent profile",
  spec: "always-on-agent",
  surface: "S4 S5 S8",
  route: "/agents/triage",
  states: [
    { id: "persistent", label: "Stay-on, working" },
    { id: "needs-you", label: "Needs you" },
    { id: "paused", label: "Paused by you" },
    { id: "paused-failures", label: "Paused after failures" },
    { id: "cant-resume", label: "Can't resume" },
    { id: "no-longer-stay-on", label: "Can't resume, now on demand" },
    { id: "ephemeral", label: "On demand" },
    { id: "extension", label: "From an extension" },
    { id: "no-activity", label: "No activity yet" },
    { id: "invalid", label: "Can't start" },
    { id: "many-places", label: "Working in 50 projects" },
    { id: "deep-link", label: "Old link opens its panel" },
    { id: "identity", label: "Panel · Name and look" },
    { id: "identity-avatar", label: "Panel · Picture, Generated" },
    { id: "identity-avatar-color", label: "Panel · Generated, color picked" },
    { id: "identity-symbol", label: "Panel · Icon or emoji" },
    { id: "identity-invalid", label: "Panel · Too long" },
    { id: "identity-extension", label: "Panel · Read-only" },
    { id: "where-active", label: "Where it works" },
    { id: "where-many", label: "Where it works, 50 projects" },
    { id: "where-add-scope", label: "S16 · Add picks a project" },
    { id: "where-add-none", label: "S16 · Nowhere left to add" },
    { id: "where-add-cap", label: "S16 · At the limit" },
    { id: "where-add-auth", label: "S16 · Not signed in" },
    { id: "where-empty", label: "Where it works, empty" },
    { id: "where-cap", label: "Where it works, at the limit" },
  ],
});

const ACTIVE = PROFILES.persistent.places;

const WHERE: Record<string, { places: readonly ProfilePlace[]; capped?: boolean }> = {
  "where-active": { places: ACTIVE },
  "where-many": { places: PROFILES["many-places"].places },
  "where-add-scope": { places: ACTIVE },
  "where-add-none": { places: ACTIVE },
  "where-add-cap": { places: ACTIVE },
  "where-add-auth": { places: ACTIVE },
  "where-empty": { places: [] },
  "where-cap": { places: PROFILES["paused-failures"].places, capped: true },
};

const ADD_MODE: Record<string, ScopePickerProps["mode"]> = {
  "where-add-scope": "scopes",
  "where-add-none": "none",
  "where-add-cap": "cap",
  "where-add-auth": "auth",
};

type Identity = Partial<ComponentProps<typeof IdentityEditor>>;

const PANELS: Record<string, { panel: ProfilePanel; identity?: Identity; on?: ProfileId }> = {
  "deep-link": { panel: "model" },
  identity: { panel: "identity" },
  "identity-avatar": { panel: "identity", identity: { popover: "generated" } },
  "identity-avatar-color": {
    panel: "identity",
    identity: { popover: "generated", popoverColor: "#4cb782" },
  },
  "identity-symbol": { panel: "identity", identity: { popover: "symbol" } },
  "identity-invalid": { panel: "identity", identity: { invalid: true } },
  "identity-extension": { panel: "identity", on: "extension" },
};

export default function AgentProfileDraft({ state }: PrototypeProps) {
  const where = WHERE[state];
  if (where) {
    const mode = ADD_MODE[state];
    return (
      <AgentAppPage
        actions={<AddPlace mode={mode} open={mode !== undefined} places={where.places} primary />}
        agent={TRIAGE}
        leaf="Where it works"
        width="max-w-5xl"
      >
        <WhereItWorks {...where} />
      </AgentAppPage>
    );
  }
  const opened = PANELS[state];
  const id: ProfileId = opened
    ? (opened.on ?? "persistent")
    : ((state in PROFILES ? state : "persistent") as ProfileId);
  const data = PROFILES[id];
  return (
    <AgentAppPage agent={data.agent} width="max-w-6xl">
      <ProfileHome
        data={data}
        identity={opened?.identity}
        invalid={id === "invalid"}
        key={state}
        panel={opened?.panel ?? null}
      />
    </AgentAppPage>
  );
}
