import { useSelector, useStore } from "@xstate/store-react";
import { type FormEvent } from "react";

import { createClientId } from "@/lib/client-id";

import type { FanOutTaskRunsRequest, FanOutTaskRunsResponse } from "../types";
import { createTaskFanOutDialogLogic } from "./task-fan-out-dialog-store";

const taskFanOutDialogLogic = createTaskFanOutDialogLogic();

export interface UseTaskFanOutDialogParams {
  onOpenChange: (open: boolean) => void;
  onFanOut: (data: FanOutTaskRunsRequest) => Promise<FanOutTaskRunsResponse | void>;
}

/** Controlled form state for the fan-out dialog opened from the task head overflow. */
export function useTaskFanOutDialog({ onOpenChange, onFanOut }: UseTaskFanOutDialogParams) {
  const store = useStore(taskFanOutDialogLogic);

  const state = useSelector(store, snapshot => snapshot.context);

  const handleOpenChange = (nextOpen: boolean) => {
    store.trigger.openChanged({ open: nextOpen, setOpen: onOpenChange });
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    store.trigger.submitRequested({
      execute: onFanOut,
      idempotencyKey: createClientId(),
      setOpen: onOpenChange,
    });
  };

  return {
    designationsText: state.designationsText,
    formError: state.formError,
    handleOpenChange,
    handleSubmit,
    networkParticipation: state.networkParticipation,
    networkStrategies: ["named", "run"] as const,
    setDesignationsText: (value: string) => store.trigger.designationsChanged({ value }),
    setNetworkParticipation: (value: typeof state.networkParticipation) =>
      store.trigger.networkParticipationChanged({ value }),
    setWorktreePerRun: (value: boolean) => store.trigger.worktreePerRunChanged({ value }),
    worktreePerRun: state.worktreePerRun,
    result: state.result,
    /**
     * How many worktrees the current text would create — one per non-empty
     * assignment line. Derived from the same parse the payload uses, so the
     * confirmation line can never disagree with what gets sent.
     */
    designationCount: countDesignations(state.designationsText),
  };
}

function countDesignations(text: string): number {
  return text.split(/\r?\n/u).filter(line => line.trim() !== "").length;
}
