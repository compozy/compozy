import type { Effect, ViewFrame, ViewPatch } from "@compozy/extension-sdk";
import { createElement } from "react";
import type { ReactElement } from "react";
import { LegacyRoot } from "react-reconciler/constants.js";

import { HandlerRegistry } from "./handler-registry.js";
import type { HandlerRegistryOptions } from "./handler-registry.js";
import type { HostContainer } from "./host-types.js";
import { flushViewWork, runViewSync, updateViewContainer, viewReconciler } from "./reconciler.js";
import type { ViewRoot } from "./reconciler.js";
import { ViewRuntimeContext } from "./runtime-context.js";
import type { ViewRuntime } from "./runtime-context.js";
import { serializeView } from "./serializer.js";

const CONTROLLED_HANDLER_PROPERTIES = new Set([
  "onSearchTextChange",
  "onChipToggle",
  "onSelectionChange",
  "onChange",
]);

export interface ViewRendererOptions extends HandlerRegistryOptions {
  viewSession: string;
  viewID: string;
  signal: AbortSignal;
  publish: (frame: ViewFrame) => Promise<void> | void;
  scheduleFrame?: (callback: () => void) => void;
}

interface ViewStackEntry {
  readonly container: HostContainer;
  readonly root: ViewRoot;
}

export class ViewRenderer {
  private readonly handlers: HandlerRegistry;
  private readonly effects: Effect[] = [];
  private readonly stack: ViewStackEntry[] = [];
  private readonly runtime: ViewRuntime;
  private readonly sessionController = new AbortController();
  private readonly abortFromOwner: () => void;
  private readonly scheduleFrame: (callback: () => void) => void;
  private scheduled = false;
  private opened = false;
  private closed = false;
  private allowUnsolicitedPublish = true;
  private invokeSignal: AbortSignal | undefined;
  private revision = 0;
  private generation = 0;
  private previousPayloadJSON = "";

  public constructor(private readonly options: ViewRendererOptions) {
    this.handlers = new HandlerRegistry(options);
    this.scheduleFrame = options.scheduleFrame ?? (callback => setTimeout(callback, 16));
    this.abortFromOwner = () => this.sessionController.abort(options.signal.reason);
    options.signal.addEventListener("abort", this.abortFromOwner, { once: true });
    if (options.signal.aborted) this.abortFromOwner();
    this.runtime = {
      signal: this.sessionController.signal,
      navigation: {
        push: target => this.push(target),
        pop: () => this.pop(),
        popToRoot: () => this.popToRoot(),
      },
      enqueueEffect: effect => {
        if (this.closed || this.sessionController.signal.aborted) return;
        if (this.invokeSignal?.aborted) return;
        if (!this.allowUnsolicitedPublish && this.invokeSignal === undefined) return;
        this.effects.push(effect);
        this.onCommit();
      },
    };
  }

  public open(element: ReactElement): ViewFrame {
    this.stack.push(this.createStackEntry(element));
    const frame = this.flushFrame(0, 0, true);
    if (!frame) {
      throw new Error("view renderer did not produce a first frame");
    }
    this.opened = true;
    return frame;
  }

  public async event(
    handler: string,
    args: unknown[],
    inReplyTo: number,
    generation: number,
    signal: AbortSignal
  ): Promise<ViewFrame | undefined> {
    this.assertOpen();
    const eventGeneration = generation;
    this.allowUnsolicitedPublish = false;
    this.invokeSignal = signal;
    const property = this.handlers.property(handler);
    const eventCount = args.at(-1);
    if (
      property &&
      CONTROLLED_HANDLER_PROPERTIES.has(property) &&
      typeof eventCount === "number" &&
      Number.isSafeInteger(eventCount) &&
      eventCount >= 0
    ) {
      this.handlers.recordEventCount(handler, eventCount);
    }
    try {
      if (signal.aborted) return undefined;
      const operation = runViewSync(() =>
        this.handlers.invoke(handler, args, { ...this.runtime, signal })
      );
      await operation;
      flushViewWork();
      if (signal.aborted || this.closed) return undefined;
      this.generation = eventGeneration;
      return this.flushFrame(inReplyTo, eventGeneration, false);
    } catch (error) {
      if (signal.aborted || this.closed) return undefined;
      throw error;
    } finally {
      this.invokeSignal = undefined;
      if (!signal.aborted && !this.closed) {
        this.allowUnsolicitedPublish = true;
      }
    }
  }

