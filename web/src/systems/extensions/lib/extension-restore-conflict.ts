import { ExtensionsApiError } from "../adapters/extensions-api";

export const EXTENSION_EXISTS_CODE = "extension_exists";

/**
 * The daemon refuses a restore while a non-bundled row (the operator's marketplace or local copy)
 * holds the built-in's name. The row stays listed; the operator removes that copy and tries again.
 */
export function isExtensionExistsConflict(error: unknown): boolean {
  return (
    error instanceof ExtensionsApiError &&
    error.status === 409 &&
    error.code === EXTENSION_EXISTS_CODE
  );
}
