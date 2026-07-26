import { defaultApiErrorMessage } from "@/lib/api-client";

export class MarketplaceApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "MarketplaceApiError";
  }
}

export function marketplaceApiError(
  fallback: string,
  response: Response,
  error: unknown
): MarketplaceApiError {
  return new MarketplaceApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status
  );
}
