import { useState } from "react";

import type { LoopRunConfirmVerb } from "@/systems/loops";

interface LoopRunDetailDialogsContext {
  resetRunControlErrors: () => void;
  handleCancel: () => Promise<boolean>;
}

export interface LoopRunDetailDialogs {
  inspectOpen: boolean;
  setInspectOpen: (open: boolean) => void;

  runVerb: LoopRunConfirmVerb | null;
  openRunControl: (verb: LoopRunConfirmVerb) => void;
  closeRunControl: () => void;
  confirmRunControl: (verb: LoopRunConfirmVerb) => Promise<void>;
}

export function useLoopRunDetailDialogs(
  context: LoopRunDetailDialogsContext
): LoopRunDetailDialogs {
  const [inspectOpen, setInspectOpen] = useState(false);
  const [runVerb, setRunVerb] = useState<LoopRunConfirmVerb | null>(null);

  return {
    inspectOpen,
    setInspectOpen,
    runVerb,
    openRunControl: verb => {
      context.resetRunControlErrors();
      setRunVerb(verb);
    },
    closeRunControl: () => {
      context.resetRunControlErrors();
      setRunVerb(null);
    },
    confirmRunControl: async () => {
      const accepted = await context.handleCancel();
      if (accepted) setRunVerb(null);
    },
  };
}
