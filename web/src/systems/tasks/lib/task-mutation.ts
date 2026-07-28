import { toast } from "sonner";

type SuccessMessage<Result> = string | ((result: Result) => string);

function mutationErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

/** Records that the mutateAsync caller owns the error while cache convergence still runs. */
export function acknowledgeTaskMutationSettlement(error: unknown): void {
  void error;
}

/** Toasts a mutation result and consumes failures for fire-and-forget controls. */
export async function notifyTaskMutation<Result>(
  action: () => Promise<Result>,
  success: SuccessMessage<Result>,
  failure: string
): Promise<void> {
  try {
    const result = await action();
    toast.success(typeof success === "function" ? success(result) : success);
  } catch (error) {
    toast.error(mutationErrorMessage(error, failure));
  }
}

/** Toasts and rethrows failures so forms keep their overlay open for correction. */
export async function submitTaskMutation<Result>(
  action: () => Promise<Result>,
  success: SuccessMessage<Result>,
  failure: string
): Promise<Result> {
  try {
    const result = await action();
    toast.success(typeof success === "function" ? success(result) : success);
    return result;
  } catch (error) {
    toast.error(mutationErrorMessage(error, failure));
    throw error;
  }
}
