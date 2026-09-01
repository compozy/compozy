import type { TaskRunResultPage } from "../types";

export interface TaskResultPageController {
  canGoNext: boolean;
  canGoPrevious: boolean;
  copyState: "idle" | "copying" | "copied" | "error";
  errorMessage: string | null;
  isLoading: boolean;
  onCopy: () => Promise<void>;
  onNextPage: () => void;
  onOpenChange: (open: boolean) => void;
  onPreviousPage: () => void;
  onRetry: () => void;
  open: boolean;
  page: TaskRunResultPage | undefined;
  pageText: string;
}