  public close(): void {
    if (this.closed) {
      return;
    }
    this.closed = true;
    this.options.signal.removeEventListener("abort", this.abortFromOwner);
    this.sessionController.abort(new DOMException("View session closed", "AbortError"));
    for (const entry of this.stack) updateViewContainer(null, entry.root);
    this.stack.length = 0;
    this.effects.length = 0;
  }

  private push(target: ReactElement): void {
    this.assertOpen();
    this.stack.push(this.createStackEntry(target));
  }

  private pop(): void {
    this.assertOpen();
    if (this.stack.length > 1) {
      const popped = this.stack.pop();
      if (popped) updateViewContainer(null, popped.root);
      this.onCommit();
    }
  }

  private popToRoot(): void {
    this.assertOpen();
    if (this.stack.length > 1) {
      const popped = this.stack.splice(1);
      for (const entry of popped) updateViewContainer(null, entry.root);
      this.onCommit();
    }
  }

  private createStackEntry(element: ReactElement): ViewStackEntry {
    let entry: ViewStackEntry;
    const container: HostContainer = {
      children: [],
      onCommit: () => {
        if (this.stack.at(-1) === entry) this.onCommit();
      },
    };
    const reportError = (error: Error): void => {
      throw error;
    };
    const root = viewReconciler.createContainer(
      container,
      LegacyRoot,
      null,
      false,
      null,
      "compozy-view-",
      reportError,
      reportError,
      reportError,
      () => undefined,
      null
    );
    entry = { container, root };
    updateViewContainer(createElement(ViewRuntimeContext, { value: this.runtime }, element), root);
    return entry;
  }

  private onCommit(): void {
    if (!this.opened || this.closed || this.scheduled) {
      return;
    }
    if (!this.allowUnsolicitedPublish) {
      return;
    }
    if (this.invokeSignal?.aborted) {
      return;
    }
    this.scheduled = true;
    const generation = this.generation;
    const signal = this.invokeSignal;
    this.scheduleFrame(() => {
      this.scheduled = false;
      if (this.closed) {
        return;
      }
      if (signal?.aborted || this.invokeSignal?.aborted) {
        return;
      }
      if (generation !== this.generation) {
        return;
      }
      const frame = this.flushFrame(0, generation, false);
      if (frame) {
        void Promise.resolve(this.options.publish(frame)).catch(error => {
          this.options.diagnostics?.warn(`view frame publish failed: ${String(error)}`);
        });
      }
    });
  }

  private flushFrame(
    inReplyTo: number,
    generation: number,
    forcePayload: boolean
  ): ViewFrame | undefined {
    const active = this.stack.at(-1);
    if (!active) throw new Error("view session is closed");
    const serialized = serializeView(active.container.children, this.handlers);
    const payloadJSON = JSON.stringify(serialized.payload);
    const effects = this.effects.splice(0);
    if (!forcePayload && payloadJSON === this.previousPayloadJSON && effects.length === 0) {
      return undefined;
    }
    const previousRevision = this.revision === 0 ? "" : `vr_${this.revision}`;
    this.revision++;
    const revision = `vr_${this.revision}`;
    const patch: ViewPatch | undefined = forcePayload
      ? undefined
      : {
          view_id: this.options.viewID,
          from: previousRevision,
          to: revision,
          ops:
            payloadJSON === this.previousPayloadJSON
              ? []
              : [{ op: "replace", path: "", value: serialized.payload as never }],
        };
    this.previousPayloadJSON = payloadJSON;
    return {
      view_session: this.options.viewSession,
      revision,
      ...(inReplyTo > 0 ? { in_reply_to: inReplyTo } : {}),
      generation,
      ...(forcePayload ? { payload: serialized.payload } : { patch: patch! }),
      ...(effects.length > 0 ? { effects } : {}),
      handlers: serialized.handlers,
    };
  }

  private assertOpen(): void {
    if (this.closed || this.stack.length === 0) {
      throw new Error("view session is closed");
    }
  }
}
