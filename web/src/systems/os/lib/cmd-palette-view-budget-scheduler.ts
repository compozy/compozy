export interface CmdPaletteViewBudgetSchedule {
  readonly onHardBudget: () => void;
  readonly onSoftBudget: () => void;
}

export interface CmdPaletteViewBudgetScheduler {
  cancel: () => void;
  schedule: (schedule: CmdPaletteViewBudgetSchedule) => void;
}

export function createCmdPaletteViewBudgetScheduler(
  softBudgetMs: number,
  hardBudgetMs: number
): CmdPaletteViewBudgetScheduler {
  let hardTimer: ReturnType<typeof setTimeout> | null = null;
  let softTimer: ReturnType<typeof setTimeout> | null = null;

  const cancel = () => {
    if (softTimer !== null) clearTimeout(softTimer);
    if (hardTimer !== null) clearTimeout(hardTimer);
    softTimer = null;
    hardTimer = null;
  };

  return {
    cancel,
    schedule: schedule => {
      cancel();
      softTimer = setTimeout(schedule.onSoftBudget, softBudgetMs);
      hardTimer = setTimeout(schedule.onHardBudget, hardBudgetMs);
    },
  };
}
