import { createStoreLogic } from "@xstate/store";

import {
  createLoopEditorState,
  type LoopEditorEmitted,
  type LoopEditorEvents,
  type LoopEditorState,
} from "./loop-editor-store-contracts";
import { loopEditorDraftTransitions } from "./loop-editor-store-draft";
import { loopEditorLifecycleTransitions } from "./loop-editor-store-lifecycle";
import { loopEditorPositionTransitions } from "./loop-editor-store-positions";
import { loopEditorPublishTransitions } from "./loop-editor-store-publish";
import { loopEditorValidationTransitions } from "./loop-editor-store-validation";

export type {
  LoopEditorSidebarTab,
  LoopEditorState,
  LoopEditorView,
  LoopPositionAnnotation,
} from "./loop-editor-store-contracts";

export const loopEditorLogic = createStoreLogic<
  LoopEditorState,
  LoopEditorEvents,
  LoopEditorEmitted,
  undefined
>({
  context: () => createLoopEditorState(),
  on: {
    ...loopEditorDraftTransitions,
    ...loopEditorLifecycleTransitions,
    ...loopEditorPositionTransitions,
    ...loopEditorPublishTransitions,
    ...loopEditorValidationTransitions,
  },
});
