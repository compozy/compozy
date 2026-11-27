export class AccountApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly retryAfterSeconds?: number
  ) {
    super(message);
    this.name = "AccountApiError";
  }
}
