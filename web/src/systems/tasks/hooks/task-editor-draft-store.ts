import { createStoreLogic } from "@xstate/store";

import type { TaskEditorDraft } from "../lib/task-editor";

export interface TaskEditorDraftStoreInput {
  draft: TaskEditorDraft;
  scopeKey: string;
  variantKey: string;
}

interface TaskEditorDraftState extends TaskEditorDraftStoreInput {
  submissionPhase: "idle" | "submitting";
}

type TaskEditorDraftEvents = {
  draftChanged: { draft: TaskEditorDraft };
  submissionFinished: Record<never, never>;
  submissionStarted: Record<never, never>;
};

export const taskEditorDraftLogic = createStoreLogic<
  TaskEditorDraftState,
  TaskEditorDraftEvents,
  never,
  TaskEditorDraftStoreInput
>({
  context: input => ({ ...input, submissionPhase: "idle" }),
  on: {
    draftChanged: (context, event) => ({ ...context, draft: event.draft }),
    submissionStarted: context => {
      if (context.submissionPhase === "submitting") return;
      return { ...context, submissionPhase: "submitting" };
    },
    submissionFinished: context => {
      if (context.submissionPhase === "idle") return;
      return { ...context, submissionPhase: "idle" };
    },
  },
});
