import { createStore } from "@xstate/store";

import type { FileBlock } from "../lib/agent-frontmatter";

/** One read of an authored file, as its editor holds it. */
export interface EditorSnapshot<TForm> {
  digest: string;
  source: string;
  form: TForm;
  /** Set when the form can't hold this file, so only the raw view opens. */
  block: FileBlock | null;
  /** The file doesn't exist yet. */
  missing?: boolean;
}

export type EditorPhase = "idle" | "saving";

export interface EditorDocumentContext<TForm> {
  baseline: EditorSnapshot<TForm>;
  form: TForm;
  /** The raw file while it's being edited by hand; it wins over the form on save. */
  raw: string | null;
  /** The newer version a save ran into, held until the person picks one. */
  conflict: EditorSnapshot<TForm> | null;
  phase: EditorPhase;
  problem: string | undefined;
}

function seed<TForm>(snapshot: EditorSnapshot<TForm>): EditorDocumentContext<TForm> {
  return {
    baseline: snapshot,
    form: snapshot.form,
    raw: null,
    conflict: null,
    phase: "idle",
    problem: undefined,
  };
}

/** Form, raw file and conflict for one authored file; reads and writes stay in the owning hook. */
export function createEditorDocumentStore<TForm>(snapshot: EditorSnapshot<TForm>) {
  return createStore({
    context: seed(snapshot),
    on: {
      formChanged: (context, event: { form: TForm }) => ({ ...context, form: event.form }),
      rawChanged: (context, event: { raw: string | null }) => ({
        ...context,
        raw: event.raw,
        problem: undefined,
      }),
      discarded: context => ({ ...seed(context.baseline), conflict: context.conflict }),
      saveStarted: context => ({ ...context, phase: "saving" as const, problem: undefined }),
      saved: (_context, event: { snapshot: EditorSnapshot<TForm> }) => seed(event.snapshot),
      saveFailed: (context, event: { problem: string }) => ({
        ...context,
        phase: "idle" as const,
        problem: event.problem,
      }),
      conflicted: (context, event: { theirs: EditorSnapshot<TForm> }) => ({
        ...context,
        phase: "idle" as const,
        conflict: event.theirs,
      }),
      theirsChosen: context => (context.conflict ? seed(context.conflict) : context),
    },
  });
}

export type EditorDocumentStore<TForm> = ReturnType<typeof createEditorDocumentStore<TForm>>;

/** A clean editor follows the server; one holding edits waits for Save to meet the conflict. */
export function shouldAdoptSnapshot<TForm>(
  context: EditorDocumentContext<TForm>,
  next: EditorSnapshot<TForm>,
  countChanges: (a: TForm, b: TForm) => number
) {
  return (
    next.digest !== context.baseline.digest &&
    context.phase === "idle" &&
    context.conflict === null &&
    context.raw === null &&
    countChanges(context.form, context.baseline.form) === 0
  );
}
