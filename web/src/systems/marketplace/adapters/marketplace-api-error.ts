import { defaultApiErrorMessage } from "@/lib/api-client";

export class MarketplaceApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly diagnosticCode?: string
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
    response.status,
    diagnosticCode(error)
  );
}

function diagnosticCode(error: unknown): string | undefined {
  if (error === null || typeof error !== "object") return undefined;
  const diagnostic = Reflect.get(error, "diagnostic");
  if (diagnostic === null || typeof diagnostic !== "object") return undefined;
  const code = Reflect.get(diagnostic, "code");
  return typeof code === "string" && code.trim() !== "" ? code.trim() : undefined;
}
