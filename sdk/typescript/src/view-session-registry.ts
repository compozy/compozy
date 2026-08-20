import { InvalidParamsError } from "./errors.js";
import type { ViewOpenRequest } from "./types.js";

interface ViewSessionEntry {
  request: ViewOpenRequest;
  generation: number;
}

export class ViewSessionRegistry {
  private readonly sessions = new Map<string, ViewSessionEntry>();

  public open(request: ViewOpenRequest): void {
    if (this.sessions.has(request.view_session)) {
      throw new InvalidParamsError(`view session is already open: ${request.view_session}`);
    }
    this.sessions.set(request.view_session, { request, generation: 0 });
  }

  public require(viewSession: string): ViewOpenRequest {
    const entry = this.sessions.get(viewSession);
    if (!entry) {
      throw new InvalidParamsError(`view session is not open: ${viewSession}`);
    }
    return entry.request;
  }

  public admitGeneration(viewSession: string, generation: number): void {
    const entry = this.sessions.get(viewSession);
    if (!entry) {
      throw new InvalidParamsError(`view session is not open: ${viewSession}`);
    }
    if (!Number.isSafeInteger(generation) || generation <= entry.generation) {
      throw new InvalidParamsError(`view generation must increase for ${viewSession}`);
    }
    entry.generation = generation;
  }

  public close(viewSession: string): boolean {
    return this.sessions.delete(viewSession);
  }

  public clear(): void {
    this.sessions.clear();
  }

  public get size(): number {
    return this.sessions.size;
  }
}
