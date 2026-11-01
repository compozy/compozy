import { ConfirmDialog } from "@compozy/ui";

import { definePrototype, protoHref, type PrototypeProps } from "@compozy/proto";

import { goToState } from "../persistent-agents-shared/go-to-state";
import { LISTED_EPHEMERAL, LISTED_PERSISTENT } from "../persistent-agents-shared/agents-fixtures";
import { AgentsPage } from "../persistent-agents-shared/agents-page";
import { ORCHESTRATOR, RELEASE, TRIAGE } from "../persistent-agents-shared/fixtures";

import { CreateScreen, type CreateScreenProps } from "./create-screen";
import { PLACE_EVERYWHERE, PLACE_LATER, TEMPLATES, type Draft } from "./model";

export const meta = definePrototype({
  title: "Agent create",
  spec: "always-on-agent",
  surface: "S3",
  route: "/agents",
  states: [
    { id: "empty", label: "Empty" },
    { id: "filled", label: "Filled · Runs when you ask" },
    { id: "stays-on", label: "Filled · Stays on" },
    { id: "where-search", label: "Where it works · Search 50 projects" },
    { id: "everywhere", label: "Where it works · Everywhere" },
    { id: "decide-later", label: "Where it works · Decide later" },
    { id: "from-project", label: "From a project's + · Project marked" },
    { id: "avatar-character", label: "Avatar · Character" },
    { id: "avatar-symbol", label: "Avatar · Icon or emoji" },
    { id: "advanced", label: "Advanced open" },
    { id: "name-taken", label: "Name taken" },
    { id: "name-no-letters", label: "Name has no letters" },
    { id: "limit-reached", label: "Limit reached" },
    { id: "not-signed-in", label: "Not signed in to a model" },
    { id: "failed-to-start", label: "Failed to start" },
    { id: "extension-owned", label: "Extension-owned, read only" },
    { id: "duplicate", label: "Duplicate" },
    { id: "discard", label: "Unsaved discard" },
  ],
});

const INSTRUCTIONS = TEMPLATES[0]?.body ?? "";

const FILLED: Partial<Draft> = {
  name: "Reviewer Two",
  description: "Reads a change and says what would break",
  instructions: TEMPLATES[1]?.body ?? "",
  appearance: { avatar: "generated", seed: "reviewer" },
};

const TRIAGE_DRAFT: Partial<Draft> = {
  name: TRIAGE.label,
  description: TRIAGE.description,
  instructions: INSTRUCTIONS,
  appearance: TRIAGE.appearance,
  mode: "stays-on",
  place: "compozy",
};

const FAILED: CreateScreenProps = {
  initial: TRIAGE_DRAFT,
  title: TRIAGE.label,
  notice: {
    tone: "danger",
    title: "Triage Lead was created, but it couldn't start in compozy",
    body: "Nothing was lost. You can try again from its page.",
  },
  previewState: "failed",
  previewLabel: "Couldn't start",
  readOnly: {
    primary: "Open agent",
    onPrimary: () => window.location.assign(protoHref("agent-profile", "persistent")),
  },
};

const OWNED: CreateScreenProps = {
  initial: {
    name: ORCHESTRATOR.label,
    description: ORCHESTRATOR.description,
    instructions:
      "Break each task into small pieces.\n\n- Give each piece to one worker\n- Check the result before moving on\n- Tell me when a decision is needed",
    appearance: ORCHESTRATOR.appearance,
    mode: "stays-on",
    place: "compozy",
  },
  title: ORCHESTRATOR.label,
  notice: {
    tone: "info",
    title: "This agent comes from the spec-cycle extension",
    body: "Updates to the extension replace it, so it can't be changed here. Duplicate it to make your own copy.",
  },
  readOnly: { owner: "spec-cycle", primary: "Duplicate", onPrimary: () => goToState("duplicate") },
};

const SCREENS: Record<string, CreateScreenProps> = {
  empty: {},
  filled: { initial: FILLED },
  "stays-on": { initial: TRIAGE_DRAFT },
  "where-search": { initial: TRIAGE_DRAFT, placesOpen: true },
  everywhere: {
    initial: {
      ...TRIAGE_DRAFT,
      name: RELEASE.label,
      description: RELEASE.description,
      appearance: RELEASE.appearance,
      place: PLACE_EVERYWHERE,
    },
  },
  "decide-later": { initial: { ...TRIAGE_DRAFT, place: PLACE_LATER } },
  "from-project": { initial: { mode: "stays-on", place: "compozy" } },
  "avatar-character": { initial: FILLED, avatarOpen: "generated" },
  "avatar-symbol": {
    initial: { ...FILLED, appearance: RELEASE.appearance },
    avatarOpen: "symbol",
  },
  advanced: { initial: { ...FILLED, shortName: "reviewer-two" }, advancedOpen: true },
  "name-taken": { initial: { ...FILLED, name: "Reviewer" } },
  "name-no-letters": { initial: { ...FILLED, name: "🦊" } },
  "limit-reached": { initial: TRIAGE_DRAFT, atLimit: true },
  "not-signed-in": { initial: TRIAGE_DRAFT, signedOut: "Claude" },
  "failed-to-start": FAILED,
  "extension-owned": OWNED,
  duplicate: {
    initial: { ...OWNED.initial, name: "Orchestrator copy" },
    title: "Duplicate Orchestrator",
  },
  discard: { initial: FILLED },
};

export default function AgentCreateDraft({ state }: PrototypeProps) {
  const props = SCREENS[state] ?? SCREENS.empty!;
  return (
    <AgentsPage
      ephemeral={LISTED_EPHEMERAL}
      overlay={
        <>
          <CreateScreen key={state} {...props} />
          {state === "discard" ? (
            <ConfirmDialog
              cancelLabel="Keep editing"
              confirmLabel="Discard"
              description="What you entered won't be saved."
              onConfirm={() => goToState("empty")}
              onOpenChange={() => goToState("filled")}
              open
              title="Discard this agent?"
              tone="danger"
            />
          ) : null}
        </>
      }
      persistent={LISTED_PERSISTENT}
    />
  );
}
