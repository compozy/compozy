import { useSelector } from "@xstate/store-react";
import { useMemo } from "react";

import { toast } from "@compozy/ui";

import { useStoreBinding } from "@/hooks/use-store-binding";

import { isAgentDigestConflict } from "../adapters/agent-api";
import {
  isDuplicateDraftStore,
  retainedDraftKey,
  retainedEditorDraft,
} from "../lib/retained-editor-drafts";
import {
  countEditorDocumentChanges,
  createEditorDocumentStore,
  shouldAdoptSnapshot,
  type EditorDocumentContext,
  type EditorSnapshot,
} from "../stores/agent-editor-document-store";
import { useRetainedEditorDraft } from "./use-retained-editor-draft";

export interface UseEditorDocumentArgs<TForm> {
  /** Identity of the file; a new key is a new editor. */
  resourceKey: string;
  snapshot: EditorSnapshot<TForm>;
  compose: (form: TForm) => string;
  countChanges: (a: TForm, b: TForm) => number;
  /** The problem with a hand-edited file, if any. */
  validate: (source: string) => Promise<string | undefined>;
  write: (source: string, expectedDigest: string) => Promise<EditorSnapshot<TForm>>;
  /** The file as it is now, past any cache; read after a conflict. */
  reread: () => Promise<EditorSnapshot<TForm>>;
  savedCopy: string;
}

function problemOf(error: unknown) {
  return error instanceof Error ? error.message : "Couldn't save. Try again.";
}

/**
 * The save path all three editors share: one write against the digest last read, a conflict
 * that rereads and waits for a choice, and Keep mine writing again on the newer digest.
 */
export function useEditorDocument<TForm>({
  resourceKey,
  snapshot,
  compose,
  countChanges,
  validate,
  write,
  reread,
  savedCopy,
}: UseEditorDocumentArgs<TForm>) {
  const draftKey = retainedDraftKey("document", resourceKey);
  // Edits kept from an editor of this file that went away unsaved come back as unsaved (#1653);
  // a store replaced under the same key (a reload, following the server) does not resume them.
  const resume = () =>
    createEditorDocumentStore(
      snapshot,
      retainedEditorDraft<EditorDocumentContext<TForm>>(draftKey)
    );
  const { replace, store } = useStoreBinding(
    resourceKey,
    resume,
    (previous, previousKey) =>
      previousKey === resourceKey && !isDuplicateDraftStore(previous)
        ? createEditorDocumentStore(snapshot)
        : resume(),
    (current, key) =>
      current.key !== key ||
      shouldAdoptSnapshot(current.store.getSnapshot().context, snapshot, countChanges)
  );
  const draftPolicy = useMemo(
    () => ({
      isDirty: (context: EditorDocumentContext<TForm>) =>
        countEditorDocumentChanges(context, countChanges) > 0,
      isBusy: (context: EditorDocumentContext<TForm>) => context.phase === "saving",
      idle: (context: EditorDocumentContext<TForm>) =>
        context.phase === "saving" ? { ...context, phase: "idle" as const } : context,
    }),
    [countChanges]
  );
  const { forget } = useRetainedEditorDraft(draftKey, store, draftPolicy, replace);
  const document = useSelector(store, state => state.context);
  const { baseline, form, raw, conflict, phase, problem } = document;

  const save = async () => {
    const current = store.getSnapshot().context;
    const source = current.raw ?? compose(current.form);
    const digest = current.conflict?.digest ?? current.baseline.digest;
    store.trigger.saveStarted();
    try {
      const invalid = current.raw === null ? undefined : await validate(source);
      if (invalid) {
        store.trigger.saveFailed({ problem: invalid });
        return;
      }
      store.trigger.saved({
        snapshot: await write(source, digest),
        submitted: { form: current.form, raw: current.raw },
      });
      toast.message(current.conflict ? "Kept your version" : savedCopy);
    } catch (error) {
      if (!isAgentDigestConflict(error)) {
        store.trigger.saveFailed({ problem: problemOf(error) });
        return;
      }
      try {
        store.trigger.conflicted({ theirs: await reread() });
      } catch (rereadError) {
        store.trigger.saveFailed({ problem: problemOf(rereadError) });
      }
    }
  };

  // Changes whenever the document's state does; opaque to the editor's views.
  const revision: unknown = document;
  const shownSource = raw ?? (baseline.block ? baseline.source : compose(form));

  return {
    resourceKey,
    revision,
    baseline,
    form,
    raw,
    conflict,
    saving: phase === "saving",
    problem,
    source: shownSource,
    block: baseline.block,
    changes: countEditorDocumentChanges(document, countChanges),
    setForm: (next: TForm) => store.trigger.formChanged({ form: next }),
    setRaw: (next: string) =>
      store.trigger.rawChanged({ raw: next === compose(form) ? null : next }),
    discard: () => {
      store.trigger.discarded();
      forget();
    },
    save,
    chooseTheirs: () => store.trigger.theirsChosen(),
  };
}

/** What a pure editor view needs of a document. */
export type EditorDocument<TForm> = ReturnType<typeof useEditorDocument<TForm>>;
