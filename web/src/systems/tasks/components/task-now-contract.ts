export interface TaskNowStripHandlers {
  onOpenRun: (runId: string) => void;
  onOpenTask: (taskId: string) => void;
  onApprove: () => void;
  onReject: () => void;
  onResume: () => void;
  onRecover: () => void;
  onClearBlock: (blockId: string) => void;
  onViewResult: () => void;
}

export interface TaskNowPendingState {
  approve?: boolean;
  reject?: boolean;
  resume?: boolean;
  recover?: boolean;
  clearBlock?: boolean;
}
