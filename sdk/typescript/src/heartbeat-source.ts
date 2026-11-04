import type { JSONValue, HeartbeatSourceKind } from "./types.js";
export interface HeartbeatSourceRequest {
  version: number;
  source_id: string;
  kind: string;
  params: JSONValue;
  state: JSONValue;
  limits: { max_items: number; max_summary_bytes: number; max_state_bytes: number };
  workspace: { id: string; root_dir: string };
  now: string;
  dry_run: boolean;
}
export interface HeartbeatSourceItem {
  key: string;
  title: string;
  summary?: string;
  url?: string;
  attributes?: JSONValue;
}
export interface HeartbeatSourceResult {
  items: HeartbeatSourceItem[];
  state?: JSONValue;
  more?: boolean;
}
export type HeartbeatSourceHandler = (
  request: HeartbeatSourceRequest
) => HeartbeatSourceResult | Promise<HeartbeatSourceResult>;
export type { HeartbeatSourceKind };

import { InvalidParamsError } from "./errors.js";
import type { ExtensionDefinition } from "./types.js";
export class ExtensionHeartbeatSources {
  private readonly handlers = new Map<
    string,
    { descriptor: HeartbeatSourceKind; handler: HeartbeatSourceHandler }
  >();
  public hasHandlers(): boolean {
    return this.handlers.size > 0;
  }
  public register(
    definition: ExtensionDefinition,
    kind: string,
    options: Omit<HeartbeatSourceKind, "kind">,
    handler: HeartbeatSourceHandler
  ): void {
    if (!/^[a-zA-Z0-9_-]+$/.test(kind) || this.handlers.has(kind))
      throw new Error("invalid or duplicate heartbeat source kind");
    this.handlers.set(kind, { descriptor: structuredClone({ ...options, kind }), handler });
    const caps = definition.capabilities ?? {};
    caps.provides = [...new Set([...(caps.provides ?? []), "heartbeat.source"])].sort();
    definition.capabilities = caps;
  }
  public kinds(): HeartbeatSourceKind[] {
    return [...this.handlers.values()]
      .map(v => structuredClone(v.descriptor))
      .sort((a, b) => a.kind.localeCompare(b.kind));
  }
  public async poll(params: unknown): Promise<HeartbeatSourceResult> {
    if (typeof params !== "object" || params === null)
      throw new InvalidParamsError("heartbeat/source_poll params must be an object");
    const req = params as HeartbeatSourceRequest;
    const registered = this.handlers.get(req.kind);
    if (!registered) throw new InvalidParamsError("heartbeat source kind is not registered");
    return await registered.handler(req);
  }
}
