import { useState } from "react";

import type { TaskExecutionProfile, TaskExecutionProfileSetRequest } from "../types";

interface UseProfileEditorState {
  taskId: string;
  profile: TaskExecutionProfile | null;
  onSetProfile: (data: TaskExecutionProfileSetRequest) => Promise<void>;
}

export interface TaskProfileEditor {
  open: boolean;
  setOpen: (open: boolean) => void;
  value: TaskExecutionProfileSetRequest | null;
  setValue: (value: TaskExecutionProfileSetRequest) => void;
  submit: () => Promise<void>;
}

function emptyProfile(taskId: string): TaskExecutionProfileSetRequest {
  const now = new Date().toISOString();
  return {
    task_id: taskId,
    coordinator: { mode: "inherit" },
    worker: { mode: "inherit" },
    review: {},
    sandbox: { mode: "inherit" },
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
  const [open, setOpenState] = useState(false);
  const [value, setValue] = useState<TaskExecutionProfileSetRequest | null>(null);

  const setOpen = (next: boolean) => {
    if (next) {
      setValue(profile ? { ...profile, task_id: taskId } : emptyProfile(taskId));
    }
    setOpenState(next);
  };

  const submit = async () => {
    if (!value) return;
    try {
      await onSetProfile({ ...value, task_id: taskId, updated_at: new Date().toISOString() });
      setOpen(false);
    } catch {
      // Caller reports the failure; keep the form open for correction.
    }
  };

  return { open, setOpen, value, setValue, submit };
}
