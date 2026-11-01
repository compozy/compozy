import { useState, type FormEvent } from "react";

import type { RuntimeProviderOption } from "@/systems/runtime";

import {
  agentCreateSlug,
  agentSlugFor,
  type AgentCreateDialogDraft,
  type AgentCreateValidation,
} from "../lib/agent-create-draft";

export type AgentCreateNameIssue =
  | { kind: "taken"; suggestion: string }
  | { kind: "no-letters" }
  | null;

const MAX_NAME_SUGGESTIONS = 20;

/** The slug rules, spoken about the name: the slug itself stays under Advanced. */
export function agentCreateNameIssue(
  draft: Pick<AgentCreateDialogDraft, "displayName" | "name">,
  takenNames: ReadonlySet<string>
): AgentCreateNameIssue {
  const name = draft.displayName.trim();
  if (name === "" && draft.name.trim() === "") return null;
  const slug = agentCreateSlug(draft);
  if (slug === "") return { kind: "no-letters" };
  if (!takenNames.has(slug)) return null;
  for (let count = 2; count < MAX_NAME_SUGGESTIONS; count += 1) {
    const suggestion = `${name || slug} ${count}`;
    if (!takenNames.has(agentSlugFor(suggestion))) return { kind: "taken", suggestion };
  }
  return { kind: "taken", suggestion: `${name || slug} ${Date.now()}` };
}

/** Text copied from a duplicate's source isn't entered data: only a change from it is. */
function hasEntries(draft: AgentCreateDialogDraft, seed: AgentCreateEntrySeed) {
  return (
    draft.displayName !== "" ||
    draft.description !== seed.description ||
    draft.prompt !== seed.prompt
  );
}

/** A field speaks up only once the person has touched it. */
function shownCreateErrors(
  draft: AgentCreateDialogDraft,
  fields: AgentCreateValidation["fields"],
  shownProviderError: boolean
) {
  return {
    scope: (fields as Record<string, string | undefined>).scope,
    name: agentCreateSlug(draft).length > 0 ? fields.name : undefined,
    categoryPath: draft.categoryPath.trim().length > 0 ? fields.categoryPath : undefined,
    provider: shownProviderError ? fields.provider : undefined,
    reasoningEffort: draft.reasoningEffort !== "" ? fields.reasoningEffort : undefined,
    prompt: draft.prompt.trim().length > 0 ? fields.prompt : undefined,
    tools: draft.tools.length > 0 ? fields.tools : undefined,
    toolsets: draft.toolsets.length > 0 ? fields.toolsets : undefined,
    denyTools: draft.denyTools.length > 0 ? fields.denyTools : undefined,
  } as Record<string, string | undefined>;
}

export type AgentCreateEntrySeed = Pick<AgentCreateDialogDraft, "description" | "prompt">;

const NO_ENTRY_SEED: AgentCreateEntrySeed = { description: "", prompt: "" };

interface AgentCreateViewStateArgs {
  draft: AgentCreateDialogDraft;
  /** What the draft opened with; a blank create starts from nothing. */
  entrySeed?: AgentCreateEntrySeed | null;
  providerOptions: readonly RuntimeProviderOption[];
  providersError: string | null;
  providersLoading: boolean;
  takenNames: ReadonlySet<string>;
  validation: AgentCreateValidation;
  startFailure: string | null;
  isSubmitting: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
  onOpenCreatedAgent: () => void;
}

/** What the screen derives from the draft: the name's issue, sign-in, and the errors worth showing. */
export function useAgentCreateViewState({
  draft,
  entrySeed,
  providerOptions,
  providersError,
  providersLoading,
  takenNames,
  validation,
  startFailure,
  isSubmitting,
  onOpenChange,
  onSubmit,
  onOpenCreatedAgent,
}: AgentCreateViewStateArgs) {
  const { fields } = validation;
  const [discarding, setDiscarding] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const issue = agentCreateNameIssue(draft, takenNames);
  const provider = providerOptions.find(option => option.id === draft.provider);
  const shownProviderError =
    Boolean(providersError || providersLoading) ||
    providerOptions.length === 0 ||
    draft.provider.trim().length > 0;
  const dirty = !startFailure && hasEntries(draft, entrySeed ?? NO_ENTRY_SEED);
  const dismiss = () => {
    if (isSubmitting) return;
    setAdvancedOpen(false);
    onOpenChange(false);
  };
  const close = () => (dirty ? setDiscarding(true) : dismiss());
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (isSubmitting) return;
    if (startFailure) onOpenCreatedAgent();
    else if (!validation.advancedValid) setAdvancedOpen(true);
    else if (validation.canSubmit && issue === null) onSubmit();
  };
  return {
    discarding,
    setDiscarding,
    dismiss,
    close,
    handleSubmit,
    showAdvanced: advancedOpen || issue?.kind === "no-letters" || !validation.advancedValid,
    setAdvancedOpen,
    issue,
    signedOut: provider?.needs_auth ? provider.name : null,
    errors: shownCreateErrors(draft, fields, shownProviderError),
  };
}
