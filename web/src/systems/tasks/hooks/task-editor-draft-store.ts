import { createStoreLogic } from "@xstate/store";
import type { SetStateAction } from "react";

import type { TaskEditorDraft } from "../lib/task-editor";

export interface TaskEditorDraftStoreInput {
  draft: TaskEditorDraft;
  scopeKey: string;
}

type TaskEditorDraftEvents = {
  draftChanged: { update: SetStateAction<TaskEditorDraft> };
};

export const taskEditorDraftLogic = createStoreLogic<
  TaskEditorDraftStoreInput,
  TaskEditorDraftEvents,
  never,
  TaskEditorDraftStoreInput
>({
  context: input => input,
  on: {
    draftChanged: (context, event) => ({
      ...context,
      draft: typeof event.update === "function" ? event.update(context.draft) : event.update,
    }),
  },
});
