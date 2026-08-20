import type { ViewRuntime } from "./runtime-context.js";
import { runWithViewRuntime } from "./runtime-context.js";
import type { HostNode } from "./host-types.js";

type ViewHandler = (...args: unknown[]) => unknown | Promise<unknown>;

interface HandlerEntry {
  handler: ViewHandler;
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
    this.handlers.set(id, { handler: value as ViewHandler, lastSeenFrame: this.frame });
    this.active.add(id);
    return id;
  }

  public activeIDs(): string[] {
    return [...this.active];
  }

  public endFrame(): void {
    for (const [id, entry] of this.handlers) {
      if (this.frame - entry.lastSeenFrame > 2) {
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
    await runWithViewRuntime(runtime, async () => await entry.handler(...args));
    const elapsed = this.now() - startedAt;
    if (elapsed > this.starvationBudgetMS) {
      this.diagnostics.warn(
        `view handler ${id} blocked the event loop for ${Math.round(elapsed)}ms`
      );
    }
  }
}
