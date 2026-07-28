import { useState, type ChangeEvent } from "react";

export type PauseTaskHandler = (reason: string) => void | Promise<void>;

export function useTaskPauseDialog(onPause?: PauseTaskHandler) {
  const [isOpen, setIsOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);

  const handleOpenChange = (next: boolean) => {
    setIsOpen(next);
    if (!next) {
      setReason("");
      setError(null);
    }
  };

  const handleReasonChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setReason(event.target.value);
    setError(null);
  };

  const open = () => {
    setIsOpen(true);
  };

  const close = () => {
    handleOpenChange(false);
  };

  const confirm = async () => {
    const normalizedReason = reason.trim();
    if (!normalizedReason) {
      setError("Provide a pause reason.");
      return;
    }
    if (!onPause) {
      return;
    }
    try {
      await onPause(normalizedReason);
      handleOpenChange(false);
    } catch (pauseError) {
      setError(pauseError instanceof Error ? pauseError.message : "Failed to pause task.");
    }
  };

  return {
    close,
    confirm,
    error,
    isOpen,
    onOpenChange: handleOpenChange,
    onReasonChange: handleReasonChange,
    open,
    reason,
  };
}
