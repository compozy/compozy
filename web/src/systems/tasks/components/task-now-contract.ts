export interface TaskNowStripHandlers {
  onOpenRun: (runId: string) => void;
  onOpenTask: (taskId: string) => void;
  /**
   * Approve/reject/resume/recover/clear-block hit task lifecycle routes
   * registered only on the local surface set (`routes.go`
   * `includeTaskMutations`), so these handlers — and the affordances they
   * drive — go absent on remote tiers, never disabled (BR-1). Read handlers
   * below stay on every tier.
   */
  onApprove?: () => void;
  onReject?: () => void;
  onResume?: () => void;
  onRecover?: () => void;
  onClearBlock?: (blockId: string) => void;
  onViewResult: () => void;
}

export interface TaskNowPendingState {
  approve?: boolean;
  reject?: boolean;
  resume?: boolean;
  recover?: boolean;
  clearBlock?: boolean;
}
