export interface CmdPaletteViewSearchScheduler {
  cancel: () => void;
  schedule: (delayMs: number, dispatch: () => void) => void;
}

export function createCmdPaletteViewSearchScheduler(): CmdPaletteViewSearchScheduler {
  let timer: ReturnType<typeof setTimeout> | null = null;

  const cancel = () => {
    if (timer !== null) clearTimeout(timer);
    timer = null;
  };

  return {
    cancel,
    schedule: (delayMs, dispatch) => {
      cancel();
      if (delayMs === 0) {
        dispatch();
        return;
      }
      timer = setTimeout(dispatch, delayMs);
    },
  };
}
