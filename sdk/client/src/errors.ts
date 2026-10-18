/** The daemon returned an HTTP error. Secrets and request headers are never retained. */
export class CompozyApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;
  readonly currentTurnId: string | undefined;

  constructor(status: number, body: unknown, statusText = "") {
    const payload = body !== null && typeof body === "object" ? body : {};
    const field = (key: string): string | undefined => {
      const value: unknown = Reflect.get(payload, key);
      return typeof value === "string" && value.length > 0 ? value : undefined;
    };
    super(field("error") ?? field("message") ?? (statusText || `HTTP ${status}`));
    this.name = "CompozyApiError";
    this.status = status;
    this.code = field("code") ?? `http_${status}`;
    this.details = Reflect.get(payload, "details");
    this.currentTurnId = field("current_turn_id");
  }
}

export class CompozyUnavailableError extends Error {
  override name = "CompozyUnavailableError";
}

export class CompozyConfigError extends Error {
  override name = "CompozyConfigError";
}

export class CompozyStreamError extends Error {
  override name = "CompozyStreamError";
}

export class CompozyBinaryNotFoundError extends Error {
  override name = "CompozyBinaryNotFoundError";
}

export async function responseError(response: Response): Promise<CompozyApiError> {
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    // A proxy's non-JSON error still has a useful HTTP status.
  }
  return new CompozyApiError(response.status, body, response.statusText);
}
