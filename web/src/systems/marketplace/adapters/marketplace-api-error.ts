import { defaultApiErrorMessage } from "@/lib/api-client";
import {
  extensionOperationErrorMetadata,
  type ExtensionOperationErrorMetadata,
} from "@/systems/extensions/api";

export class MarketplaceApiError extends Error {
  public readonly installedOrigin: ExtensionOperationErrorMetadata["installedOrigin"];
  public readonly listedDigest: string | undefined;
  public readonly fetchedDigest: string | undefined;
  public readonly inputId: string | undefined;
  public readonly requiredInputs: readonly string[] | undefined;
  public readonly inputDefinitions: ExtensionOperationErrorMetadata["inputDefinitions"];
  constructor(
    message: string,
    public readonly status: number,
    public readonly diagnosticCode?: string,
    public readonly restart = false,
    metadata: ExtensionOperationErrorMetadata = {}
  ) {
    super(message);
    this.name = "MarketplaceApiError";
    this.listedDigest = metadata.listedDigest;
    this.installedOrigin = metadata.installedOrigin;
    this.fetchedDigest = metadata.fetchedDigest;
    this.inputId = metadata.inputId;
    this.requiredInputs = metadata.requiredInputs;
    this.inputDefinitions = metadata.inputDefinitions;
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
    diagnosticCode(error),
    error !== null && typeof error === "object" && Reflect.get(error, "restart") === true,
    extensionOperationErrorMetadata(error)
  );
}

function diagnosticCode(error: unknown): string | undefined {
  if (error === null || typeof error !== "object") return undefined;
  const directCode = Reflect.get(error, "code");
  if (typeof directCode === "string" && directCode.trim() !== "") return directCode.trim();
  const diagnostic = Reflect.get(error, "diagnostic");
  if (diagnostic === null || typeof diagnostic !== "object") return undefined;
  const code = Reflect.get(diagnostic, "code");
  return typeof code === "string" && code.trim() !== "" ? code.trim() : undefined;
}

export function isMarketplaceCursorStale(error: unknown): boolean {
  return (
    error instanceof MarketplaceApiError &&
    error.status === 409 &&
    error.diagnosticCode === "marketplace_cursor_stale" &&
    error.restart
  );
}
