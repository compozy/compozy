/** S3: one screen for every agent. It shares the create dialog's host and footer. */
import { Bot, CircleAlert } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Dialog,
  DialogContent,
  EntityDialogFooter,
  EntityDialogHeader,
  Eyebrow,
  dialogShellClass,
} from "@compozy/ui";

import { protoHref } from "@compozy/proto";

import { goToState } from "../stays-on-agents-shared/go-to-state";
import type { AgentState } from "../stays-on-agents-shared/agent-state";

import { AgentPreview } from "./agent-preview";
import { Advanced, Identity, IdentityRead, Instructions, InstructionsRead } from "./details";
import { EMPTY_DRAFT, PLACE_LATER, nameIssue, type Draft } from "./model";
import { RunFacts, RunSettings } from "./run-settings";

export interface ScreenNotice {
  tone: "danger" | "warning" | "info";
  title: string;
  body: ReactNode;
}

export interface CreateScreenProps {
  initial?: Partial<Draft>;
  title?: string;
  notice?: ScreenNotice;
  /** Showing an agent that can't be edited here. */
  readOnly?: { owner?: string; primary: string; onPrimary: () => void };
  avatarOpen?: "generated" | "symbol";
  placesOpen?: boolean;
  advancedOpen?: boolean;
  atLimit?: boolean;
  /** Provider that isn't signed in. */
  signedOut?: string;
  previewState?: AgentState;
  previewLabel?: string;
}

function outcome(draft: Draft, blocked: boolean) {
  if (draft.mode === "on-demand") {
    return { primary: "Create agent", hint: "Start it from any session." };
  }
  if (blocked) return { primary: "Create agent", hint: "You can start it from its page later." };
  if (draft.place === PLACE_LATER) {
    return { primary: "Create agent", hint: "Nothing runs until you add it to a project." };
  }
  return { primary: "Create and start", hint: "It works while CompozyOS runs on this computer." };
}

function Notice({ notice }: { notice: ScreenNotice }) {
  return (
    <Alert variant={notice.tone}>
      <CircleAlert aria-hidden="true" className="size-4" />
      <AlertTitle>{notice.title}</AlertTitle>
      <AlertDescription>{notice.body}</AlertDescription>
    </Alert>
  );
}

function submit(_draft: Draft) {
  window.location.assign(protoHref("stays-on-sessions", "sidebar-rows"));
}

export function CreateScreen({
  initial,
  title = "New agent",
  notice,
  readOnly,
  avatarOpen,
  placesOpen,
  advancedOpen,
  atLimit = false,
  signedOut,
  previewState,
  previewLabel,
}: CreateScreenProps) {
  const [draft, setDraft] = useState<Draft>({ ...EMPTY_DRAFT, ...initial });
  const patch = (next: Partial<Draft>) => setDraft(current => ({ ...current, ...next }));

  const issue = nameIssue(draft.name, draft.shortName);
  const needsSignIn = signedOut !== undefined && draft.runtime.provider === signedOut.toLowerCase();
  const blocked = atLimit || needsSignIn;
  const { primary, hint } = outcome(draft, blocked);
  const ready = draft.name.trim() !== "" && issue === null;
  const dirty = draft.name !== "" || draft.description !== "" || draft.instructions !== "";
  const close = () => goToState(dirty && !readOnly ? "discard" : "empty");

  return (
    <Dialog onOpenChange={next => (next ? undefined : close())} open>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("xl", { fill: true })}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader icon={Bot} onClose={close} title={title} />
        <div className="grid min-h-0 grid-cols-[minmax(0,1fr)_23rem] max-[760px]:block max-[760px]:overflow-y-auto">
          <div className="flex min-h-0 flex-col gap-7 overflow-y-auto px-8 py-7">
            {notice ? <Notice notice={notice} /> : null}
            {readOnly ? (
              <>
                <IdentityRead draft={draft} state={previewState} />
                <InstructionsRead value={draft.instructions} />
              </>
            ) : (
              <>
                <Identity avatarOpen={avatarOpen} draft={draft} issue={issue} onChange={patch} />
                <Instructions
                  onChange={instructions => patch({ instructions })}
                  value={draft.instructions}
                />
                <Advanced defaultOpen={advancedOpen} draft={draft} onChange={patch} />
              </>
            )}
          </div>
          <aside className="flex min-h-0 flex-col gap-6 overflow-y-auto bg-sunken p-6">
            {readOnly ? (
              <RunFacts draft={draft} owner={readOnly.owner} />
            ) : (
              <RunSettings
                atLimit={atLimit}
                draft={draft}
                onMode={mode => patch({ mode })}
                onPlace={place => patch({ place })}
                onRuntime={runtime => patch({ runtime })}
                openPlaces={placesOpen}
                signedOut={needsSignIn ? signedOut : undefined}
              />
            )}
            <div className="mt-auto flex flex-col gap-2 pt-2">
              <Eyebrow>{readOnly ? "How it looks" : "How it will look"}</Eyebrow>
              <AgentPreview
                draft={draft}
                state={previewState}
                stateLabel={previewLabel ?? (blocked ? "Not working yet" : undefined)}
              />
            </div>
          </aside>
        </div>
        <EntityDialogFooter
          cancelLabel={readOnly ? "Close" : "Cancel"}
          hint={readOnly ? undefined : hint}
          onCancel={close}
          onPrimary={() => (readOnly ? readOnly.onPrimary() : submit(draft))}
          primaryDisabled={readOnly ? false : !ready}
          primaryLabel={readOnly?.primary ?? primary}
        />
      </DialogContent>
    </Dialog>
  );
}
