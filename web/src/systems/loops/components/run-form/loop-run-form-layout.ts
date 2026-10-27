// The run form's static pieces, shared with its skeleton so the page's column, its gaps and the
// note under the actions are the same either way. In a module of their own: a component file that
// also exports constants cannot be hot-reloaded without losing component state.

const DRY_RUN_SENTENCE = "Dry run checks your inputs without starting anything.";

/** The idle note under the form. */
export const IDLE_ACTION_NOTE = { kind: "idle", text: DRY_RUN_SENTENCE } as const;

/** The page's column and the stack of sections inside it. */
export const RUN_FORM_PAGE_CLASS = "flex max-w-2xl min-w-0 flex-col gap-6";
export const RUN_FORM_STACK_CLASS = "flex min-w-0 flex-col gap-6";
