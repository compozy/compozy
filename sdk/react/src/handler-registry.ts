import type { ViewRuntime } from "./runtime-context.js";
import { runWithViewRuntime } from "./runtime-context.js";
import type { HostNode } from "./host-types.js";

type ViewHandler = (...args: unknown[]) => unknown | Promise<unknown>;

const HANDLER_RETENTION_FRAMES = 2;

interface HandlerEntry {
  handler: ViewHandler;
  node: HostNode;
  property: string;
  eventCount: number;
  lastSeenFrame: number;
}

export interface HandlerDiagnostics {
  warn: (message: string) => void;
}

export interface HandlerRegistryOptions {
  diagnostics?: HandlerDiagnostics;
  now?: () => number;
  starvationBudgetMS?: number;
}

export class HandlerRegistry {
  private readonly handlers = new Map<string, HandlerEntry>();
  private readonly diagnostics: HandlerDiagnostics;
  private readonly now: () => number;
  private readonly starvationBudgetMS: number;
  private readonly active = new Set<string>();
  private frame = 0;
  private nextID = 0;

  public constructor(options: HandlerRegistryOptions = {}) {
    this.diagnostics = options.diagnostics ?? { warn: message => console.warn(message) };
    this.now = options.now ?? performance.now.bind(performance);
    this.starvationBudgetMS = options.starvationBudgetMS ?? 50;
  }

  public beginFrame(): void {
    this.frame++;
    this.active.clear();
  }

  public bind(node: HostNode, property: string, value: unknown): string | undefined {
    if (typeof value !== "function") {
      return undefined;
    }
    let id = node.handlerIDs.get(property);
    if (!id) {
      this.nextID++;
      id = `handler_${this.nextID}`;
      node.handlerIDs.set(property, id);
    }
    const eventCount = this.handlers.get(id)?.eventCount ?? 0;
    this.handlers.set(id, {
      handler: value as ViewHandler,
      node,
      property,
      eventCount,
      lastSeenFrame: this.frame,
    });
    this.active.add(id);
    return id;
  }

  public activeIDs(): string[] {
    return [...this.active];
  }

  public property(id: string): string | undefined {
    return this.handlers.get(id)?.property;
  }

  public recordEventCount(id: string, eventCount: number): void {
    const entry = this.handlers.get(id);
    if (entry) entry.eventCount = eventCount;
  }

  public eventCount(node: HostNode, properties: readonly string[]): number {
    let latest = 0;
    for (const property of properties) {
      const id = node.handlerIDs.get(property);
      const entry = id ? this.handlers.get(id) : undefined;
      if (entry?.node === node) latest = Math.max(latest, entry.eventCount);
    }
    return latest;
  }

  public endFrame(): void {
    for (const [id, entry] of this.handlers) {
      if (this.frame - entry.lastSeenFrame > HANDLER_RETENTION_FRAMES) {
        this.handlers.delete(id);
      }
    }
  }

  public async invoke(id: string, args: unknown[], runtime: ViewRuntime): Promise<void> {
    const entry = this.handlers.get(id);
    if (!entry) {
      this.diagnostics.warn(`unknown view handler ignored: ${id}`);
      return;
    }
    const startedAt = this.now();
    await runWithViewRuntime(runtime, () => entry.handler(...args));
    const elapsed = this.now() - startedAt;
    if (elapsed > this.starvationBudgetMS) {
      this.diagnostics.warn(
        `view handler ${id} blocked the event loop for ${Math.round(elapsed)}ms`
      );
    }
  }
}
