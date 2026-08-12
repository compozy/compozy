import { useSelector, useStore } from "@xstate/store-react";

import { createProfileEditorLogic } from "./profile-editor-store";
import type { TaskExecutionProfile, TaskExecutionProfileSetRequest } from "../types";

const profileEditorLogic = createProfileEditorLogic();

interface UseProfileEditorState {
  taskId: string;
  profile: TaskExecutionProfile | null;
  onSetProfile: (data: TaskExecutionProfileSetRequest) => Promise<void>;
}

export interface TaskProfileEditor {
  error: string | null;
  open: boolean;
  setOpen: (open: boolean) => void;
  value: TaskExecutionProfileSetRequest | null;
  setValue: (value: TaskExecutionProfileSetRequest) => void;
  submit: () => void;
}

function emptyProfile(taskId: string): TaskExecutionProfileSetRequest {
  const now = new Date().toISOString();
  return {
    task_id: taskId,
    coordinator: { mode: "inherit" },
    worker: { mode: "inherit" },
    review: {},
    sandbox: { mode: "inherit" },
    worktree: { mode: "inherit" },
    participants: {},
    runtime: { mode: "default" },
    created_at: now,
    updated_at: now,
  };
}

export function useProfileEditor({
  taskId,
  profile,
  onSetProfile,
}: UseProfileEditorState): TaskProfileEditor {
  const store = useStore(profileEditorLogic);

  const state = useSelector(store, snapshot => snapshot.context);

  return {
    error: state.error,
    open: state.phase !== "closed",
    setOpen: open => {
      if (open) {
        store.trigger.opened({
          value: profile ? { ...profile, task_id: taskId } : emptyProfile(taskId),
        });
        return;
      }
      store.trigger.closed({});
    },
    setValue: value => store.trigger.valueChanged({ value }),
    submit: () =>
      store.trigger.submitRequested({
        execute: onSetProfile,
        taskId,
        updatedAt: new Date().toISOString(),
      }),
    value: state.value,
  };
}
