import { ConfirmDialog } from "@compozy/ui";

import { definePrototype, setParam, type PrototypeProps } from "@compozy/proto";

import { LISTED_EPHEMERAL, LISTED_PERSISTENT } from "../persistent-agents-shared/agents-fixtures";
import { AgentsPage } from "../persistent-agents-shared/agents-page";
import { AgentWizard, WizardAlert, type WizardProps } from "../persistent-agents-shared/wizard";

export const meta = definePrototype({
  title: "Agent create wizard",
  spec: "always-on-agent",
  surface: "S3",
  route: "/agents",
  states: [
    { id: "kind", label: "Kind" },
    { id: "ephemeral-identity", label: "Ephemeral · Identity with instructions" },
    { id: "ephemeral-advanced", label: "Ephemeral · Advanced open" },
    { id: "persistent-identity", label: "Persistent · Identity" },
    { id: "persistent-runtime", label: "Persistent · Runtime" },
    { id: "persistent-persona", label: "Persistent · Persona, plain until S6" },
    { id: "persistent-where", label: "Persistent · Where it runs" },
    { id: "persistent-global", label: "Persistent · Global" },
    { id: "persistent-not-now", label: "Persistent · Not now" },
    { id: "slug-conflict", label: "Slug conflict" },
    { id: "no-valid-slug", label: "No valid slug" },
    { id: "discard", label: "Unsaved discard" },
    { id: "back-navigation", label: "Back keeps values" },
    { id: "instance-failure", label: "Instance failed after create" },
    { id: "cap-reached", label: "Cap reached" },
    { id: "provider-auth", label: "Provider not authenticated" },
    { id: "duplicate", label: "Duplicate chooses kind" },
  ],
});

const INSTRUCTIONS = "You review pull requests and report what would break.";
const EPHEMERAL_IDENTITY = {
  name: "Reviewer",
  description: "Reads a diff and reports what would break",
  slug: "reviewer",
  instructions: INSTRUCTIONS,
};

const WIZARD: Record<string, WizardProps> = {
  kind: { kind: null, step: "kind", primaryDisabled: true },
  "ephemeral-identity": {
    kind: "ephemeral",
    step: "identity",
    identity: EPHEMERAL_IDENTITY,
    back: "kind",
    primary: "Create agent",
  },
  "ephemeral-advanced": {
    kind: "ephemeral",
    step: "identity",
    identity: { ...EPHEMERAL_IDENTITY, showSlug: true },
    advanced: true,
    back: "ephemeral-identity",
    primary: "Create agent",
  },
  "persistent-identity": {
    kind: "persistent",
    step: "identity",
    back: "kind",
    next: "persistent-runtime",
  },
  "persistent-runtime": {
    kind: "persistent",
    step: "runtime",
    back: "persistent-identity",
    next: "persistent-persona",
  },
  "persistent-persona": {
    kind: "persistent",
    step: "persona",
    back: "persistent-runtime",
    next: "persistent-where",
  },
  "persistent-where": {
    kind: "persistent",
    step: "where",
    where: "compozy",
    back: "persistent-persona",
    primary: "Create and start",
    hint: "Runs while CompozyOS is running on this computer.",
  },
  "persistent-global": {
    kind: "persistent",
    step: "where",
    where: "global",
    back: "persistent-persona",
    primary: "Create and start",
    hint: "Runs while CompozyOS is running on this computer.",
  },
  "persistent-not-now": {
    kind: "persistent",
    step: "where",
    where: "not-now",
    back: "persistent-persona",
    primary: "Create agent",
    hint: "Nothing runs until you add it to a workspace.",
  },
  "slug-conflict": {
    kind: "ephemeral",
    step: "identity",
    identity: {
      ...EPHEMERAL_IDENTITY,
      showSlug: true,
      slugError: "reviewer already exists. Try reviewer-2.",
    },
    back: "kind",
    primary: "Create agent",
    primaryDisabled: true,
  },
  "no-valid-slug": {
    kind: "ephemeral",
    step: "identity",
    identity: {
      name: "🦊",
      description: "",
      slug: "",
      instructions: INSTRUCTIONS,
      showSlug: true,
      slugError: "This name has no letters or numbers. Enter a slug.",
    },
    back: "kind",
    primary: "Create agent",
    primaryDisabled: true,
  },
  discard: { kind: "persistent", step: "identity", back: "kind", next: "persistent-runtime" },
  "back-navigation": {
    kind: "persistent",
    step: "identity",
    back: "kind",
    next: "persistent-runtime",
  },
  "instance-failure": {
    kind: "persistent",
    step: "where",
    where: "compozy",
    alert: (
      <WizardAlert>
        Triage Lead was created, but it couldn't start in compozy. Retry from its page.
      </WizardAlert>
    ),
    primary: "Open agent",
    cancel: "Close",
  },
  "cap-reached": {
    kind: "persistent",
    step: "where",
    where: "compozy",
    alert: (
      <WizardAlert tone="warning">
        You have 10 active persistent agents. Triage Lead was created. Pause one to start it, or
        raise agents.persistent.max_active_instances.
      </WizardAlert>
    ),
    primary: "Open agent",
    cancel: "Close",
  },
  "provider-auth": {
    kind: "persistent",
    step: "where",
    where: "not-now",
    alert: (
      <WizardAlert tone="warning">
        Claude isn't signed in, so Triage Lead can't start yet. Choose Not now to create it.
      </WizardAlert>
    ),
    back: "persistent-persona",
    primary: "Create agent",
  },
  duplicate: {
    kind: null,
    step: "kind",
    duplicateOf: "Triage Lead",
    primary: "Continue",
  },
};

export default function AgentCreateWizardDraft({ state }: PrototypeProps) {
  const props = WIZARD[state] ?? WIZARD.kind!;
  return (
    <AgentsPage
      ephemeral={LISTED_EPHEMERAL}
      overlay={
        <>
          <AgentWizard key={state} {...props} />
          {state === "discard" ? (
            <ConfirmDialog
              cancelLabel="Keep editing"
              confirmLabel="Discard"
              description="What you entered in this agent won't be saved."
              onConfirm={() => setParam("state", "kind")}
              onOpenChange={() => setParam("state", "persistent-identity")}
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
