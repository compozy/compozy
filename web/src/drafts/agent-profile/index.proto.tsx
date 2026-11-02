import { definePrototype, type PrototypeProps } from "@compozy/proto";

import type { ScopePickerProps } from "../stays-on-agents-shared/add-pickers";
import { AgentAppPage } from "../stays-on-agents-shared/agent-page";
import { TRIAGE } from "../stays-on-agents-shared/fixtures";
import { PROFILES, type ProfileId, type ProfilePlace } from "./profile-fixtures";
import type { HeroEdit } from "./profile-hero";
import { ProfileHome } from "./profile-home";
import type { ProfilePanel } from "./profile-setup";
import { AddPlace, WhereItWorks } from "./where-it-works";

export const meta = definePrototype({
  title: "Agent profile",
  spec: "always-on-agent",
  surface: "S4 S5 S8",
  route: "/agents/triage",
  states: [
    { id: "stays-on", label: "Stays on, working" },
    { id: "needs-you", label: "Needs you" },
    { id: "paused", label: "Paused by you" },
    { id: "paused-failures", label: "Paused after failures" },
    { id: "cant-resume", label: "Can't resume" },
    { id: "no-longer-stays-on", label: "Can't resume, now on demand" },
    { id: "on-demand", label: "On demand" },
    { id: "extension", label: "From an extension" },
    { id: "no-activity", label: "No activity yet" },
    { id: "invalid", label: "Can't start" },
    { id: "many-places", label: "Working in 50 projects" },
    { id: "deep-link", label: "Old link opens its panel" },
    { id: "identity", label: "Header · Editing the name" },
    { id: "identity-avatar", label: "Header · Picture, Generated" },
    { id: "identity-avatar-color", label: "Header · Generated, color picked" },
    { id: "identity-symbol", label: "Header · Icon or emoji" },
    { id: "identity-invalid", label: "Header · Too long" },
    { id: "identity-extension", label: "Header · Read-only" },
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

const ACTIVE = PROFILES["stays-on"].places;

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

const PANELS: Record<string, ProfilePanel> = { "deep-link": "model" };

const EDITS: Record<string, { edit?: HeroEdit; on?: ProfileId }> = {
  identity: { edit: { editing: true } },
  "identity-avatar": { edit: { popover: "generated" } },
  "identity-avatar-color": { edit: { popover: "generated", popoverColor: "#4cb782" } },
  "identity-symbol": { edit: { popover: "symbol" } },
  "identity-invalid": { edit: { invalid: true } },
  "identity-extension": { on: "extension" },
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
  const edit = EDITS[state];
  const panel = PANELS[state] ?? null;
  const id: ProfileId = edit?.on ?? (state in PROFILES ? (state as ProfileId) : "stays-on");
  const data = PROFILES[id];
  return (
    <AgentAppPage agent={data.agent} width="max-w-6xl">
      <ProfileHome
        data={data}
        edit={edit?.edit}
        invalid={id === "invalid"}
        key={state}
        panel={panel}
      />
    </AgentAppPage>
  );
}
