import type { ValidateLoopResult } from "../types";

export class LoopsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "LoopsApiError";
  }
}

/** A structured 422 lint rejection that the editor can map back onto nodes. */
export class LoopValidationError extends LoopsApiError {
  constructor(
    message: string,
    public readonly result: ValidateLoopResult
  ) {
    super(message, 422);
    this.name = "LoopValidationError";
  }
}
